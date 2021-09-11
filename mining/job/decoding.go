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
	"gitlab.com/jaxnet/core/miner/core/common"
	"math/big"
	"strconv"
	"time"

	"gitlab.com/jaxnet/core/miner/core/e"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

func (h *Job) decodeBeaconResponse(c *jaxjson.GetBeaconBlockTemplateResult) (task *Task, err error) {
	transactions, prevHash, merkleHash, bits, target, err := h.decodeTemplateValues(c.PreviousHash, c.Bits, c.Target, c.CoinbaseTxn, c.Transactions)
	if err != nil {
		return
	}
	aux, err := parseBtcAux(c.BTCAux)
	if err != nil {
		return
	}

	header := wire.NewBeaconBlockHeader(wire.BVersion(c.Version), *prevHash, *merkleHash,
		chainhash.Hash{}, time.Unix(c.CurTime, 0), bits, 0)

	header.SetShards(c.Shards)
	header.SetK(c.K)
	header.SetVoteK(c.VoteK)
	header.SetBTCAux(aux)

	return &Task{
		ShardID: 0,
		Block: &wire.MsgBlock{
			Header:       header,
			Transactions: transactions,
		},
		Height: c.Height,
		Target: target,
	}, nil

}

func (h *Job) decodeShardBlockTemplateResponse(c *jaxjson.GetShardBlockTemplateResult, shardID common.ShardID) (task *Task, err error) {
	if h.Beacon == nil {
		return nil, fmt.Errorf("can't initialise SC header: %w", e.ErrNoBCHeader)
	}

	transactions, prevHash, merkleHash, bits, target, err := h.decodeTemplateValues(c.PreviousHash, c.Bits, c.Target, c.CoinbaseTxn, c.Transactions)
	if err != nil {
		return nil, err
	}

	header := wire.NewShardBlockHeader(*prevHash, *merkleHash, time.Unix(c.CurTime, 0), bits,
		*h.Beacon.Block.Header.BeaconHeader(), *h.lastBCCoinbaseAux)

	return &Task{
		ShardID: shardID,
		Block: &wire.MsgBlock{
			ShardBlock:   true,
			Header:       header,
			Transactions: transactions,
		},
		Height: c.Height,
		Target: target,
	}, nil
}

func (h *Job) decodeTemplateValues(
	prevHashS, bitsS, targetS string, coinbaseTx *jaxjson.GetBlockTemplateResultTx, txs []jaxjson.GetBlockTemplateResultTx) (
	transactions []*wire.MsgTx, prevHash, merkleHash *chainhash.Hash, bits uint32, target *big.Int, err error) {

	transactions, err = h.unmarshalTransactions(coinbaseTx, txs)
	if err != nil {
		return
	}

	prevHash, err = chainhash.NewHashFromStr(prevHashS)
	if err != nil {
		return
	}

	fakeBlock := jaxutil.NewBlock(&wire.MsgBlock{Transactions: transactions})
	merkles := chaindata.BuildMerkleTreeStore(fakeBlock.Transactions(), false)
	merkleHash = merkles[len(merkles)-1]

	bits64, err := strconv.ParseUint(bitsS, 16, 64)
	if err != nil {
		return
	}
	bits = uint32(bits64)

	targetBytes, err := hex.DecodeString(targetS)
	if err != nil {
		return
	}
	target = (&big.Int{}).SetBytes(targetBytes)

	return
}

func (h *Job) unmarshalTransactions(coinbaseTx *jaxjson.GetBlockTemplateResultTx, txs []jaxjson.GetBlockTemplateResultTx) (transactions []*wire.MsgTx, err error) {
	unmarshalTx := func(txHash string) (tx *wire.MsgTx, err error) {
		txBinary, err := hex.DecodeString(txHash)
		if err != nil {
			return
		}

		tx = &wire.MsgTx{}
		err = tx.Deserialize(bytes.NewReader(txBinary))
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

func parseBtcAux(auxS string) (aux wire.BTCBlockAux, err error) {
	rawAux, err := hex.DecodeString(auxS)
	if err != nil {
		return
	}
	aux = wire.BTCBlockAux{}
	err = aux.Deserialize(bytes.NewReader(rawAux))
	return
}
