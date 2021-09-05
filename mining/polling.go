package mining

import (
	"context"
	"gitlab.com/jaxnet/core/miner/core/common"
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"log"
	"time"
)

const (
	getTemplateInverval = time.Second
)

type Poller struct {
	Miner
	shards map[uint32]context.CancelFunc
	log    *log.Logger
}

func (p *Poller) Do() {
	go p.fetchBeaconTemplate()
	for {
		p.fetchShards()
		time.Sleep(time.Second * 600)
	}
}

func (p *Poller) fetchShards() {
	res, err := p.rpcClient.ListShards()
	if err != nil {
		p.log.Println("ERR", err)
		return
	}
	for id, shard := range res.Shards {
		if !shard.Enabled {
			continue
		}
		if _, ok := p.shards[id]; !ok {
			ctx, cancel := context.WithCancel(context.Background())
			p.shards[id] = cancel
			go p.fetchShardTemplate(ctx, id)
		}
	}
	for id, _ := range p.shards {
		if _, ok := res.Shards[id]; !ok {
			// TODO shard deleted
		}
	}
}

func (p *Poller) fetchBeaconTemplate() {
	params := &jaxjson.TemplateRequest{
		Capabilities: []string{
			"coinbasetxn",
		},
	}
	for {
		template, err := p.rpcClient.GetBeaconBlockTemplate(params)
		if err == nil {
			params.LongPollID = template.LongPollID
			p.log.Println("beacon", template.Height)

			err := p.job.ProcessBeaconTemplate(template)
			if err != nil {
				p.log.Println("ERR", err)
			}

		} else {
			p.log.Println("ERR", err)
			time.Sleep(getTemplateInverval)
		}
	}
}

func (p *Poller) fetchShardTemplate(ctx context.Context, id uint32) {
	params := &jaxjson.TemplateRequest{
		Capabilities: []string{
			"coinbasetxn",
		},
	}
	for {
		ch := GetShardBlockTemplateAsync(p.rpcClient.ForShard(id), params)
		select {
		case r := <-ch:
			if r.err == nil {
				template := r.result
				params.LongPollID = template.LongPollID
				p.log.Println("shard", id, template.Height)

				err := p.job.ProcessShardTemplate(template, common.ShardID(id))
				if err != nil {
					p.log.Println("ERR", err)
				}

			} else {
				p.log.Println("ERR", r.err)
				time.Sleep(getTemplateInverval)
			}
		case <-ctx.Done():
			p.log.Println("stop fetching template shard", id)
			return
		}
	}
}

type resShardBlockTemplate struct {
	result *jaxjson.GetShardBlockTemplateResult
	err    error
}

func GetShardBlockTemplateAsync(rpc *rpcclient.Client, reqData *jaxjson.TemplateRequest) chan resShardBlockTemplate {
	ch := make(chan resShardBlockTemplate)
	go func() {
		result, err := rpc.GetShardBlockTemplateAsync(reqData).Receive()
		ch <- resShardBlockTemplate{result, err}
	}()
	return ch
}
