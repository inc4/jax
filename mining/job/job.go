/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package job

import (
	"fmt"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"math/big"
	"sort"
	"sync"

	mm "gitlab.com/jaxnet/jaxnetd/types/merge_mining_tree"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type Configuration struct {
	ShardsCount  uint32
	BurnBtc      bool
	JaxNetParams *chaincfg.Params

	btcMiningAddress jaxutil.Address
	jaxMiningAddress jaxutil.Address
}

type Task struct {
	ShardID uint32
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

	Config *Configuration

	Beacon *Task

	shards        map[uint32]*Task
	ShardsTargets []*Task // it's `shards` sorted by Target. sort on update

	UpdateCh chan bool

	lastBCCoinbaseAux *wire.CoinbaseAux
}

func NewJob(BtcAddress, JaxAddress string, jaxNetParams *chaincfg.Params, burnBtc bool) (job *Job, err error) {
	job = &Job{
		Config: &Configuration{
			ShardsCount:  3,
			BurnBtc:      burnBtc,
			JaxNetParams: jaxNetParams,
		},
		shards:   make(map[uint32]*Task),
		UpdateCh: make(chan bool),
	}

	job.Config.btcMiningAddress, err = jaxutil.DecodeAddress(BtcAddress, jaxNetParams)
	if err != nil {
		return
	}

	job.Config.jaxMiningAddress, err = jaxutil.DecodeAddress(JaxAddress, jaxNetParams)
	if err != nil {
		return
	}

	return
}

func (h *Job) ProcessShardTemplate(template *jaxjson.GetShardBlockTemplateResult, shardID uint32) (err error) {
	h.Lock()
	defer h.Unlock()

	h.shards[shardID], err = h.decodeShardBlockTemplateResponse(template, shardID)
	if err != nil {
		return fmt.Errorf("can't decode shard block template response: %w", err)
	}

	// clear, populate and sort array by Target
	h.ShardsTargets = h.ShardsTargets[:0]
	for _, shardTask := range h.shards {
		h.ShardsTargets = append(h.ShardsTargets, shardTask)
	}
	sort.Slice(h.ShardsTargets, func(i, j int) bool { return h.ShardsTargets[i].Target.Cmp(h.ShardsTargets[j].Target) == -1 })

	if err := h.updateMergedMiningProof(); err != nil {
		return fmt.Errorf("can't update merged mining proof: %w", err)
	}

	h.updateBitcoinCoinbase()
	return nil
}

func (h *Job) ProcessBeaconTemplate(template *jaxjson.GetBeaconBlockTemplateResult) (err error) {
	h.Lock()

	h.Beacon, err = h.decodeBeaconResponse(template)
	if err != nil {
		return fmt.Errorf("can't decode beacon block template response: %w", err)
	}

	h.updateBeaconCoinbaseAux()
	if err := h.updateMergedMiningProof(); err != nil {
		return fmt.Errorf("can't update merged mining proof: %w", err)
	}

	h.Unlock()

	h.updateBitcoinCoinbase()
	return nil
}

func (h *Job) GetMinTarget() *big.Int {
	h.RLock()
	defer h.RUnlock()

	if len(h.ShardsTargets) > 0 {
		if shard := h.ShardsTargets[0].Target; shard.Cmp(h.Beacon.Target) == -1 {
			return shard
		}
	}
	if h.Beacon == nil {
		return nil
	}
	return h.Beacon.Target
}

func (h *Job) GetBitcoinCoinbase(reward, fee int64, height uint32) (*CoinBaseTx, error) {
	h.Lock()
	defer h.Unlock()

	if h.Beacon == nil {
		return nil, fmt.Errorf("job.Beacon is nil")
	}

	beaconHash := h.Beacon.Block.Header.BeaconHeader().BeaconExclusiveHash()

	coinbaseTx, err := chaindata.CreateBitcoinCoinbaseTx(reward, fee, int32(height), h.Config.btcMiningAddress, beaconHash[:], h.Config.BurnBtc)
	if err != nil {
		return nil, err
	}

	fakeBlock := wire.MsgBlock{Transactions: []*wire.MsgTx{coinbaseTx.MsgTx()}}
	part1, part2 := SplitCoinbase(&fakeBlock)
	return &CoinBaseTx{part1, part2}, nil
}

func (h *Job) updateMergedMiningProof() (err error) {
	tree := mm.NewSparseMerkleTree(h.Config.ShardsCount)

	for id, shard := range h.shards {
		shardBlockHash := shard.Block.Header.ExclusiveHash()
		err = tree.SetShardHash(id-1, shardBlockHash) // tree expects slots to be indexed from 0
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

	for id, shard := range h.shards {
		path, err := tree.MerkleProofPath(id - 1) // tree expects slots to be indexed from 0
		if err != nil {
			return err
		}

		shard.Block.Header.SetShardMerkleProof(path)
		shard.Block.Header.BeaconHeader().SetMergeMiningRoot(*rootHash)
		shard.Block.Header.BeaconHeader().SetMergedMiningTreeCodingProof(hashes, coding, codingBitLength)
	}

	return
}

func (h *Job) updateBeaconCoinbaseAux() {
	txs := h.Beacon.Block.Transactions
	h.lastBCCoinbaseAux = &wire.CoinbaseAux{
		Tx:            *txs[0].Copy(),
		TxMerkleProof: make([]chainhash.Hash, len(txs)),
	}
	for i, tx := range txs {
		h.lastBCCoinbaseAux.TxMerkleProof[i] = tx.TxHash()
	}
}

func (h *Job) updateBitcoinCoinbase() {
	go func() { // avoid deadlocks
		h.UpdateCh <- true
	}()
}
