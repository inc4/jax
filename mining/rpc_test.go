package mining

import (
	"encoding/hex"
	"github.com/inc4/jax/mining/job"
	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/core/miner/core/common"
	"testing"
	"time"
)

const server = "http://jaxnetrpc:AUL6VBjoQnhP3bfFzl@128.199.64.36:18333"

func TestXXX(t *testing.T) {
	conf := &Config{
		serverAddress: server,
	}
	client, err := NewRPCClient(conf)
	if err != nil {
		t.Fatal(err)
	}
	go client.Do()
	for {
		time.Sleep(time.Second)
		t.Log(client.job)
	}
}

func TestCoinbase(t *testing.T) {
	jobConfig := &job.Configuration{
		Shards:          make(map[common.ShardID]job.ShardConfig),
		EnableBTCMining: true,
	}
	job := job.NewJob(jobConfig)

	p1, p2, err := job.GetBitcoinCoinbase(625540727, 666, 703687)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff3c03c7bc0a08", hex.EncodeToString(p1))
	assert.Equal(t, "2000000000000000000000000000000000000000000000000000000000000000000d2f503253482f6a61786e65742fffffffff030000000000000000176a152068747470733a2f2f6a61782e6e6574776f726b2077fe48250000000001519a02000000000000015100000000", hex.EncodeToString(p2))
}
