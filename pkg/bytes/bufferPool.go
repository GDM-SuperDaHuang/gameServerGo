package bytes

import (
	"fmt"
	"time"
)

const (
	bufferThreshold = uint32(256 * 1024) // buffer 阈值，超出部分不会回收
)

var (
	binaryPool      *BufferPool // buffer 内存池
	reflectTypePool *TypePools  // 包含多种类型对象池
)

// 获取全局 buff 内存池
func Get() *BufferPool {
	return binaryPool
}

// Types 获取类型对象池
func Types() *TypePools {
	return reflectTypePool
}

func init() {
	fmt.Println("Debug start")
	binaryPool = NewBufferPool(32, int(bufferThreshold))
	reflectTypePool = NewTypePools(16)

	// 启动定时统计日志记录器，默认每分钟记录一次
	StartStatsLogger(5 * time.Minute)
}
