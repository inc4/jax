/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package job

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	"gitlab.com/jaxnet/core/miner/core/e"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

var (
	// Fixture date is used as a placeholder for block generation timestamp set in block's header.
	// Because of the deduplication mechanics that is applied to the blocks,
	// it is important to keep deserialized blocks free from volatile data
	// (like constantly changing timestamp).
	// Otherwise, deduplication fails even if original blocks are totally the same.
	fixtureMMProofHash = chainhash.Hash{} // todo ?
	fixtureNonce       = uint32(0)

	lastBCHeader      *wire.BeaconHeader
	lastBCCoinbaseAux wire.CoinbaseAux
	lastBCHeaderMutex sync.Mutex
)

func (h *Job) decodeBeaconResponse(c *jaxjson.GetBeaconBlockTemplateResult) (block *wire.MsgBlock, target *big.Int, height int64, err error) {

	// Block initialisation.
	height = c.Height

	beaconBlock := wire.EmptyBeaconBlock()
	block = &beaconBlock

	// Transactions processing.
	block.Transactions, err = h.unmarshalTransactions(c.CoinbaseTxn, c.Transactions)
	if err != nil {
		return
	}

	// Block header processing.
	previousBlockHash, err := chainhash.NewHashFromStr(c.PreviousHash)
	if err != nil {
		return
	}

	bits, err := unmarshalBits(c.Bits)
	if err != nil {
		return
	}

	targetBinary, err := hex.DecodeString(c.Target)
	target = (&big.Int{}).SetBytes(targetBinary)
	if err != nil {
		return
	}

	// Recalculate the merkle root with the updated extra nonce.
	uBlock := jaxutil.NewBlock(block)
	merkles := chaindata.BuildMerkleTreeStore(uBlock.Transactions(), false)

	block.Header = wire.NewBeaconBlockHeader(
		wire.BVersion(c.Version), *previousBlockHash,
		*merkles[len(merkles)-1], fixtureMMProofHash, time.Now(), bits, fixtureNonce)

	block.Header.BeaconHeader().SetShards(c.Shards)
	block.Header.BeaconHeader().SetK(c.K)
	block.Header.BeaconHeader().SetVoteK(c.VoteK)

	var rawAux []byte
	rawAux, err = hex.DecodeString(c.BTCAux)
	if err != nil {
		return
	}

	aux := wire.BTCBlockAux{}
	err = aux.Deserialize(bytes.NewBuffer(rawAux))
	if err != nil {
		return
	}

	block.Header.BeaconHeader().SetBTCAux(aux)
	return
}

func (h *Job) decodeShardBlockTemplateResponse(c *jaxjson.GetShardBlockTemplateResult) (block *wire.MsgBlock, target *big.Int, height int64, err error) {

	if lastBCHeader == nil {
		// No beacon block candidate has been fetched yet -> no beacon header is available.
		// No way to generate SC block header, cause there is a dependency on a BC header.
		err = fmt.Errorf("can't initialise SC header: %w", e.ErrNoBCHeader)
		return
	}

	// Block initialisation.
	height = c.Height
	shardBlock := wire.EmptyShardBlock()
	block = &shardBlock

	// Transactions processing.
	block.Transactions, err = h.unmarshalTransactions(c.CoinbaseTxn, c.Transactions)
	if err != nil {
		return
	}

	// Block header processing.
	previousBlockHash, err := chainhash.NewHashFromStr(c.PreviousHash)
	if err != nil {
		return
	}

	bits, err := unmarshalBits(c.Bits)
	if err != nil {
		return
	}

	targetBinary, err := hex.DecodeString(c.Target)
	target = (&big.Int{}).SetBytes(targetBinary)
	if err != nil {
		return
	}

	lastBCHeaderMutex.Lock()
	defer lastBCHeaderMutex.Unlock()

	// Recalculate the merkle root with the updated extra nonce.
	uBlock := jaxutil.NewBlock(block)
	merkles := chaindata.BuildMerkleTreeStore(uBlock.Transactions(), false)

	block.Header = wire.NewShardBlockHeader(
		*previousBlockHash, *merkles[len(merkles)-1], time.Now(), bits,
		*lastBCHeader, *lastBCCoinbaseAux.Copy())

	return
}

func (h *Job) unmarshalTransactions(coinbaseTx *jaxjson.GetBlockTemplateResultTx, txs []jaxjson.GetBlockTemplateResultTx) (transactions []*wire.MsgTx, err error) {

	unmarshalTx := func(txHash string) (tx *wire.MsgTx, err error) {
		txBinary, err := hex.DecodeString(txHash)
		if err != nil {
			return
		}

		tx = &wire.MsgTx{}
		txReader := bytes.NewReader(txBinary)
		err = tx.Deserialize(txReader)
		return
	}

	// Coinbase transaction must be processed first.
	// (transactions order in transactions slice is significant)
	cTX, err := unmarshalTx(coinbaseTx.Data)
	if err != nil {
		return
	}

	// set miningAddress into coinbase tx
	cTX.TxOut[1].PkScript = h.config.pkScript
	cTX.TxOut[2].PkScript = h.config.pkScript

	transactions = make([]*wire.MsgTx, 0)
	transactions = append(transactions, cTX)

	// Regular transactions processing.
	for _, marshalledTx := range txs {
		tx, err := unmarshalTx(marshalledTx.Data)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, tx)
	}

	return
}

func unmarshalBits(hexBits string) (uint32, error) {
	val, err := strconv.ParseUint(hexBits, 16, 64)
	return uint32(val), err
}
