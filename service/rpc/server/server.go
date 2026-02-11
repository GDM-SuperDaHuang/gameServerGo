package server

import (
	"sync/atomic"
	"time"

	"github.com/rpcxio/rpcx-etcd/serverplugin"
	rpcxServer "github.com/smallnest/rpcx/server"
)

// 节点信息配置
type RpcConfig struct {
	// 服务器编号，唯一值
	ID uint32
	// 服务器名称，例如 gate-1, game-1, game-2
	Name string
	// 服务器版本
	Version uint32

	// 向 etcd 更新信息间隔
	UpdateInterval time.Duration
	// etcd 服务器地址列表
	EtcdEndpoints []string

	// etcd 服务注册基础路径，相当于缓存前缀，避免不同项目使用同一个 etcd 造成混乱，例如 MODOU_LDL
	BasePath string
	// 服务名称，例如 gate, game, battle。例如组成 MODOU_LDL/gate
	ServiceName string
}

// Server rpcx 服务端 接口实现
type Server struct {
	// 配置信息
	config *RpcConfig
	// rpcx 服务器
	server *rpcxServer.Server
	// etcd 服务注册插件
	registry *serverplugin.EtcdV3RegisterPlugin
	// 添加关闭标志
	closed atomic.Bool
}
