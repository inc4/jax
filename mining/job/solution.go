package job

import (
	btcwire "github.com/btcsuite/btcd/wire"
	"gitlab.com/jaxnet/core/miner/core/common"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"

	"log"
)

func (h *Job) CheckSolution(btcHeader btcwire.BlockHeader, coinbaseTx *wire.MsgTx) {
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
	wireBlock := jaxutil.NewBlock(block)
	log.Println(wireBlock)
	// todo
	//h.client.ForBeacon().SubmitBlock(wireBlock, nil)
}

func (h *Job) submitShard(block *wire.MsgBlock, shardID common.ShardID) {
	wireBlock := jaxutil.NewBlock(block)
	log.Println(wireBlock)
	// todo
	//h.client.ForShard(shardID).SubmitBlock(wireBlock, nil)

}
