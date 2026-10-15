// Package user 单测（uid 发号部分）：覆盖 common/db/user/uidModel.go 的异常分支。
//
// 被测对象职责回顾：
//   - InitUid：服务启动时把 Redis 计数器 uid:Inc 收敛到 startUID(10000) 以上，幂等；
//   - GenUid / GenUidE：Redis INCR 原子发号（发号源），再直写 Mongo 快照 uid_increment
//     （$max 守卫防并发回退；快照失败只打日志/透错，uid 依然可用）；
//   - persistUidSnapshot：快照直写的内部 helper。
//
// 设计说明（为什么只测异常分支）：
//  1. 正常发号依赖真实 Redis（INCR）+ 真实 Mongo（Upsert 快照），本地单测默认不起
//     中间件，因此正常路径无法 hermetic 覆盖，留给集成环境；
//  2. 这里锁住的是“中间件缺失时不 panic、错误可辨识”的约定：redis==nil 报
//     "redis client is nil"、mongo==nil 报 "mongodb not initialized"、GenUid 按约定回 0；
//  3. 有真实中间件时（GetRedisClient()/GetDB() 非空）自动 Skip，避免在集成环境误红；
//  4. 白盒同包（package user）以便直接调用未导出的 persistUidSnapshot。
//
// 运行：go test ./common/db/user/ -v -count=1 -run 'TestInitUid|TestGenUid|TestPersistUid'。
package user

import (
	"context" // 构造 Background；另测 GenUidE(nil) 的入参规整分支
	"testing" // 标准单测入口

	"gameServer/pkg/logger/log2" // GenUid 失败分支依赖全局日志，需先兜底初始化
	"gameServer/pkg/mongodb"     // 探测 GetDB() 是否已初始化，决定是否 Skip
	"gameServer/pkg/redis"       // 探测 GetRedisClient() 是否已初始化，决定是否 Skip

	"github.com/stretchr/testify/require" // 断言库
	"go.uber.org/zap/zapcore"             // 构造 log2.Config 的日志级别
)

// requireRedisUninitialized 环境探针：仅在无 Redis 时断言 nil-Redis 错误。
//
// 背景：本地单测默认不起 Redis，GetRedisClient()==nil；但 CI 若起了真实 Redis，
// 同一调用会走 INCR 网络并成功，此时硬断言 "redis client is nil" 会误红。
// 因此探测到 client 非空就 t.Skip，提示需要无中间件环境。
func requireRedisUninitialized(t *testing.T) {
	t.Helper()
	if redis.GetRedisClient() != nil {
		t.Skip("redis 已初始化，跳过 nil-Redis 断言（需无中间件环境）")
	}
}

// requireMongoUninitializedForUID 环境探针：仅在无 Mongo 时断言 nil-Mongo 错误。
//
// 背景：同上。persistUidSnapshot 在有真实 Mongo 时会真实 Upsert 快照文档，
// 不应在本文件中断言其报错，故探测到 DB 非空即 Skip。
func requireMongoUninitializedForUID(t *testing.T) {
	t.Helper()
	if mongodb.GetDB() != nil {
		t.Skip("mongo 已初始化，跳过 nil-Mongo 断言（需无中间件环境）")
	}
}

// initNopLogger 给 GenUid 的失败日志兜底，避免 nil *zap.Logger panic。
//
// 背景：GenUid 内部在 GenUidE 出错时调 log2.Get().Warn(...)。log2 全局变量默认 nil
// （未调 log2.Init 前），对 nil *zap.Logger 调 Warn 会解引用 panic，单测会崩。
// 这里用 Docker 模式（纯 stdout、不写文件、不建目录）初始化一次，保证 hermetic；
// 已初始化过则不再重复 Init，避免覆盖集成环境的日志配置。
func initNopLogger() {
	if log2.Get() == nil {
		log2.Init(log2.Config{Level: zapcore.ErrorLevel, IsDocker: true})
	}
}

// TestInitUid_NilRedis 测服务启动收敛逻辑在无 Redis 时的快速失败。
//
// 对应源码 InitUid：第一行即 rdb := redis.GetRedisClient()，nil 直接返回
// "uid: redis client is nil"，不会继续碰 Mongo。
// 断言用 Contains 而非 ErrorIs：源码用 errors.New 裸报，没有哨兵误差，只能匹配文本。
func TestInitUid_NilRedis(t *testing.T) {
	requireRedisUninitialized(t)
	err := InitUid()
	require.Error(t, err)
	require.Contains(t, err.Error(), "redis client is nil")
}

// TestGenUidE_NilRedis 测发号函数的可透错版本在无 Redis 时的行为，兼测 ctx==nil 规整。
//
// 对应源码 GenUidE：先把 nil ctx 规整为 context.Background()，再取 redis client，
// nil 则返回 (0, "uid: redis client is nil")，约定“uid==0 表示 Redis 发号失败”。
// 两段断言：
//  1. 正常 Background 下报 redis-nil；
//  2. 传 nil ctx 同样报 redis-nil 而非 panic——证明入参规整分支工作，
//     调用方（如 CreateUser 中透传的上游 ctx 恰为 nil 时）不会崩。
func TestGenUidE_NilRedis(t *testing.T) {
	requireRedisUninitialized(t)

	_, err := GenUidE(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "redis client is nil")

	// ctx == nil 时应被规整为 Background，依然走到 redis-nil 分支而非 panic。
	_, err = GenUidE(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "redis client is nil")
}

// TestGenUid_NilRedis_ReturnsZero 测吞错版本 GenUid 在无 Redis 时按约定回 0。
//
// 对应源码 GenUid：调 GenUidE，出错只 log2.Warn 打日志并返回 0（不把 error 透出），
// 调用方（CreateUser）以 uid==0 判定“发号失败”。
// 前置必须先 initNopLogger，否则 Warn 即 panic，本用例会崩而非红。
// 断言 Zero：锁死“失败回 0”约定，防止将来有人改成回 startUID/回随机数导致 uid 冲突。
func TestGenUid_NilRedis_ReturnsZero(t *testing.T) {
	requireRedisUninitialized(t)
	initNopLogger()

	require.Zero(t, GenUid(), "Redis 不可用时 GenUid 按约定返回 0")
}

// TestPersistUidSnapshot_NilMongo 测快照直写在无 Mongo 时的快速失败。
//
// 对应源码 persistUidSnapshot：第一行即 mdb := mongodb.GetDB()，nil 直接返回
// "uid: mongodb not initialized"，不会拼 filter/update，更不会发网络。
// 入参 uid=10001 为任意合法值（startUID 附近），证明失败与入参无关、纯由环境缺失触发。
// 正常路径（$max+Upsert）需真实 Mongo，留给集成测试，本文件不覆盖。
func TestPersistUidSnapshot_NilMongo(t *testing.T) {
	requireMongoUninitializedForUID(t)

	err := persistUidSnapshot(context.Background(), 10001)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")
}
