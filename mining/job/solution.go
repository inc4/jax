package job

import (
	"encoding/hex"
	btcwire "github.com/btcsuite/btcd/wire"
	"gitlab.com/jaxnet/core/miner/core/common"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
	"log"

	"bytes"
)

func (h *Job) Solution(btcHeader, coinbaseTx []byte) {
	header := &btcwire.BlockHeader{}
	header.Deserialize(bytes.NewReader(btcHeader))

	tx := &wire.MsgTx{}
	tx.Deserialize(bytes.NewReader(coinbaseTx))

	h.CheckSolution(header, tx)
}

func (h *Job) CheckSolution(btcHeader *btcwire.BlockHeader, coinbaseTx *wire.MsgTx) {
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

	beaconBlock := h.BeaconBlock.Copy()
	beaconBlock.Header.BeaconHeader().SetBTCAux(btcAux)

	if bitHashRepresentation.Cmp(h.BeaconTarget) <= 0 {
		h.submitBeacon(beaconBlock)
	}

	for _, t := range h.ShardsTargets {
		if bitHashRepresentation.Cmp(t.Target) <= 0 {
			shardBlock := t.BlockCandidate.Copy()
			shardBlock.Header.SetBeaconHeader(beaconBlock.Header.BeaconHeader())
			h.submitShard(shardBlock, t.ID)

		} else {
			break // Other targets are higher than current one.
		}
	}
}

func (h *Job) submitBeacon(block *wire.MsgBlock) {
	he, err := blockToHex(block)
	if err != nil {
		log.Println(err) // todo ?
	}
	h.rpcClient.SubmitBeacon(he)
}

func (h *Job) submitShard(block *wire.MsgBlock, shardID common.ShardID) {
	he, err := blockToHex(block)
	if err != nil {
		log.Println(err) // todo ?
	}
	h.rpcClient.SubmitShard(he, shardID)

}

func blockToHex(block *wire.MsgBlock) (string, error) {
	buf := new(bytes.Buffer)
	err := block.Serialize(buf)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(buf.Bytes()), nil
}
