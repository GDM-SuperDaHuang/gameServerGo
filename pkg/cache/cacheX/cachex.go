package cachex

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// CacheX 双层缓存结构体,支持复杂数据结构
// 前提需要开启 Encoding:true,支持非基本数据类型
type CacheX[T any] struct {
	local *cache.Cache

	dbFlag bool // 是否持久化到 SSDB
}

// NewCacheX 创建新的泛型缓存实例
func NewCacheX[T any](dbFlag bool, localDefaultTTL, localCleanupInterval time.Duration) *CacheX[T] {
	return &CacheX[T]{
		local:  cache.New(localDefaultTTL, localCleanupInterval),
		dbFlag: dbFlag,
	}
}
