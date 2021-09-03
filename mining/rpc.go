package mining

import (
	jsonrpc "github.com/Arhius/jsonrpc-codec/jsonrpc1"
)

type RPCClient struct {
	config *Config
	rpc    *jsonrpc.Client
}

func NewRPCClient(config *Config) *RPCClient {
	return &RPCClient{
		config: config,
		rpc: jsonrpc.NewHTTPClient(config.serverAddress),
	}
}

func (c *RPCClient) Do() {
	for {
	}
}
