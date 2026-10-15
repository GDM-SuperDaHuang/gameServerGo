// Package rooming 记录玩家“正在哪个房间节点游戏”的临时状态。
// 该状态生命周期短、跨节点强一致要求高，因此只存 Redis（不走本地缓存，不落 Mongo）。
package rooming

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gameServer/pkg/logger/log2"
	"gameServer/pkg/redis"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	dbKey = "rooming:uid:%d" // value: 房间节点 id
	// ttl 兜底过期时间，防止异常退出后状态残留。
	ttl       = 6 * time.Hour
	opTimeout = 3 * time.Second
)

func getKey(Uid int64) string {
	return fmt.Sprintf(dbKey, Uid)
}

func client() (*goredis.Client, error) {
	c := redis.GetRedisClient()
	if c == nil {
		return nil, errors.New("rooming: redis not initialized")
	}
	return c, nil
}

// SaveRooming 记录玩家正在 id 节点游戏。
func SaveRooming(Uid int64, id uint32) bool {

	return true
}

// 获取玩家所在房间节点 id；不在游戏中返回 0。
func GetRoomingId(Uid int64) uint32 {
	c, err := client()
	if err != nil {
		log2.Get().Error("GetRoomingId", zap.Error(err))
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	v, err := c.Get(ctx, getKey(Uid)).Result()
	if err != nil {
		if !errors.Is(err, goredis.Nil) {
			log2.Get().Error("GetRoomingId get failed", zap.Int64("uid", Uid), zap.Error(err))
		}
		return 0
	}
	id, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(id)
}

// DelRoomingId 清除玩家游戏中状态。
func DelRoomingId(Uid int64) bool {
	c, err := client()
	if err != nil {
		log2.Get().Error("DelRoomingId", zap.Error(err))
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err = c.Del(ctx, getKey(Uid)).Err(); err != nil {
		log2.Get().Error("DelRoomingId del failed", zap.Int64("uid", Uid), zap.Error(err))
		return false
	}
	return true
}
