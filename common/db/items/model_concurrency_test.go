// Package items 并发回归 + 集成测试：把“高并发下会不会有问题”的分析结论逐条落成用例。
//
// 本文件全部是真实落库（Mongo；连不上自动 Skip）。断言的都是“正确行为”：
//   - TestItemsConcurrent_Reward_SameUID_SumCorrect       ← N 协程同 uid 发放，总和必须精确（$inc 原子性）
//   - TestItemsConcurrent_Consume_LimitedStock_NoOverspend ← 限量库存下抢兑，成功数收敛且永不超发/负数
//   - TestItemsConcurrent_Consume_AllOrNothing            ← 多道具一次扣完：要么全扣，要么一分不扣
//   - TestItemsConcurrent_Reward_DistinctItems             ← 同 uid 不同道具并发发放，互不干扰
//   - TestItemsConcurrent_DistinctUIDs_Throughput         ← 不同 uid 风暴，互不踩踏
//   - TestItemsCache_AfterWrite_Fresh                     ← 写后本机缓存即新鲜；Invalidate 后重读仍一致
//
// 数据隔离：uid 落在 95_000_000 段（与 Batch 的 91M、user 集成测试的发号 uid 错开），
// 每个用例开头清、结尾经 t.Cleanup 再清（Mongo user_items 文档 + localCache.Flush）。
// 注意 items 已无 Redis 依赖，本文件不需要 Redis。
//
// 跑法（Windows PowerShell）：
//
//	$env:MONGO_URI="mongodb://root:123456@127.0.0.1:27017"
//	go test ./common/db/items/ -v -count=1 -timeout 180s
//	go test ./common/db/items/ -race -count=1 -timeout 300s
package items

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gameServer/pkg/config"
	"gameServer/pkg/mongodb"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	// itestUIDBase 集成测试 uid 段基址，各用例按偏移划分，避免互踩。
	itestUIDBase int64 = 95000000
	// itestTimeout 单次 DB 操作超时。
	itestTimeout = 10 * time.Second
)

var (
	itestOnce   sync.Once
	itestReady  bool
	itestReason string
)

// requireItemsIntegration 初始化真实 Mongo 并做连通性探测，失败即 Skip。
// 写法对齐 common/db/user/model_integration_test.go 的 requireIntegration。
func requireItemsIntegration(t *testing.T) context.Context {
	t.Helper()
	itestOnce.Do(func() {
		func() {
			defer func() {
				if r := recover(); r != nil {
					itestReason = fmt.Sprintf("infra init panic: %v", r)
				}
			}()
			// 测试 working dir 是 common/db/items，../../../config 即仓库 config/。
			if err := config.Init("home-1", "home", "../../../config/", "home", "toml"); err != nil {
				itestReason = fmt.Sprintf("config init: %v", err)
				return
			}
			if err := mongodb.InitFromConfig(); err != nil {
				itestReason = fmt.Sprintf("mongodb init: %v", err)
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := mongodb.GetDB().Client().Ping(ctx, nil); err != nil {
				itestReason = fmt.Sprintf("mongo ping: %v", err)
				return
			}
			itestReady = true
		}()
	})
	if !itestReady {
		t.Skipf("无真实 Mongo，跳过集成用例（%s）", itestReason)
	}
	// 索引幂等，保证 uid 唯一约束生效。
	require.NoError(t, EnsureIndexes(context.Background()))
	localCache.Flush()
	return context.Background()
}

// itemsCleanup 清理指定 uid 的全部残留：Mongo user_items 文档 + 本地缓存。
// 用 t.Cleanup 注册，保证断言失败时同样清理。
func itemsCleanup(t *testing.T, uids []int64) {
	t.Helper()
	if len(uids) == 0 {
		localCache.Flush()
		return
	}
	if mongodb.GetDB() != nil {
		ctx, cancel := context.WithTimeout(context.Background(), itestTimeout)
		defer cancel()
		_, _ = mongodb.GetDB().Collection(itemCollection).DeleteMany(ctx, bson.M{"uid": bson.M{"$in": uids}})
	}
	localCache.Flush()
}

// readItemsMongo 直读 Mongo 落库结果，绕过本地缓存做“是否真落库”断言。
func readItemsMongo(t *testing.T, ctx context.Context, uid int64) *UserItems {
	t.Helper()
	u := &UserItems{}
	c, cancel := context.WithTimeout(ctx, itestTimeout)
	defer cancel()
	require.NoError(t, mongodb.GetDB().Collection(itemCollection).FindOne(c, bson.M{"uid": uid}).Decode(u))
	if u.Items == nil {
		u.Items = map[int]int64{}
	}
	return u
}

// seedItems 建仓：先清该 uid 文档与缓存，再一次性发放初始库存。
// 返回清理用的 uids 切片，调用方应在起跑前注册 t.Cleanup。
func seedItems(t *testing.T, ctx context.Context, uid int64, stock map[int]int64) {
	t.Helper()
	itemsCleanup(t, []int64{uid})
	if len(stock) > 0 {
		require.True(t, RewardItem(uid, stock, 0, "itest-seed"), "建仓发放不应失败")
	}
	localCache.Flush() // 踢掉写穿透的缓存，后续读强制走 Mongo，起跑线公平
}

// itemsStartBarrier 让 N 个 goroutine 真正同时起跑，挤大竞态窗口，
// 避免“串行化”造成假绿（与 user 并发测试同构）。
func itemsStartBarrier(n int) (start chan struct{}, fire func()) {
	start = make(chan struct{})
	var once sync.Once
	return start, func() { once.Do(func() { close(start) }) }
}

// ---------- 高并发正确性 ----------

// TestItemsConcurrent_Reward_SameUID_SumCorrect 同 uid 并发发放求和。
// 场景：50 writer × 20 次，每次 +1（共 1000），模拟结算/活动 discovered 高峰。
// 正确性由 Mongo 单文档 $inc 原子性保证：最终必须精确等于 1000，
// 少 1 即说明存在 lost-update（读-改-写），多 1 即说明重复记账。
func TestItemsConcurrent_Reward_SameUID_SumCorrect(t *testing.T) {
	ctx := requireItemsIntegration(t)
	uid := itestUIDBase + 11
	t.Cleanup(func() { itemsCleanup(t, []int64{uid}) })
	seedItems(t, ctx, uid, nil)

	const writers = 50
	const perWriter = 20
	const itemID = 1001
	total := int64(writers * perWriter)

	var failCount atomic.Int64
	start, fire := itemsStartBarrier(writers)
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < perWriter; i++ {
				// bool 语义下失败只记数，主 goroutine 统一断言
				// （testify require 禁止在子 goroutine 里 FailNow）。
				if !RewardItem(uid, map[int]int64{itemID: 1}, 0, "itest") {
					failCount.Add(1)
				}
			}
		}()
	}
	fire()
	wg.Wait()

	require.Zero(t, failCount.Load(), "并发发放不应失败")
	localCache.Flush()
	require.Equal(t, total, readItemsMongo(t, ctx, uid).Items[itemID],
		"50×20 次 +1 必须精确求和，差值即 lost-update/重复记账")
}

// TestItemsConcurrent_Consume_LimitedStock_NoOverspend 限量库存抢兑不超发。
// 场景：库存 100，50 协程各抢 10（需求 500 >> 库存），模拟限量礼包/门票。
// 正确性由 filter 中 items.<id> $gte 守卫 + 单文档原子更新保证：
// 恰好 10 路成功、最终余 0、余量永不为负。若实现退化为“先读后写”，
// 成功数会 >10（超发）或余量变负，本用例即红。
func TestItemsConcurrent_Consume_LimitedStock_NoOverspend(t *testing.T) {
	ctx := requireItemsIntegration(t)
	uid := itestUIDBase + 12
	t.Cleanup(func() { itemsCleanup(t, []int64{uid}) })
	const itemID = 2001
	seedItems(t, ctx, uid, map[int]int64{itemID: 100})

	const racers = 50
	var successCount atomic.Int64
	start, fire := itemsStartBarrier(racers)
	var wg sync.WaitGroup
	for w := 0; w < racers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if ConsumeItem(uid, map[int]int64{itemID: 10}, 0, "itest") {
				successCount.Add(1)
			}
		}()
	}
	fire()
	wg.Wait()

	localCache.Flush()
	final := readItemsMongo(t, ctx, uid).Items[itemID]
	require.Equal(t, int64(10), successCount.Load(),
		"库存 100、每路抢 10 时必须恰好 10 路成功，多 1 即超发")
	require.Equal(t, int64(0), final, "成功 10 路后余量必须为 0")
	require.GreaterOrEqual(t, final, int64(0), "余量永不为负")
}

// TestItemsConcurrent_Consume_AllOrNothing 多道具原子“一单全扣”。
// 场景：{3001:10, 3002:10}，20 协程各尝试一次扣 {3001:6, 3002:6}。
// 单文档多字段 $inc 天然原子：首路扣完剩 {4,4}，其余 19 路因任一不足整单失败。
// 若实现是“逐道具循环扣”，会出现 3001 被扣完而 3002 剩 4 的半扣残留，本用例即红。
func TestItemsConcurrent_Consume_AllOrNothing(t *testing.T) {
	ctx := requireItemsIntegration(t)
	uid := itestUIDBase + 13
	t.Cleanup(func() { itemsCleanup(t, []int64{uid}) })
	seedItems(t, ctx, uid, map[int]int64{3001: 10, 3002: 10})

	const n = 20
	var successCount atomic.Int64
	start, fire := itemsStartBarrier(n)
	var wg sync.WaitGroup
	for w := 0; w < n; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if ConsumeItem(uid, map[int]int64{3001: 6, 3002: 6}, 0, "itest") {
				successCount.Add(1)
			}
		}()
	}
	fire()
	wg.Wait()

	localCache.Flush()
	final := readItemsMongo(t, ctx, uid)
	require.Equal(t, int64(1), successCount.Load(), "整单 6+6 在库存 10+10 下只能成功 1 路")
	require.Equal(t, int64(4), final.Items[3001], "半扣残留：3001 应剩 4")
	require.Equal(t, int64(4), final.Items[3002], "半扣残留：3002 应剩 4")
}

// TestItemsConcurrent_Reward_DistinctItems 同 uid 不同道具并发发放互不干扰。
// 场景：50 协程各发各的道具（5000+i），模拟开箱一次到账多格子。
// 单文档多字段 $inc 各管各的 field：50 个格子应齐全且各为 1，
// 少一个即说明并发 $inc 互相覆盖（整文档覆盖写会犯此错）。
func TestItemsConcurrent_Reward_DistinctItems(t *testing.T) {
	ctx := requireItemsIntegration(t)
	uid := itestUIDBase + 14
	t.Cleanup(func() { itemsCleanup(t, []int64{uid}) })
	seedItems(t, ctx, uid, nil)

	const n = 50
	var failCount atomic.Int64
	start, fire := itemsStartBarrier(n)
	var wg sync.WaitGroup
	for w := 0; w < n; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			if !RewardItem(uid, map[int]int64{5000 + w: 1}, 0, "itest") {
				failCount.Add(1)
			}
		}(w)
	}
	fire()
	wg.Wait()

	require.Zero(t, failCount.Load())
	localCache.Flush()
	final := readItemsMongo(t, ctx, uid)
	require.Len(t, final.Items, n, "50 个不同道具应齐全，缺失即并发覆盖")
	for w := 0; w < n; w++ {
		require.Equal(t, int64(1), final.Items[5000+w], "道具 %d 应为 1", 5000+w)
	}
}

// TestItemsConcurrent_DistinctUIDs_Throughput 不同 uid 风暴互不踩踏。
// 场景：100 个 uid 各自 1 协程发放 10（金币 6001），模拟开服/活动整点脉冲。
// 断言：100 个背包逐个精确为 10。任一缺失/错值即说明 uid 间串扰
// （filter 漏 uid、缓存 key 串等），与“同 uid 求和”用例互补。
func TestItemsConcurrent_DistinctUIDs_Throughput(t *testing.T) {
	ctx := requireItemsIntegration(t)
	const n = 100
	uids := make([]int64, n)
	for i := range uids {
		uids[i] = itestUIDBase + 1000 + int64(i)
	}
	// 清理必须在起跑前注册：100 路落库失败一半时也能清掉已建部分。
	t.Cleanup(func() { itemsCleanup(t, uids) })
	itemsCleanup(t, uids)

	const itemID = 6001
	var failCount atomic.Int64
	start, fire := itemsStartBarrier(n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if !RewardItem(uids[i], map[int]int64{itemID: 10}, 0, "itest") {
				failCount.Add(1)
			}
		}(i)
	}
	fire()
	wg.Wait()

	require.Zero(t, failCount.Load(), "不同 uid 并发发放不应失败")
	localCache.Flush()
	for _, uid := range uids {
		require.Equal(t, int64(10), readItemsMongo(t, ctx, uid).Items[itemID],
			"uid %d 背包应精确为 10", uid)
	}
}

// TestItemsConcurrent_Mixed_RewardConsume_Converges 读写混合最终一致。
// 场景：先建仓 1000（金币 7001）；随后 20 发放协程各 +5×10 次（+1000），
// 20 消耗协程各 -3×10 次（-600，如不足则整单失败不扣）。
// 发放侧 upsert + 消耗侧 $gte 守卫都是原子操作：最终值 = 1000+1000-成功消耗，
// 且成功消耗数×3 + 余量 == 2000（能量守恒：既无凭空产生，也无丢失）。
// 若实现有竞态（先读后写），守恒即破，本用例即红。
func TestItemsConcurrent_Mixed_RewardConsume_Converges(t *testing.T) {
	ctx := requireItemsIntegration(t)
	uid := itestUIDBase + 15
	t.Cleanup(func() { itemsCleanup(t, []int64{uid}) })
	const itemID = 7001
	seedItems(t, ctx, uid, map[int]int64{itemID: 1000})

	const writers = 20
	const rounds = 10
	var rewardFails atomic.Int64
	var consumeSuccess atomic.Int64
	start, fire := itemsStartBarrier(2 * writers)
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < rounds; i++ {
				if !RewardItem(uid, map[int]int64{itemID: 5}, 0, "itest") {
					rewardFails.Add(1)
				}
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < rounds; i++ {
				if ConsumeItem(uid, map[int]int64{itemID: 3}, 0, "itest") {
					consumeSuccess.Add(1)
				}
			}
		}()
	}
	fire()
	wg.Wait()

	require.Zero(t, rewardFails.Load(), "发放侧不应失败")
	localCache.Flush()
	final := readItemsMongo(t, ctx, uid).Items[itemID]
	// 守恒：初始 1000 + 发放 20×10×5 − 成功消耗×3 == 余量。
	require.Equal(t, int64(1000)+int64(writers*rounds*5)-consumeSuccess.Load()*3, final,
		"能量守恒被打破：成功消耗=%d 余量=%d", consumeSuccess.Load(), final)
	require.GreaterOrEqual(t, final, int64(0), "余量永不为负")
	t.Logf("混合读写：发放+1000，消耗成功 %d 路×3，余量=%d", consumeSuccess.Load(), final)
}

// ---------- 缓存行为（真实落库版） ----------

// TestItemsCache_AfterWrite_Fresh 写后本机缓存即新鲜，且与 Mongo 一致。
// 链路：Reward 写穿透回填 localCache → GetItems 命中缓存 → Flush 后重读 Mongo，
// 三者必须相等。若写后忘回填，会读到旧缓存，本用例即红。
func TestItemsCache_AfterWrite_Fresh(t *testing.T) {
	ctx := requireItemsIntegration(t)
	uid := itestUIDBase + 21
	t.Cleanup(func() { itemsCleanup(t, []int64{uid}) })
	seedItems(t, ctx, uid, map[int]int64{8001: 5})

	require.True(t, RewardItem(uid, map[int]int64{8001: 7}, 0, "itest"))

	cached, err := GetItems(uid)
	require.NoError(t, err)
	require.Equal(t, int64(12), cached.Items[8001], "写后本机缓存应即新鲜")

	localCache.Flush()
	reloaded, err := GetItems(uid)
	require.NoError(t, err)
	require.Equal(t, int64(12), reloaded.Items[8001])
	require.Equal(t, int64(12), readItemsMongo(t, ctx, uid).Items[8001],
		"缓存、重读、Mongo 三方必须一致")
}

// TestItemsCache_InvalidateReload 跨实例写入模拟：本机缓存预热后，
// 另一实例直写 Mongo（绕过本机缓存），Invalidate 后本机重读必须看到新值。
// 不调 Invalidate 时仍读旧值（5 分钟窗口的现状锁死，见 user 的 P1-2 同类用例）。
func TestItemsCache_InvalidateReload(t *testing.T) {
	ctx := requireItemsIntegration(t)
	uid := itestUIDBase + 22
	t.Cleanup(func() { itemsCleanup(t, []int64{uid}) })
	seedItems(t, ctx, uid, map[int]int64{8002: 5})

	_, err := GetItems(uid) // 预热本机缓存：8002=5
	require.NoError(t, err)

	// 别机的写入：直写 Mongo，不碰本机 localCache。
	mdb := mongodb.GetDB()
	c, cancel := context.WithTimeout(ctx, itestTimeout)
	_, err = mdb.Collection(itemCollection).UpdateOne(c,
		bson.M{"uid": uid}, bson.M{"$set": bson.M{"items.8002": int64(99)}})
	cancel()
	require.NoError(t, err)

	stale, err := GetItems(uid)
	require.NoError(t, err)
	require.Equal(t, int64(5), stale.Items[8002], "未失效前读到旧缓存（5 分钟窗口现状）")

	InvalidateCache(uid)
	fresh, err := GetItems(uid)
	require.NoError(t, err)
	require.Equal(t, int64(99), fresh.Items[8002], "Invalidate 后必须看到别机的新值")
	require.Equal(t, int64(99), readItemsMongo(t, ctx, uid).Items[8002])
}
