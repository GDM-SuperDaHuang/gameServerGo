package services

import (
	"Server/service/engine"

	"go.uber.org/zap"
)

// 节点服务器
type nodeServer struct {
	engine  *engine.Engine
	id      uint32
	name    string
	version uint32

	rpcServer pkgrpc.Server
}

// ID 返回服务唯一 ID
func (a *nodeServer) ID() uint32 {
	return a.id
}

// Name 返回服务名称
func (a *nodeServer) Name() string {
	return a.name
}

// Version 返回服务版本号
func (a *nodeServer) Version() uint32 {
	return a.version
}

// Init 初始化服务
func (a *nodeServer) Init() error {
	if err := database.InitFromConfig(); err != nil {
		return err
	}

	if err := cache.InitFromConfig(); err != nil {
		return err
	}

	if err := entity.Init(); err != nil {
		return err
	}

	if err := a.initRPC(); err != nil {
		return err
	}

	logger.Get().Info("[account] initialized")
	return nil
}

// Start 启动服务
func (a *nodeServer) Start() error {
	// 启动定时器
	crontab.Start()
	logger.Get().Info("[account] started")
	return nil
}

// Close 关闭服务
func (a *nodeServer) Close() error {
	if err := a.rpcServer.Stop(); err != nil {
		logger.Get().Error("[account] close failed", zap.Error(err))
		return err
	}

	logger.Get().Info("[account] closed")
	return nil
}
