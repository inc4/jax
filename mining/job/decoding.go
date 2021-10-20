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
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types"
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

var (
	burnScript, _ = txscript.NullDataScript([]byte(types.JaxBurnAddr))
)

func (h *Job) decodeBeaconResponse(c *jaxjson.GetBeaconBlockTemplateResult) (task *Task, err error) {
	// burn beacon only if burnBtc is true
	transactions, err := h.unmarshalTransactions(c.CoinbaseTxn, c.Transactions, h.Config.BurnBtc)
	if err != nil {
		return nil, err
	}

	actualMMRRoot, merkleHash, bits, target, err := h.decodeTemplateValues(c.BlocksMMRRoot, c.Bits, c.Target, transactions)
	if err != nil {
		return nil, err
	}

	aux, err := parseBtcAux(c.BTCAux)
	if err != nil {
		return
	}

	header := wire.NewBeaconBlockHeader(wire.BVersion(c.Version), *actualMMRRoot, *merkleHash,
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

	// burn shard only if burnBtc is false
	transactions, err := h.unmarshalTransactions(c.CoinbaseTxn, c.Transactions, !h.Config.BurnBtc)
	if err != nil {
		return nil, err
	}

	actualMMRRoot, merkleHash, bits, target, err := h.decodeTemplateValues(c.BlocksMMRRoot, c.Bits, c.Target, transactions)
	if err != nil {
		return nil, err
	}

	header := wire.NewShardBlockHeader(*actualMMRRoot, *merkleHash, bits, *h.Beacon.Block.Header.BeaconHeader(), *h.lastBCCoinbaseAux)

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
	BlocksMMRRootS, bitsS, targetS string, transactions []*wire.MsgTx) (actualMMRRoot, merkleHash *chainhash.Hash, bits uint32, target *big.Int, err error) {

	actualMMRRoot, err = chainhash.NewHashFromStr(BlocksMMRRootS)
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

func (h *Job) unmarshalTransactions(coinbaseTx *jaxjson.GetBlockTemplateResultTx, txs []jaxjson.GetBlockTemplateResultTx, burn bool) (transactions []*wire.MsgTx, err error) {
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
	cTX.TxOut[1].PkScript = h.script(burn)
	cTX.TxOut[2].PkScript = h.Config.feeScript

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

func (h *Job) script(burn bool) []byte {
	if burn {
		return burnScript
	}
	return h.Config.feeScript
}
