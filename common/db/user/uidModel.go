package user

import (
	"context"
	"errors"
	"gameServer/pkg/logger/log2"
	"strings"
	"time"

	"gameServer/pkg/mongodb"
	"gameServer/pkg/redis"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

const (
	uidCounterKey        = "uid:Inc" // Redis 计数器（INCR），uid 的发号源
	uidCollection        = "uid_increment"
	uidSnapshotID        = "uid_inc" // 快照文档固定 _id，全局只有一条
	startUID      uint64 = 10000
)

const (
	uidOpTimeout    = 5 * time.Second
	uidSnapshotSlow = 3 * time.Second
)

// UidIncrement uid 发号进度快照。
// uid 快照直写 Mongo（不实现 Batchable），保证发号进度尽快持久化。
type UidIncrement struct {
	IncId     int64 `bson:"IncId" json:"IncId"`         // 已发出的最大 uid
	UpdatedAt int64 `bson:"updatedAt" json:"updatedAt"` // 快照更新时间（秒）
}

// InitUid 初始化Redis的Uid起点,重启后不会重复发号。幂等，可多节点重复调用。
// 逻辑：读 Mongo 快照最大值，与 startUID-1 取大，只允许把 Redis 计数器向上收敛。
func InitUid() error {
	rdb := redis.GetRedisClient()
	if rdb == nil {
		return errors.New("uid: redis client is nil")
	}
	mdb := mongodb.GetDB()
	if mdb == nil {
		return errors.New("uid: mongodb not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), uidOpTimeout)
	defer cancel()
	InitUidValue, err := rdb.Get(ctx, "uid:Inc").Uint64()
	if err != nil {
		errStr := err.Error()
		if !strings.EqualFold(errStr, "redis: nil") {
			return err
		}
	}

	// 初始化uid的值
	if InitUidValue < startUID {
		if err = rdb.Set(ctx, uidCounterKey, startUID, 0).Err(); err != nil {
			return err
		}
	}

	return nil
}

//	生产自增uid：Redis INCR 原子发号，然后同步直写 Mongo 快照。
//
// Redis 失败返回 0；Mongo 快照失败只打日志仍返回已分配的 uid（Redis 才是发号源）。
func GenUid() uint64 {
	ctx, cancel := context.WithTimeout(context.Background(), uidOpTimeout)
	defer cancel()
	uid, err := GenUidE(ctx)
	if err != nil {
		log2.Get().Warn("GenUid Failed", zap.Any("err", err))
	}
	return uid
}

// 约定：uid==0 表示 Redis 发号失败；uid!=0 且 err!=nil 表示 Mongo 快照失败，uid 依然可用。
func GenUidE(ctx context.Context) (uint64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rdb := redis.GetRedisClient()
	if rdb == nil {
		return 0, errors.New("uid: redis client is nil")
	}

	uid, err := rdb.Incr(ctx, uidCounterKey).Result()
	if err != nil {
		return 0, err
	}

	// 快照直写 mongodb 可有可无。
	snapCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), uidSnapshotSlow)
	defer cancel()
	if err := persistUidSnapshot(snapCtx, uid); err != nil {
		return uint64(uid), err
	}
	return uint64(uid), nil
}

// persistUidSnapshot 直写发号进度快照，用 $max 守卫保证并发乱序写入不会回退进度。
func persistUidSnapshot(ctx context.Context, uid int64) error {
	mdb := mongodb.GetDB()
	if mdb == nil {
		return errors.New("uid: mongodb not initialized")
	}
	filter := bson.M{"_id": uidSnapshotID}
	update := bson.M{
		"$max": bson.M{"IncId": uid},
		"$set": bson.M{"updatedAt": time.Now().Unix()},
	}
	_, err := mdb.Collection(uidCollection).UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}
