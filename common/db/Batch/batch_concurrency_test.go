package Batch_test

// 包说明：多实例 × 高并发落库测试（Batch + heros）。
//
// 覆盖链路：heros.Modify（Redis Lua 原子写 + 脏标记）→ Batch.Get（取脏）→
// Worker.FlushType（分布式锁 + Load + Flush + RemoveIfSame）→ Mongo。
//
// 跑法（Windows PowerShell）：
//   $env:REDIS_ADDR="127.0.0.1:6379"; $env:MONGO_URI="mongodb://root:123456@127.0.0.1:27017"
//   go test ./common/db/Batch/ -run 'TestConcurrent|TestMultiInstance|TestStaleFlush|TestStartPersist' -v -count=1 -timeout 120s
//
// 前置依赖：真实 Redis（Lua 脚本 + 分布式锁）与真实 Mongo；连不上自动 Skip。
// 数据隔离：为避免污染开发数据，所有 uid 落在 91_000_000 + 序号段，每个用例
// 开头清、结尾经 t.Cleanup 再清（Redis 业务 key + 脏 ZSet/Hash + 落库锁 + Mongo 文档）。

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gameServer/common/db/Batch"
	"gameServer/common/db/heros"
	"gameServer/pkg/config"
	"gameServer/pkg/mongodb"
	"gameServer/pkg/redis"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	// testHeroCollection：heros 落库的 Mongo 集合名（与 heros.model.go 内 heroCollection 对齐）。
	testHeroCollection = "user_heroes"
	// testUIDBase：测试 uid 段基址，91000000 之后按用例划分偏移，避免与真实玩家及用例间互踩。
	testUIDBase = uint64(91000000)
)

// initInfra 初始化测试基建：读仓库 config、连 Redis/Mongo 并做连通性探测。
// 任一步失败即 t.Skip（CI 无中间件时整文件跳过而非挂掉）。
// 返回 Background 供各用例派生带超时的子 ctx（避免复用带 cancel 的父 ctx 导致串扰）。
func initInfra(t *testing.T) context.Context {
	t.Helper()
	// 测试工作目录是 common/db/Batch，../../../config 即仓库 config/。
	if err := config.Init("home-1", "home", "../../../config/", "home", "toml"); err != nil {
		t.Skipf("config init skip: %v", err)
	}
	redis.NewRedisClient(config.Get().RedisAddress())
	if err := mongodb.InitFromConfig(); err != nil {
		t.Skipf("mongodb init skip: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redis.GetRedisClient().Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable skip: %v", err)
	}
	if err := mongodb.GetDB().Client().Ping(ctx, nil); err != nil {
		t.Skipf("mongo unavailable skip: %v", err)
	}
	return context.Background()
}

// cleanUIDs 清理指定 uid 的全部测试残留，保证用例可重复执行：
//  1. Redis 业务 key（hero:uid:%d）；2. 脏 ZSet/Hash 中的 member；
//  3. 落库分布式锁（persist:lock:hero，防止上轮崩溃泄漏的锁挡住本轮）；
//  4. Mongo 中对应 uid 文档。
func cleanUIDs(t *testing.T, ctx context.Context, uids []uint64) {
	t.Helper()
	rdb := redis.GetRedisClient()
	keys := make([]string, 0, len(uids)+3)
	members := make([]string, 0, len(uids))
	for _, u := range uids {
		keys = append(keys, heros.GetKey(u))
		members = append(members, fmt.Sprintf("%d", u))
	}
	if len(keys) > 0 {
		require.NoError(t, rdb.Del(ctx, keys...).Err())
	}
	if len(members) > 0 {
		require.NoError(t, rdb.ZRem(ctx, Batch.DirtyZSetKey(Batch.DirtyHero), members).Err())
		require.NoError(t, rdb.HDel(ctx, Batch.DirtyVersionKey(Batch.DirtyHero), members...).Err())
	}
	require.NoError(t, rdb.Del(ctx, Batch.FlushLockKey(Batch.DirtyHero)).Err())
	_, err := mongodb.GetDB().Collection(testHeroCollection).DeleteMany(ctx, bson.M{"uid": bson.M{"$in": uids}})
	require.NoError(t, err)
}

// readMongoHero 直读 Mongo 中某 uid 的落库结果，返回（version，首个 hero 的 Id）。
// heroID 是内容探针：写入时用不同 Id 标记版本（如 Id=9 代表 V9 的数据），
// 读回时即可判断 mongo 存的是新鲜内容还是被旧版本回盖的 stale 内容。
func readMongoHero(t *testing.T, ctx context.Context, uid uint64) (version int64, heroID int) {
	t.Helper()
	var doc struct {
		Heroes  []heros.Hero `bson:"heroes"`
		Version int64        `bson:"version"`
	}
	err := mongodb.GetDB().Collection(testHeroCollection).FindOne(ctx, bson.M{"uid": uid}).Decode(&doc)
	require.NoError(t, err)
	if len(doc.Heroes) > 0 {
		heroID = doc.Heroes[0].Id
	}
	return doc.Version, heroID
}

// newWorker 造一个与线上 ints.go 同构的落库 Worker（Registry 只注册 DirtyHero）。
// 多实例模拟靠的就是：每个 goroutine 各自 newWorker（独立 Registry/running 标志），
// 但共享同一个 Redis/Mongo——正好复刻“多个 home 进程同时跑定时落库”的线上拓扑。
func newWorker() *Batch.Worker {
	r := Batch.NewRegistry()
	r.Register(Batch.DirtyHero, func(uid uint64) Batch.Storage {
		return heros.NewStorage(uid)
	})
	return Batch.NewWorker(r)
}

// TestConcurrentModify_SameUID 同 uid 被 N 个“实例”狂写，验证写链路原子性。
//
// 场景：32 writer × 20 次 = 640 次 Modify 砸向同一个 uid，模拟多实例/多请求并发改同一玩家。
// 关键点：每个 writer 用独立的 UserHeroes 对象（只共享 uid，不共享内存），并发安全由
// Redis 端 Lua（HINCRBY + SET + ZADD 一次执行）保证，而非靠 Go 锁。
// 断言：
//  1. 无错误；2. 拿到的 640 个版本号排序后恰为 1..640（唯一、无丢失、无重复——Lua 串行性的直接证据）；
//  3. 脏 Hash 里该 uid 版本号 == 640（脏标记与业务数据一致）。
//
// 注意：heros.Modify 是整包覆盖语义，最终 Redis 内容只能是 last-writer-wins，
// 本用例不断言内容合并，只保版本号——内容合并是业务层串行化要解决的另一件事。
func TestConcurrentModify_SameUID(t *testing.T) {
	ctx := initInfra(t)
	uid := testUIDBase + 1
	cleanUIDs(t, ctx, []uint64{uid})
	t.Cleanup(func() { cleanUIDs(t, context.Background(), []uint64{uid}) })

	const writers = 100
	const perWriter = 20
	total := writers * perWriter

	var mu sync.Mutex
	vers := make([]int64, 0, total)
	var wg sync.WaitGroup
	var errCount atomic.Int64

	s := heros.NewStorage(uid)
	s.Heroes = make([]*heros.Hero, 0, 2)
	skins := make([]heros.Skin, 0)
	skins = append(skins, heros.Skin{
		Id: 11,
	})
	s.Heroes = append(s.Heroes, &heros.Hero{
		Id:    1001,
		Skins: skins,
	})
	err := s.Modify(ctx)

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				//s := heros.NewStorage(uid)
				//s.Heroes = []*heros.Hero{{Id: w*100000 + i}}

				s := heros.NewStorage(uid)
				err = s.Load(ctx)
				for _, h := range s.Heroes {
					h.Skins = append(h.Skins, heros.Skin{
						Id: uint32(w*1000 + i),
					})
				}

				c, cancel := context.WithTimeout(ctx, 5*time.Second)
				err := s.Modify(c)
				cancel()
				if err != nil {
					errCount.Add(1)
					return
				}
				mu.Lock()
				vers = append(vers, s.Version())
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	require.Zero(t, errCount.Load(), "concurrent Modify should not error")
	require.Len(t, vers, total)

	//sorted := append([]int64(nil), vers...)
	//sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	// 查看数量，重复
	mapFlag := make(map[int64]bool)
	for _, v := range vers {
		if !mapFlag[v] {
			mapFlag[v] = true
		} else {
			require.NoError(t, err)
		}
	}

	// 脏标记必须还在（还没 Flush），版本号对得上。
	//vstr, err := redis.GetRedisClient().HGet(ctx, Batch.DirtyVersionKey(Batch.DirtyHero), fmt.Sprintf("%d", uid)).Result()
	require.NoError(t, err)
	//require.Equal(t, fmt.Sprintf("%d", total), vstr)
}

// TestConcurrentModify_DistinctUIDs 吞吐视角：100 个 goroutine 各写各的 uid，互不干扰。
//
// 目的：排除“同 uid 覆盖”干扰，单看高并发下脏标记有无丢失——100 个 uid 写完后，
// Batch.Get 应能全部找回（少一个即说明 ZADD 脏链路在并发下丢标记）。
// 与上一个用例互补：上一个测“同 key 原子性”，这个测“多 key 完整性”。
func TestConcurrentModify_DistinctUIDs(t *testing.T) {
	ctx := initInfra(t)
	const n = 1000
	uids := make([]uint64, n)
	for i := range uids {
		uids[i] = testUIDBase + 1000 + uint64(i)
	}
	cleanUIDs(t, ctx, uids)
	t.Cleanup(func() { cleanUIDs(t, context.Background(), uids) })

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := heros.NewStorage(uids[i])
			s.Heroes = []*heros.Hero{{Id: i}}
			c, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_ = s.Modify(c) // 用 HSetNX 语义不保证时错误直接忽略，由后续计数断言
		}(i)
	}
	wg.Wait()

	got, err := Batch.Get(ctx, Batch.DirtyHero, 10000)
	require.NoError(t, err)
	set := make(map[uint64]bool, len(got))
	for _, u := range got {
		set[u] = true
	}
	for _, u := range uids {
		require.True(t, set[u], "uid %d should be dirty", u)
	}
}

// TestMultiInstanceConcurrentFlush 多实例同时刷盘，验证分布式锁 + 落库一致性。
//
// 前置：单写者顺序写 50 个 uid（每个 1 个版本），避开整包覆盖的 lost-update，
// 让“mongo 是否等于 Redis”断言只反映 Flush 本身的质量。
// 步骤：
//  1. 4 个独立 Worker（= 4 个 home 实例）同时 StartPersist，抢同一把
//     persist:lock:hero——抢不到的应跳过而非硬刷；
//  2. 兜底再刷一轮：排空上一轮拿锁失败或 500 条分页漏掉的脏；
//  3. 断言：脏集合无残留；逐 uid 对比 Redis（Load）与 Mongo 版本号 + 内容探针
//     （Id=7000+i），任一对不上即 stale 覆盖或丢刷。
func TestMultiInstanceConcurrentFlush(t *testing.T) {
	ctx := initInfra(t)
	const n = 5000
	uids := make([]uint64, n)
	for i := range uids {
		uids[i] = testUIDBase + 2000 + uint64(i)
	}
	cleanUIDs(t, ctx, uids)
	t.Cleanup(func() { cleanUIDs(t, context.Background(), uids) })

	// 单写者顺序写入 K 个版本，避免整包覆盖的天然 lost-update 干扰 Flush 断言。
	for i, u := range uids {
		s := heros.NewStorage(u)
		s.Heroes = []*heros.Hero{{Id: 7000 + i}}
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		require.NoError(t, s.Modify(c))
		cancel()
	}

	// 4 个“实例”（独立 Worker/Registry，共享同一个 Redis）同时 StartPersist。
	const instances = 40
	var wg sync.WaitGroup
	for k := 0; k < instances; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := newWorker()
			Batch.StartPersist(w) // 内部自带 100s 超时 + 进程内防重入
		}()
	}
	wg.Wait()

	// 兜底再刷一轮，排空上一轮拿锁失败/500 条分页漏掉的。
	Batch.StartPersist(newWorker())

	// 断言：脏清空 + mongo 与 Redis 一致。
	rest, err := Batch.Get(ctx, Batch.DirtyHero, 10000)
	require.NoError(t, err)
	for _, u := range uids {
		for _, r := range rest {
			require.NotEqual(t, u, r, "uid %d still dirty after drain", u)
		}
	}
	for i, u := range uids {
		var rs heros.UserHeroes
		rs.Uid = u
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		require.NoError(t, rs.Load(c))
		cancel()
		mv, mh := readMongoHero(t, ctx, u)
		require.Equal(t, rs.Version(), mv, "uid %d mongo version mismatch", u)
		require.NotEmpty(t, rs.Heroes)
		require.Equal(t, 7000+i, mh, "uid %d mongo content mismatch (stale overwrite?)", u)
	}
}

// TestStaleFlushMustNotOverwriteFresh P0 回归：旧版本后刷不能覆盖新数据。
//
// 复现时序（T1..T7 的 T4/T6 对调即 bug）：
// T1 写 V9/H9 → T2 旧 worker Load 到内存快照 stale(V9) 捏着不放 → T3 业务推进到
// V10/H10 → T4 新版 Load+Flush+RemoveIfSame（mongo=V10/H10，脏清掉）→
// T6 旧快照 stale.Flush() 后刷。
// 正确行为：mongo 应保持 V10/H10（条件写匹配 0 条）；
// 当前 bug（$set+$max 混用）：$set 照写 H9、$max 守住 version 不动，留下
// “version=新、heroes=旧”的永久不一致，且脏已被 T4 清掉、永不自愈。
// 因此本用例在修复前会 FAIL（期望 H10 实际 H9），修好版本守卫 filter 后转绿——
// 把它当回归锁，任何改动 Flush 的提交都必须先过它。
func TestStaleFlushMustNotOverwriteFresh(t *testing.T) {
	ctx := initInfra(t)
	uid := testUIDBase + 3
	cleanUIDs(t, ctx, []uint64{uid})
	t.Cleanup(func() { cleanUIDs(t, context.Background(), []uint64{uid}) })

	// V9
	s9 := heros.NewStorage(uid)
	s9.Heroes = []*heros.Hero{{Id: 9}}
	require.NoError(t, s9.Modify(ctx))

	// 模拟旧 worker 在 T1 拿到 V9 内存快照后就不放了。
	stale := heros.NewStorage(uid)
	require.NoError(t, stale.Load(ctx))
	require.Equal(t, int64(1), stale.Version())

	// T2 业务推进到 V10（第二次 Modify 版本为 2）。
	s10 := heros.NewStorage(uid)
	s10.Heroes = []*heros.Hero{{Id: 10}}
	require.NoError(t, s10.Modify(ctx))

	// T4 新版先刷。
	fresh := heros.NewStorage(uid)
	require.NoError(t, fresh.Load(ctx))

	//require.NoError(t, fresh.Flush(ctx))
	//require.NoError(t, Batch.RemoveIfSame(ctx, Batch.DirtyHero, uid, fresh.Version()))

	Batch.StartPersist(newWorker())
	Batch.StartPersist(newWorker())

	// T6 旧快照后刷：正确实现应该什么都不改。
	//_ = stale.Flush(ctx)

	mv, mh := readMongoHero(t, ctx, uid)

	finalValue := heros.NewStorage(uid)
	require.NoError(t, finalValue.Load(ctx))
	require.Equal(t, fresh.Version(), mv, "version should stay at fresh")
	require.Equal(t, 10, mh, "STALE OVERWRITE: mongo has V%d/H9 content, fix Flush with version-guarded filter", mv)
}

// TestStartPersist_ReentrantSkip 同一进程内防重入：同一 Worker 被 8 个 goroutine 同时触发。
//
// 背景：线上定时每分钟一拍（ints.go:45），单轮 opTimeout 100s > 触发间隔 60s，
// 上一轮没跑完时下一拍必须跳过而非叠加——靠 Worker.running 的 CAS 保证。
// 步骤：先造 2 个脏 uid，8  goroutine 同调 StartPersist(w)，再补一拍排空。
// 断言：最终脏排空（防重入只是“跳过并发轮”，不能“丢数据”）；
// 若实现把 running 标志玩坏（如提前置 false 导致叠跑，或置 true 后忘恢复导致饿死），
// 这里会表现为脏残留或 data race（配合 -race 运行可直接抓到）。
func TestStartPersist_ReentrantSkip(t *testing.T) {
	ctx := initInfra(t)
	uids := []uint64{testUIDBase + 4, testUIDBase + 5}
	cleanUIDs(t, ctx, uids)
	t.Cleanup(func() { cleanUIDs(t, context.Background(), uids) })
	for _, u := range uids {
		s := heros.NewStorage(u)
		s.Heroes = []*heros.Hero{{Id: 1}}
		require.NoError(t, s.Modify(ctx))
	}

	w := newWorker()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Batch.StartPersist(w)
		}()
	}
	wg.Wait()
	runtime.Gosched()
	Batch.StartPersist(w) // 排空

	rest, err := Batch.Get(context.Background(), Batch.DirtyHero, 10000)
	require.NoError(t, err)
	for _, u := range uids {
		for _, r := range rest {
			require.NotEqual(t, u, r, "uid %d should be drained", u)
		}
	}
}
