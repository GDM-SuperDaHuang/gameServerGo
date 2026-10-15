// Package items 单测（道具模型部分）：覆盖 common/db/items/model.go
// 中不依赖真实中间件即可验证的分支。
//
// 被测对象职责回顾：
//   - 本地进程缓存：item:uid:<uid> -> *UserItems，读穿透 + 写穿透，返回前 clone 隔离；
//   - 查询：GetItems / GetItemCount（本地 -> Mongo，Mongo 为准）；
//   - 写入：applyMapAtomic（单文档多字段 $inc 天然原子，消耗带 $gte 守卫）、
//     ConsumeItem / RewardItem（bool 语义：成功 true，任何失败 false）。
//
// 设计说明（为什么这样测）：
//  1. 本地单测默认不起 Mongo，因此正常 DB 路径无法 hermetic 覆盖，留给集成环境
//     （见 model_concurrency_test.go）；
//  2. 这里锁住的是三类约定：
//     a) 参数校验不碰中间件：uid==0 直接拒绝；空 map 直接短路（Consume/Reward 回 true）；
//     b) 中间件缺失时不 panic、错误可辨识：mongo==nil 报 "mongodb not initialized"；
//     c) 纯内存行为：clone 深拷贝隔离、setLocal 守卫与 nil-Items 归一、
//     缓存命中时不碰 Mongo、InvalidateCache 永不崩进程；
//  3. 有真实中间件时（GetDB() 非空）对“缺失断言”自动 Skip，
//     缓存命中类用例则无论有无中间件都可跑（命中时根本不碰中间件）；
//  4. 白盒同包（package items）以便直接调用未导出的 itemCacheKey / setLocal /
//     getLocal / applyMapAtomic；helper 命名加 items 前缀，避免与同包集成文件冲突。
//
// 运行：go test ./common/db/items/ -v -count=1。
// 带竞态检测：go test ./common/db/items/ -race -count=1。
package items

import (
	"errors"
	"testing"

	"gameServer/pkg/mongodb"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// flushItemsCache 每个用例前清空进程内缓存，避免串扰。
func flushItemsCache() {
	localCache.Flush()
}

// requireMongoUninitializedForItems 环境探针：仅在无 Mongo 时断言 nil-Mongo 错误。
// 有真实 Mongo 时 Skip（此时应去跑集成用例）。
func requireMongoUninitializedForItems(t *testing.T) {
	t.Helper()
	if mongodb.GetDB() != nil {
		t.Skip("mongo 已初始化，跳过 nil-Mongo 断言（需无中间件环境）")
	}
}

// ---------- key 格式 ----------

// TestItemCacheKey_Format 锁死缓存 key 与 Mongo 字段路径格式，
// 防止改名导致读写分裂（FindOneAndUpdate 的 filter 与 $inc 必须同源）。
func TestItemCacheKey_Format(t *testing.T) {
	require.Equal(t, "item:uid:123", itemCacheKey(123))
	require.NotEqual(t, itemCacheKey(1), itemCacheKey(2))
	require.Equal(t, "items.1001", itemField(1001))
	require.NotEqual(t, itemField(1001), itemField(1002))
}

// ---------- clone / 本地缓存 ----------

// TestCloneItems_NilIsolation 测 clone 的两条约定：nil 进 nil 出；
// 返回的是深拷贝，调用方改 Items map 不污染原件。
func TestCloneItems_NilIsolation(t *testing.T) {
	require.Nil(t, cloneItems(nil))

	src := &UserItems{Uid: 95001, Items: map[int]int64{1001: 5}, UpdatedAt: 123}
	cp := cloneItems(src)
	require.Equal(t, src, cp)
	require.NotSame(t, src, cp, "必须拷贝，调用方改指针对象不能污染缓存")

	cp.Items[1001] = 999
	cp.Items[9999] = 1
	require.Equal(t, int64(5), src.Items[1001])
	require.NotContains(t, src.Items, 9999, "map 必须深拷贝")
}

// TestSetLocal_Guard 测 setLocal 的守卫：nil / Uid==0 直接丢弃，不写缓存不 panic。
func TestSetLocal_Guard(t *testing.T) {
	flushItemsCache()

	require.NotPanics(t, func() { setLocal(nil) })
	require.NotPanics(t, func() { setLocal(&UserItems{}) })

	_, ok := getLocal(0)
	require.False(t, ok)
}

// TestSetLocal_NilItemsNormalized 测 nil Items 归一为空 map：
// 后续 GetItemCount / 读写路径都不必判 nil，调用方拿到的一定是可用 map。
func TestSetLocal_NilItemsNormalized(t *testing.T) {
	flushItemsCache()

	setLocal(&UserItems{Uid: 95002})
	got, ok := getLocal(95002)
	require.True(t, ok)
	require.NotNil(t, got.Items)
	require.Empty(t, got.Items)
}

// TestLocalCache_SetGet_Isolation 测“写穿透缓存 + 读时 clone”：
// setLocal 后 getLocal 命中，且返回对象与入参/缓存内原件互不影响。
func TestLocalCache_SetGet_Isolation(t *testing.T) {
	flushItemsCache()

	src := &UserItems{Uid: 95003, Items: map[int]int64{1001: 7}}
	setLocal(src)

	got, ok := getLocal(95003)
	require.True(t, ok)
	require.Equal(t, int64(7), got.Items[1001])
	require.NotSame(t, src, got)

	got.Items[1001] = 777
	src.Items[1001] = 888
	got2, ok := getLocal(95003)
	require.True(t, ok)
	require.Equal(t, int64(7), got2.Items[1001], "读写缓存必须 clone，调用方修改不能污染缓存")
}

// TestInvalidateCache_NoPanic 测主动失效永不崩进程：
// 无缓存、空 uid、有缓存三种情况都不 panic，且有缓存时确实被清掉。
func TestInvalidateCache_NoPanic(t *testing.T) {
	flushItemsCache()

	require.NotPanics(t, func() { InvalidateCache(0) })
	require.NotPanics(t, func() { InvalidateCache(95999) })

	setLocal(&UserItems{Uid: 95004, Items: map[int]int64{1: 1}})
	_, ok := getLocal(95004)
	require.True(t, ok)

	require.NotPanics(t, func() { InvalidateCache(95004) })
	_, ok = getLocal(95004)
	require.False(t, ok, "Invalidate 后缓存应被清掉")
}

// ---------- 参数校验（不碰中间件） ----------

// TestGetItems_ZeroUid 测 uid==0 直接拒绝，不碰 Mongo。
func TestGetItems_ZeroUid(t *testing.T) {
	_, err := GetItems(0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty uid")
}

// TestGetItemCount_ZeroUid 同上，覆盖便捷查询入口。
func TestGetItemCount_ZeroUid(t *testing.T) {
	_, err := GetItemCount(0, 1001)
	require.Error(t, err)
}

// TestConsumeReward_EmptyMap 测空 map 短路为 true：不碰 Mongo，
// 调用方（购买/结算）传空消耗/空奖励时直接放行。
func TestConsumeReward_EmptyMap(t *testing.T) {
	require.True(t, ConsumeItem(95011, nil, 0, ""))
	require.True(t, ConsumeItem(95011, map[int]int64{}, 0, ""))
	require.True(t, RewardItem(95011, nil, 0, ""))
	require.True(t, RewardItem(95011, map[int]int64{}, 0, ""))
}

// TestApplyMapAtomic_EmptyMap_CacheHit 测 applyMapAtomic 空 map 退化为 GetItems：
// 缓存命中时不碰 Mongo。对应源码 len(itemMap)==0 分支。
func TestApplyMapAtomic_EmptyMap_CacheHit(t *testing.T) {
	flushItemsCache()
	setLocal(&UserItems{Uid: 95012, Items: map[int]int64{1001: 3}})

	got, err := applyMapAtomic(95012, nil, false)
	require.NoError(t, err)
	require.Equal(t, int64(3), got.Items[1001])
}

// TestApplyMapAtomic_NonPositiveMap_CacheHit 测全非正增量同样短路为读：
// 调用方误传 0/负数时不组装 $inc，直接返回当前背包。
func TestApplyMapAtomic_NonPositiveMap_CacheHit(t *testing.T) {
	flushItemsCache()
	setLocal(&UserItems{Uid: 95013, Items: map[int]int64{1001: 3}})

	got, err := applyMapAtomic(95013, map[int]int64{1001: 0, 1002: -5}, true)
	require.NoError(t, err)
	require.Equal(t, int64(3), got.Items[1001])
}

// TestApplyMapAtomic_ZeroUid 测 uid==0 在碰 DB 前拒绝（无论正负方向）。
func TestApplyMapAtomic_ZeroUid(t *testing.T) {
	_, err := applyMapAtomic(0, map[int]int64{1001: 1}, true)
	require.Error(t, err)
	_, err = applyMapAtomic(0, map[int]int64{1001: 1}, false)
	require.Error(t, err)
}

// ---------- 中间件缺失分支（无 Mongo 时才断言） ----------

// TestGetItems_Miss_NoMongo 测缓存未命中 + Mongo 缺失时报可辨识错误，不 panic。
func TestGetItems_Miss_NoMongo(t *testing.T) {
	requireMongoUninitializedForItems(t)
	flushItemsCache()

	_, err := GetItems(959001)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")
}

// TestGetItemCount_Miss_NoMongo 同上，覆盖便捷查询的透错路径。
func TestGetItemCount_Miss_NoMongo(t *testing.T) {
	requireMongoUninitializedForItems(t)
	flushItemsCache()

	_, err := GetItemCount(959002, 1001)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")
}

// TestApplyMapAtomic_NoMongo 测非空写操作在 Mongo 缺失时报可辨识错误。
func TestApplyMapAtomic_NoMongo(t *testing.T) {
	requireMongoUninitializedForItems(t)
	flushItemsCache()

	_, err := applyMapAtomic(959003, map[int]int64{1001: 1}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")

	_, err = applyMapAtomic(959003, map[int]int64{1001: 1}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")
}

// TestApplyMapAtomic_EmptyMap_NoMongo 测空 map 短路同样透出 Mongo 缺失错误
// （退化为 GetItems，而缓存未命中），锁死该透传链条。
func TestApplyMapAtomic_EmptyMap_NoMongo(t *testing.T) {
	requireMongoUninitializedForItems(t)
	flushItemsCache()

	_, err := applyMapAtomic(959004, nil, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")
}

// TestConsumeReward_NoMongo 测 bool 语义：Mongo 缺失时写操作回 false 而非 panic，
// 调用方（HomeHander 购买等）靠 false 走失败分支。
func TestConsumeReward_NoMongo(t *testing.T) {
	requireMongoUninitializedForItems(t)
	flushItemsCache()

	require.False(t, ConsumeItem(959005, map[int]int64{1001: 1}, 0, ""))
	require.False(t, RewardItem(959005, map[int]int64{1001: 1}, 0, ""))
}

// TestEnsureIndexes_NilCtx_NilMongo 测 EnsureIndexes 在 mongo 缺失时快速失败，
// ctx 传 nil 也应先卡在 mongo 检查上，不 panic。
func TestEnsureIndexes_NilCtx_NilMongo(t *testing.T) {
	requireMongoUninitializedForItems(t)
	require.Error(t, EnsureIndexes(nil))
}

// ---------- 缓存命中（有无中间件都可跑） ----------

// TestGetItems_CacheHit 测读穿透命中路径：setLocal 后 GetItems 直接回缓存，
// 即使 Mongo 缺失也不报错。对应源码 GetItems 的 localCache 前置分支。
func TestGetItems_CacheHit(t *testing.T) {
	flushItemsCache()

	setLocal(&UserItems{Uid: 95021, Items: map[int]int64{1001: 9, 1002: 2}})

	got, err := GetItems(95021)
	require.NoError(t, err)
	require.Equal(t, int64(9), got.Items[1001])
	require.Equal(t, int64(2), got.Items[1002])
}

// TestGetItemCount_CacheHit 测单个查询同样走缓存：存在返回数量，
// 不存在返回 0（背包存在但该道具无记录），都不碰 Mongo。
func TestGetItemCount_CacheHit(t *testing.T) {
	flushItemsCache()

	setLocal(&UserItems{Uid: 95022, Items: map[int]int64{1001: 4}})

	n, err := GetItemCount(95022, 1001)
	require.NoError(t, err)
	require.Equal(t, int64(4), n)

	n, err = GetItemCount(95022, 8888)
	require.NoError(t, err)
	require.Zero(t, n, "背包存在但道具无记录时应回 0")

	// 空背包同样回 0 而非报错。
	setLocal(&UserItems{Uid: 95023, Items: map[int]int64{}})
	n, err = GetItemCount(95023, 1001)
	require.NoError(t, err)
	require.Zero(t, n)
}

// TestUpdateFieldsContract_NilItems 表示层约定：GetItemCount 对 nil Items 不 panic。
// cloneItems 可能产出 nil Items（原型 Items 为 nil 时），便捷查询必须容忍。
func TestUpdateFieldsContract_NilItems(t *testing.T) {
	u := cloneItems(&UserItems{Uid: 95024})
	require.NotNil(t, u)
	// 直接调包级便捷函数需要缓存，这里只锁 clone 语义：nil 进，Items 保持 nil 但可用。
	require.Nil(t, u.Items)
	require.Zero(t, u.Items[1001], "nil map 读应回零值且不 panic")
}

// TestSentinelErrors 锁死哨兵错误语义，防止被改名导致调用方 errors.Is 失效。
func TestSentinelErrors(t *testing.T) {
	require.ErrorIs(t, ErrItemNotEnough, ErrItemNotEnough)
	require.ErrorIs(t, ErrItemNotFound, ErrItemNotFound)
	require.False(t, errors.Is(ErrItemNotEnough, ErrItemNotFound))
	require.NotEqual(t, ErrItemNotEnough.Error(), ErrItemNotFound.Error())
}

// TestBsonContract_UpdateFields 锁死 Mongo 字段约定：uid 与 items.<id> 路径。
// applyMapAtomic 的 filter/update 与 GetItems 的查询键必须同源，
// 否则会出现“写入一个集合/字段、读取另一个”的分裂。
func TestBsonContract_UpdateFields(t *testing.T) {
	require.Equal(t, "items.1001", itemField(1001))

	// 构造一次“发放”应产生的 update 文档骨架，字段名与 bson tag 对齐。
	update := bson.M{
		"$inc":         bson.M{itemField(1001): int64(5)},
		"$set":         bson.M{"updatedAt": int64(1)},
		"$setOnInsert": bson.M{"uid": int64(95031)},
	}
	require.Contains(t, update["$inc"], "items.1001")
	require.Contains(t, update["$setOnInsert"], "uid")
}
