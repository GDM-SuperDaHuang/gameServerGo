package test

import (
	"fmt"
	"gameServer/pkg/cache/cacheX"
	"gameServer/pkg/cache/ssdb"
	"testing"
	"time"

	"github.com/seefan/gossdb/v2/conf"
)

type Role struct {
	Id    int
	Name  string
	Level int
}

func TestRun(t *testing.T) {
	cfg := &conf.Config{
		Host:        "127.0.0.1",
		Port:        8888,
		MinPoolSize: 20,
		MaxPoolSize: 100,
		Encoding:    true, // 支持非基本数据类型
		Password:    "123",
		// 更多配置可参考pkg文档
	}
	err := ssdb.Init(cfg)
	if err != nil {
		panic(err)
	}
	defer ssdb.Close()
	roleCache := cachex.NewCacheX[Role](true, 10*time.Minute, 5*time.Minute)
	// 写入缓存

	// KV
	role := Role{Id: 1, Name: "战士", Level: 10}
	err = roleCache.Set("user:role:1", role, 3600)
	if err != nil {
	}
	// 读取缓存
	r, _ := roleCache.Get("role:1")
	fmt.Println("读取角色:", r)

	// hash
	err = roleCache.HSet("role:2", "role", role)
	if err != nil {
	}
	hGet, err := roleCache.HGet("role:2", "role")
	fmt.Println("读取角色:", hGet)
}
