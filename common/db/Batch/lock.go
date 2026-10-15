package Batch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"gameServer/pkg/redis"

	goredis "github.com/redis/go-redis/v9"
)

// FlushLockTTL 落库分布式锁默认 TTL。
// 定时落库每分钟触发一次，TTL 大于触发间隔，保证同一时刻只有一个实例在刷盘；
// 持有者崩溃时锁自动过期，不会饿死其他实例。
const FlushLockTTL = 120 * time.Second

// unlockScript 仅当 token 匹配才释放锁，避免误删他人持有的锁。
var unlockScript = goredis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)

// FlushLockKey 落库分布式锁 key。
func FlushLockKey(t DirtyType) string {
	return "persist:lock:" + string(t)
}

func newLockToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("pid-%d-%d", os.Getpid(), time.Now().UnixNano())
}

// TryAcquireFlushLock 尝试获取某类数据的落库锁。
// 返回 token（释放时需原样传回）、是否获取成功。
func TryAcquireFlushLock(ctx context.Context, t DirtyType, ttl time.Duration) (string, bool, error) {
	if ttl <= 0 {
		ttl = FlushLockTTL
	}
	rdb := redis.GetRedisClient()
	if rdb == nil {
		return "", false, errors.New("batch: redis client is nil")
	}
	token := newLockToken()
	ok, err := rdb.SetNX(ctx, FlushLockKey(t), token, ttl).Result()
	if err != nil {
		return "", false, err
	}
	return token, ok, nil
}

// ReleaseFlushLock 释放落库锁，token 不匹配时不做任何事。
func ReleaseFlushLock(ctx context.Context, t DirtyType, token string) error {
	if token == "" {
		return nil
	}
	rdb := redis.GetRedisClient()
	if rdb == nil {
		return errors.New("batch: redis client is nil")
	}
	return unlockScript.Run(ctx, rdb, []string{FlushLockKey(t)}, token).Err()
}
