package Batch

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gameServer/pkg/redis"
	"gameServer/pkg/utils"

	goredis "github.com/redis/go-redis/v9"
)

// DefaultBatchSize 单次落库扫描条数默认值。
const DefaultBatchSize = 0

// saveAndDirtyScript 原子执行：版本号自增 + 写业务数据 + 标记脏数据。
//
// KEYS[1] 业务数据 key（STRING，JSON 对象）
// KEYS[2] 脏数据 ZSet key
// KEYS[3] 脏数据版本号 Hash key
// ARGV[1] uid 字符串
// ARGV[2] 业务数据 JSON（对象，version 字段会被服务端覆盖，无需调用方保证正确）
// ARGV[3] 脏时间戳（毫秒）
//
// 返回自增后的新版本号。单 Redis 下 Lua 脚本串行执行，多实例并发 Modify
// 各自拿到不同版本号，不会丢失更新，也不会出现数据与脏版本号不一致。
var saveAndDirtyScript = goredis.NewScript(`
local data = cjson.decode(ARGV[2])
local ver = redis.call('HINCRBY', KEYS[3], ARGV[1], 1)
data['version'] = ver
data['Version'] = nil
redis.call('SET', KEYS[1], cjson.encode(data))
redis.call('ZADD', KEYS[2], ARGV[3], ARGV[1])
return ver
`)

// removeIfSameScript 原子执行：版本号一致才清除脏标记。
//
// KEYS[1] 脏数据 ZSet key
// KEYS[2] 脏数据版本号 Hash key
// ARGV[1] uid 字符串
// ARGV[2] 落库时从 Redis 读到的版本号
//
// 返回 1 表示本次清除，0 表示版本号已变化或标记已消失（无需处理，非错误）。
var removeIfSameScript = goredis.NewScript(`
local cur = redis.call('HGET', KEYS[2], ARGV[1])
if not cur then
  return 0
end
if cur ~= ARGV[2] then
  return 0
end
redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('HDEL', KEYS[2], ARGV[1])
return 1
`)

// SaveAndMarkDirty 原子保存业务数据并标记脏，返回新的版本号。
// payload 必须是 JSON 对象；其内部 version 字段会被 Redis 端覆盖，调用方只需
// 把返回的版本号回填到内存对象，保证与 Redis 一致。
func SaveAndMarkDirty(ctx context.Context, dataKey string, t DirtyType, uid uint64, payload []byte) (int64, error) {
	if dataKey == "" {
		return 0, errors.New("batch: dataKey is empty")
	}
	if len(payload) == 0 || payload[0] != '{' {
		return 0, fmt.Errorf("batch: payload must be a JSON object")
	}
	rdb := redis.GetRedisClient()
	if rdb == nil {
		return 0, errors.New("batch: redis client is nil")
	}

	ver, err := saveAndDirtyScript.Run(
		ctx, rdb,
		[]string{dataKey, DirtyZSetKey(t), DirtyVersionKey(t)},
		utils.ToString(uid), string(payload), time.Now().UnixMilli(),
	).Int64()
	if err != nil {
		return 0, err
	}
	return ver, nil
}

// RemoveIfSame 仅当脏版本号与落库版本号一致时清除脏标记，原子执行。
// 脏标记不存在或版本号已变化都返回 nil（无需处理，不是错误）。
func RemoveIfSame(ctx context.Context, t DirtyType, uid uint64, version int64) error {
	rdb := redis.GetRedisClient()
	if rdb == nil {
		return errors.New("batch: redis client is nil")
	}

	_, err := removeIfSameScript.Run(
		ctx, rdb,
		[]string{DirtyZSetKey(t), DirtyVersionKey(t)},
		utils.ToString(uid), strconv.FormatInt(version, 10),
	).Int64()
	return err
}

// 获取所有的需要批处理的uid
func Get(ctx context.Context, t DirtyType, limit int64) ([]uint64, error) {
	if limit <= 0 {
		limit = DefaultBatchSize
	}
	rdb := redis.GetRedisClient()
	if rdb == nil {
		return nil, errors.New("batch: redis client is nil")
	}
	list, err := rdb.ZRange(ctx, DirtyZSetKey(t), 0, limit-1).Result()
	if err != nil {
		return nil, err
	}

	result := make([]uint64, 0, len(list))
	for _, v := range list {
		// 脏 member 非法时直接跳过，避免解析失败变成 uid=0 误刷
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			continue
		}
		result = append(result, id)
	}
	return result, nil
}
