package main

import (
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log1"
	"gameServer/pkg/logger/log2"
	"gameServer/service/services/gate"
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>
func main() {
	// 打docker的时候需要填写路径
	log1.Init(zapcore.DebugLevel, "gate-1", "./logs", !config.Get().IsDevelop())
	log2.Init(log2.Config{Level: zapcore.DebugLevel, LogDir: "./logs", IsDocker: false})

	//log2.Get().Debug("etcd Register==", zap.String("address=", "123123"))
	//log2.Get().Info("etcd Register==", zap.String("address=", "44444"))
	//log2.Get().Warn("etcd Register==", zap.String("address=", "5555"))
	//log2.Get().Error("etcd Register==", zap.String("address=", "6666"))

	// [E:\gowork\gameServer]
	if err := config.Init("gate-1", "gate", "./config", "test", "toml"); err != nil {
		panic(err)
	}
	// pprof
	go func() {
		pprofAddr := config.Get().PProfAddress()
		if len(pprofAddr) == 0 {
			return
		}
		log1.Get().Info("pprof listen", zap.String("address", pprofAddr))

		server := &http.Server{
			Addr:        pprofAddr,
			ReadTimeout: 3 * time.Second,
		}

		if err2 := server.ListenAndServe(); err2 != nil {
			log1.Get().Error("pprof listen error", zap.String("address", pprofAddr), zap.Error(err2))
		}
	}()

	// 初始化配置
	service, err := gate.NewServer()
	if err != nil {
		panic(err)
	}

	//f := rpc.NewForward()
	//if err = f.AddModules([]interface {
	//}{
	//	new(.Account),
	//}); err != nil {
	//	return err
	//}

	// 启动服务
	if err = service.Start(); err != nil {
		panic(err)
	}
	log1.Get().Info("server stop ！")

}
