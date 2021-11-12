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
	"time"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

func (h *Job) decodeBeaconResponse(c *jaxjson.GetBeaconBlockTemplateResult) (task *Task, err error) {
	actualMMRRoot, prevBlockHash, bits, target, chainWeight, err := h.decodeTemplateValues(c.PrevBlocksMMRRoot, c.PreviousHash, c.Bits, c.Target, c.ChainWeight)
	if err != nil {
		return nil, err
	}

	coinbaseTx, err := h.getCoinbaseTx(0, int32(c.Height), 0, c.CoinbaseTxn)
	if err != nil {
		return nil, err
	}
	transactions, err := h.unmarshalTransactions(coinbaseTx, c.Transactions)
	if err != nil {
		return nil, err
	}

	btcAux, err := parseBtcAux(c.BTCAux)
	if err != nil {
		return
	}

	header := wire.NewBeaconBlockHeader(wire.BVersion(c.Version), int32(c.Height), *actualMMRRoot, *prevBlockHash,
		*h.merkleHash(transactions), chainhash.Hash{}, time.Unix(c.CurTime, 0), bits, chainWeight, 0)

	header.SetShards(c.Shards)
	header.SetK(c.K)
	header.SetVoteK(c.VoteK)
	header.SetBTCAux(btcAux)

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

func (h *Job) decodeShardBlockTemplateResponse(c *jaxjson.GetShardBlockTemplateResult, shardID uint32) (task *Task, err error) {
	if h.Beacon == nil {
		return nil, fmt.Errorf("can't initialise SC header")
	}

	actualMMRRoot, prevBlockHash, bits, target, chainWeight, err := h.decodeTemplateValues(c.PrevBlocksMMRRoot, c.PreviousHash, c.Bits, c.Target, c.ChainWeight)
	if err != nil {
		return nil, err
	}

	coinbaseTx, err := h.getCoinbaseTx(shardID, int32(c.Height), bits, nil)
	if err != nil {
		return nil, err
	}
	transactions, err := h.unmarshalTransactions(coinbaseTx, c.Transactions)
	if err != nil {
		return nil, err
	}

	header := wire.NewShardBlockHeader(int32(c.Height), *actualMMRRoot, *prevBlockHash, *h.merkleHash(transactions),
		bits, chainWeight, *h.Beacon.Block.Header.BeaconHeader(), *h.lastBCCoinbaseAux)

	return &Task{
		ShardID: shardID,
		Block: &wire.MsgBlock{
			Header:       header,
			Transactions: transactions,
		},
		Height: c.Height,
		Target: target,
	}, nil
}

func (h *Job) decodeTemplateValues(BlocksMMRRootS, prevBlockHashS, bitsS, targetS string, chainWeightS string) (
	actualMMRRoot, prevBlockHash *chainhash.Hash, bits uint32, target, chainWeight *big.Int, err error) {

	actualMMRRoot, err = chainhash.NewHashFromStr(BlocksMMRRootS)
	if err != nil {
		return
	}

	prevBlockHash, err = chainhash.NewHashFromStr(prevBlockHashS)
	if err != nil {
		return
	}

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

	chainWeight, ok := new(big.Int).SetString(chainWeightS, 10)
	if !ok {
		err = fmt.Errorf("can't parse chainWeight")
		return
	}

	return
}

func (h *Job) merkleHash(transactions []*wire.MsgTx) *chainhash.Hash {
	fakeBlock := jaxutil.NewBlock(&wire.MsgBlock{Transactions: transactions})
	merkles := chaindata.BuildMerkleTreeStore(fakeBlock.Transactions(), false)
	return merkles[len(merkles)-1]
}

func unmarshalTx(txHash string) (tx *wire.MsgTx, err error) {
	txBinary, err := hex.DecodeString(txHash)
	if err != nil {
		return
	}

	tx = &wire.MsgTx{}
	err = tx.Deserialize(bytes.NewReader(txBinary))
	return
}

func (h *Job) unmarshalTransactions(coinbaseTx *jaxutil.Tx, txs []jaxjson.GetBlockTemplateResultTx) (transactions []*wire.MsgTx, err error) {
	transactions = make([]*wire.MsgTx, 0)
	transactions = append(transactions, coinbaseTx.MsgTx())

	for _, marshalledTx := range txs {
		tx, err := unmarshalTx(marshalledTx.Data)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	return
}

func (h *Job) getCoinbaseTx(shardID uint32, height int32, bits uint32, coinbaseTxn *jaxjson.GetBlockTemplateResultTx) (*jaxutil.Tx, error) {
	var reward int64
	var burn bool

	if shardID == 0 {
		cTx, err := unmarshalTx(coinbaseTxn.Data)
		if err != nil {
			return nil, err
		}
		reward = cTx.TxOut[1].Value + cTx.TxOut[2].Value
		burn = h.Config.BurnBtc // burn beacon only if burnBtc is true

	} else {
		reward = chaindata.CalcShardBlockSubsidy(h.Config.ShardsCount, bits, h.Beacon.Block.Header.BeaconHeader().K())
		burn = !h.Config.BurnBtc // burn shard only if burnBtc is false

	}

	return chaindata.CreateJaxCoinbaseTx(reward, 0, height, shardID, h.Config.jaxMiningAddress, burn, shardID == 0)
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
