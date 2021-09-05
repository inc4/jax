package mining

import (
	"github.com/inc4/jax/mining/job"
	"gitlab.com/jaxnet/core/miner/core/common"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
	"net/url"
)

type RpcClient interface {
	SubmitBeacon(block *jaxutil.Block) error
	SubmitShard(block *jaxutil.Block, shardID common.ShardID) error
}

type Miner struct {
	job       job.Job
	rpcClient *rpcclient.Client
}

func NewMiner(serverAddress string) (*Miner, error) {
	rpc, err := rpcclient.New(jaxRPCConfig(serverAddress), nil)
	if err != nil {
		return nil, err
	}
	return &Miner{
		job:       job.Job{},
		rpcClient: rpc,
	}, nil
}

func jaxRPCConfig(address string) *rpcclient.ConnConfig {
	params, _ := url.Parse(address)
	user := params.User.Username()
	pass, _ := params.User.Password()
	return &rpcclient.ConnConfig{
		Host:         params.Host,
		Endpoint:     "ws",
		Params:       "testnet",
		User:         user,
		Pass:         pass,
		HTTPPostMode: true,
		DisableTLS:   true,
	}
}
