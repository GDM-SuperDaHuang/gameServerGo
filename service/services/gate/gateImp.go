package gate

import (
	"Server/service/compress"
	"Server/service/config"
	"Server/service/logger"
	"Server/service/rpc"
	"Server/service/services"
	"Server/service/services/gate/datapack"

	rpcxServer "Server/service/rpc/server"
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

	tcpServer *gNetServer
	rpcServer rpc.ServerInterface
}

// New 创建一个网格服务
func NewServer() (services.ServiceInterface, error) {
	c := config.Get()

	g := &Gate{
		id:      c.NodeID(),
		name:    c.NodeName(),
		version: c.NodeVersion(),
	}
	if err := g.parse(); err != nil {
		return nil, err
	}

	logger.Get().Info("[gate] create success", zap.Uint32("id", g.id), zap.String("node", g.name), zap.Uint32("version", g.version))
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
func (g *Gate) Init() error {
	// 1. jwt 验证
	return g.initJWT()
}

func (g *Gate) Start() error {
	// 启动网络监听
	if err := g.gNetStart(); err != nil {
		return err
	}

	if err := g.initRPC(); err != nil {
		return err
	}

	logger.Get().Info("[gate] service started")

	return nil
}

func (g *Gate) Close() error {
	// 清理 JWT 资源
	//g.loginJWT = nil

	logger.Get().Info("[gate] service closed")
	return nil
}

// -------------------------------------- 内部 --------------------------------------

func (g *Gate) parse() error {
	c := config.Get()

	if err := c.CheckService(
		"whethercompress",
		"compressthreshold",
		"whethercrypto",
		"whetherchecksum",
		"poolsize",
	); err != nil {
		return err
	}

	return c.CheckNode("id", "version", "address")
}

func (g *Gate) initJWT() error {
	c := config.Get()

	g.loginJWT = jwt.New(&jwt.Option{
		Secret: []byte(c.Secret()),
	})
	return nil
}

func (g *Gate) gNetStart() error {
	c := config.Get()

	var (
		//address           = c.Node().GetString("address")
		//whetherCompress   = c.Service().GetBool("whethercompress")
		//compressThreshold = c.Service().GetInt("compressthreshold")
		//whetherCrypto     = c.Service().GetBool("whethercrypto")
		//whetherChecksum   = c.Service().GetBool("whetherchecksum")
		//poolSize          = c.Service().GetInt("poolsize")

		address = "127.0.0.1"
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
		datapack: datapack.NewLTD(compress.NewZlib(), logger.Get()),
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
	s, err := rpcxServer.NewServer(rpcxServer.BuildServerConfig())
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
