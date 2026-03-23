package heros

import (
	"fmt"
	"gameServer/pkg/cache/ssdb"
	"gameServer/pkg/redis"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
)

var (
	heroKey = "hero:userId:%d" //道具表,hash, key:itemId
	//dbItem  map[uint64]ItemInfo   // 道具数据结构 道具Id-道具详情
	//localCache = cachex.NewCacheX[[]*hero](true, 10*time.Minute, 5*time.Minute)
	localCache = cache.New(5*time.Minute, 10*time.Minute) // <int,map[int]*Item>

	channelKey = "hero"
)

type hero struct {
	Id    int      //人物id
	Skins []uint32 //皮肤种类 需要分布式锁或锁id
}

func getKey(userId uint64) string {
	return fmt.Sprintf(heroKey, userId)
}

// 解锁人物 添加一个新人物
//func UnLockCharacter(userId uint64, idList []uint32) bool {
//	key := getKey(userId)
//
//	infos, err := cache.Get(key)
//	if err != nil {
//		log2.Get().Error("get ssdb heros Info false", zap.Any("err:", err))
//		return false
//	}
//	if infos == nil {
//		infos = make([]*hero, 0)
//	}
//
//	ls := make([]uint32, 0)
//	for _, id := range idList {
//		flag := false
//		for _, info := range infos {
//			if info.Id == id {
//				flag = true
//				break
//			}
//		}
//		if !flag {
//			ls = append(ls, id)
//		}
//	}
//	for _, id := range ls {
//		infos = append(infos, &hero{Id: id})
//	}
//	err = cache.Set(key, infos)
//	if err != nil {
//		log2.Get().Warn("UnLockCharacter false", zap.Any("err:", err))
//		return false
//	}
//
//	// 发布信息
//	err = redis.PublishMessage(channelKey, getKey(userId))
//	return true
//}

func UnLockCharacter(userId uint64, idList []int) bool {
	key := getKey(userId)
	for _, id := range idList {
		h := hero{Id: id, Skins: []uint32{}}
		err := ssdb.GetClient().HSet(key, strconv.Itoa(id), h)
		if err != nil {
			return false
		}
		// 删除本地缓存
	}
	localCache.Delete(key)
	// 发布信息
	//err = redis.PublishMessage(channelKey, getKey(userId))
	return true
}

//func GetAllUnLockCharacter(userId uint64) []*hero {
//	key := getKey(userId)
//	itemInfos, err := cache.Get(key)
//	if err != nil {
//		log2.Get().Warn("get heros Info false", zap.Any("err:", err))
//		return nil
//	}
//	return itemInfos
//}

// 获取所有
func GetAllUnLockCharacter(userId uint64) []*hero {
	key := getKey(userId)
	// 本地缓存
	allHero, found := localCache.Get(key)
	if found {
		if val, ok := allHero.([]*hero); ok {
			return val
		}
		return nil
	}
	val, err := ssdb.GetClient().HGetAll(key)
	if err != nil {
		return nil
	}
	heroes := make([]*hero, 0, len(val))
	for _, dbVal := range val {
		var h hero
		err = dbVal.As(&h)
		heroes = append(heroes, &h)
	}
	localCache.Set(key, heroes, cache.DefaultExpiration)

	return heroes
}

func Listening() {
	getMes := redis.GetRedisClient().Subscribe(redis.GetCtx(), channelKey)
	go func() {
		for msg := range getMes.Channel() {
			localCache.Delete(msg.Payload)
		}
	}()
}
