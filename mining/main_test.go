package mining

import (
	"github.com/inc4/jax/mining/test"
	"gitlab.com/jaxnet/core/miner/core/communicator/events"
	"gitlab.com/jaxnet/core/miner/core/logger"
	"gitlab.com/jaxnet/core/miner/core/settings"
	"gitlab.com/jaxnet/core/miner/core/state"
	"log"
	"testing"
)

func TestTask(t *testing.T) {
	logger.Init()
	shards := make(chan events.ShardBlockCandidate)
	bc := make(chan events.BeaconBlockCandidate)
	btc := make(chan events.BitcoinBlockCandidate)

	c := state.New(&settings.Configuration{})
	go c.RunUsing(shards, bc, btc)

	btc <- events.BitcoinBlockCandidate{Candidate: test.GetBtc()}
	bc <- events.BeaconBlockCandidate{Candidate: test.GetBeacon()}
	shards <- events.ShardBlockCandidate{ShardID: 0, Candidate: test.GetShard()}

	task := c.NextTask()
	log.Println(task)
}
