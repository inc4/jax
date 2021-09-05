package mining

import (
	"github.com/inc4/jax/mining/job"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"net/url"
)

//go:generate mockgen -source=miner.go -destination mock_job_test.go -package mining

type RpcClient interface {
	SubmitBlock(*jaxutil.Block, *jaxjson.SubmitBlockOptions) error
	ForShard(uint32) *rpcclient.Client
	ForBeacon() *rpcclient.Client
	ListShards() (*jaxjson.ShardListResult, error)
	GetBeaconBlockTemplate(*jaxjson.TemplateRequest) (*jaxjson.GetBeaconBlockTemplateResult, error)
}

type Miner struct {
	job       *job.Job
	rpcClient RpcClient
}

func NewMiner(serverAddress, BtcAddress, JaxAddress string) (*Miner, error) {
	rpc, err := rpcclient.New(jaxRPCConfig(serverAddress), nil)
	if err != nil {
		return nil, err
	}
	j, err := job.NewJob(BtcAddress, JaxAddress)
	if err != nil {
		return nil, err
	}

	return &Miner{
		job:       j,
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
