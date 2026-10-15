package main

import (
	"fmt"
	"gameServer/app/gate/hander/ws"
	"gameServer/app/gate/inits"
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/mongodb"
	"gameServer/pkg/redis"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Version = "2"

func main() {

	configPath := "./config/" // ./config/
	logPath := "./logs/"      // ./logs/
	// NODE_NAME 优先(方便 swarm 同一镜像起 gate-1/gate-2), 否则按编译期 Version 推导
	nodeName := strings.TrimSpace(os.Getenv("NODE_NAME"))
	if nodeName == "" {
		nodeName = fmt.Sprintf("gate-%s", Version)
	}

	log2.Init(log2.Config{Level: zapcore.DebugLevel, LogDir: logPath, IsDocker: false})
	if err := config.Init(nodeName, "gate", configPath, "gate", "toml"); err != nil {
		panic(err)
	}
	go func() {
		_ = http.ListenAndServe(config.Get().PProfAddress(), nil) //"localhost:6060"
	}()
	log2.Get().Info("Run Starting!!!", zap.String("Main==", nodeName))

	// 初始化redis
	redis.NewRedisClient(config.Get().RedisAddress())

	// 2. 初始化 MongoDB
	if err := mongodb.InitFromConfig(); err != nil {
		panic(err)
	}
	// 3. 优雅关闭（通常在 main 函数退出时执行）
	defer func() {
		if err := mongodb.Close(); err != nil {
			log.Printf("Failed to close MongoDB connection: %v", err)
		}
	}()

	// 4. 初始化数据层
	inits.Init()

	// 启动服务
	if err := ws.NewServer().Start(); err != nil {
		panic(err)
	}
	log2.Get().Info("server stop ！")

}
