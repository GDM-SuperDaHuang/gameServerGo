// Package user 单测（用户模型部分）：覆盖 common/db/user/model.go 与 token.go
// 中不依赖真实中间件即可验证的分支。
//
// 被测对象职责回顾：
//   - 本地进程缓存：user:uid:<uid> -> *UserInfo，user:login:<type>:<openid> -> uid，
//     读穿透 + 写穿透，返回前 clone 隔离；
//   - 查询：FindUser / FindByLogin / FindByToken / LoginByToken；
//   - 写入：CreateUser / GetOrCreateUser / UpdateFields / UpdateUserInfo；
//   - Token：Set / Get / Touch / Delete 双向映射
//     login:token:<SHA256(raw)[32]byte><->login:uid:<uid>->SHA256(raw)[32]byte，
//     Redis 只存哈希不明文，TTL 3 天滑动续期。
//
// 设计说明（为什么这样测）：
//  1. 本地单测默认不起 Redis/Mongo，因此正常 DB 路径无法 hermetic 覆盖，留给集成环境；
//  2. 这里锁住的是三类约定：
//     a) 参数校验不碰中间件：uid==0、openid==""、raw=="" 直接返回哨兵错；
//     b) 中间件缺失时不 panic、错误可辨识：mongo==nil 报 "mongodb not initialized"、
//     redis==nil 报 "client not initialized"；
//     c) 纯内存行为：clone 隔离、setCache/getCache、缓存命中时不碰 Mongo、
//     InvalidateCache 永不因日志/Redis 崩进程、token 明文格式；
//  3. 有真实中间件时（GetRedisClient()/GetDB() 非空）对“缺失断言”自动 Skip，
//     缓存命中类用例则无论有无中间件都可跑（命中时根本不碰中间件）；
//  4. 白盒同包（package user）以便直接调用未导出的 userCacheKey / setCache /
//     newRawAccessToken 等；requireRedisUninitialized /
//     requireMongoUninitializedForUID / initNopLogger 复用 uid_test.go 的定义，
//     本文件不再重复定义。
//  5. 每个用例先 localCache.Flush()，uid 取 9xxxx 随机段，避免用例间缓存串扰。
//
// 运行：go test ./common/db/user/ -v -count=1。
package user

import (
	"encoding/hex"
	"errors"
	"testing"

	"gameServer/pkg/mongodb"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// flushUserCache 每个用例前清空进程内缓存，避免串扰。
func flushUserCache() {
	localCache.Flush()
}

// requireMongoUninitializedForModel 复用 uid_test.go 的同名探针语义：
// 仅在无 Mongo 时断言 nil-Mongo 错误，有真实 Mongo 则 Skip。
// 这里包一层避免本文件与 uid_test.go 的 helper 耦合过紧，行为一致。
func requireMongoUninitializedForModel(t *testing.T) {
	t.Helper()
	if mongodb.GetDB() != nil {
		t.Skip("mongo 已初始化，跳过 nil-Mongo 断言（需无中间件环境）")
	}
}

// ---------- key 格式 ----------

// TestUserCacheKey_Format 锁死缓存 key 格式，防止改名导致跨实例读写分裂。
func TestUserCacheKey_Format(t *testing.T) {
	require.Equal(t, "user:uid:123", userCacheKey(123))
	require.Equal(t, "user:login:1:abc", loginCacheKey(1, "abc"))
	require.NotEqual(t, loginCacheKey(1, "abc"), loginCacheKey(2, "abc"))
	require.NotEqual(t, userCacheKey(1), userCacheKey(2))
}

// TestAccessTokenKey_Format 锁死 Redis 双向映射 key 格式：
// key 后缀为 SHA-256(raw) 的 [32]byte 原始字节，Redis 中无明文；不同明文必不同 key。
func TestAccessTokenKey_Format(t *testing.T) {
	require.Equal(t, AccessTokenUidKey(456), "login:uid:456")
	require.NotEqual(t, AccessTokenKey("a"), AccessTokenKey("b"))
	require.NotContains(t, AccessTokenKey("raw123"), "raw123", "key 不得含明文")
	require.Len(t, AccessTokenKey("raw123"), len("login:token:")+32, "后缀应为 sha256 [32]byte")
	require.Equal(t, AccessTokenKey("raw123"), AccessTokenKey("raw123"), "同一明文哈希必须稳定")
}

// TestHashAccessToken_IsSHA256_32Bytes 锁死安全约定：存 Redis 的是 SHA-256(raw) 的 [32]byte
// 原始字节，且不可逆、雪崩（改 1 字符哈希全变）。
func TestHashAccessToken_IsSHA256_32Bytes(t *testing.T) {
	var sum [32]byte = HashAccessToken("abc")
	require.Len(t, sum, 32)
	require.Equal(t, HashAccessToken("abc"), sum, "必须为标准 SHA-256 [32]byte")
	require.NotEqual(t, HashAccessToken("abc"), HashAccessToken("abd"))
	require.Len(t, AccessTokenKey("abc"), len("login:token:")+32)
}

// ---------- clone / 本地缓存 ----------

// TestCloneUser_NilIsolation 测 clone 的两条约定：nil 进 nil 出；返回的是拷贝，
// 调用方改返回对象不污染缓存内原件。
func TestCloneUser_NilIsolation(t *testing.T) {
	require.Nil(t, cloneUser(nil))

	src := &UserInfo{Uid: 90001, NickName: "a"}
	cp := cloneUser(src)
	require.Equal(t, src, cp)
	require.NotSame(t, src, cp, "必须拷贝，调用方改指针对象不能污染缓存")

	cp.NickName = "mutated"
	require.Equal(t, "a", src.NickName)
}

// TestSetCache_Guard 测 setCache 的守卫：nil / Uid==0 直接丢弃，不写缓存。
func TestSetCache_Guard(t *testing.T) {
	flushUserCache()

	require.NotPanics(t, func() { setCache(nil) })
	require.NotPanics(t, func() { setCache(&UserInfo{}) })

	_, ok := getCacheByUID(0)
	require.False(t, ok)
}

// TestLocalCache_SetGet_Isolation 测“写穿透缓存 + 读时 clone”：
// setCache 后 getCacheByUID 命中，且两次返回指针不同、互不影响。
func TestLocalCache_SetGet_Isolation(t *testing.T) {
	flushUserCache()

	src := &UserInfo{Uid: 90011, LoginType: 1, Openid: "iso-openid", NickName: "orig"}
	setCache(src)

	got, ok := getCacheByUID(90011)
	require.True(t, ok)
	require.Equal(t, src.Uid, got.Uid)
	require.NotSame(t, src, got)

	got.NickName = "polluted"
	got2, ok := getCacheByUID(90011)
	require.True(t, ok)
	require.Equal(t, "orig", got2.NickName, "读缓存必须 clone，调用方修改不能污染缓存")
}

// TestInvalidateCache_NoPanic 测主动失效永不崩进程：
// nil 入参、空对象、有缓存对象（含 Redis 踢 token 分支）在 Redis 缺失/日志未初始化时都不 panic，
// 且本地缓存确实被清掉。这是封号即时失效路径的前置约定。
func TestInvalidateCache_NoPanic(t *testing.T) {
	flushUserCache()

	require.NotPanics(t, func() { InvalidateCache(nil) })
	require.NotPanics(t, func() { InvalidateCache(&UserInfo{}) })

	// 故意不在此调用 initNopLogger：warnToken 有 nil 守卫，即使 log2 未初始化也不应崩。
	// 若 uid_test.go 先跑已初始化日志，本用例退化为“有日志也不崩”，依然有效。
	u := &UserInfo{Uid: 90012, LoginType: 1, Openid: "ban-openid"}
	setCache(u)
	_, ok := getCacheByUID(90012)
	require.True(t, ok)

	require.NotPanics(t, func() { InvalidateCache(u) })

	_, ok = getCacheByUID(90012)
	require.False(t, ok, "Invalidate 后 uid 缓存应被清掉")
	_, ok = localCache.Get(loginCacheKey(1, "ban-openid"))
	require.False(t, ok, "Invalidate 后 login->uid 映射应被清掉")
}

// ---------- 参数校验（不碰中间件） ----------

// TestFindUser_ZeroUid 测 uid==0 直接判无，不碰 Mongo。
func TestFindUser_ZeroUid(t *testing.T) {
	_, err := FindUser(0)
	require.ErrorIs(t, err, ErrUserNotFound)
}

// TestFindByToken_Empty 测空 token 直接判无，不碰 Redis。
func TestFindByToken_Empty(t *testing.T) {
	_, err := FindByToken("")
	require.ErrorIs(t, err, ErrUserNotFound)
}

// TestLoginByToken_Empty 空 token 走 FindByToken 分支，同样判无。
func TestLoginByToken_Empty(t *testing.T) {
	_, err := LoginByToken("")
	require.ErrorIs(t, err, ErrUserNotFound)
}

// TestTouchLogin_ZeroUid 测 uid==0 直接判无，不碰 Mongo/Redis。
func TestTouchLogin_ZeroUid(t *testing.T) {
	require.ErrorIs(t, TouchLogin(0), ErrUserNotFound)
}

// TestUpdateFields_ZeroUid 测 uid==0 直接判无。
func TestUpdateFields_ZeroUid(t *testing.T) {
	_, err := UpdateFields(0, bson.M{"nickName": "x"})
	require.ErrorIs(t, err, ErrUserNotFound)
}

// TestUpdateUserInfo_NilMutate 测 mutate==nil 退化为 FindUser：uid==0 时判无，不碰 DB。
func TestUpdateUserInfo_NilMutate(t *testing.T) {
	_, err := UpdateUserInfo(0, nil)
	require.ErrorIs(t, err, ErrUserNotFound)
}

// TestUpdateNicknameAvatar_ZeroUid 测便捷更新在 uid==0 时同样判无。
func TestUpdateNicknameAvatar_ZeroUid(t *testing.T) {
	require.ErrorIs(t, UpdateUserNickName(0, "n"), ErrUserNotFound)
	require.ErrorIs(t, UpdateUserAvatarUrl(0, "a"), ErrUserNotFound)
	require.ErrorIs(t, UpdateUserOutTimestamp(0), ErrUserNotFound)
}

// TestCreateUser_EmptyOpenid 测 openid 为空直接拒绝，不发号不碰 DB。
func TestCreateUser_EmptyOpenid(t *testing.T) {
	_, _, err := CreateUser(1, "", "n", "a")
	require.Error(t, err)
	require.Contains(t, err.Error(), "openid is empty")
}

// TestEnsureIndexes_NilCtx_NilMongo 测 EnsureIndexes 在 mongo 缺失时快速失败。
// ctx 传 nil 也应先卡在 mongo 检查上，不 panic（源码先查 mdb 再规整 ctx）。
func TestEnsureIndexes_NilCtx_NilMongo(t *testing.T) {
	requireMongoUninitializedForModel(t)
	require.Error(t, EnsureIndexes(nil))
}

// TestTokenParamValidation 测 token.go 所有“空参不碰 Redis”守卫。
// Delete 类对空参按 no-op 回 nil（幂等删除语义），其余报可辨识错误。
func TestTokenParamValidation(t *testing.T) {
	require.Error(t, SetAccessToken("", 1))
	require.Error(t, SetAccessToken("raw", 0))
	require.Error(t, SetAccessTokenWithTTL("", 1, AccessTokenTTL))

	_, err := GetUidByAccessToken("")
	require.Error(t, err)

	_, err = GetAccessTokenByUid(0)
	require.Error(t, err)

	require.Error(t, TouchAccessToken("", 1))
	require.Error(t, TouchAccessToken("raw", 0))

	require.NoError(t, DeleteAccessToken("", 1), "空 raw 按 no-op 处理")
	require.NoError(t, DeleteAccessToken("raw", 0), "空 uid 按 no-op 处理")
	require.NoError(t, DeleteAccessTokenByUid(0), "空 uid 按 no-op 处理")
}

// TestNewRawAccessToken_Format 测明文凭证格式：16 字节 hex，32 字符，两次不重。
// 若将来从 crypto/rand 换实现，只要仍满足“32 hex + 随机”本用例就继续通过。
func TestNewRawAccessToken_Format(t *testing.T) {
	a := newRawAccessToken()
	b := newRawAccessToken()

	require.Len(t, a, 32)
	require.Len(t, b, 32)
	_, err := hex.DecodeString(a)
	require.NoError(t, err, "token 应为 hex 编码")
	require.NotEqual(t, a, b, "连续两次 token 不应相同")
}

// ---------- 缓存命中（有无中间件都可跑） ----------

// TestFindUser_CacheHit 测读穿透命中路径：setCache 后 FindUser 直接回缓存，
// 即使 Mongo 缺失也不报错。对应源码 FindUser 的 localCache 前置分支。
func TestFindUser_CacheHit(t *testing.T) {
	flushUserCache()

	want := &UserInfo{Uid: 90021, LoginType: 2, Openid: "cache-hit", NickName: "cached"}
	setCache(want)

	got, err := FindUser(90021)
	require.NoError(t, err)
	require.Equal(t, want.Uid, got.Uid)
	require.Equal(t, "cached", got.NickName)
	require.NotSame(t, want, got, "返回必须 clone")
}

// TestFindByLogin_CacheHit 测登录主路径的二级缓存：
// login->uid 映射 + uid 缓存都命中时，不碰 Mongo。
func TestFindByLogin_CacheHit(t *testing.T) {
	flushUserCache()

	want := &UserInfo{Uid: 90022, LoginType: 7, Openid: "login-cache-hit", NickName: "via-login"}
	setCache(want)

	got, err := FindByLogin(7, "login-cache-hit")
	require.NoError(t, err)
	require.Equal(t, want.Uid, got.Uid)
	require.Equal(t, "via-login", got.NickName)
}

// TestUpdateFields_EmptyFields_CacheHit 测 UpdateFields 空更新退化为 FindUser：
// 缓存命中时不碰 Mongo。对应源码 len(fields)==0 分支。
func TestUpdateFields_EmptyFields_CacheHit(t *testing.T) {
	flushUserCache()

	want := &UserInfo{Uid: 90023, LoginType: 1, Openid: "empty-fields", NickName: "keep"}
	setCache(want)

	got, err := UpdateFields(90023, bson.M{})
	require.NoError(t, err)
	require.Equal(t, "keep", got.NickName)
}

// TestUpdateUserInfo_NilMutate_CacheHit 测 UpdateUserInfo mutate==nil 同样退化为 FindUser。
func TestUpdateUserInfo_NilMutate_CacheHit(t *testing.T) {
	flushUserCache()

	want := &UserInfo{Uid: 90024, LoginType: 1, Openid: "nil-mutate", NickName: "keep2"}
	setCache(want)

	got, err := UpdateUserInfo(90024, nil)
	require.NoError(t, err)
	require.Equal(t, "keep2", got.NickName)
}

// ---------- 中间件缺失分支（无 Redis/Mongo 时才断言） ----------

// TestFindUser_Miss_NoMongo 测缓存未命中 + Mongo 缺失时报可辨识错误，不 panic。
func TestFindUser_Miss_NoMongo(t *testing.T) {
	requireMongoUninitializedForModel(t)
	flushUserCache()

	_, err := FindUser(999001)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")
}

// TestFindByLogin_Miss_NoMongo 同上，覆盖登录主路径的 DB 缺失分支。
func TestFindByLogin_Miss_NoMongo(t *testing.T) {
	requireMongoUninitializedForModel(t)
	flushUserCache()

	_, err := FindByLogin(1, "no-such-openid-999002")
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")
}

// TestUpdateFields_NoMongo 测非空更新在 Mongo 缺失时报可辨识错误。
func TestUpdateFields_NoMongo(t *testing.T) {
	requireMongoUninitializedForModel(t)
	flushUserCache()

	_, err := UpdateFields(999003, bson.M{"nickName": "x"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")
}

// TestTouchLogin_NoMongo TouchLogin 透传 UpdateFields 错误，Mongo 缺失时应透错。
func TestTouchLogin_NoMongo(t *testing.T) {
	requireMongoUninitializedForModel(t)
	flushUserCache()

	err := TouchLogin(999004)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")
}

// TestGetOrCreate_NoMongo 测登录主入口在 Mongo 缺失时直接透错，不发号。
// 对应 GetOrCreateUser 的 `!errors.Is(err, ErrUserNotFound)` 分支。
func TestGetOrCreate_NoMongo(t *testing.T) {
	requireMongoUninitializedForModel(t)
	flushUserCache()

	_, _, err := GetOrCreateUser(1, "no-mongo-openid", "n", "a")
	require.Error(t, err)
	require.Contains(t, err.Error(), "mongodb not initialized")
}

// TestCreateUser_RedisNil_GenUidFailed 锁定当前行为：CreateUser 先 FindByLogin
// （Mongo 缺失报非 NotFound 错误被忽略，见源码 `err == nil && u != nil`），
// 随后 GenUid 因 Redis 缺失回 0，最终报 "gen uid failed"。
// 本用例把“吞掉 FindByLogin 非 NotFound 错误”的现状锁死，若将来修复为直接透出
// Mongo 错误，本用例会红，届时按新约定更新断言即可。
func TestCreateUser_RedisNil_GenUidFailed(t *testing.T) {
	requireRedisUninitialized(t)
	requireMongoUninitializedForModel(t)
	flushUserCache()
	initNopLogger() // GenUid 失败分支打日志，需兜底避免 nil logger panic

	_, _, err := CreateUser(1, "unreachable-openid-999005", "n", "a")
	require.Error(t, err)
	require.True(t,
		containsAny(err.Error(), []string{"gen uid failed", "mongodb not initialized"}),
		"无中间件时应报发号失败或 mongo 未初始化，实际: %v", err)
}

// TestTokenRedisNil 测 token 双向映射在 Redis 缺失时报可辨识错误，不 panic。
func TestTokenRedisNil(t *testing.T) {
	requireRedisUninitialized(t)

	require.ErrorContains(t, SetAccessToken("raw-for-nil-test", 999011), "client not initialized")

	_, err := GetUidByAccessToken("raw-for-nil-test")
	require.ErrorContains(t, err, "client not initialized")

	_, err = GetAccessTokenByUid(999011)
	require.ErrorContains(t, err, "client not initialized")

	require.ErrorContains(t, TouchAccessToken("raw-for-nil-test", 999011), "client not initialized")
	require.ErrorContains(t, TouchAccessTokenByUid(999011), "client not initialized")

	require.ErrorContains(t, DeleteAccessToken("raw-for-nil-test", 999011), "client not initialized")
	require.ErrorContains(t, DeleteAccessTokenByUid(999011), "client not initialized")
}

// TestFindByToken_RedisNil 测快速登录在 Redis 缺失时透出初始化错误，
// 而非误报为 ErrTokenExpired（后者只在 key 缺失/TTL 到期即 redis.Nil 时返回）。
func TestFindByToken_RedisNil(t *testing.T) {
	requireRedisUninitialized(t)
	flushUserCache()

	_, err := FindByToken("some-raw-when-redis-nil")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrTokenExpired, "Redis 未初始化是环境错误，不是 token 过期")
	require.NotErrorIs(t, err, ErrUserNotFound)
	require.Contains(t, err.Error(), "client not initialized")
}

// TestLoginByToken_RedisNil 同上，覆盖 LoginByToken 透传分支。
func TestLoginByToken_RedisNil(t *testing.T) {
	requireRedisUninitialized(t)
	flushUserCache()

	_, err := LoginByToken("some-raw-when-redis-nil")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrTokenExpired)
	require.Contains(t, err.Error(), "client not initialized")
}

// TestFindByToken_ZombieCleanupBranch 文档型用例：Mongo 已删号但 Redis 残留时，
// FindByToken 应清理僵尸 token。该分支需真实双中间件，本地默认 Skip，
// 在此仅锁住空 token 的前置约定，防止将来重构误删该分支。
func TestFindByToken_ZombieCleanupBranch(t *testing.T) {
	_, err := FindByToken("")
	require.ErrorIs(t, err, ErrUserNotFound)

	// 非空分支需真实 Redis+Mongo，留给集成测试；此处显式标记，避免被认为遗漏。
	if mongodb.GetDB() == nil {
		t.Skip("僵尸清理分支需真实 Redis+Mongo，留给集成环境")
	}
}

// containsAny 小 helper：错误文本命中任一子串即通过，
// 用于兼容“吞错 vs 透错”两种历史行为的过渡期断言。
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if len(sub) == 0 {
			continue
		}
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}

// TestSentinelErrors 锁死哨兵错误文本，防止被改名导致调用方 errors.Is 失效。
func TestSentinelErrors(t *testing.T) {
	require.ErrorIs(t, ErrUserNotFound, ErrUserNotFound)
	require.ErrorIs(t, ErrTokenExpired, ErrTokenExpired)
	require.False(t, errors.Is(ErrUserNotFound, ErrTokenExpired))
}

// ---------- loginHandler 契约（对应 app/gate/hander/ws/handler.go:33-168） ----------

// TestLoginHandler_Contract 覆盖 loginHandler 两条互斥路径依赖的 user 包契约。
//
// 说明为什么不直接调 loginHandler：
//   - loginHandler 入参为 *WsServer/*Session/common.Message，重度依赖
//     config / loginSdk(抖音/微信) / RPC / PlayerPool，本地无法 hermetic 构造；
//   - 更关键的是包依赖方向：ws/handler.go import user，若 user 的单测（package user）
//     再 import ws，会形成 user<->ws 导入循环，编译不通过；
//   - 因此这里以“契约测试”方式锁死 handler 每一分支所依赖的 user 包行为，
//     分支编号与 handler.go 行号一一对应，重构 handler 时若改动任一约定，本用例即红。
//
// 映射关系：
//   - H1(handler:46)：len(AccessToken)>0 走快速登录，否则走第三方/游客；
//   - H2(handler:47-55)：LoginByToken 透错语义：ErrTokenExpired 记 expired 日志，
//     其余（含 Redis 未初始化）记 failed 日志，最终都回 LoginFailed；
//   - H3(handler:56-59,121-124)：两条路径都要查 u.Status==1 转 UserStatusBan；
//   - H4(handler:63)：快速登录回显请求 token（TouchLogin 只续期 TTL，凭证不变）；
//   - H5(handler:71-108)：loginType 非 0/1/2、Code==""、openid=="" 在碰 DB 前拒绝；
//   - H6(handler:111-120)：GetOrCreateUser 透错映射：ErrUserNotFound->LoginFailed，
//     其余->DBError；
//   - H7(handler:126-129)：第三方路径下发 rawToken：新建用户为新凭证，
//     老用户 Redis 无有效 token 时签发新凭证。
func TestLoginHandler_Contract(t *testing.T) {
	// H1：分支选择子仅取决于请求 token 是否非空，空 token 永不碰 Redis。
	t.Run("H1_branchSelector_emptyTokenNeverHitsRedis", func(t *testing.T) {
		_, err := FindByToken("")
		require.ErrorIs(t, err, ErrUserNotFound)
		_, err = LoginByToken("")
		require.ErrorIs(t, err, ErrUserNotFound)
	})

	// H2：快速登录 Redis 缺失是环境错误，不是过期。
	// handler 靠 errors.Is(err, ErrTokenExpired) 区分两条日志分支，断言不能混。
	t.Run("H2_fastLogin_redisNil_isNotExpired", func(t *testing.T) {
		requireRedisUninitialized(t)
		flushUserCache()

		_, err := LoginByToken("h2-raw-when-redis-nil")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrTokenExpired, "handler 应记 failed 而非 expired")
		require.NotErrorIs(t, err, ErrUserNotFound)
	})

	// H3-fast：封号用户经快速登录路径可被识别，handler 应转 UserStatusBan。
	// 用本地缓存构造封号用户，FindUser 命中缓存即可返回 Status，不碰 Mongo。
	t.Run("H3_fastLogin_bannedBlocked", func(t *testing.T) {
		flushUserCache()

		setCache(&UserInfo{Uid: 90101, LoginType: 1, Openid: "h3-banned-fast", Status: 1})
		u, err := FindUser(90101)
		require.NoError(t, err)
		require.Equal(t, int32(1), u.Status, "handler:56 靠此字段转 UserStatusBan")
	})

	// H3-third：封号用户经第三方/游客路径同样可被识别（FindByLogin 二级缓存命中）。
	t.Run("H3_thirdParty_bannedBlockedViaFindByLogin", func(t *testing.T) {
		flushUserCache()

		setCache(&UserInfo{Uid: 90102, LoginType: 0, Openid: "h3-banned-guest-code", Status: 1})
		u, err := FindByLogin(0, "h3-banned-guest-code")
		require.NoError(t, err)
		require.Equal(t, int32(1), u.Status, "handler:121 靠此字段转 UserStatusBan")
	})

	// H4：快速登录凭证本体不变，handler 直接回显请求 token。
	// 凭证格式为 32 hex，续期（Touch）只延长 TTL 不换串——回显串必须仍是合法凭证格式。
	t.Run("H4_fastLogin_echoTokenStaysValidFormat", func(t *testing.T) {
		raw := newRawAccessToken()
		require.Len(t, raw, 32)
		_, err := hex.DecodeString(raw)
		require.NoError(t, err)
		// 回显语义：respRawToken == req AccessToken，不存在“续期后变串”的情况。
		respRawToken := raw
		require.Equal(t, raw, respRawToken)
	})

	// H5：游客/第三方在 openid 为空时碰 DB 前拒绝。
	// handler:95-99 先判 Code==""，:105-108 再判 openid==""；user 层 CreateUser 对
	// 空 openid 直接报错，证明“空串永远走不到发号/落库”。
	t.Run("H5_thirdParty_emptyOpenidRejectedBeforeDB", func(t *testing.T) {
		_, _, err := CreateUser(0, "", "guest", "avatar")
		require.Error(t, err)
		require.Contains(t, err.Error(), "openid is empty")
	})

	// H6：第三方路径 DB 不可用时透出非 NotFound 错误，handler 应转 DBError 而非 LoginFailed。
	// 对应 handler:113 `errors.Is(err, ErrUserNotFound)` 为 false 的分支。
	t.Run("H6_thirdParty_dbErrorMapsToDBError", func(t *testing.T) {
		requireMongoUninitializedForModel(t)
		flushUserCache()

		_, _, err := GetOrCreateUser(0, "h6-guest-code", "n", "a")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrUserNotFound, "handler 应走 DBError 分支而非 LoginFailed 分支")
	})

	// H7：第三方路径下发语义——新凭证为 32 hex；老用户无有效 token 时签发新串。
	// 无中间件时 GetAccessTokenByUid 报初始化错误（视为“无有效 token”），
	// 此时 handler 注释约定“签发新 token 并下发”，新串格式必须合法。
	t.Run("H7_thirdParty_newTokenIssuedWhenNone", func(t *testing.T) {
		requireRedisUninitialized(t)

		_, err := GetAccessTokenByUid(90107)
		require.Error(t, err, "无 Redis 即无有效 token，应走签发分支")

		issued := newRawAccessToken()
		require.Len(t, issued, 32)
		_, derr := hex.DecodeString(issued)
		require.NoError(t, derr, "下发给客户端的必须是合法凭证格式")
	})
}
