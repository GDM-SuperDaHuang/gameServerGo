package node

import (
	"gameServer/pkg/config"
	"gameServer/pkg/logger"
	"gameServer/service/rpc"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

// 节点服务器 接口实现
type NodeServer struct {
	//engine  *engine.Engine
	id      uint32
	name    string
	version uint32

	rpcServer rpc.ServerInterface
}

// New 创建一个网格服务
func NewServer() (*NodeServer, error) {
	c := config.Get()

	g := &NodeServer{
		id:      c.NodeID(),
		name:    c.NodeName(),
		version: c.NodeVersion(),
	}
	//if err := g.parse(); err != nil {
	//	return nil, err
	//}

	logger.Get().Info("[gate] create success", zap.Uint32("id", g.id), zap.String("node", g.name), zap.Uint32("version", g.version))
	return g, nil
}

// ID 返回服务唯一 ID
func (a *NodeServer) ID() uint32 {
	return a.id
}

// Name 返回服务名称
func (a *NodeServer) Name() string {
	return a.name
}

// Version 返回服务版本号
func (a *NodeServer) Version() uint32 {
	return a.version
}

//// Init 初始化服务
//func (a *nodeServer) Init() error {
//	if err := database.InitFromConfig(); err != nil {
//		return err
//	}
//
//	if err := cache.InitFromConfig(); err != nil {
//		return err
//	}
//
//	if err := entity.Init(); err != nil {
//		return err
//	}
//
//	if err := a.initRPC(); err != nil {
//		return err
//	}
//
//	logger.Get().Info("[account] initialized")
//	return nil
//}

// Start 启动服务
func (n *NodeServer) Start(f *rpc.Forward) error {
	// 启动定时器
	//crontab.Start()
	err := n.initRPC(f)
	if err != nil {
		return err
	}
	logger.Get().Info("[account] started")
	n.listenSignal()
	return nil
}

// Close 关闭服务
func (a *NodeServer) Close() error {
	if err := a.rpcServer.Stop(); err != nil {
		logger.Get().Error("[account] close failed", zap.Error(err))
		return err
	}

	logger.Get().Info("[account] closed")
	return nil
}

// listenSignal 监听信号
func (e *NodeServer) listenSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	sig := <-ch
	signal.Stop(ch)

	logger.Get().Sugar().Infof("received signal: %+v", sig)

	switch sig {
	case syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT:
		logger.Get().Info("close signal .. shufhown server ..")
		e.close()
	default:
		logger.Get().Sugar().Errorf("unsupport signal: %s", sig.String())
	}
}

// close 关闭服务
func (g *NodeServer) close() {
	//e.closeOnce.Do(func() {
	//	for _, s := range e.services {
	//		if err := s.Close(); err != nil {
	//			logger.Get().Sugar().Errorf("service: %s(%d) close failed: %s", s.Name(), s.ID(), err.Error())
	//		} else {
	//			logger.Get().Sugar().Infof("service: %s(%d) closed successfully", s.Name(), s.ID())
	//		}
	//	}
	//	logger.Get().Info("all services closed")
	//})
	if err := g.Close(); err != nil {
		logger.Get().Sugar().Errorf("service: %s(%d) close failed: %s", g.Name(), g.ID(), err.Error())
	} else {
		logger.Get().Sugar().Infof("service: %s(%d) closed successfully", g.Name(), g.ID())
	}
	logger.Get().Info("all services closed")
}
