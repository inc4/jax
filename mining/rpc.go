package mining

import (
	"context"
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
	shards map[uint32]context.CancelFunc
	log    *log.Logger
}

func NewRPCClient(config *Config) (*RPCClient, error) {
	rpc, err := rpcclient.New(jaxRPCConfig(config.serverAddress), nil)
	if err != nil {
		return nil, err
	}
	return &RPCClient{
		config: config,
		rpc:    rpc,
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
	for {
		c.fetchShards()
		time.Sleep(1)
	}
}

func (c *RPCClient) fetchShardTemplate(ctx context.Context, id uint32) {
	clientConfig := jaxRPCConfig(c.config.serverAddress)
	clientConfig.ShardID = id
	rpc, err := rpcclient.New(clientConfig, nil)
	if err != nil {
		// TODO should we return here?
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
				// TODO job.update(template)
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
