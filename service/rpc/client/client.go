package client

import (
	"time"

	xclient "github.com/smallnest/rpcx/client"
)

// Client 客户端
//type ClientInterface interface {
//	// Wrap 指定参数调用
//	//Wrap(id, versionMin, versionMax uint32) client.WrapClient
//
//	// Call 同步调用
//	Call(ctx context.Context, serviceMethod string, args any, reply any) error
//
//	// Go 异步调用
//	Go(ctx context.Context, serviceMethod string, args any, reply any, done chan *xclient.Call) (*xclient.Call, error)
//
//	// Close 关闭
//	Close() error
//
//	// Name rpc 服务提供者名称
//	Name() string
//}

// 客户端rpcx 配置
type Config struct {
	// etcd 心跳间隔
	HeartbeatInterval time.Duration
	// etcd 服务器地址列表
	EtcdEndpoints []string
	// etcd 服务注册基础路径，相当于缓存前缀，避免不同项目使用同一个 etcd 造成混乱，例如 MODOU_LDL
	BasePath string
	// 服务名称，例如 gate, game, battle
	ServiceName string
	// 服务对象名称，例如 Forward
	ServicePath string

	// 客户端连接池数量
	PoolSize int
	// 客户端调用失败处理，见 https://doc.rpcx.io/part3/failmode.html#%E5%A4%B1%E8%B4%A5%E6%A8%A1%E5%BC%8F
	FailMode xclient.FailMode
	// 客户端路由方式，见 https://doc.rpcx.io/part3/selector.html#%E8%B7%AF%E7%94%B1
	SelectMode xclient.SelectMode
	// 当路由方式为自定义时，此处传入自定义的路由选择器
	Selector xclient.Selector
}

// Client rpcx 客户端接口实现
type Client struct {
	// 配置信息
	config *Config
	// rpcx 客户端
	pool *xclient.XClientPool
	// etcd 服务发现
	discovery xclient.ServiceDiscovery
}
