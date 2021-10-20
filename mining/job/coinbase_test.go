package job

import (
	"encoding/hex"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCoinbase(t *testing.T) {
	job, _ := NewJob("mzDGR33maDBujpqjkvxVzY2ssYDcQG51p3", "mzDGR33maDBujpqjkvxVzY2ssYDcQG51p3", true, true)

	coinbase, err := job.GetBitcoinCoinbase(&CoinBaseData{Reward: 625540727, Fee: 666, Height: 703687})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff3c03c7bc0a08", hex.EncodeToString(coinbase.Part1))
	assert.Equal(t, "2000000000000000000000000000000000000000000000000000000000000000000d2f503253482f6a61786e65742fffffffff030000000000000000176a152068747470733a2f2f6a61782e6e6574776f726b2077fe4825000000001976a914cd120759aa39d9184d19b8c390d30da979218cea88ac9a020000000000001976a914cd120759aa39d9184d19b8c390d30da979218cea88ac00000000", hex.EncodeToString(coinbase.Part2))
}
