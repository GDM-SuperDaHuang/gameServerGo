package rpc

import (
	"context"
	"sync"
	"time"

	xclient "github.com/smallnest/rpcx/client"
	"go.etcd.io/etcd/client/v2"
)

// Client 客户端
type ClientInterface interface {
	// Wrap 指定参数调用
	//Wrap(id, versionMin, versionMax uint32) client.WrapClient

	// Call 同步调用
	Call(ctx context.Context, serviceMethod string, args any, reply any) error

	// Go 异步调用
	Go(ctx context.Context, serviceMethod string, args any, reply any, done chan *xclient.Call) (*xclient.Call, error)

	// Close 关闭
	Close() error

	// Name rpc 服务提供者名称
	Name() string
}

// NewClient 创建客户端
func NewClient(config *Config) (Client, error) {
	return client.New(config)
}

// RPCClient 远程调用客户端
var rpcClient Client

// SetRPCClient 设置 rpc 调用，用于测试
func SetRPCClient(c pkgrpc.Client) {
	rpcClient = c
}

// RPCClients 获取远程调用客户端
var RPCClients = sync.OnceValue(func() pkgrpc.Client {
	if rpcClient != nil {
		return rpcClient
	}

	c, err := pkgrpc.NewClient(internalrpc.BuildClientConfig(
		"account",
		new(Forward),
		8,
		xclient.Failfast,     // account 节点具有唯一性，不需要重复尝试
		xclient.SelectByUser, // 使用自定义选择器
		&pkgrpcclient.UniqueSelector{},
	))
	if err != nil {
		panic(err)
	}
	return c
})
