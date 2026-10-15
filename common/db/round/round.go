// Package rooming 记录玩家“正在哪个房间节点游戏”的临时状态。
// 该状态生命周期短、跨节点强一致要求高，因此只存 Redis（不走本地缓存，不落 Mongo）。
package round

import (
	"context"
	"fmt"
	"time"

	"gameServer/pkg/logger/log2"
	"gameServer/pkg/redis"

	"go.uber.org/zap"
)

const (
	roundIdKey = "round:Uid:%d" // value: 房间节点 id
	// ttl 兜底过期时间，防止异常退出后状态残留。
	uidOpTimeout = 5 * time.Second
)

func getRoundIdKey(uid int64) string {
	return fmt.Sprintf(roundIdKey, uid)
}

// 局id++
func IncrRoundId(uid int64) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), uidOpTimeout)
	defer cancel()
	rdb := redis.GetRedisClient()
	if rdb == nil {
		return 0
	}
	roundId, err := rdb.Incr(ctx, getRoundIdKey(uid)).Result()
	if err != nil {
		log2.Get().Error("GetRoomingId", zap.Error(err))
		return 0
	}
	return roundId
}
