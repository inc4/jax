package mining

import (
	"bytes"
	"fmt"
	btcwire "github.com/btcsuite/btcd/wire"
	"github.com/inc4/jax/mining/job"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
	"time"
)

var (
	hashSortingSlotNumber = chaincfg.MainNetParams.PowParams.HashSortingSlotNumber
)

type MinerResult struct {
	ShardId     uint32
	Amount      int64
	BlockHeight int64
	BlockHash   chainhash.Hash
	BlockTime   time.Time
	Err         error
}

func (m *Miner) Solution(btcHeader, coinbaseTx []byte) (results []*MinerResult, err error) {
	header := &btcwire.BlockHeader{}
	if err = header.Deserialize(bytes.NewReader(btcHeader)); err != nil {
		return
	}

	tx := &wire.MsgTx{}
	if err = tx.Deserialize(bytes.NewReader(coinbaseTx)); err != nil {
		return
	}

	results = m.CheckSolution(header, tx)
	for _, r := range results {
		if r.Err == nil {
			continue
		}
		if err == nil {
			err = fmt.Errorf("failed to submit blocks: ")
		}
		err = fmt.Errorf(" %w \n %v", err, r.Err)
	}

	return
}

func (m *Miner) CheckSolution(btcHeader *btcwire.BlockHeader, coinbaseTx *wire.MsgTx) (results []*MinerResult) {
	m.Job.RLock()
	defer m.Job.RUnlock()

	btcAux := wire.BTCBlockAux{
		Version:     btcHeader.Version,
		PrevBlock:   chainhash.Hash(btcHeader.PrevBlock),
		MerkleRoot:  chainhash.Hash(btcHeader.MerkleRoot),
		Timestamp:   btcHeader.Timestamp,
		Bits:        btcHeader.Bits,
		Nonce:       btcHeader.Nonce,
		CoinbaseAux: wire.CoinbaseAux{Tx: *coinbaseTx},
	}

	beaconBlock := m.Job.Beacon.Block.Copy()
	hash := beaconBlock.Header.BeaconHeader().PoWHash()

	beaconBlock.Header.BeaconHeader().SetBTCAux(btcAux)

	if m.checkHash(&hash, m.Job.Beacon) {
		result := m.newMinerResult(beaconBlock, 0, m.Job.Beacon.Height)
		results = append(results, result)
	}

	for _, t := range m.Job.ShardsTargets {
		if m.checkHash(&hash, t) {
			shardBlock := t.Block.Copy()
			coinbaseAux := wire.CoinbaseAux{}.FromBlock(beaconBlock, true)

			shardBlock.Header.SetBeaconHeader(beaconBlock.Header.BeaconHeader(), coinbaseAux)

			result := m.newMinerResult(shardBlock, t.ShardID, t.Height)
			results = append(results, result)
		} else {
			break // Other targets are higher than current one.
		}
	}

	return
}

func (m *Miner) submitBlock(block *wire.MsgBlock, shardID uint32) error {
	wireBlock := jaxutil.NewBlock(block)
	// TODO we need new client due to bug in jaxnetd/network/rpcclient
	rpcClient, err := rpcclient.New(m.rpcConf, nil)
	if err != nil {
		return err
	}
	return rpcClient.ForShard(shardID).SubmitBlock(wireBlock, nil)
}

func (m *Miner) newMinerResult(block *wire.MsgBlock, shardID uint32, height int64) *MinerResult {
	err := m.submitBlock(block, shardID)
	if err != nil {
		err = fmt.Errorf("can't submit block (shardId=%v): %w", shardID, err)
	}

	return &MinerResult{
		ShardId:     shardID,
		Amount:      block.Transactions[0].TxOut[1].Value + block.Transactions[0].TxOut[2].Value,
		BlockHeight: height,
		BlockHash:   block.BlockHash(),
		BlockTime:   block.Header.Timestamp(),
		Err:         err,
	}
}

func (m *Miner) checkHash(hash *chainhash.Hash, t *job.Task) bool {
	if pow.HashToBig(hash).Cmp(t.Target) > 0 {
		fmt.Println("hash < target for shardId", t.ShardID)
		return false
	}
	if m.Job.Config.HashSorting && !pow.ValidateHashSortingRule(pow.HashToBig(hash), hashSortingSlotNumber, 0) {
		fmt.Println("ValidateHashSortingRule failed for shardId", t.ShardID)
		return false
	}
	return true
}
