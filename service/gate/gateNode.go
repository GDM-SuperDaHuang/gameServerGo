package gate

import (
	"Server/service/config"
	"Server/service/datapack"
	"compress/zlib"

	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

// gate 网关服务
type Gate struct {
	//engine  *engine.Engine
	id      uint32
	name    string
	version uint32

	loginJWT jwt.JWT

	tcpServer *tcpServer
	rpcServer pkgrpc.Server
}

// ID 返回服务唯一 ID
func (a *Gate) ID() uint32 {
	return a.id
}

// Name 返回服务名称
func (a *Gate) Name() string {
	return a.name
}

// Version 返回服务版本号
func (a *Gate) Version() uint32 {
	return a.version
}

// Init 初始化服务
//func (a *Gate) Init() error {
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
func (g *Gate) Start() error {
	// 启动网络监听
	if err := g.gnetStart(); err != nil {
		return err
	}
	if err := g.initRPC(); err != nil {
		return err
	}
	//logger.Get().Info("[gate] service started")
	return nil
}

// Close 关闭服务
func (a *Gate) Close() error {
	if err := a.rpcServer.Stop(); err != nil {
		logger.Get().Error("[account] close failed", zap.Error(err))
		return err
	}

	logger.Get().Info("[account] closed")
	return nil
}

func (g *Gate) gnetStart() error {
	c := config.Get()

	var (
		//address           = c.Node().GetString("address")
		//whetherCompress   = c.Service().GetBool("whethercompress")
		//compressThreshold = c.Service().GetInt("compressthreshold")
		//whetherCrypto     = c.Service().GetBool("whethercrypto")
		//whetherChecksum   = c.Service().GetBool("whetherchecksum")
		//poolSize          = c.Service().GetInt("poolsize")

		address           = "127.0.0.1"
		whetherCompress   = c.Service().GetBool("whethercompress")
		compressThreshold = c.Service().GetInt("compressthreshold")
		whetherCrypto     = c.Service().GetBool("whethercrypto")
		whetherChecksum   = c.Service().GetBool("whetherchecksum")
		poolSize          = c.Service().GetInt("poolsize")
	)

	s := &tcpServer{
		//engine:  g.engine,
		gate:    g,
		address: address,
		datapack: datapack.NewLTD(
			whetherCompress,
			compressThreshold,
			zlib.NewZlib(),
			whetherCrypto,
			whetherChecksum,
			logger.Get(),
		),
		isTest: c.IsDevelop(),
	}

	if err := s.InitPool(poolSize); err != nil {
		return err
	}

	g.tcpServer = s

	go func() {
		err := gnet.Run(
			s,
			"tcp://"+s.address,
			gnet.WithLogger(logger.Get().Sugar()),
			gnet.WithMulticore(true),
			gnet.WithReusePort(true),
		)
		if err != nil {
			//logger.Get().Fatal("[gate] tcp server failed to start", zap.Error(err))
		}
	}()

	return nil
}

func (g *Gate) initRPC() error {
	s, err := pkgrpc.NewServer(internalrpc.BuildServerConfig())
	if err != nil {
		return err
	}
	if err := s.Start(); err != nil {
		return err
	}

	if err := s.Register(g.newPush()); err != nil {
		return err
	}

	logger.Get().Info("[initRPC] server start", zap.String("info", s.Output()))

	g.rpcServer = s

	return nil
}
