/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package job

import (
	"fmt"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"math/big"
	"sort"
	"sync"

	btcdwire "github.com/btcsuite/btcd/wire"

	mm "gitlab.com/jaxnet/core/merged-mining-tree"
	"gitlab.com/jaxnet/core/miner/core/common"
	"gitlab.com/jaxnet/core/miner/core/utils"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

var (
	jaxNetParams = &chaincfg.TestNet3Params
)

type Configuration struct {
	BtcMiningAddress jaxutil.Address
	ShardsCount      uint32

	pkScript []byte
}

type Task struct {
	ShardID common.ShardID
	Block   *wire.MsgBlock
	Height  int64
	Target  *big.Int
}

type CoinBaseTx struct {
	Part1, Part2 []byte
}
type CoinBaseData struct {
	Reward, Fee int64
	Height      uint32
}

type Job struct {
	sync.RWMutex

	config *Configuration

	Beacon *Task

	shards        map[common.ShardID]*Task
	ShardsTargets []*Task // it's `shards` sorted by Target. sort on update

	CoinBaseCh       chan *CoinBaseTx
	lastCoinbaseData *CoinBaseData
}

func NewJob(BtcAddress, JaxAddress string) (job *Job, err error) {
	job = &Job{
		config: &Configuration{
			ShardsCount: 3,
		},
		shards:     make(map[common.ShardID]*Task),
		CoinBaseCh: make(chan *CoinBaseTx),
	}

	job.config.BtcMiningAddress, err = jaxutil.DecodeAddress(BtcAddress, jaxNetParams)
	if err != nil {
		return
	}

	jaxMiningAddress, err := jaxutil.DecodeAddress(JaxAddress, jaxNetParams)
	if err != nil {
		return
	}
	job.config.pkScript, err = txscript.PayToAddrScript(jaxMiningAddress)
	if err != nil {
		return
	}

	return
}

func (h *Job) ProcessShardTemplate(template *jaxjson.GetShardBlockTemplateResult, shardID common.ShardID) error {
	h.Lock()
	defer h.Unlock()

	block, target, height, err := h.decodeShardBlockTemplateResponse(template)
	if err != nil {
		return fmt.Errorf("can't decode shard block template response: %w", err)
	}

	// todo: add the sme deduplication mechanics as was added for beacon block.
	//		 (see processBeaconTemplate() method for the details)

	shardRecord, isPresent := h.shards[shardID]
	if !isPresent {
		shardRecord = &Task{}
		h.shards[shardID] = shardRecord
	}

	shardRecord.ShardID = shardID
	shardRecord.Block = block
	shardRecord.Block.Header.(*wire.ShardHeader).SetMergeMiningNumber(uint32(len(h.shards)))
	shardRecord.Target = target
	shardRecord.Height = height

	// clear, populate and sort array by Target
	h.ShardsTargets = []*Task{}
	for _, shardTask := range h.shards {
		h.ShardsTargets = append(h.ShardsTargets, shardTask)
	}
	sort.Slice(h.ShardsTargets, func(i, j int) bool { return h.ShardsTargets[i].Target.Cmp(h.ShardsTargets[j].Target) == -1 })

	return h.update()
}

func (h *Job) ProcessBeaconTemplate(template *jaxjson.GetBeaconBlockTemplateResult) error {
	h.Lock()
	defer h.Unlock()

	block, target, height, err := h.decodeBeaconResponse(template)
	if err != nil {
		return fmt.Errorf("can't decode beacon block template response: %w", err)
	}

	h.Beacon = &Task{
		Block:  block,
		Height: height,
		Target: target,
	}
	lastBCHeader = block.Header.BeaconHeader().Copy().BeaconHeader()
	lastBCCoinbaseAux = wire.CoinbaseAux{
		Tx:       *block.Transactions[0].Copy(),
		TxMerkle: make([]chainhash.Hash, len(block.Transactions)),
	}
	for i, tx := range block.Transactions {
		lastBCCoinbaseAux.TxMerkle[i] = tx.TxHash()
	}

	return h.update()
}

func (h *Job) GetMinTarget() *big.Int {
	h.RLock()
	defer h.RUnlock()

	if len(h.ShardsTargets) > 0 {
		if shard := h.ShardsTargets[0].Target; shard.Cmp(h.Beacon.Target) == -1 {
			return shard
		}
	}
	return h.Beacon.Target
}

func (h *Job) GetBitcoinCoinbase(d *CoinBaseData) (*CoinBaseTx, error) {
	h.Lock()
	defer h.Unlock()

	if h.Beacon == nil {
		return nil, fmt.Errorf("job.Beacon is nil")
	}

	jaxCoinbaseTx, err := mining.CreateJaxCoinbaseTx(d.Reward, d.Fee, int32(d.Height), 0, h.config.BtcMiningAddress, false)
	if err != nil {
		return nil, err
	}
	coinbaseTx := utils.JaxTxToBtcTx(jaxCoinbaseTx.MsgTx())
	h.lastCoinbaseData = d

	beaconHash := h.Beacon.Block.Header.BeaconHeader().BeaconExclusiveHash()
	coinbaseTx.TxIn[0].SignatureScript, err = utils.BTCCoinbaseScript(int64(d.Height), utils.PackUint64LE(0x00), beaconHash[:])
	if err != nil {
		return nil, err
	}

	fakeBlock := btcdwire.MsgBlock{Transactions: []*btcdwire.MsgTx{&coinbaseTx}}
	part1, part2 := utils.SplitCoinbase(&fakeBlock)
	return &CoinBaseTx{part1, part2}, nil
}

func (h *Job) update() error {
	if err := h.updateMergedMiningProof(); err != nil {
		return fmt.Errorf("can't update merged mining proof: %w", err)
	}

	if err := h.updateCoinbase(); err != nil {
		return fmt.Errorf("can't update coinbase: %w", err)
	}
	return nil
}

func (h *Job) updateMergedMiningProof() (err error) {
	tree := mm.NewSparseMerkleTree(h.config.ShardsCount)
	for id, shard := range h.shards {
		// Shard IDs are going to be indexed from 1,
		// but the tree expects slots to be indexed from 0.
		slotIndex := uint32(id - 1)

		shardBlockHash := shard.Block.Header.(*wire.ShardHeader).ShardBlockHash()
		err = tree.SetShardHash(slotIndex, shardBlockHash)
		if err != nil {
			return
		}
	}

	root, err := tree.Root()
	if err != nil {
		return
	}

	rootHash, err := chainhash.NewHash(root[:])
	if err != nil {
		return
	}

	coding, codingBitLength, err := tree.CatalanNumbersCoding()
	if err != nil {
		return
	}

	hashes := tree.MarshalOrangeTreeLeafs()

	h.Beacon.Block.Header.BeaconHeader().SetMergeMiningRoot(*rootHash)
	h.Beacon.Block.Header.BeaconHeader().SetMergedMiningTreeCodingProof(hashes, coding, codingBitLength)

	return
}

func (h *Job) updateCoinbase() error {
	if h.lastCoinbaseData == nil {
		// todo do smth?
		return nil
	}

	coinbase, err := h.GetBitcoinCoinbase(h.lastCoinbaseData)
	if err != nil {
		return err
	}
	go func() { // avoid deadlocks
		h.CoinBaseCh <- coinbase
	}()
	return nil
}
