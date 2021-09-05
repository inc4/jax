/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package job

import (
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"log"
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
	JaxMiningAddress jaxutil.Address
	ShardsCount      uint32
}

type ShardTask struct {
	ID             common.ShardID
	BlockCandidate *wire.MsgBlock
	BlockHeight    int64
	Target         *big.Int
}

type CoinBaseTx struct {
	Part1, Part2 []byte
}
type CoinBaseData struct {
	Reward, Fee int64
	Height      uint32
}

type RpcClient interface {
	SubmitBeacon(block *jaxutil.Block)
	SubmitShard(block *jaxutil.Block, shardID common.ShardID)
}

type Job struct {
	utils.StoppableMixin
	sync.Mutex

	config    *Configuration
	rpcClient RpcClient

	BeaconBlock       *wire.MsgBlock
	BeaconBlockHeight int64

	// Represents target of the beacon chain.
	// (this is the main target of the mining process).
	BeaconTarget *big.Int

	// Represents targets of the shards.
	// The slice is supposed to be sorted in descendant order.
	// Sorting is needed for the mining process efficiency:
	// during mining, on each hash mined, it is applied to the shards targets,
	// from smallest one to the biggest one.
	// The generated hash would not be applied to the next shard target
	// in case if current one does not suite the generated hash.

	shards        map[common.ShardID]*ShardTask
	ShardsTargets []*ShardTask // it's `shards` sorted by Target. sort on update

	BeaconHash chainhash.Hash

	CoinBaseCh       chan *CoinBaseTx
	lastCoinbaseData *CoinBaseData
}

func NewJob(rpcClient RpcClient, BtcAddress, JaxAddress string) (job *Job, err error) {
	job = &Job{
		config:     &Configuration{ShardsCount: 3},
		rpcClient:  rpcClient,
		shards:     make(map[common.ShardID]*ShardTask),
		CoinBaseCh: make(chan *CoinBaseTx),
	}
	job.config.BtcMiningAddress, err = jaxutil.DecodeAddress(BtcAddress, jaxNetParams)
	if err != nil {
		return
	}
	job.config.JaxMiningAddress, err = jaxutil.DecodeAddress(JaxAddress, jaxNetParams)
	if err != nil {
		return
	}

	return
}

func (h *Job) ProcessShardTemplate(template *jaxjson.GetShardBlockTemplateResult, shardID common.ShardID) {
	h.Lock()
	defer h.Unlock()

	block, target, height, err := h.decodeShardBlockTemplateResponse(template)
	if err != nil {
		log.Println("Can't decode shard block template response", err)
		return
	}

	// todo: add the sme deduplication mechanics as was added for beacon block.
	//		 (see processBeaconTemplate() method for the details)

	shardRecord, isPresent := h.shards[shardID]
	if !isPresent {
		shardRecord = &ShardTask{}
		h.shards[shardID] = shardRecord
	}

	shardRecord.ID = shardID
	shardRecord.BlockCandidate = block
	shardRecord.BlockCandidate.Header.(*wire.ShardHeader).SetMergeMiningNumber(uint32(len(h.shards)))
	shardRecord.Target = target
	shardRecord.BlockHeight = height

	// clear, populate and sort array by Target
	h.ShardsTargets = []*ShardTask{}
	for _, shardTask := range h.shards {
		h.ShardsTargets = append(h.ShardsTargets, shardTask)
	}
	sort.Slice(h.ShardsTargets, func(i, j int) bool { return h.ShardsTargets[i].Target.Cmp(h.ShardsTargets[j].Target) == -1 })

	if err = h.updateMergedMiningProof(); err != nil {
		log.Println(err)
		return
	}

	if err = h.updateCoinbase(); err != nil {
		log.Println(err)
		return
	}
}

func (h *Job) ProcessBeaconTemplate(template *jaxjson.GetBeaconBlockTemplateResult) {
	h.Lock()
	defer h.Unlock()

	block, target, height, err := h.decodeBeaconResponse(template)
	if err != nil {
		log.Println("Can't decode beacon block template response")
		return
	}

	h.BeaconBlock = block
	h.BeaconTarget = target
	h.BeaconBlockHeight = height

	lastBCHeader = block.Header.BeaconHeader().Copy().BeaconHeader()
	lastBCCoinbaseAux = wire.CoinbaseAux{
		Tx:       *block.Transactions[0].Copy(),
		TxMerkle: make([]chainhash.Hash, len(block.Transactions)),
	}
	for i, tx := range block.Transactions {
		lastBCCoinbaseAux.TxMerkle[i] = tx.TxHash()
	}

	if err = h.updateMergedMiningProof(); err != nil {
		log.Println(err)
		return
	}

	if err = h.updateCoinbase(); err != nil {
		log.Println(err)
		return
	}
}

func (h *Job) GetBitcoinCoinbase(d *CoinBaseData) (*CoinBaseTx, error) {
	jaxCoinbaseTx, err := mining.CreateJaxCoinbaseTx(d.Reward, d.Fee, int32(d.Height), 0, h.config.BtcMiningAddress, false)
	if err != nil {
		return nil, err
	}
	coinbaseTx := utils.JaxTxToBtcTx(jaxCoinbaseTx.MsgTx())
	h.lastCoinbaseData = d

	coinbaseTx.TxIn[0].SignatureScript, err = utils.BTCCoinbaseScript(int64(d.Height), utils.PackUint64LE(0x00), h.BeaconHash[:])
	if err != nil {
		return nil, err
	}

	fakeBlock := btcdwire.MsgBlock{Transactions: []*btcdwire.MsgTx{&coinbaseTx}}
	part1, part2 := utils.SplitCoinbase(&fakeBlock)
	return &CoinBaseTx{part1, part2}, nil
}

func (h *Job) updateMergedMiningProof() (err error) {
	tree := mm.NewSparseMerkleTree(h.config.ShardsCount)
	for id, shard := range h.shards {
		// Shard IDs are going to be indexed from 1,
		// but the tree expects slots to be indexed from 0.
		slotIndex := uint32(id - 1)

		shardBlockHash := shard.BlockCandidate.Header.(*wire.ShardHeader).ShardBlockHash()
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

	h.BeaconBlock.Header.BeaconHeader().SetMergeMiningRoot(*rootHash)
	h.BeaconBlock.Header.BeaconHeader().SetMergedMiningTreeCodingProof(hashes, coding, codingBitLength)

	h.BeaconHash = h.BeaconBlock.Header.BeaconHeader().BeaconExclusiveHash()
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
