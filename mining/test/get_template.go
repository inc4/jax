package test

import (
	"encoding/json"
	"github.com/btcsuite/btcd/btcjson"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"log"
	"os"
)

func GetBtc() *btcjson.GetBlockTemplateResult {
	r := new(btcjson.GetBlockTemplateResult)
	getTemplate("btc", r)
	return r
}

func GetShard() *jaxjson.GetShardBlockTemplateResult {
	r := new(jaxjson.GetShardBlockTemplateResult)
	getTemplate("shard", r)
	return r
}

func GetBeacon() *jaxjson.GetBeaconBlockTemplateResult {
	r := new(jaxjson.GetBeaconBlockTemplateResult)
	getTemplate("bc", r)
	return r
}

func getTemplate(name string, v interface{}) {
	dat, err := os.ReadFile("test/" + name + ".json")
	if err != nil {
		log.Fatalln(err)
	}
	if err := json.Unmarshal(dat, v); err != nil {
		log.Fatalln(err)
	}
}
