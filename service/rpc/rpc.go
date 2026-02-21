package rpc

import (
	"context"
	xclient "github.com/smallnest/rpcx/client"
)

// Server 服务端
type ServerInterface interface {
	// Start 启动
	Start() error

	// Stop 停止
	Stop() error

	// Register 注册服务
	//
	// - rcvr: 服务对象，例如 &Forward{}
	Register(rcvr any) error

	// Output 输出当前信息
	Output() string
}

// Client 客户端
type ClientInterface interface {
	// Wrap 指定参数调用
	Wrap(id, versionMin, versionMax uint32) WrapClient

	// Call 同步调用
	Call(ctx context.Context, serviceMethod string, args any, reply any) error

	// Go 异步调用
	Go(ctx context.Context, serviceMethod string, args any, reply any, done chan *xclient.Call) (*xclient.Call, error)

	// Close 关闭
	Close() error

	// Name rpc 服务提供者名称
	Name() string
}

// WrapClient 指定额外参数调用
type WrapClient interface {
	// Call 同步调用
	Call(ctx context.Context, serviceMethod string, args any, reply any) error

	// Go 异步调用
	Go(ctx context.Context, serviceMethod string, args any, reply any, done chan *xclient.Call) (*xclient.Call, error)
}
