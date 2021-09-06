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

func (m *Miner) Solution(btcHeader, coinbaseTx []byte) error {
	header := &btcwire.BlockHeader{}
	if err := header.Deserialize(bytes.NewReader(btcHeader)); err != nil {
		return err
	}

	tx := &wire.MsgTx{}
	if err := tx.Deserialize(bytes.NewReader(coinbaseTx)); err != nil {
		return err
	}

	errs := m.CheckSolution(header, tx)
	if len(errs) != 0 {
		err := fmt.Errorf("failed to submit blocks: ")
		for _, e := range errs {
			err = fmt.Errorf(" %w \n %v", err, e)
		}
		return err
	}

	return nil
}

func (m *Miner) CheckSolution(btcHeader *btcwire.BlockHeader, coinbaseTx *wire.MsgTx) (submitErrors []error) {
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

	beaconBlock := m.job.BeaconBlock.Copy()
	beaconBlock.Header.BeaconHeader().SetBTCAux(btcAux)

	if bitHashRepresentation.Cmp(m.job.BeaconTarget) <= 0 {
		if err := m.submitBeacon(beaconBlock); err != nil {
			submitErrors = append(submitErrors, fmt.Errorf("can't submit beacon block: %w", err))
		}
	}

	for _, t := range m.job.ShardsTargets {
		if bitHashRepresentation.Cmp(t.Target) <= 0 {
			shardBlock := t.BlockCandidate.Copy()
			shardBlock.Header.SetBeaconHeader(beaconBlock.Header.BeaconHeader())

			if err := m.submitShard(shardBlock, t.ID); err != nil {
				submitErrors = append(submitErrors, fmt.Errorf("can't submit shard(%v) block: %w", t.ID, err))
			}

		} else {
			break // Other targets are higher than current one.
		}
	}

	return submitErrors
}

func (m *Miner) submitBeacon(block *wire.MsgBlock) error {
	wireBlock := jaxutil.NewBlock(block)
	return m.rpcClient.ForBeacon().SubmitBlock(wireBlock, nil)
}

func (m *Miner) submitShard(block *wire.MsgBlock, shardID common.ShardID) error {
	wireBlock := jaxutil.NewBlock(block)
	return m.rpcClient.ForShard(uint32(shardID)).SubmitBlock(wireBlock, nil)
}