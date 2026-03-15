package heros

import (
	"fmt"
	cachex "gameServer/pkg/cache/cacheX"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/redis"
	"time"

	"go.uber.org/zap"
)

var (
	heroKey = "hero:userId:%d" //道具表,KV, key:itemId
	//dbItem  map[uint64]ItemInfo   // 道具数据结构 道具Id-道具详情
	cache      = cachex.NewCacheX[[]*hero](true, 10*time.Minute, 5*time.Minute)
	channelKey = "hero"
)

type hero struct {
	Id    uint32   //人物id
	Skins []uint32 //皮肤种类
}

func getKey(userId uint64) string {
	return fmt.Sprintf(heroKey, userId)
}

// 解锁人物 添加一个新人物
func UnLockCharacter(userId uint64, idList []uint32) bool {
	key := getKey(userId)
	infos, err := cache.Get(key)
	if err != nil {
		log2.Get().Error("get ssdb heros Info false", zap.Any("err:", err))
		return false
	}
	if infos == nil {
		infos = make([]*hero, 0)
	}

	ls := make([]uint32, 0)
	for _, id := range idList {
		flag := false
		for _, info := range infos {
			if info.Id == id {
				flag = true
				break
			}
		}
		if !flag {
			ls = append(ls, id)
		}
	}
	for _, id := range ls {
		infos = append(infos, &hero{Id: id})
	}
	err = cache.Set(key, infos)
	if err != nil {
		log2.Get().Warn("UnLockCharacter false", zap.Any("err:", err))
		return false
	}

	// 发布信息
	err = redis.PublishMessage(channelKey, getKey(userId))
	return true
}

func GetAllUnLockCharacter(userId uint64) []*hero {
	key := getKey(userId)
	itemInfos, err := cache.Get(key)
	if err != nil {
		log2.Get().Warn("get heros Info false", zap.Any("err:", err))
		return nil
	}
	return itemInfos
}

func Listening() {
	getMes := redis.GetRedisClient().Subscribe(redis.GetCtx(), channelKey)
	go func() {
		for msg := range getMes.Channel() {
			cache.DeleteCache(msg.Payload)
		}
	}()
}
