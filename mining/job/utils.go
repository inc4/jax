package job

import (
	"bytes"
	"encoding/binary"
	btcdchainhash "github.com/btcsuite/btcd/chaincfg/chainhash"

	btcdwire "github.com/btcsuite/btcd/wire"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

func JaxTxToBtcTx(tx *wire.MsgTx) btcdwire.MsgTx {
	msgTx := btcdwire.MsgTx{
		Version:  tx.Version,
		TxIn:     make([]*btcdwire.TxIn, len(tx.TxIn)),
		TxOut:    make([]*btcdwire.TxOut, len(tx.TxOut)),
		LockTime: tx.LockTime,
	}

	for i := range msgTx.TxIn {
		msgTx.TxIn[i] = &btcdwire.TxIn{
			PreviousOutPoint: btcdwire.OutPoint{
				Hash:  btcdchainhash.Hash(tx.TxIn[i].PreviousOutPoint.Hash),
				Index: tx.TxIn[i].PreviousOutPoint.Index,
			},
			SignatureScript: tx.TxIn[i].SignatureScript,
			Witness:         btcdwire.TxWitness(tx.TxIn[i].Witness),
			Sequence:        tx.TxIn[i].Sequence,
		}
	}

	for i := range msgTx.TxOut {
		msgTx.TxOut[i] = &btcdwire.TxOut{
			Value:    tx.TxOut[i].Value,
			PkScript: tx.TxOut[i].PkScript,
		}
	}
	return msgTx
}

func SplitCoinbase(block *btcdwire.MsgBlock) ([]byte, []byte) {
	buf := bytes.NewBuffer(nil)
	block.Transactions[0].Serialize(buf)

	rawTx := buf.Bytes()
	heightLenIdx := 42
	heightLen := int(rawTx[heightLenIdx])
	if heightLen > 0xF {
		// if value more than 0xF,
		// this indicates that height is packed as an opcode OP_0 .. OP_16 (small int)
		// height = int(rawTx[heightLenIdx] - (txscript.OP_1 - 1))
		// so height value doesn't gives additional padding
		heightLen = 0
	}

	extraNonceLenIdx := heightLenIdx + 1 + heightLen
	extraNonceLen := int(rawTx[extraNonceLenIdx])
	// extraNonceLenIdx := 42
	// extraNonceLen := int(rawTx[extraNonceLenIdx])

	part1 := rawTx[0:extraNonceLenIdx]
	part1 = append(part1, 0x08)
	part2 := rawTx[extraNonceLenIdx+extraNonceLen+1:]
	return part1, part2
}

func PackUint64LE(n uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n)
	return b
}
