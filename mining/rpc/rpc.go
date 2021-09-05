package rpc

import (
	"context"
	"gitlab.com/jaxnet/core/miner/core/common"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"log"
	"net/url"
	"time"
)

const (
	getTemplateInverval = time.Second
)

var (
	jaxNetParams = &chaincfg.TestNet3Params
)

type RPCClient struct {
	serverAddress    string
	JaxRewardAddress *jaxutil.Address
	BTCRewardAddress *jaxutil.Address // TODO
	rpc              *rpcclient.Client
	shards           map[uint32]context.CancelFunc
	log              *log.Logger

	BeaconCallback func(*jaxjson.GetBeaconBlockTemplateResult)
	ShardCallback  func(*jaxjson.GetShardBlockTemplateResult, common.ShardID)
}

func NewRPCClient(serverAddress, JaxRewardAddress, BTCRewardAddress string) (*RPCClient, error) {
	rpc, err := rpcclient.New(jaxRPCConfig(serverAddress), nil)
	if err != nil {
		return nil, err
	}
	jaxRewardAddress, err := jaxutil.DecodeAddress(
		JaxRewardAddress, jaxNetParams)
	if err != nil {
		return nil, err
	}
	btcRewardAddress, err := jaxutil.DecodeAddress(
		BTCRewardAddress, jaxNetParams)
	if err != nil {
		return nil, err
	}

	// todo set callbacks
	return &RPCClient{
		serverAddress:    serverAddress,
		JaxRewardAddress: &jaxRewardAddress,
		BTCRewardAddress: &btcRewardAddress,
		rpc:              rpc,
		shards:           make(map[uint32]context.CancelFunc),
		log:              log.Default(),
	}, nil
}

func (c *RPCClient) SetCallbacks(beaconCallback func(*jaxjson.GetBeaconBlockTemplateResult), shardCallback func(*jaxjson.GetShardBlockTemplateResult, common.ShardID)) {
	c.BeaconCallback = beaconCallback
	c.ShardCallback = shardCallback
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
		time.Sleep(time.Second)
	}
}

func (c *RPCClient) fetchBeaconTemplate() {
	clientConfig := jaxRPCConfig(c.serverAddress)
	rpc, err := rpcclient.New(clientConfig, nil)
	if err != nil {
		c.log.Println("ERR", err)
		return
	}

	params := &jaxjson.TemplateRequest{
		Capabilities: []string{
			"coinbasetxn",
		},
	}
	for {
		template, err := rpc.GetBeaconBlockTemplate(params)
		if err == nil {
			params.LongPollID = template.LongPollID
			c.log.Println("beacon", template.Height)
			c.BeaconCallback(template)
			//c.Job.ProcessBeaconTemplate(template)
		} else {
			c.log.Println("ERR", err)
			time.Sleep(getTemplateInverval)
		}
	}
}

func (c *RPCClient) fetchShardTemplate(ctx context.Context, id uint32) {
	clientConfig := jaxRPCConfig(c.serverAddress)
	clientConfig.ShardID = id
	rpc, err := rpcclient.New(clientConfig, nil)
	if err != nil {
		c.log.Println("ERR", err)
		return
	}

	params := &jaxjson.TemplateRequest{
		Capabilities: []string{
			"coinbasetxn",
		},
	}
	for {
		ch := GetShardBlockTemplateAsync(rpc, params)
		select {
		case r := <-ch:
			if r.err == nil {
				template := r.result
				params.LongPollID = template.LongPollID
				c.log.Println("shard", id, template.Height)
				c.ShardCallback(template, common.ShardID(id))
				//c.Job.ProcessShardTemplate(template, common.ShardID(id))
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

func (c *RPCClient) SubmitBeacon(block *jaxutil.Block) {
	c.rpc.SubmitBlock(block, nil)
}

func (c *RPCClient) SubmitShard(block *jaxutil.Block, shardID common.ShardID) {
	c.rpc.ForShard(uint32(shardID)).SubmitBlock(block, nil)
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
