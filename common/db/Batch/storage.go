package Batch

import (
	"context"
)

// 通用接口
//
// 约定（单 Redis 多实例安全）：
//   - Modify 必须经 Batch.SaveAndMarkDirty 原子落 Redis：版本号由 Redis 端
//     HINCRBY 生成并写回 payload，调用方把返回版本号回填内存对象；
//     业务数据 key 存 STRING 类型 JSON 对象，与 Load 的 GET 对应。
//   - Flush 写 Mongo 必须幂等：建议 upsert + $max 版本守卫，版本号小的
//     重复刷盘不得覆盖新数据。
type Storage interface {
	// 数据类型
	Type() DirtyType

	// UID
	UID() uint64

	// 当前版本
	Version() int64

	// 从 Redis 加载最新数据
	Load(ctx context.Context) error

	//修改，保存最新数据到Redis
	Modify(ctx context.Context) error

	// 保存到 MongoDB
	Flush(ctx context.Context) error
}
