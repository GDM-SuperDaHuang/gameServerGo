package client

import (
	"fmt"
	"gameServer/pkg/config"
	"time"

	xclient "github.com/smallnest/rpcx/client"
)

// 客户端rpcx 配置
type ClientConfig struct {
	// etcd 心跳间隔
	HeartbeatInterval time.Duration
	// etcd 服务器地址列表
	EtcdEndpoints []string
	// etcd 服务注册基础路径，相当于缓存前缀，避免不同项目使用同一个 etcd 造成混乱，例如 MODOU_LDL
	BasePath string
	// 服务名称，例如 gate, game, battle
	ServiceName string
	// 服务对象名称，例如 Forward，Gate
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
	config *ClientConfig
	// rpcx 客户端
	pool *xclient.XClientPool
	// etcd 服务发现
	discovery xclient.ServiceDiscovery
}

// 从服务配置中创建
//
//   - name: 服务名称，例如 gate, game
//   - rcvr: 服务端接口，例如 &Gate{}, &Game{}
//   - poolSize: 连接池大小
//   - failMode: 失败模式，例如 xclient.Failover, xclient.Failfast
//   - selectMode: 选择模式，例如 xclient.RandomSelect, xclient.RoundRobin
//   - selector: 选择器，例如 &pkgrpcclient.ServerIDSelector{}
func BuildClientConfig(name, methodName string, poolSize int, failMode xclient.FailMode, selectMode xclient.SelectMode, selector xclient.Selector) *ClientConfig {
	c := config.Get()

	basePath := fmt.Sprintf("%s", c.EtcdPrefix())

	return &ClientConfig{
		HeartbeatInterval: time.Duration(c.RPCHeart()) * time.Second,
		EtcdEndpoints:     c.EtcdAddress(), //":12379;:22329;:32329"
		BasePath:          basePath,
		ServiceName:       name, //没用上
		ServicePath:       methodName,
		PoolSize:          poolSize,
		FailMode:          failMode,
		SelectMode:        selectMode,
		Selector:          selector,
	}
}
