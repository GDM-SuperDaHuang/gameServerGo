package reward

import (
	"fmt"
	"gameServer/common/db/cacheChanel"
	cachex "gameServer/pkg/cache/cacheX"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/redis"
	"time"

	"go.uber.org/zap"
)

var (
	cache = cachex.NewCacheX[[]*reward](true, 10*time.Minute, 5*time.Minute)
	dbKey = "reward:UserId:%d" //道具表,<getItemKey,map[string(ItemId)]*Item>
)

type reward struct {
	Id        int
	Timestamp int64 //奖励时间戳,秒
}

func getKey(userId uint64) string {
	return fmt.Sprintf(dbKey, userId)
}

// 奖励
func SaveRewardInfo(userId uint64, idList []int) bool {
	key := getKey(userId)
	infos, err := cache.Get(key)
	if err != nil {
		log2.Get().Error("get rewardDb Info false", zap.Any("err:", err))
		return false
	}
	if infos == nil {
		infos = make([]*reward, 0)
	}

	ls := make([]int, 0)
	for _, id := range idList {
		flag := false
		for _, info := range infos {
			if info.Id == id {
				info.Timestamp = time.Now().Unix()
				flag = true
				break
			}
		}
		if !flag {
			ls = append(ls, id)
		}
	}
	for _, id := range ls {
		infos = append(infos, &reward{Id: id, Timestamp: time.Now().Unix()})
	}
	//存储
	err = cache.Set(key, infos)
	if err != nil {
		log2.Get().Warn("rewardDb save false", zap.Any("err:", err))
		return false
	}
	// 发布信息
	err = redis.PublishMessage(cacheChanel.ItemChanel, getKey(userId))
	return true
}

func GetAllRewardInfo(userId uint64) []*reward {
	key := getKey(userId)
	infos, err := cache.Get(key)
	if err != nil {
		log2.Get().Warn(" GetAllRewardDbInfo Info false", zap.Any("err:", err))
		return nil
	}
	return infos
}

func GetRewardInfoById(userId uint64, id int) *reward {
	key := getKey(userId)
	infos, err := cache.Get(key)
	if err != nil {
		log2.Get().Warn(" GetAllRewardDbInfo Info false", zap.Any("err:", err))
		return nil
	}
	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}
