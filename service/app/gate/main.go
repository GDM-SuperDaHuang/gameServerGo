package gate

import (
	"Server/service/services"
	"Server/service/services/gate"
	"fmt"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>

func main() {
	//TIP <p>Press <shortcut actionId="ShowIntentionActions"/> when your caret is at the underlined text
	// to see how GoLand suggests fixing the warning.</p><p>Alternatively, if available, click the lightbulb to view possible fixes.</p>
	s := "gopher"
	fmt.Printf("Hello and welcome, %s!\n", s)
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
	// pprof
	//go func() {
	//	pprofAddr := config.Get().PProfAddress()
	//	if len(pprofAddr) == 0 {
	//		return
	//	}
	//	logger.Get().Info("pprof listen", zap.String("address", pprofAddr))
	//
	//	server := &http.Server{
	//		Addr:        pprofAddr,
	//		ReadTimeout: 3 * time.Second,
	//	}
	//
	//	if err2 := server.ListenAndServe(); err2 != nil {
	//		logger.Get().Error("pprof listen error", zap.String("address", pprofAddr), zap.Error(err2))
	//	}
	//}()

	// 添加服务
	// 启动gnet服务
	service, err := gate.NewServer()
	if err != nil {
		panic(err)
	}
	//启动 rpcx

	// 注册etcd

	// 似乎没有什么用
	//if err := engine.Add(service); err != nil {
	//	panic(err)
	//}

	// 运行
	if err := service.Start(); err != nil {
		panic(err)
	}
}
