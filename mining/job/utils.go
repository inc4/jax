package job

import (
	"bytes"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

func SplitCoinbase(block *wire.MsgBlock) ([]byte, []byte) {
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
