package main

import (
	"gameServer/app/room/hander"
	"gameServer/app/room/hander/room"
	"gameServer/common/db/heros"
	"gameServer/common/db/items"
	"gameServer/pkg/cache/ssdb"
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/redis"
	"gameServer/service/rpc"
	"gameServer/service/services/node"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "gameServer/app/room/hander/inits"

	"github.com/seefan/gossdb/v2/conf"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu constValue from here.</p>

func main() {

	configPath := pflag.String("configPath", filepath.Join(filepath.Dir(os.Args[0]), "config"), "配置文件路径") // ./config/
	configName := pflag.String("configName", "room", "配置文件名称，不带后缀")                                       // gate.toml
	logPath := pflag.String("logPath", filepath.Join(filepath.Dir(os.Args[0]), "logs"), "日志路径")           // ./logs/
	//logLevel := pflag.String("logLevel", "debug", "日志等级 debug info warn error fatal")
	nodeName := pflag.String("node", "room-1", "节点名称")
	pflag.Parse()

	log2.Init(log2.Config{Level: zapcore.DebugLevel, LogDir: *logPath, IsDocker: false})

	if err := config.Init(*nodeName, "room", *configPath, *configName, "toml"); err != nil {
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

	// 获取数据源
	cfg := &conf.Config{
		Host:        config.Get().GameSSDBHost(),
		Port:        config.Get().GameSSDBPort(),
		MinPoolSize: 100,
		MaxPoolSize: config.Get().GameSSDBMaxPoolSize(),
		Encoding:    true, // 支持非基本数据类型
		AutoClose:   true,
		// 更多配置可参考pkg文档
	}
	err := ssdb.Init(cfg)
	if err != nil {
		panic(err)
	}
	defer ssdb.Close()

	// 初始化redis
	redis.NewRedisClient(config.Get().RedisAddress())
	items.Listening()
	heros.Listening()

	// 注册处理器
	f := rpc.NewForward()
	if err = f.AddModules([]interface {
	}{
		new(hander.HanderTest),
		new(room.HandlerRoom),
	}); err != nil {
		panic(err)
	}
	// 添加rpcx 服务
	nodeServer := node.NewServer()
	err = nodeServer.Start(f)
	if err != nil {
		panic(err)
	}

}
