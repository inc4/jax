package mining

import (
	"encoding/hex"
	"encoding/json"
	"github.com/inc4/jax/mining/test"
	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/core/miner/core/common"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chain"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTask(t *testing.T) {
	rpcOutputCh := make(chan string, 10)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		rpcOutputCh <- string(res)
		w.Write([]byte("{\"result\": null}"))
	}))
	defer ts.Close()

	url := "http://a:a@" + strings.TrimPrefix(ts.URL, "http://")
	miner, err := NewMiner(url, "mzDGR33maDBujpqjkvxVzY2ssYDcQG51p3", "mxQsksaTJb11i7vSxAUL6VBjoQnhP3bfFz")
	if err != nil {
		t.Fatal(err)
	}

	err = miner.Job.ProcessBeaconTemplate(test.GetBeacon())
	if err != nil {
		t.Fatal(err)
	}
	err = miner.Job.ProcessShardTemplate(test.GetShard(), 1)
	if err != nil {
		t.Fatal(err)
	}

	btcHeader, _ := hex.DecodeString("00004020b6ef34e5bcb9662ee1645ab64feb6c5ec29f4e5ab2329c010000000000000000d927ccc17e9e89d135988350c6138545a0798d12ae51adb4995dbfe9adcf71d9e1f33461ba6a0418c7a734ac")
	coinbaseTx, _ := hex.DecodeString("01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff3c0369561608ffffffffffffffff2028cd7057e92b29dc6c5fbedb17d6e3e1c1162954f066bd704d606424cf3b47db0d2f503253482f6a61786e65742fffffffff030000000000000000176a152068747470733a2f2f6a61782e6e6574776f726b200046c3230000000001511027000000000000015100000000")

	results, err := miner.Solution(btcHeader, coinbaseTx)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "96624343542776224466899884153850672663883206715290011223164084822212608", miner.Job.GetMinTarget().String())

	assert.Equal(t, 2, len(results))

	assert.Equal(t, common.ShardID(0), results[0].ShardId)
	assert.Equal(t, int64(5000), results[0].Amount)
	assert.Equal(t, int64(622805), results[0].BlockHeight)

	assert.Equal(t, common.ShardID(1), results[1].ShardId)
	assert.Equal(t, int64(5000), results[1].Amount)
	assert.Equal(t, int64(625923), results[1].BlockHeight)

	expectedShard := "{\"jsonrpc\":\"1.0\",\"method\":\"submitblock\",\"scope\":\"chain\",\"shard_id\":1,\"params\":[\"0ffc02b1cac15992ca26faf1263a1c479f5b7c80038dddedc1259bc508111864af743b57a87cceed40753c0cfb6ae20696a2e2dcd1fa3215a57b7486b9e512c1920d3661ffff0d1e0100000000000020e46e2d764652e2d75c62e5c85512696df936816b69a0daabaacdf599464a021c4ffee45bc79238f3866671b93c1b2f9a6a740698ba1688c377446f334b1053c2c3150c313f852bd6e848036a8d3b16a58cbca0738a79d490b80c0536362fa2c6cde03561ffff0d1e00000000388800033888000324000000208cbe5e514ffeccfd8a332715883ff0b9fdd24226f519d1f5bcd3dfa52482e1a0010b0600004020b6ef34e5bcb9662ee1645ab64feb6c5ec29f4e5ab2329c010000000000000000d927ccc17e9e89d135988350c6138545a0798d12ae51adb4995dbfe9adcf71d9e1f33461ba6a0418c7a734ac01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff3c0369561608ffffffffffffffff2028cd7057e92b29dc6c5fbedb17d6e3e1c1162954f066bd704d606424cf3b47db0d2f503253482f6a61786e65742fffffffff030000000000000000176a152068747470733a2f2f6a61782e6e6574776f726b200046c32300000000015110270000000000000151000000000001000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1503d5800953000e2f503253482f6a61786e6574642fffffffff030000000000000000176a152068747470733a2f2f6a61782e6e6574776f726b2088130000000000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00000000000000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00000000014ffee45bc79238f3866671b93c1b2f9a6a740698ba1688c377446f334b1053c20101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1503038d0951000e2f503253482f6a61786e6574642fffffffff030000000000000000176a152068747470733a2f2f6a61782e6e6574776f726b2088130000000000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00000000000000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00000000\"],\"id\":1}"
	expectedBeacon := "{\"jsonrpc\":\"1.0\",\"method\":\"submitblock\",\"scope\":\"chain\",\"shard_id\":0,\"params\":[\"00000020e46e2d764652e2d75c62e5c85512696df936816b69a0daabaacdf599464a021c4ffee45bc79238f3866671b93c1b2f9a6a740698ba1688c377446f334b1053c2c3150c313f852bd6e848036a8d3b16a58cbca0738a79d490b80c0536362fa2c6cde03561ffff0d1e00000000388800033888000324000000208cbe5e514ffeccfd8a332715883ff0b9fdd24226f519d1f5bcd3dfa52482e1a0010b0600004020b6ef34e5bcb9662ee1645ab64feb6c5ec29f4e5ab2329c010000000000000000d927ccc17e9e89d135988350c6138545a0798d12ae51adb4995dbfe9adcf71d9e1f33461ba6a0418c7a734ac01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff3c0369561608ffffffffffffffff2028cd7057e92b29dc6c5fbedb17d6e3e1c1162954f066bd704d606424cf3b47db0d2f503253482f6a61786e65742fffffffff030000000000000000176a152068747470733a2f2f6a61782e6e6574776f726b200046c3230000000001511027000000000000015100000000000101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1503d5800953000e2f503253482f6a61786e6574642fffffffff030000000000000000176a152068747470733a2f2f6a61782e6e6574776f726b2088130000000000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00000000000000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00000000\"],\"id\":1}"

	assert.Equal(t, expectedBeacon, <-rpcOutputCh)
	assert.Equal(t, expectedShard, <-rpcOutputCh)
}

// don't need anymore :( but steel may be useful

func deserializeJson(t *testing.T, data []byte, chain chain.IChainCtx) (*jaxjson.Request, *wire.MsgBlock) {
	request := new(jaxjson.Request)
	if err := json.Unmarshal(data, request); err != nil {
		t.Fatal(err)
	}
	hexStr := string(request.Params[0])
	hexStr = hexStr[1 : len(hexStr)-1] // remove first and last quote character
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}

	blockBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatal(err)
	}
	block, err := jaxutil.NewBlockFromBytes(chain, blockBytes)
	if err != nil {
		t.Fatal(err)
	}
	return request, block.MsgBlock()
}
