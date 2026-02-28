package main

import (
	"fmt"
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log1"
	"gameServer/pkg/logger/log2"
	"gameServer/service/services/gate"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>
func main() {

	// os.Args[0] 程序本身的启动路径
	//| 启动方式                   | `os.Args[0]` 的值                            |
	//| :--------------------- | :----------------------------------------- |
	//| `./myapp`              | `./myapp`                                  |
	//| `/home/user/app/myapp` | `/home/user/app/myapp`                     |
	//| `go run main.go`       | 临时编译路径（如 `/tmp/go-build123/b001/exe/main`） |

	configPath := pflag.String("configPath", filepath.Join(filepath.Dir(os.Args[0]), "config"), "配置文件路径") // ./config/
	configName := pflag.String("configName", "gate", "配置文件名称，不带后缀")                                       // gate.toml
	logPath := pflag.String("logPath", filepath.Join(filepath.Dir(os.Args[0]), "logs"), "日志路径")           // ./logs/
	//logLevel := pflag.String("logLevel", "debug", "日志等级 debug info warn error fatal")
	nodeName := pflag.String("node", "gate-1", "节点名称")
	pflag.Parse()

	fmt.Println("configPath:", *configPath)
	// 打docker的时候需要填写路径
	log1.Init(zapcore.DebugLevel, *nodeName, *logPath, !config.Get().IsDevelop())
	log2.Init(log2.Config{Level: zapcore.DebugLevel, LogDir: *logPath, IsDocker: false})

	//log2.Get().Debug("etcd Register==", zap.String("address=", "123123"))
	//log2.Get().Info("etcd Register==", zap.String("address=", "44444"))
	//log2.Get().Warn("etcd Register==", zap.String("address=", "5555"))
	//log2.Get().Error("etcd Register==", zap.String("address=", "6666"))

	// [E:\gowork\gameServer]
	if err := config.Init(*nodeName, "gate", *configPath, *configName, "toml"); err != nil {
		panic(err)
	}
	//if err := config.Init("gate-1", "gate", "./config", "gate", "toml"); err != nil {
	//	panic(err)
	//}
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
