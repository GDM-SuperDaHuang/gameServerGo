// Package user 并发回归单测：把“高并发下会不会有问题”的分析结论逐条落成用例。
//
// 本文件全部是真实落库（复用 model_integration_test.go 的 requireIntegration，
// 连真实 Redis + Mongo；连不上自动 Skip）。断言的都是“正确行为”，
// 因此在 bug 修好之前，这些用例是红的——红本身就是 bug 存在的验证：
//
//   - TestConcurrency_FirstLogin_AllGetToken      ← P0-1 同 openid 并发首登：收敛同 uid 且同份凭证
//   - TestConcurrency_Storm_DistinctUsers_Unique   ← 风暴：10000 不同 openid，uid 与凭证互不相同
//   - TestConcurrency_ExpiredReissue_SingleToken  ← P0-2 过期重建签出多个有效 token
//   - TestRotation_OldTokenInvalidated            ← P0-3 轮换不删旧 token（串行即现）
//   - TestStaleWrite_UpdateUserInfo_NoRegression  ← P1-1 读-改-写把新 loginTimestamp 盖回旧值
//   - TestStaleCache_BanVisible                   ← P1-2 本机缓存 5 分钟看不到别机封号
//   - TestFirstLogin_UidWaste                     ← P1-3 首登惊群：1 个用户耗掉 N 个 uid
//
// 数据隔离与清理同 model_integration_test.go（loginType=99，itest- 前缀 openid，
// t.Cleanup 删 Redis 双向 token + Mongo users 文档 + localCache.Flush）。
package user

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"gameServer/pkg/mongodb"
	"gameServer/pkg/redis"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// startBarrier 让 N 个 goroutine 真正同时起跑，挤大竞态窗口，避免“串行化”造成假绿。
func startBarrier(n int) (start chan struct{}, done func()) {
	start = make(chan struct{})
	var once sync.Once
	return start, func() { once.Do(func() { close(start) }) }
}

// TestConcurrency_FirstLogin_AllGetToken P0-1：同一 openid 10 协程同时首登，
// uid 必须收敛到同一个（已做到），且人人都要拿到非空凭证。
// 现状：仅 1 人拿到 token，其余 9 人 CreateUser 走唯一键冲突回读 `return exist, "", nil`
// （model.go:320-323），空串当成功返回，客户端无凭证下次必登不上。
func TestConcurrency_FirstLogin_AllGetToken(t *testing.T) {
	requireIntegration(t)
	openid := itestOpenid("conc-first-token")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	const n = 10
	uids := make([]int64, n)
	tokens := make([]string, n)
	errs := make([]error, n)
	start, fire := startBarrier(n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			u, raw, err := GetOrCreateUser(itestLoginType, openid, "cn", "ca")
			if err == nil && u != nil {
				uids[i] = u.Uid
				tokens[i] = raw
			}
			errs[i] = err
		}(i)
	}
	fire()
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])
		require.Equal(t, uids[0], uids[i], "uid 必须幂等收敛")
	}
	created = append(created, uids[0])

	empty := 0
	distinct := map[string]int{}
	for _, tk := range tokens {
		if tk == "" {
			empty++
		} else {
			distinct[tk]++
		}
	}
	t.Logf("首登分发：空串=%d，不同样本=%v", empty, distinct)
	require.Zero(t, empty, "10 人首登应人人有凭证，实际 %d 人拿到空 token", empty)
	require.Len(t, distinct, 1, "同一用户只能有一份有效凭证，否则单点登录即破")
}

// TestConcurrency_Storm_DistinctUsers_Unique 登录风暴：10000 个不同 openid 同时首登，
// 每个用户必须分到互不相同的 uid 和互不相同的凭证（GenUid INCR 原子性的直接验证）。
// 与上一个用例互补：上一个测“同 openid 收敛”，这个测“多 openid 互不踩踏”。
func TestConcurrency_Storm_DistinctUsers_Unique(t *testing.T) {
	requireIntegration(t)
	base := itestOpenid("conc-storm")

	const n = 10000
	uids := make([]int64, n)
	tokens := make([]string, n)
	errs := make([]error, n)
	// 清理必须在起跑前注册：10k 落库失败一半时也能清掉已建部分，不留脏数据。
	// 切片按引用捕获，Cleanup 执行时读到的是填完的结果。
	t.Cleanup(func() { itestCleanupBatch(t, uids, tokens) })

	start, fire := startBarrier(n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			// 断言一律收敛到主 goroutine：testify require 禁止在子 goroutine 里 FailNow。
			u, raw, err := GetOrCreateUser(itestLoginType, fmt.Sprintf("%s-%d", base, i), "", "")
			if err == nil && u != nil {
				uids[i] = u.Uid
				tokens[i] = raw
			}
			if err != nil {
				errs[i] = err
			}
		}(i)
	}
	fire()
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoError(t, errs[i], "第 %d 路首登不应报错", i)
		require.NotZero(t, uids[i], "第 %d 路 uid 不应为 0", i)
		require.NotEmpty(t, tokens[i], "第 %d 路凭证不应为空", i)
	}
	seenUID := make(map[int64]int, n)
	seenTok := make(map[string]int, n)
	for i := 0; i < n; i++ {
		seenUID[uids[i]]++
		seenTok[tokens[i]]++
	}
	require.Len(t, seenUID, n, "10000 个不同用户 uid 必须互不相同")
	require.Len(t, seenTok, n, "10000 个不同用户凭证必须互不相同")
	t.Logf("storm: users=%d distinctUID=%d distinctToken=%d", n, len(seenUID), len(seenTok))
}

// itestCleanupBatch 批量清理：pipeline 一次删掉全部 token 双向 key，再一次删掉 Mongo 文档。
// 单路循环版 itestCleanup 在 n=10000 时需要数万次 RTT，这里必须批量，否则光清理就几十秒。
func itestCleanupBatch(t *testing.T, uids []int64, raws []string) {
	t.Helper()
	keys := make([]string, 0, 2*len(uids))
	uniqUID := make([]int64, 0, len(uids))
	seen := make(map[int64]bool, len(uids))
	for _, uid := range uids {
		if uid == 0 || seen[uid] {
			continue
		}
		seen[uid] = true
		uniqUID = append(uniqUID, uid)
		keys = append(keys, AccessTokenUidKey(uid))
	}
	for _, raw := range raws {
		if raw == "" {
			continue
		}
		keys = append(keys, AccessTokenKey(raw))
	}
	if len(keys) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		pipe := redis.GetRedisClient().Pipeline()
		for chunk := keys; len(chunk) > 0; {
			batch := 1000
			if len(chunk) < batch {
				batch = len(chunk)
			}
			pipe.Del(ctx, chunk[:batch]...)
			chunk = chunk[batch:]
		}
		_, _ = pipe.Exec(ctx)
	}
	if len(uniqUID) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		_, _ = mongodb.GetDB().Collection(userCollection).DeleteMany(ctx, bson.M{"uid": bson.M{"$in": uniqUID}})
	}
	localCache.Flush()
}

// TestConcurrency_ExpiredReissue_SingleToken P0-2：老用户 token 过期后 20 协程同时登录，
// 最终只能剩 1 个有效凭证（单点登录语义）。
// 现状：人人 `GetAccessTokenByUid` 拿到 Nil（model.go:348-349）→ 人人签新串
// → `SetAccessToken` 后写覆盖 uid->raw，但 N 个 token->uid 全有效。
func TestConcurrency_ExpiredReissue_SingleToken(t *testing.T) {
	requireIntegration(t)
	openid := itestOpenid("conc-reissue")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	first, raw0, err := GetOrCreateUser(itestLoginType, openid, "rn", "ra")
	require.NoError(t, err)
	require.NotEmpty(t, raw0)
	created = append(created, first.Uid)

	// 模拟过期：删掉双向映射。
	require.NoError(t, DeleteAccessTokenByUid(first.Uid))

	const n = 20
	tokens := make([]string, n)
	errs := make([]error, n)
	start, fire := startBarrier(n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, raw, err := GetOrCreateUser(itestLoginType, openid, "rn", "ra")
			tokens[i] = raw
			errs[i] = err
		}(i)
	}
	fire()
	wg.Wait()
	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])
	}

	distinct := map[string]int{}
	for _, tk := range tokens {
		if tk != "" {
			distinct[tk]++
		}
	}
	valid := 0
	for tk := range distinct {
		if _, err := GetUidByAccessToken(tk); err == nil {
			valid++
		}
	}
	t.Logf("签发不同串=%d，Redis 中有效串=%d", len(distinct), valid)
	require.Equal(t, 1, valid, "同一 uid 同时只能有 1 个有效凭证")
}

// TestRotation_OldTokenInvalidated P0-3（串行即可复现）：轮换签发新串后，旧串必须失效。
// 现状：`SetAccessToken` 只写新双向映射（token.go:31），旧 login:token:<old> 活到 TTL 到期。
func TestRotation_OldTokenInvalidated(t *testing.T) {
	requireIntegration(t)
	openid := itestOpenid("rotation")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	u, oldRaw, err := CreateUser(itestLoginType, openid, "n", "a")
	require.NoError(t, err)
	created = append(created, u.Uid)

	newRaw := newRawAccessToken()
	require.NoError(t, SetAccessToken(newRaw, u.Uid), "轮换写入不应失败")

	_, err = GetUidByAccessToken(oldRaw)
	require.ErrorIs(t, err, goredis.Nil, "旧凭证轮换后应立即失效")
}

// TestStaleWrite_UpdateUserInfo_NoRegression P1-1：别机刚刷新的 loginTimestamp，
// 不能被本机读-改-写盖回旧值。
// 复现：本机 FindUser 预热缓存（T0）→ 直写 Mongo 把 loginTimestamp 抬到 T0+3600
// （模拟另一台 gate 的 TouchLogin，绕过本机缓存）→ 本机 UpdateUserInfo 改个昵称
// （读本机旧缓存，全量 $set 回写）→ Mongo 的值应保持 T0+3600。
// 现状：被盖回 T0（model.go:426 UpdateUserInfo 用旧内存组装 $set）。
func TestStaleWrite_UpdateUserInfo_NoRegression(t *testing.T) {
	ctx := requireIntegration(t)
	openid := itestOpenid("stale-write")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	u, _, err := CreateUser(itestLoginType, openid, "n", "a")
	require.NoError(t, err)
	created = append(created, u.Uid)

	cached, err := FindUser(u.Uid) // 预热本机缓存：T0
	require.NoError(t, err)
	bumped := cached.LoginTimestamp + 3600

	// 另一实例的写入：直写 Mongo，不碰本机 localCache。
	_, err = mongodb.GetDB().Collection(userCollection).UpdateOne(ctx,
		bson.M{"uid": u.Uid}, bson.M{"$set": bson.M{"loginTimestamp": bumped}})
	require.NoError(t, err)

	// 本机读-改-写：只改昵称。
	_, err = UpdateUserInfo(u.Uid, func(info *UserInfo) { info.NickName = "patched" })
	require.NoError(t, err)

	localCache.Flush()
	require.Equal(t, bumped, itestReadMongo(t, ctx, u.Uid).LoginTimestamp,
		"别机的新时间戳不应被本机旧缓存回盖")
}

// TestStaleCache_BanVisible P1-2：别机封号后，本机 5 分钟缓存窗口内必须可见
// （或者说：至少要证明窗口存在，修法是广播失效/穿透读）。
// 复现：本机 FindUser 预热缓存（Status 0）→ 直写 Mongo status=1（模拟别机 GM 封号，
// 只调 UpdateFields 会顺带刷本机缓存，所以必须直写才等价于“别机”）→ 本机 FindUser。
// 现状：仍读到 Status 0，handler 的封号检查被绕过，直到缓存过期。
func TestStaleCache_BanVisible(t *testing.T) {
	ctx := requireIntegration(t)
	openid := itestOpenid("stale-ban")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	u, _, err := CreateUser(itestLoginType, openid, "n", "a")
	require.NoError(t, err)
	created = append(created, u.Uid)

	_, err = FindUser(u.Uid) // 预热本机缓存：Status 0
	require.NoError(t, err)

	// 别机的封号：直写 Mongo，不碰本机 localCache。
	_, err = mongodb.GetDB().Collection(userCollection).UpdateOne(ctx,
		bson.M{"uid": u.Uid}, bson.M{"$set": bson.M{"status": int32(1)}})
	require.NoError(t, err)

	stale, err := FindUser(u.Uid)
	require.NoError(t, err)
	localCache.Flush()
	fresh, err := FindUser(u.Uid)
	require.NoError(t, err)
	require.Equal(t, int32(1), fresh.Status, "Mongo 里确实已封号")
	require.Equal(t, int32(1), stale.Status, "本机缓存窗口内应看到封号，实际读到旧状态 0")
}

// TestFirstLogin_UidWaste P1-3：1 个新用户首登惊群只应消耗 1 个 uid。
// 现状：10 协程全在胜者 Insert 前通过 FindByLogin miss，人人 GenUid（INCR），
// 9 个号段被空耗（可通过 Redis uid:Inc 前后差值量化）。
func TestFirstLogin_UidWaste(t *testing.T) {
	ctx := requireIntegration(t)
	openid := itestOpenid("uid-waste")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	before, err := redis.GetRedisClient().Get(ctx, uidCounterKey).Int64()
	require.NoError(t, err)

	const n = 10
	uids := make([]int64, n)
	start, fire := startBarrier(n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if u, _, err := GetOrCreateUser(itestLoginType, openid, "cn", "ca"); err == nil && u != nil {
				uids[i] = u.Uid
			}
		}(i)
	}
	fire()
	wg.Wait()
	created = append(created, uids[0])

	after, err := redis.GetRedisClient().Get(ctx, uidCounterKey).Int64()
	require.NoError(t, err)
	consumed := after - before
	t.Logf("建 1 个用户消耗 uid 号段=%d", consumed)
	require.LessOrEqual(t, consumed, int64(2), "惊群下号段消耗应收敛，实际消耗 %d", consumed)
}
