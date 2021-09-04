package mining

import (
	"context"
	"github.com/inc4/jax/mining/job"
	"gitlab.com/jaxnet/core/miner/core/common"
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"log"
	"net/url"
	"time"
)

const getTemplateInverval = time.Second

type RPCClient struct {
	config *Config
	rpc    *rpcclient.Client
	job    *job.Job
	shards map[uint32]context.CancelFunc
	log    *log.Logger
}

func NewRPCClient(config *Config) (*RPCClient, error) {
	rpc, err := rpcclient.New(jaxRPCConfig(config.serverAddress), nil)
	if err != nil {
		return nil, err
	}
	jobConfig := &job.Configuration{
		Shards:           make(map[common.ShardID]job.ShardConfig),
		EnableBTCMining:  true,
		BurnBtcReward:    false,
		BurnJaxReward:    false,
		BurnJaxNetReward: false,
		BtcMiningAddress: nil,
		JaxMiningAddress: nil,
	}
	return &RPCClient{
		config: config,
		rpc:    rpc,
		job:    job.NewJob(jobConfig),
		shards: make(map[uint32]context.CancelFunc),
		log:    log.Default(),
	}, nil
}

func (c *RPCClient) fetchShards() {
	res, err := c.rpc.ListShards()
	if err != nil {
		c.log.Println("ERR", err)
		return
	}
	for id, shard := range res.Shards {
		if !shard.Enabled {
			continue
		}
		if _, ok := c.shards[id]; !ok {
			ctx, cancel := context.WithCancel(context.Background())
			c.shards[id] = cancel
			go c.fetchShardTemplate(ctx, id)
		}
	}
	for id, _ := range c.shards {
		if _, ok := res.Shards[id]; !ok {
			// TODO shard deleted
		}
	}
}

func (c *RPCClient) Do() {
	go c.fetchBeaconTemplate()
	for {
		c.fetchShards()
		time.Sleep(1)
	}
}

func (c *RPCClient) fetchBeaconTemplate() {
	clientConfig := jaxRPCConfig(c.config.serverAddress)
	rpc, err := rpcclient.New(clientConfig, nil)
	if err != nil {
		c.log.Println("ERR", err)
		return
	}

	params := &jaxjson.TemplateRequest{}
	for {
		template, err := rpc.GetBeaconBlockTemplate(params)
		if err == nil {
			params.LongPollID = template.LongPollID
			log.Println("beacon", template.Height)
			c.job.ProcessBeaconTemplate(template)
		} else {
			c.log.Println("ERR", err)
			time.Sleep(getTemplateInverval)
		}
	}
}

func (c *RPCClient) fetchShardTemplate(ctx context.Context, id uint32) {
	clientConfig := jaxRPCConfig(c.config.serverAddress)
	clientConfig.ShardID = id
	rpc, err := rpcclient.New(clientConfig, nil)
	if err != nil {
		c.log.Println("ERR", err)
		return
	}

	params := &jaxjson.TemplateRequest{}
	for {
		ch := GetShardBlockTemplateAsync(rpc, params)
		select {
		case r := <-ch:
			if r.err == nil {
				template := r.result
				params.LongPollID = template.LongPollID
				log.Println("shard", id, template.Height)
				c.job.ProcessShardTemplate(template, common.ShardID(id))
			} else {
				c.log.Println("ERR", r.err)
				time.Sleep(getTemplateInverval)
			}
		case <-ctx.Done():
			c.log.Println("stop fetching template shard", id)
			return
		}
	}
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
