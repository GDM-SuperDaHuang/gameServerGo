package server

import (
	"gameServer/service/config"
	"time"
)

// 服务端rpcx 配置
type ServerConfig struct {
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

// BuildServerConfig 从服务配置表中创建
func BuildServerConfig() *ServerConfig {
	c := config.Get()

	return &ServerConfig{
		ID:             c.NodeID(),
		Name:           c.NodeName(),
		Version:        c.NodeVersion(),
		UpdateInterval: 10 * time.Second,
		EtcdEndpoints:  c.EtcdAddress(),
		BasePath:       c.EtcdPrefix(),
		ServiceName:    c.ServiceName(),
	}
}
