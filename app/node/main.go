package main

import (
	"gameServer/app/node/hander"
	"gameServer/pkg/config"
	"gameServer/pkg/logger"
	"gameServer/service/rpc"
	"gameServer/service/services/node"
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>

func main() {

	// room-1 配置文件标识 [room-1]
	// room [room]
	// ./config 路径
	// room.tom文件

	if err := config.Init("room-1", "room", "./config", "room", "toml"); err != nil {
		panic(err)
	}
	//if err := config.Init("room-1", "room", "./config", "room", "toml"); err != nil {
	//	panic(err)
	//}
	logger.Init(zapcore.DebugLevel, "", "", !config.Get().IsDevelop())

	// pprof
	go func() {
		pprofAddr := config.Get().PProfAddress()
		if len(pprofAddr) == 0 {
			return
		}
		logger.Get().Info("pprof listen", zap.String("address", pprofAddr))

		server := &http.Server{
			Addr:        pprofAddr,
			ReadTimeout: 3 * time.Second,
		}

		if err2 := server.ListenAndServe(); err2 != nil {
			logger.Get().Error("pprof listen error", zap.String("address", pprofAddr), zap.Error(err2))
		}
	}()

	nodeServer, err := node.NewServer()
	if err != nil {
		panic(err)
	}

	// 注册处理器
	f := rpc.NewForward()
	if err = f.AddModules([]interface {
	}{
		new(hander.HanderTest),
	}); err != nil {
		panic(err)
	}
	// 添加rpcx 服务
	err = nodeServer.Start(f)
	if err != nil {
		panic(err)
	}
}
