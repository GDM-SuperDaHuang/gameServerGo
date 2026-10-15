package node

import (
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log2"
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
func NewServer() *NodeServer {
	c := config.Get()

	g := &NodeServer{
		id:      c.NodeID(),
		name:    c.NodeName(),
		version: c.NodeVersion(),
	}

	log2.Get().Info("[gate] create success", zap.Uint32("id", g.id), zap.String("node", g.name), zap.Uint32("version", g.version))
	return g
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

// Start 启动服务
func (n *NodeServer) Start(f *rpc.Forward) {
	// 启动定时器
	//crontab.Start()
	err := n.initRPC(f)
	if err != nil {
		panic(err)
		return
	}
	log2.Get().Info("[account] started")
	n.listenSignal()
}

// Close 关闭服务
func (a *NodeServer) Close() error {
	if err := a.rpcServer.Stop(); err != nil {
		log2.Get().Error("[account] close failed", zap.Error(err))
		return err
	}

	log2.Get().Info("[account] closed")
	return nil
}

// listenSignal 监听信号
func (e *NodeServer) listenSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	sig := <-ch
	signal.Stop(ch)

	log2.Get().Sugar().Infof("received signal: %+v", sig)

	switch sig {
	case syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT:
		log2.Get().Info("close signal .. shufhown server ..")
		e.close()
	default:
		log2.Get().Sugar().Errorf("unsupport signal: %s", sig.String())
	}
}

// close 关闭服务
func (g *NodeServer) close() {
	if err := g.Close(); err != nil {
		log2.Get().Sugar().Errorf("service: %s(%d) close failed: %s", g.Name(), g.ID(), err.Error())
	} else {
		log2.Get().Sugar().Infof("service: %s(%d) closed successfully", g.Name(), g.ID())
	}
	log2.Get().Info("all services closed")
}
