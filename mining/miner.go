package mining

import (
	"github.com/inc4/jax/mining/job"
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"net/url"
)

type Miner struct {
	Job       *job.Job
	rpcClient *rpcclient.Client
	rpcConf   *rpcclient.ConnConfig
}

func NewMiner(serverAddress, BtcAddress, JaxAddress string, burnBtc bool) (*Miner, error) {
	rpcConf := jaxRPCConfig(serverAddress)
	rpc, err := rpcclient.New(rpcConf, nil)
	if err != nil {
		return nil, err
	}
	j, err := job.NewJob(BtcAddress, JaxAddress, &chaincfg.TestNet3Params, burnBtc)
	if err != nil {
		return nil, err
	}

	return &Miner{
		Job:       j,
		rpcClient: rpc,
		rpcConf:   rpcConf,
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
