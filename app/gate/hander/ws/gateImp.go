package ws

import (
	"context"
	"fmt"
	services "gameServer/app/gate/hander"
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log2"
	"gameServer/service/common"
	"gameServer/service/rpc"
	"os"
	"os/signal"
	"syscall"

	rpcxServer "gameServer/service/rpc/server"

	"go.uber.org/zap"
)

// gate 实现接口
type Gate struct {
	//engine  *engine.Engine
	id      uint32
	name    string
	version uint32

	Ws *WsServer

	rpcServer rpc.ServerInterface //rpc服务器
}

// New 创建一个网格服务
func NewServer() services.ServiceInterface {
	c := config.Get()
	g := &Gate{
		id:      c.NodeID(),
		name:    c.NodeName(),
		version: c.NodeVersion(),
	}
	return g
}

func (g *Gate) ID() uint32 {
	return g.id
}

func (g *Gate) Name() string {
	return g.name
}

func (g *Gate) Version() uint32 {
	return g.version
}

func (g *Gate) Start() error {
	g.Ws = &WsServer{}
	// 启动网络监听
	if err := g.Ws.StartWs(); err != nil {
		return err
	}

	// rpcx
	if err := g.initRPC(); err != nil {
		return err
	}

	log2.Get().Info("[gate] service started")
	g.listenSignal()
	return nil
}

func (g *Gate) Close() error {
	// 清理 JWT 资源
	//g.loginJWT = nil
	if err := g.rpcServer.Stop(); err != nil {
		log2.Get().Error("[account] close failed", zap.Error(err))
		return err
	}
	log2.Get().Info("[gate] service closed")
	return nil
}

// -------------------------------------- 内部 --------------------------------------
// listenSignal 监听信号
func (e *Gate) listenSignal() {
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
func (g *Gate) close() {
	if err := g.Close(); err != nil {
		log2.Get().Sugar().Errorf("service: %s(%d) close failed: %s", g.Name(), g.ID(), err.Error())
	} else {
		log2.Get().Sugar().Infof("service: %s(%d) closed successfully", g.Name(), g.ID())
	}
	log2.Get().Info("all services closed")
}

func (g *Gate) initRPC() error {
	// 客户端懒加载
	SetGateRPCClient(GateRPCClients())

	s, err := rpcxServer.NewServer(rpcxServer.BuildServerConfig())
	if err != nil {
		return err
	}
	if err = s.Start(); err != nil {
		return err
	}

	// ectd 注册信息
	if err = s.Register(g); err != nil {
		return err
	}

	log2.Get().Info("[initRPC] server start", zap.String("info", s.Output()))

	g.rpcServer = s

	return nil
}

// Receive 网关接收其它服务的单个消息推送,必须实现，不然无法注册，rpcx协程
func (g *Gate) Receive(_ context.Context, req *common.RpcMessage, resp *common.Resp) error {
	//找到对应的 session，写入消息
	session := g.Ws.FindSession(req.Player.Uid)
	if session == nil {
		log2.Get().Warn("[Receive] session not found", zap.Int64("roleID", req.Player.Uid))
		return fmt.Errorf("session not found, roleID: %d", req.Player.Uid)
	}
	resp.Body = req.Data.Body
	status := session.Send(encodeMessage(common.NewMessageResp(resp, req.Data), session))
	if CloseStatus == status {
		if session.Player != nil && session.Player.Uid != 0 {
			g.Ws.DelSession(session.Player.Uid)
		}
	} else if status == ErrorStatus {
		log2.Get().Warn("[Receive] Send false", zap.Int64("roleID", req.Player.Uid))
		return fmt.Errorf("[Receive] Send false, roleID: %d", req.Player.Uid)
	}

	return nil
}
