package job

//go:generate mockgen -source=job.go -destination solution_rpcmock_test.go -package job

import (
	"github.com/golang/mock/gomock"
	"github.com/inc4/jax/mining/test"
	"gitlab.com/jaxnet/core/miner/core/common"
	"testing"
)

func TestTask(t *testing.T) {
	ctrl := gomock.NewController(t)

	jobConfig := &Configuration{
		Shards:          make(map[common.ShardID]ShardConfig),
		EnableBTCMining: true,
	}
	client := NewMockRpcClient(ctrl)
	job := NewJob(jobConfig, client)

	job.ProcessBeaconTemplate(test.GetBeacon())
	job.ProcessShardTemplate(test.GetShard(), 0)

	// todo
	client.EXPECT().SubmitBeacon(gomock.Eq(nil))
	client.EXPECT().SubmitShard(gomock.Eq(nil), 0)

	// todo
	job.CheckSolution(nil, nil)

}
