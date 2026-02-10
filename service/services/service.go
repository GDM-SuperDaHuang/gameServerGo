package services

import (
	"Server/service/config"
	"Server/service/gate"
)

// Service 对外服务
type Service interface {
	// ID 服务唯一 id
	ID() uint32
	// Name 服务名称
	Name() string
	// Version 服务版本号
	Version() uint32

	//// Init 进行初始化，当尚未启动对外服务
	//Init() error

	// Start 启动服务，可以接入新数据
	Start() error
	// Close 关闭服务
	Close() error
}

// New 创建一个网格服务
func NewGate() (Service, error) {
	c := config.Get()

	g := &gate.Gate{
		id:      c.NodeID(),
		name:    c.NodeName(),
		version: c.NodeVersion(),
	}
	//if err := g.parse(); err != nil {
	//	return nil, err
	//}

	//logger.Get().Info("[gate] create success", zap.Uint32("id", g.id), zap.String("node", g.name), zap.Uint32("version", g.version))
	return g, nil
}

// New 创建一个节点服务
func NewNode() (Service, error) {
	c := config.Get()
	a := &nodeServer{
		//engine:  engine,
		id:      c.NodeID(),
		name:    c.NodeName(),
		version: c.NodeVersion(),
	}

	//logger.Get().Info("[account] create success", zap.Uint32("id", a.id), zap.String("node", a.name), zap.Uint32("version", a.version))
	return a, nil
}
