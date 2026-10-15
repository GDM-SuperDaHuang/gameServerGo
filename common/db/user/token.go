package user

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"gameServer/pkg/redis"
	"time"
	//"github.com/redis/go-redis/v9"
)

// AccessToken 存放在 Redis（不再存 Mongo/UserInfo）。
// 双向索引，保证“续期/轮换”只用 uid 也能找到旧 token，Redis 中不存明文，只存字节：
//  1. login:token:<SHA256(raw)[32]byte> -> uid （快速登录主路径，TTL 滑动续期）
//  2. login:uid:<uid> -> SHA256(raw)[32]byte   （按 uid 续期/轮换/踢旧 token 用，TTL 与 1 一致）
//
// raw 只在客户端持有、服务端内存中短暂出现（签发/校验瞬间），绝不落 Redis。
// 过期即登录失效：Redis TTL 到期后两条 key 自动消失，FindByToken 视为过期。
const AccessTokenTTL = 3 * 24 * time.Hour

// HashAccessToken 对客户端明文凭证做 SHA-256，返回 [32]byte。
// Redis 落库一律用该 [32]byte（key 后缀与反向 value 均为其原始字节），不明文存 raw。
func HashAccessToken(raw string) [32]byte {
	return sha256.Sum256([]byte(raw))
}

// accessTokenKeyByHash 已有哈希 [32]byte 时直接拼 key，避免对哈希再哈希。
// key 为 "login:token:" + 32字节原始字节（Redis key 二进制安全）。
func accessTokenKeyByHash(sum [32]byte) string {
	return "login:token:" + string(sum[:])
}

// AccessTokenKey 快速登录主 key：入参仍为下发客户端的明文凭证，
// 函数内部做 SHA-256，返回的 key 后缀为 [32]byte 原始字节，不含明文。
func AccessTokenKey(raw string) string {
	return accessTokenKeyByHash(HashAccessToken(raw))
}

// AccessTokenUidKey 按 uid 反查当前有效 token 的 key（单点登录：一个 uid 同时只保留一个有效 token）。
func AccessTokenUidKey(uid int64) string {
	return fmt.Sprintf("login:uid:%d", uid)
}

// SetAccessToken 写入双向映射并设置统一 TTL。轮换/新建/续期统一走这里。
func SetAccessToken(raw string, uid int64) error {
	return SetAccessTokenWithTTL(raw, uid, AccessTokenTTL)
}

// SetAccessTokenWithTTL 同上，但允许调用方覆盖 TTL（单测/灰度用）。
// Redis 只存 SHA-256(raw) 的 [32]byte 原始字节（key 后缀与反向 value 均为字节），不明文存 raw。
func SetAccessTokenWithTTL(raw string, uid int64, ttl time.Duration) error {
	if raw == "" || uid == 0 {
		return errors.New("redis: empty token or uid")
	}
	if redis.GetRedisClient() == nil {
		return errors.New("redis: client not initialized")
	}
	c, cancel := context.WithTimeout(redis.GetCtx(), 5*time.Second)
	defer cancel()
	sum := HashAccessToken(raw)
	tokenKey := accessTokenKeyByHash(sum)
	uidKey := AccessTokenUidKey(uid)
	// 轮换时删旧 token key，保证单点登录：旧 hash 若存在且与新 hash 不同则清理，
	// 否则旧 login:token:<oldHash> 会活到 TTL 到期，旧凭证仍可登录。
	oldHashStr, _ := redis.GetRedisClient().Get(c, uidKey).Result()
	pipe := redis.GetRedisClient().Pipeline()
	pipe.Set(c, tokenKey, uid, ttl)
	pipe.Set(c, uidKey, sum[:], ttl)
	newHashStr := string(sum[:])
	if oldHashStr != "" && oldHashStr != newHashStr {
		pipe.Del(c, "login:token:"+oldHashStr)
	}
	_, err := pipe.Exec(c)
	return err
}

// GetUidByAccessToken 快速登录：token -> uid。入参为客户端明文，内部哈希后查 key。
// key 缺失（过期/无效）返回 redis.Nil。
func GetUidByAccessToken(raw string) (int64, error) {
	if raw == "" {
		return 0, errors.New("redis: empty token")
	}
	if redis.GetRedisClient() == nil {
		return 0, errors.New("redis: client not initialized")
	}
	c, cancel := context.WithTimeout(redis.GetCtx(), 5*time.Second)
	defer cancel()
	return redis.GetRedisClient().Get(c, AccessTokenKey(raw)).Int64()
}

// GetAccessTokenByUid 按 uid 反查当前有效 token 的 SHA-256 原始 [32]byte。
// 返回的不是明文，不可下发客户端、不可当凭证用，仅用于续期/轮换判断存在性。
// 明文不可逆：需要下发凭证时只能签发新 raw（见 GetOrCreateUser），不得指望从此处找回 raw。
func GetAccessTokenByUid(uid int64) ([32]byte, error) {
	if uid == 0 {
		return [32]byte{}, errors.New("redis: empty uid")
	}
	if redis.GetRedisClient() == nil {
		return [32]byte{}, errors.New("redis: client not initialized")
	}
	c, cancel := context.WithTimeout(redis.GetCtx(), 5*time.Second)
	defer cancel()
	s, err := redis.GetRedisClient().Get(c, AccessTokenUidKey(uid)).Result()
	if err != nil {
		return [32]byte{}, err
	}
	if len(s) != 32 {
		return [32]byte{}, errors.New("redis: invalid token hash length")
	}
	var sum [32]byte
	copy(sum[:], s)
	return sum, nil
}

// TouchAccessToken 滑动续期：同时延长 token->uid 与 uid->token 的 TTL，token 本体不变。
// 任一 key 已消失则返回对应错误（调用方一般视为 token 已过期）。
func TouchAccessToken(raw string, uid int64) error {
	return TouchAccessTokenWithTTL(raw, uid, AccessTokenTTL)
}

// TouchAccessTokenWithTTL 同上，允许覆盖 TTL。
func TouchAccessTokenWithTTL(raw string, uid int64, ttl time.Duration) error {
	if raw == "" || uid == 0 {
		return errors.New("redis: empty token or uid")
	}
	if redis.GetRedisClient() == nil {
		return errors.New("redis: client not initialized")
	}
	c, cancel := context.WithTimeout(redis.GetCtx(), 5*time.Second)
	defer cancel()
	pipe := redis.GetRedisClient().Pipeline()
	pipe.Expire(c, AccessTokenKey(raw), ttl)
	pipe.Expire(c, AccessTokenUidKey(uid), ttl)
	_, err := pipe.Exec(c)
	return err
}

// TouchAccessTokenByUid 只知道 uid 时的续期：按 uid 查到哈希字节后直接延长两条 key。
// 注意不能调 TouchAccessTokenWithTTL(raw...)，否则会对哈希再哈希。
// 反查不到说明该 uid 当前无有效 token，返回 redis.Nil（调用方忽略即可，Mongo 登录时间照常更新）。
func TouchAccessTokenByUid(uid int64) error {
	if uid == 0 {
		return errors.New("redis: empty uid")
	}
	if redis.GetRedisClient() == nil {
		return errors.New("redis: client not initialized")
	}
	c, cancel := context.WithTimeout(redis.GetCtx(), 5*time.Second)
	defer cancel()
	hashStr, err := redis.GetRedisClient().Get(c, AccessTokenUidKey(uid)).Result()
	if err != nil {
		return err
	}
	if hashStr == "" {
		return errors.New("redis: empty token hash")
	}
	pipe := redis.GetRedisClient().Pipeline()
	pipe.Expire(c, "login:token:"+hashStr, AccessTokenTTL)
	pipe.Expire(c, AccessTokenUidKey(uid), AccessTokenTTL)
	_, err = pipe.Exec(c)
	return err
}

// DeleteAccessToken 删除指定 token 的双向映射（轮换/登出/封号时调用）。
// 入参为客户端明文，内部求 [32]byte 后删除；为避免误删“已轮换出的新 token”，
// 只有当 uid->hash 仍指向该 hash 字节时才删反向 key。
func DeleteAccessToken(raw string, uid int64) error {
	if raw == "" || uid == 0 {
		return nil
	}
	if redis.GetRedisClient() == nil {
		return errors.New("redis: client not initialized")
	}
	c, cancel := context.WithTimeout(redis.GetCtx(), 5*time.Second)
	defer cancel()
	sum := HashAccessToken(raw)
	hashStr := string(sum[:])
	pipe := redis.GetRedisClient().Pipeline()
	pipe.Del(c, accessTokenKeyByHash(sum))
	pipe.Exec(c)
	// 反向 key 用 Lua 保证“值相等才删”，防止并发轮换时误删新 token（字节比较，二进制安全）。
	_, err := redis.GetRedisClient().Eval(c,
		`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`,
		[]string{AccessTokenUidKey(uid)}, hashStr).Result()
	return err
}

// DeleteAccessTokenByUid 按 uid 踢掉当前有效 token（封号/登出全端失效）。
// 直接按存量哈希字节删两条 key，不对哈希再哈希。
func DeleteAccessTokenByUid(uid int64) error {
	if uid == 0 {
		return nil
	}
	if redis.GetRedisClient() == nil {
		return errors.New("redis: client not initialized")
	}
	c, cancel := context.WithTimeout(redis.GetCtx(), 5*time.Second)
	defer cancel()
	hashStr, err := redis.GetRedisClient().Get(c, AccessTokenUidKey(uid)).Result()
	if err != nil {
		return err
	}
	if hashStr == "" {
		return nil
	}
	pipe := redis.GetRedisClient().Pipeline()
	pipe.Del(c, "login:token:"+hashStr)
	pipe.Del(c, AccessTokenUidKey(uid))
	_, err = pipe.Exec(c)
	return err
}
