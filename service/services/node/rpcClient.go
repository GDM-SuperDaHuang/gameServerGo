package node

import (
	"gameServer/service/rpc"
	"gameServer/service/rpc/client"
	"gameServer/service/rpc/client/selector"
	"sync"

	xclient "github.com/smallnest/rpcx/client"
)

// RPCClient 远程调用客户端
var RpcNodeClient rpc.ClientInterface

// SetRPCClient 设置 rpc 调用，用于测试
func SetNodeRPCClient(c rpc.ClientInterface) {
	RpcNodeClient = c
}

// 获取远程调用客户端
var RPCNodeClients = sync.OnceValue(func() rpc.ClientInterface {
	if RpcNodeClient != nil {
		return RpcNodeClient
	}

	// gate 集群
	c, err := client.NewClient(client.BuildClientConfig(
		"gate",
		"Gate",
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
