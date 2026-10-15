package main

import (
	"fmt"
	_ "gameServer/app/room/hander/inits"
	"gameServer/app/room/hander/room"
	"gameServer/common/db"
	"os"
	"strings"

	"gameServer/pkg/config"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/recoverSafe"
	"gameServer/pkg/redis"
	"gameServer/service/node"
	"gameServer/service/rpc"
	"net/http"
	_ "net/http/pprof"

	"go.uber.org/zap/zapcore"
)

var Version = ""

func main() {

	configPath := "./config/" // ./config/
	logPath := "./logs/"      // ./logs/
	// NODE_NAME 优先(方便 swarm 同一镜像起 room-1/room-2), 否则按编译期 Version 推导
	nodeName := strings.TrimSpace(os.Getenv("NODE_NAME"))
	if nodeName == "" {
		nodeName = fmt.Sprintf("room-%s", Version)
	}
	recoverSafe.RecoverSafe(nodeName)
	log2.Init(log2.Config{Level: zapcore.DebugLevel, LogDir: logPath, IsDocker: false})
	if err := config.Init(nodeName, "room", configPath, "room", "toml"); err != nil {
		panic(err)
	}

	// pprof
	go func() {
		_ = http.ListenAndServe(config.Get().PProfAddress(), nil)
	}()

	// 获取数据源

	// 初始化redis
	redis.NewRedisClient(config.Get().RedisAddress())
	// 初始化数据层（本节点未初始化 Mongo：写入进入落库队列，由持有 Mongo 的节点批量落库）
	if err := db.Init(db.Options{}); err != nil {
		panic(err)
	}

	// 注册处理器
	f := rpc.NewForward()
	if err := f.AddModules([]interface {
	}{new(room.HandlerRoom)}); err != nil {
		panic(err)
	}
	// 添加rpcx 服务
	nodeServer := node.NewServer()
	nodeServer.Start(f)
}
