// Code generated by MockGen. DO NOT EDIT.
// Source: job.go

// Package job is a generated GoMock package.
package job

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	common "gitlab.com/jaxnet/core/miner/core/common"
	jaxutil "gitlab.com/jaxnet/jaxnetd/jaxutil"
)

// MockRpcClient is a mock of RpcClient interface.
type MockRpcClient struct {
	ctrl     *gomock.Controller
	recorder *MockRpcClientMockRecorder
}

// MockRpcClientMockRecorder is the mock recorder for MockRpcClient.
type MockRpcClientMockRecorder struct {
	mock *MockRpcClient
}

// NewMockRpcClient creates a new mock instance.
func NewMockRpcClient(ctrl *gomock.Controller) *MockRpcClient {
	mock := &MockRpcClient{ctrl: ctrl}
	mock.recorder = &MockRpcClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRpcClient) EXPECT() *MockRpcClientMockRecorder {
	return m.recorder
}

// SubmitBeacon mocks base method.
func (m *MockRpcClient) SubmitBeacon(block *jaxutil.Block) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SubmitBeacon", block)
}

// SubmitBeacon indicates an expected call of SubmitBeacon.
func (mr *MockRpcClientMockRecorder) SubmitBeacon(block interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitBeacon", reflect.TypeOf((*MockRpcClient)(nil).SubmitBeacon), block)
}

// SubmitShard mocks base method.
func (m *MockRpcClient) SubmitShard(block *jaxutil.Block, shardID common.ShardID) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SubmitShard", block, shardID)
}

// SubmitShard indicates an expected call of SubmitShard.
func (mr *MockRpcClientMockRecorder) SubmitShard(block, shardID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitShard", reflect.TypeOf((*MockRpcClient)(nil).SubmitShard), block, shardID)
}
