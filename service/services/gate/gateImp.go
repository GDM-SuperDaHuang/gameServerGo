package gate

import (
	"gameServer/pkg/compress"
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log1"
	"gameServer/service/rpc"
	"gameServer/service/services"
	"gameServer/service/services/gate/datapack"
	"os"
	"os/signal"
	"syscall"

	rpcxServer "gameServer/service/rpc/server"

	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

// gate 实现接口
type Gate struct {
	//engine  *engine.Engine
	id      uint32
	name    string
	version uint32

	//loginJWT jwt.JWT

	tcpServer *gNetServer         // 返回给用户
	rpcServer rpc.ServerInterface //rpc服务器

}

// New 创建一个网格服务
func NewServer() (services.ServiceInterface, error) {
	c := config.Get()

	g := &Gate{
		id:      c.NodeID(),
		name:    c.NodeName(),
		version: c.NodeVersion(),
	}
	//if err := g.parse(); err != nil {
	//	return nil, err
	//}

	log1.Get().Info("[gate] create success", zap.Uint32("id", g.id), zap.String("node", g.name), zap.Uint32("version", g.version))
	return g, nil
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

// Init 进行初始化，当尚未启动对外服务
//func (g *Gate) Init() error {
//	// 1. jwt 验证
//	return g.initJWT()
//}

func (g *Gate) Start() error {
	// 启动网络监听
	if err := g.gNetStart(); err != nil {
		return err
	}

	// rpcx
	if err := g.initRPC(); err != nil {
		return err
	}

	log1.Get().Info("[gate] service started")
	g.listenSignal()
	return nil
}

func (g *Gate) Close() error {
	// 清理 JWT 资源
	//g.loginJWT = nil
	if err := g.rpcServer.Stop(); err != nil {
		log1.Get().Error("[account] close failed", zap.Error(err))
		return err
	}
	log1.Get().Info("[gate] service closed")
	return nil
}

// -------------------------------------- 内部 --------------------------------------
// listenSignal 监听信号
func (e *Gate) listenSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	sig := <-ch
	signal.Stop(ch)

	log1.Get().Sugar().Infof("received signal: %+v", sig)

	switch sig {
	case syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT:
		log1.Get().Info("close signal .. shufhown server ..")
		e.close()
	default:
		log1.Get().Sugar().Errorf("unsupport signal: %s", sig.String())
	}
}

// close 关闭服务
func (g *Gate) close() {
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
		log1.Get().Sugar().Errorf("service: %s(%d) close failed: %s", g.Name(), g.ID(), err.Error())
	} else {
		log1.Get().Sugar().Infof("service: %s(%d) closed successfully", g.Name(), g.ID())
	}
	log1.Get().Info("all services closed")
}

//func (g *Gate) parse() error {
//	c := config.Get()
//
//	if err := c.CheckService(
//		"whethercompress",
//		"compressthreshold",
//		"whethercrypto",
//		"whetherchecksum",
//		"poolsize",
//	); err != nil {
//		return err
//	}
//
//	return c.CheckNode("id", "version", "address")
//}

//func (g *Gate) initJWT() error {
//	c := config.Get()
//
//	g.loginJWT = jwt.New(&jwt.Option{
//		Secret: []byte(c.Secret()),
//	})
//	return nil
//}

func (g *Gate) gNetStart() error {
	c := config.Get()

	var (
		address = c.Node().GetString("address")
		//whetherCompress   = c.Service().GetBool("whethercompress")
		//compressThreshold = c.Service().GetInt("compressthreshold")
		//whetherCrypto     = c.Service().GetBool("whethercrypto")
		//whetherChecksum   = c.Service().GetBool("whetherchecksum")
		//poolSize          = c.Service().GetInt("poolsize")

		//address = "127.0.0.1"
		//whetherCompress   = c.Service().GetBool("whethercompress")
		//compressThreshold = c.Service().GetInt("compressthreshold")
		//whetherCrypto     = c.Service().GetBool("whethercrypto")
		//whetherChecksum   = c.Service().GetBool("whetherchecksum")
		//poolSize = c.Service().GetInt("poolsize")
		poolSize = 10000
	)

	s := &gNetServer{
		//engine:  g.engine,
		gate:     g,
		address:  address,
		datapack: datapack.NewLTD(compress.NewZlib(), log1.Get()),
		isTest:   c.IsDevelop(),
	}

	if err := s.InitPool(poolSize); err != nil {
		return err
	}

	g.tcpServer = s

	go func() {
		err := gnet.Run(
			s,
			"tcp://"+s.address,
			gnet.WithLogger(log1.Get().Sugar()),
			gnet.WithMulticore(true),
			gnet.WithReusePort(true),
		)
		if err != nil {
			log1.Get().Fatal("[gate] tcp server failed to start", zap.Error(err))
		}
	}()

	return nil
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

	log1.Get().Info("[initRPC] server start", zap.String("info", s.Output()))

	g.rpcServer = s

	return nil
}
