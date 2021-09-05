package rpc

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
