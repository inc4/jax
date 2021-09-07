package mining

import (
	"bytes"
	"fmt"
	btcwire "github.com/btcsuite/btcd/wire"
	"gitlab.com/jaxnet/core/miner/core/common"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type MinerResult struct {
	ShardId     common.ShardID
	Amount      int64
	BlockHeight int64
	BlockHash   chainhash.Hash
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
	btcAux := wire.BTCBlockAux{
		Version:     btcHeader.Version,
		PrevBlock:   chainhash.Hash(btcHeader.PrevBlock),
		MerkleRoot:  chainhash.Hash(btcHeader.MerkleRoot),
		Timestamp:   btcHeader.Timestamp,
		Bits:        btcHeader.Bits,
		Nonce:       btcHeader.Nonce,
		CoinbaseAux: wire.CoinbaseAux{Tx: *coinbaseTx},
	}

	hash := btcHeader.BlockHash()
	bitHashRepresentation := pow.HashToBig((*chainhash.Hash)(&hash))

	beaconBlock := m.Job.BeaconBlock.Copy()
	beaconBlock.Header.BeaconHeader().SetBTCAux(btcAux)

	if bitHashRepresentation.Cmp(m.Job.BeaconTarget) <= 0 {
		result := m.newMinerResult(beaconBlock, 0, m.Job.BeaconBlockHeight)
		results = append(results, result)
	}

	for _, t := range m.Job.ShardsTargets {
		if bitHashRepresentation.Cmp(t.Target) <= 0 {
			shardBlock := t.BlockCandidate.Copy()
			shardBlock.Header.SetBeaconHeader(beaconBlock.Header.BeaconHeader())

			result := m.newMinerResult(shardBlock, t.ID, t.BlockHeight)
			results = append(results, result)

		} else {
			break // Other targets are higher than current one.
		}
	}

	return
}

func (m *Miner) submitBlock(block *wire.MsgBlock, shardID common.ShardID) error {
	wireBlock := jaxutil.NewBlock(block)
	return m.rpcClient.ForShard(uint32(shardID)).SubmitBlock(wireBlock, nil)
}

func (m *Miner) newMinerResult(block *wire.MsgBlock, shardID common.ShardID, height int64) *MinerResult {
	err := m.submitBlock(block, shardID)
	if err != nil {
		err = fmt.Errorf("can't submit block (shardId=%v): %w", shardID, err)
	}

	return &MinerResult{
		ShardId:     shardID,
		Amount:      block.Transactions[0].TxOut[1].Value + block.Transactions[0].TxOut[2].Value,
		BlockHeight: height,
		BlockHash:   block.BlockHash(),
		Err:         err,
	}
}
