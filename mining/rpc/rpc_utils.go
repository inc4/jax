package rpc

import (
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
)

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
