/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package job

import (
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
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

type NetworkConfig struct {
	Host       string `yaml:"host"`
	Port       uint16 `yaml:"port"`
	DisableTLS bool   `yaml:"disableTLS"`
}
type RPCConfig struct {
	User         string            `yaml:"user"`
	Pass         string            `yaml:"pass"`
	Network      NetworkConfig     `yaml:"network"`
	ExtraHeaders map[string]string `yaml:"extra_headers"`
}
type ShardConfig struct {
	ID  common.ShardID `yaml:"id"`
	RPC RPCConfig      `yaml:"rpc"`
}

type Configuration struct {
	Shards           map[common.ShardID]ShardConfig
	EnableBTCMining  bool
	BurnBtcReward    bool
	BurnJaxReward    bool
	BurnJaxNetReward bool
	BtcMiningAddress jaxutil.Address
	JaxMiningAddress jaxutil.Address
}

type ShardTask struct {
	ID             common.ShardID
	BlockCandidate *wire.MsgBlock
	BlockHeight    int64
	Target         *big.Int
}

type Job struct {
	utils.StoppableMixin
	sync.Mutex

	config *Configuration

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

	lastExtraNonce *uint64
}

func NewJob(config *Configuration) *Job {
	return &Job{
		config: config,
		shards: make(map[common.ShardID]*ShardTask),
	}
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

	err = h.updateMergedMiningProof()
	if err != nil {
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
	h.BeaconHash = block.Header.BeaconHeader().BeaconExclusiveHash()

	lastBCHeader = block.Header.BeaconHeader().Copy().BeaconHeader()
	lastBCCoinbaseAux = wire.CoinbaseAux{
		Tx:       *block.Transactions[0].Copy(),
		TxMerkle: make([]chainhash.Hash, len(block.Transactions)),
	}
	for i, tx := range block.Transactions {
		lastBCCoinbaseAux.TxMerkle[i] = tx.TxHash()
	}

	err = h.updateMergedMiningProof()
	if err != nil {
		log.Println(err)
	}
}

func (h *Job) GetBitcoinCoinbase(reward, fee int64, height uint32) (part1, part2 []byte, err error) {
	jaxCoinbaseTx, err := mining.CreateJaxCoinbaseTx(reward, fee, int32(height), 0, h.config.BtcMiningAddress, h.config.BurnBtcReward)
	if err != nil {
		return
	}
	coinbaseTx := utils.JaxTxToBtcTx(jaxCoinbaseTx.MsgTx())

	coinbaseTx.TxIn[0].SignatureScript, err = utils.BTCCoinbaseScript(int64(height), utils.PackUint64LE(0x00), h.BeaconHash[:])
	if err != nil {
		return
	}

	fakeBlock := btcdwire.MsgBlock{Transactions: []*btcdwire.MsgTx{&coinbaseTx}}
	part1, part2 = utils.SplitCoinbase(&fakeBlock)
	return
}

func (h *Job) updateMergedMiningProof() (err error) {
	knownShardsCount := len(h.config.Shards)
	fetchedShardsCount := len(h.ShardsTargets)

	if knownShardsCount == 0 || fetchedShardsCount == 0 {
		return
	}

	tree := mm.NewSparseMerkleTree(uint32(knownShardsCount))
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
	return
}
