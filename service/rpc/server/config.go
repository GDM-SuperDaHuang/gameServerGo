package server

import (
	"Server/service/config"
	"fmt"
	"reflect"
	"time"

	xclient "github.com/smallnest/rpcx/client"
)

// BuildServerConfig 从服务配置表中创建
func BuildServerConfig() *RpcConfig {
	c := config.Get()

	return &RpcConfig{
		ID:             c.NodeID(),
		Name:           c.NodeName(),
		Version:        c.NodeVersion(),
		UpdateInterval: 10 * time.Second,
		EtcdEndpoints:  c.EtcdAddress(),
		BasePath:       c.EtcdPrefix(),
		ServiceName:    c.ServiceName(),
	}
}

// BuildClientConfig 从服务配置中创建
//
//   - name: 服务名称，例如 gate, game
//   - rcvr: 服务端接口，例如 &Gate{}, &Game{}
//   - poolSize: 连接池大小
//   - failMode: 失败模式，例如 xclient.Failover, xclient.Failfast
//   - selectMode: 选择模式，例如 xclient.RandomSelect, xclient.RoundRobin
//   - selector: 选择器，例如 &pkgrpcclient.ServerIDSelector{}
func BuildClientConfig(name string, rcvr any, poolSize int, failMode xclient.FailMode, selectMode xclient.SelectMode, selector xclient.Selector) *RpcConfig {
	c := config.Get()

	basePath := fmt.Sprintf("%s/%s", c.EtcdPrefix(), name)
	servicePath := reflect.Indirect(reflect.ValueOf(rcvr)).Type().Name()

	return &RpcConfig{
		HeartbeatInterval: time.Duration(c.RPCHeart()) * time.Second,
		EtcdEndpoints:     c.EtcdAddress(),
		BasePath:          basePath,
		ServiceName:       name,
		ServicePath:       servicePath,
		PoolSize:          poolSize,
		FailMode:          failMode,
		SelectMode:        selectMode,
		Selector:          selector,
	}
}
