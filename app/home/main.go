package main

import (
	"gameServer/app/home/hander"
	"gameServer/app/home/hander/inits"
	_ "gameServer/app/home/hander/inits"
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/recoverSafe"
	"gameServer/pkg/redis"
	"gameServer/service/node"
	"gameServer/service/rpc"
	"net/http"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Version = ""

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu constValue from here.</p>

func main() {
	configPath := "./config/" // ./config/
	logPath := "./logs/"      // ./logs/
	// NODE_NAME 优先(修复原来写死 home-1, swarm 下同一镜像可起 home-1/home-2), 默认 home-1
	nodeName := strings.TrimSpace(os.Getenv("NODE_NAME"))
	if nodeName == "" {
		nodeName = "home-1"
	}
	defer recoverSafe.RecoverSafe(nodeName)
	log2.Init(log2.Config{Level: zapcore.DebugLevel, LogDir: logPath, IsDocker: false})
	if err := config.Init(nodeName, "home", configPath, "home", "toml"); err != nil {
		panic(err)
	}
	log2.Get().Info("Run Starting!!!", zap.String("Main==", nodeName))

	// pprof
	go func() {
		_ = http.ListenAndServe(config.Get().PProfAddress(), nil)
	}()

	// 初始化redis
	redis.NewRedisClient(config.Get().RedisAddress())
	// 初始化数据层（本节点未初始化 Mongo：写入进入落库队列，由持有 Mongo 的节点批量落库）

	// 初始化
	inits.Init()

	// 获取数据源

	// 注册处理器
	f := rpc.NewForward()
	if err := f.AddModules([]interface {
	}{new(hander.HomeHandler)}); err != nil {
		panic(err)
	}

	// 添加rpcx 服务
	nodeServer := node.NewServer()
	nodeServer.Start(f)

}
