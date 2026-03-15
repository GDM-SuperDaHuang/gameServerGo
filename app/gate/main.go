package main

import (
	"gameServer/common/db/heros"
	"gameServer/common/db/items"
	"gameServer/pkg/cache/ssdb"
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/redis"
	"gameServer/service/services/gate"
	"net/http"
	"time"

	"github.com/seefan/gossdb/v2/conf"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu constValue from here.</p>
func main() {

	// os.Args[0] 程序本身的启动路径
	//| 启动方式                   | `os.Args[0]` 的值                            |
	//| :--------------------- | :----------------------------------------- |
	//| `./myapp`              | `./myapp`                                  |
	//| `/home/user/app/myapp` | `/home/user/app/myapp`                     |
	//| `go run main.go`       | 临时编译路径（如 `/tmp/go-build123/b001/exe/main`） |

	//configPath := pflag.String("configPath", filepath.Join(filepath.Dir(os.Args[0]), "config"), "配置文件路径") // ./config/
	////configName := pflag.String("configName", "gate", "配置文件名称，不带后缀")                                       // gate.toml
	//logPath := pflag.String("logPath", filepath.Join(filepath.Dir(os.Args[0]), "logs"), "日志路径")           // ./logs/
	////logLevel := pflag.String("logLevel", "debug", "日志等级 debug info warn error fatal")
	//nodeName := pflag.String("node", "gate-1", "节点名称")
	//pflag.Parse()

	configPath := "./config/" // ./config/
	logPath := "./logs/"      // ./logs/
	nodeName := "gate-1"

	// 打docker的时候需要填写路径
	//log1.Init(zapcore.DebugLevel, *nodeName, *logPath, !config.Get().IsDevelop())
	log2.Init(log2.Config{Level: zapcore.DebugLevel, LogDir: logPath, IsDocker: false})
	if err := config.Init(nodeName, "gate", configPath, "gate", "toml"); err != nil {
		panic(err)
	}

	// pprof
	go func() {
		pprofAddr := config.Get().PProfAddress()
		if len(pprofAddr) == 0 {
			return
		}
		log2.Get().Info("pprof listen", zap.String("address", pprofAddr))

		server := &http.Server{
			Addr:        pprofAddr,
			ReadTimeout: 3 * time.Second,
		}

		if err2 := server.ListenAndServe(); err2 != nil {
			log2.Get().Error("pprof listen error", zap.String("address", pprofAddr), zap.Error(err2))
		}
	}()

	// 初始化配置
	service, err := gate.NewServer()
	if err != nil {
		panic(err)
	}

	// 初始化redis
	redis.NewRedisClient(config.Get().RedisAddress())
	items.Listening()
	heros.Listening()
	// 获取数据源
	cfg := &conf.Config{
		Host:        config.Get().GameSSDBHost(),
		Port:        config.Get().GameSSDBPort(),
		MinPoolSize: 100,
		MaxPoolSize: config.Get().GameSSDBMaxPoolSize(),
		Encoding:    true, // 支持非基本数据类型
		AutoClose:   true,
		Password:    config.Get().GameSSDBPassword(),
		// 更多配置可参考pkg文档
	}
	err = ssdb.Init(cfg)
	if err != nil {
		panic(err)
	}
	defer ssdb.Close()

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
	log2.Get().Info("server stop ！")

}
