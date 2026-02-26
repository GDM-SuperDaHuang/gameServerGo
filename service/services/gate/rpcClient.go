package gate

import (
	"gameServer/service/rpc"
	"gameServer/service/rpc/client"
	"gameServer/service/rpc/client/selector"
	"sync"

	xclient "github.com/smallnest/rpcx/client"
)

// RPCClient 远程调用客户端
var RpcGateClient rpc.ClientInterface

// SetRPCClient 设置 rpc 调用，用于测试
func SetGateRPCClient(c rpc.ClientInterface) {
	RpcGateClient = c
}

// 获取远程调用客户端
var GateRPCClients = sync.OnceValue(func() rpc.ClientInterface {
	if RpcGateClient != nil {
		return RpcGateClient
	}

	// node
	c, err := client.NewClient(client.BuildClientConfig(
		"node",
		"Forward",
		8,
		xclient.Failfast,     // account 节点具有唯一性，不需要重复尝试
		xclient.SelectByUser, // 使用自定义选择器
		&selector.DefaultSelector{},
	))
	if err != nil {
		panic(err)
	}
	return c
})
