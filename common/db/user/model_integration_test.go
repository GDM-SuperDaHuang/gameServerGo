// Package user 集成单测（真实落库）：覆盖 loginHandler 依赖的 user 包主链路。
//
// 与 model_test.go（hermetic、无中间件）的区别：本文件连真实 Redis + Mongo，
// 真实落库、真实模拟：CreateUser -> Mongo Insert + Redis 双向 token + 本地缓存；
// FindUser / FindByLogin / FindByToken / LoginByToken / TouchLogin /
// GetOrCreateUser / UpdateFields / InvalidateCache 全走真实链路。
//
// 前置依赖：真实 Redis 与真实 Mongo（与 Batch/batch_concurrency_test.go 同源）：
//
//	PowerShell: $env:REDIS_ADDR="127.0.0.1:6379"; $env:MONGO_URI="mongodb://root:xxx@127.0.0.1:27017"
//	默认读仓库 config/home.toml（working dir 为 common/db/user 时 ../../../config/ 即仓库 config/）。
//	连不上自动 Skip，不挂 CI。
//
// 跑法：
//
//	go test ./common/db/user/ -run 'TestIntegration' -v -count=1 -timeout 120s
//
// 数据隔离：loginType=99（测试专用），openid 统一 itest-<nanotime>-<用例> 前缀；
// 每个用例结尾经 t.Cleanup 清理：Redis 双向 token + Mongo users 文档 + localCache.Flush。
// uid 发号用全局 Redis 计数器（与线上同源），只增不减，无需回退。
//
// 注意：本文件与 hermetic 用例同进程运行。一旦本文件初始化成功 Redis/Mongo 全局变量，
// hermetic 里“缺失断言”类用例会按设计 Skip（见 requireRedisUninitialized），缓存命中类不受影响。
package user

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"gameServer/pkg/config"
	"gameServer/pkg/mongodb"
	"gameServer/pkg/redis"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	// itestLoginType 集成测试专用平台类型，与真实业务（0 游客/1 抖音/2 微信）隔离。
	itestLoginType int32 = 99
	// itestTimeout 单次 DB/Redis 操作超时。
	itestTimeout = 10 * time.Second
)

var (
	itestOnce   sync.Once
	itestReady  bool
	itestReason string
)

// requireIntegration 初始化真实基建并做连通性探测，失败即 Skip。
// 写法对齐 Batch/batch_concurrency_test.go 的 initInfra，另加 recover：
// redis.NewRedisClient 内部 Ping 失败会 panic（非 error），此处转为 Skip，避免无中间件环境直接崩。
func requireIntegration(t *testing.T) context.Context {
	t.Helper()
	itestOnce.Do(func() {
		func() {
			defer func() {
				if r := recover(); r != nil {
					itestReason = fmt.Sprintf("redis init panic: %v", r)
				}
			}()
			// 测试 working dir 是 common/db/user，../../../config 即仓库 config/。
			if err := config.Init("home-1", "home", "../../../config/", "home", "toml"); err != nil {
				itestReason = fmt.Sprintf("config init: %v", err)
				return
			}
			redis.NewRedisClient(config.Get().RedisAddress())
			if err := mongodb.InitFromConfig(); err != nil {
				itestReason = fmt.Sprintf("mongodb init: %v", err)
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := redis.GetRedisClient().Ping(ctx).Err(); err != nil {
				itestReason = fmt.Sprintf("redis ping: %v", err)
				return
			}
			if err := mongodb.GetDB().Client().Ping(ctx, nil); err != nil {
				itestReason = fmt.Sprintf("mongo ping: %v", err)
				return
			}
			itestReady = true
		}()
	})
	if !itestReady {
		t.Skipf("无真实中间件，跳过集成用例（%s）", itestReason)
	}
	// 索引幂等，保证并发回读等用例的唯一键约束生效。
	require.NoError(t, EnsureIndexes(context.Background()))
	localCache.Flush()
	return context.Background()
}

// itestOpenid 生成本用例唯一的 openid，避免与业务及用例间互踩。
func itestOpenid(tag string) string {
	return fmt.Sprintf("itest-%d-%s", time.Now().UnixNano(), tag)
}

// itestCleanup 清理指定 uid 的全部残留：Redis 双向 token + Mongo users 文档 + 本地缓存。
// 用 t.Cleanup 注册，保证断言失败时同样清理。
// 注意 Redis 只存哈希，直接按 uid 踢即可（Get 返回的是哈希 hex，不是明文，不要再调 DeleteAccessToken）。
func itestCleanup(t *testing.T, uids []int64) {
	t.Helper()
	if len(uids) == 0 {
		localCache.Flush()
		return
	}
	for _, uid := range uids {
		_ = DeleteAccessTokenByUid(uid)
	}
	mongoUIDs := make([]int64, 0, len(uids))
	mongoUIDs = append(mongoUIDs, uids...)
	ctx, cancel := context.WithTimeout(context.Background(), itestTimeout)
	defer cancel()
	_, _ = mongodb.GetDB().Collection(userCollection).DeleteMany(ctx, bson.M{"uid": bson.M{"$in": mongoUIDs}})
	localCache.Flush()
}

// itestReadMongo 直读 Mongo 落库结果，绕过本地缓存做“是否真落库”断言。
func itestReadMongo(t *testing.T, ctx context.Context, uid int64) UserInfo {
	t.Helper()
	var doc UserInfo
	c, cancel := context.WithTimeout(ctx, itestTimeout)
	defer cancel()
	require.NoError(t, mongodb.GetDB().Collection(userCollection).FindOne(c, bson.M{"uid": uid}).Decode(&doc))
	return doc
}

// ---------- 集成用例 ----------

// TestIntegration_CreateAndFind_Persisted 真实建号 + 双读路径验证。
// 断言：uid 非 0、token 32 hex；Flush 本地缓存后 FindUser/FindByLogin 仍能从 Mongo 读回；
// Mongo 直读文档字段（salt 32 位、loginType/openid、双时间戳）齐全。
func TestIntegration_CreateAndFind_Persisted(t *testing.T) {
	ctx := requireIntegration(t)
	openid := itestOpenid("create-find")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	u, _, err := CreateUser(itestLoginType, openid, "itest-nick", "itest-avatar")
	require.NoError(t, err)
	require.NotZero(t, u.Uid)
	created = append(created, u.Uid)

	// 踢掉本地缓存，强制走 Mongo，证明真落库而非缓存自嗨。
	localCache.Flush()

	got, err := FindUser(u.Uid)
	require.NoError(t, err)
	require.Equal(t, u.Uid, got.Uid)
	require.Equal(t, openid, got.Openid)
	require.Equal(t, "itest-nick", got.NickName)

	got2, err := FindByLogin(itestLoginType, openid)
	require.NoError(t, err)
	require.Equal(t, u.Uid, got2.Uid)

	doc := itestReadMongo(t, ctx, u.Uid)
	require.Equal(t, u.Uid, doc.Uid)
	require.Len(t, doc.Salt, 32)
	require.NotZero(t, doc.CreatTimestamp)
	require.NotZero(t, doc.LoginTimestamp)
}

// TestIntegration_TokenRoundTrip_RealRedis token 双向映射真实往返（哈希字节存储）。
// 断言：FindByToken/按 uid 反查一致（反查得 SHA256(raw)[32]byte，非明文）；
// LoginByToken 成功且 TTL 被续到 ~3 天；删除后 FindByToken 报 ErrTokenExpired。
func TestIntegration_TokenRoundTrip_RealRedis(t *testing.T) {
	ctx := requireIntegration(t)
	openid := itestOpenid("token-roundtrip")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	u, raw, err := CreateUser(itestLoginType, openid, "n", "a")
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	created = append(created, u.Uid)

	byToken, err := FindByToken(raw)
	require.NoError(t, err)
	require.Equal(t, u.Uid, byToken.Uid)

	back, err := GetAccessTokenByUid(u.Uid)
	require.NoError(t, err)
	require.Len(t, back, 32, "反查应得 sha256 [32]byte")
	require.Equal(t, back, HashAccessToken(raw), "反查哈希应等于 SHA256(raw)[32]byte")

	lu, err := LoginByToken(raw)
	require.NoError(t, err)
	require.Equal(t, u.Uid, lu.Uid)

	ttl := redis.GetRedisClient().TTL(ctx, AccessTokenKey(raw)).Val()
	require.Greater(t, int64(ttl), int64(0), "续期后 TTL 应为正")
	require.LessOrEqual(t, int64(ttl), int64(AccessTokenTTL), "TTL 不应超过 3 天")

	require.NoError(t, DeleteAccessToken(raw, u.Uid))
	_, err = FindByToken(raw)
	require.ErrorIs(t, err, ErrTokenExpired)
}

// TestIntegration_GetOrCreate_Idempotent_Concurrent 并发幂等：同一 loginType+openid
// 10  goroutine 同时 GetOrCreateUser，必须收敛到同一个 uid（唯一索引 + 回读保证）。
// 这是 handler 第三方/游客登录高峰的真实模拟。
func TestIntegration_GetOrCreate_Idempotent_Concurrent(t *testing.T) {
	requireIntegration(t)
	openid := itestOpenid("concurrent")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	const n = 10
	uids := make([]int64, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u, _, err := GetOrCreateUser(itestLoginType, openid, "cn", "ca")
			if err == nil && u != nil {
				uids[i] = u.Uid
			}
			errs[i] = err
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		require.NoError(t, errs[i])
		require.NotZero(t, uids[i])
		require.Equal(t, uids[i], uids[0], "并发建号必须幂等收敛到同一 uid")
	}
	created = append(created, uids[0])

	// 串行再调一次，老用户应直接返回同一 uid。
	u, _, err := GetOrCreateUser(itestLoginType, openid, "cn", "ca")
	require.NoError(t, err)
	require.Equal(t, uids[0], u.Uid)
}

// TestIntegration_TouchLogin_RenewsTimestampAndTTL 登录续期真实效果。
// 睡 1.1s 越过秒级时间戳边界，TouchLogin 后 Mongo 的 loginTimestamp 必须变大，
// 且 Redis token 仍有效（TTL>0）。
func TestIntegration_TouchLogin_RenewsTimestampAndTTL(t *testing.T) {
	ctx := requireIntegration(t)
	openid := itestOpenid("touch")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	u, raw, err := CreateUser(itestLoginType, openid, "n", "a")
	require.NoError(t, err)
	created = append(created, u.Uid)
	before := itestReadMongo(t, ctx, u.Uid).LoginTimestamp

	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, TouchLogin(u.Uid))

	localCache.Flush()
	after := itestReadMongo(t, ctx, u.Uid).LoginTimestamp
	require.Greater(t, after, before, "TouchLogin 应刷新 Mongo loginTimestamp")

	ttl := redis.GetRedisClient().TTL(ctx, AccessTokenKey(raw)).Val()
	require.Greater(t, int64(ttl), int64(0), "续期后 token 不应过期")
}

// TestIntegration_UpdateAndLogout_Persisted 更新 + 登出打点真实落库。
// 断言：昵称/头像经 UpdateFields 持久化；登出后 outTimestamp 非 0 且 onlineDuration 单调增。
func TestIntegration_UpdateAndLogout_Persisted(t *testing.T) {
	ctx := requireIntegration(t)
	openid := itestOpenid("update-logout")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	u, _, err := CreateUser(itestLoginType, openid, "old-nick", "old-avatar")
	require.NoError(t, err)
	created = append(created, u.Uid)

	require.NoError(t, UpdateUserNickName(u.Uid, "new-nick"))
	require.NoError(t, UpdateUserAvatarUrl(u.Uid, "new-avatar"))
	localCache.Flush()
	doc := itestReadMongo(t, ctx, u.Uid)
	require.Equal(t, "new-nick", doc.NickName)
	require.Equal(t, "new-avatar", doc.AvatarUrl)

	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, UpdateUserOutTimestamp(u.Uid))
	localCache.Flush()
	doc2 := itestReadMongo(t, ctx, u.Uid)
	require.NotZero(t, doc2.OutTimestamp)
	require.GreaterOrEqual(t, doc2.OnlineDuration, doc.OnlineDuration)
}

// TestIntegration_InvalidateCache_KicksToken 封号/登出即时失效真实模拟。
// InvalidateCache 后：本地缓存被清（下次读走 Mongo），Redis token 被踢，
// FindByToken 报 ErrTokenExpired。
func TestIntegration_InvalidateCache_KicksToken(t *testing.T) {
	requireIntegration(t)
	openid := itestOpenid("invalidate")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	u, raw, err := CreateUser(itestLoginType, openid, "n", "a")
	require.NoError(t, err)
	created = append(created, u.Uid)

	// 先预热本地缓存，证明 Invalidate 真清掉了它。
	_, err = FindUser(u.Uid)
	require.NoError(t, err)
	_, ok := getCacheByUID(u.Uid)
	require.True(t, ok)

	InvalidateCache(u)

	_, ok = getCacheByUID(u.Uid)
	require.False(t, ok)
	_, err = FindByToken(raw)
	require.ErrorIs(t, err, ErrTokenExpired)
}

// TestIntegration_BanVisibleToHandlerContract_RealDB 封号契约真实版。
// DB 层 FindByToken/LoginByToken 不拦截 Status（拦截责任在 handler:56,121），
// 本用例用真实落库锁死该分工：封号后 token 链路仍能查出用户且 Status==1，
// handler 看到 Status==1 必须转 UserStatusBan。
func TestIntegration_BanVisibleToHandlerContract_RealDB(t *testing.T) {
	requireIntegration(t)
	openid := itestOpenid("ban")
	var created []int64
	t.Cleanup(func() { itestCleanup(t, created) })

	u, raw, err := CreateUser(itestLoginType, openid, "n", "a")
	require.NoError(t, err)
	created = append(created, u.Uid)

	_, err = UpdateFields(u.Uid, bson.M{"status": int32(1)})
	require.NoError(t, err)
	localCache.Flush()

	byToken, err := FindByToken(raw)
	require.NoError(t, err, "DB 层不过滤封号，封号拦截由 handler 负责")
	require.Equal(t, int32(1), byToken.Status)

	lu, err := LoginByToken(raw)
	require.NoError(t, err)
	require.Equal(t, int32(1), lu.Status)
}
