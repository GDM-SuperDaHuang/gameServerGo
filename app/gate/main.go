package main

import (
	"gameServer/pkg/config"
	"gameServer/pkg/logger"
	"gameServer/service/services/gate"
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>
func main() {
	//TIP <p>Press <shortcut actionId="ShowIntentionActions"/> when your caret is at the underlined text
	// to see how GoLand suggests fixing the warning.</p><p>Alternatively, if available, click the lightbulb to view possible fixes.</p>
	// 创建应用,读取配置启动
	//engine, err := services.New(&services.Options{
	//	NodeName:    "gate-1",
	//	ServiceName: "gate",
	//	ConfigPath:  "",
	//	ConfigName:  "",
	//	ConfigType:  "",
	//	LogPath:     "",
	//	LogName:     "",
	//	LogLevel:    "",
	//})

	//if err != nil {
	//	panic(err)
	//}
	logger.Init(zapcore.DebugLevel, "", "", !config.Get().IsDevelop())
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
		logger.Get().Info("pprof listen", zap.String("address", pprofAddr))

		server := &http.Server{
			Addr:        pprofAddr,
			ReadTimeout: 3 * time.Second,
		}

		if err2 := server.ListenAndServe(); err2 != nil {
			logger.Get().Error("pprof listen error", zap.String("address", pprofAddr), zap.Error(err2))
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
	logger.Get().Info("server stop ！")

}
