package mining

import (
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
		time.Sleep(1)
		t.Log(client.job)
	}
}
