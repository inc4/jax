package mining

import (
	btc_wire "github.com/btcsuite/btcd/wire"
	//shard_wire "gitlab.com/jaxnet/core/shard.core/types/wire"
	beacon_wire "gitlab.com/jaxnet/jaxnetd/types/wire"
)

type ShardJob struct {
	Block *btc_wire.MsgBlock
}

type Job struct {
	BeaconBlock *beacon_wire.MsgBlock
}
