package items

import (
	"fmt"
	"gameServer/common/db/cacheChanel"
	"gameServer/pkg/cache/ssdb"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/redis"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

var (
	itemCache = cache.New(20*time.Second, 10*time.Second) // <int,map[int]*Item>
	//itemKey   = "item:UserId:%d"                          //道具表,<getItemKey,map[string(ItemId)]*Item>
	itemKey = "item:UserId:%d" //道具表,<getItemKey,map[string]int64>

)

//type Item struct {
//	ItemId   int
//	ItemType int
//	Count    int64
//	Version  int64 // 新增
//}

func getItemKey(userId uint64) string {
	return fmt.Sprintf(itemKey, userId)
}

// 验证自身道具是否充足
func VerifyItem(userId uint64, itemMap map[int]int64) bool {
	itemInfos, err := GetAllItems(userId)
	if err != nil {
		log2.Get().Warn("get ItemInfo false", zap.Any("err:", err))
		return false
	}
	for itemId, chanceValue := range itemMap {
		count, ok := itemInfos[itemId]
		if !ok {
			return false
		}
		if count < chanceValue {
			return false
		}
	}
	return true
}

func GetItem(userId uint64, itemId int) (int64, error) {
	items, err := GetAllItems(userId)
	if v, ok := items[itemId]; ok {
		return v, nil
	}
	return 0, err
}

func GetItemNoCache(userId uint64, itemId int) (int64, error) {
	//value, err := ssdb.GetClient().HGet(getItemKey(userId), strconv.Itoa(itemId))
	value, err := ssdb.GetClient().HGet(getItemKey(userId), strconv.Itoa(itemId))
	if err != nil {
		return 0, err
	}
	if value.IsEmpty() {
		return 0, nil
	}
	return value.Int64(), nil
}

func RewardItem(userId uint64, itemMap map[int]int64) bool {
	for itemId, delta := range itemMap {
		_, err := AddItem(userId, itemId, delta)
		if err != nil {
			return false
		}
	}
	return true
}

func GetAllItems(userId uint64) (map[int]int64, error) {
	key := getItemKey(userId)
	//  本地缓存（返回副本）
	if v, ok := itemCache.Get(key); ok {
		return cloneMap(v.(map[int]int64)), nil
	}

	// miss -> 查SSDB
	vals, err := ssdb.GetClient().HGetAll(key)
	if err != nil {
		return nil, err
	}

	result := make(map[int]int64)
	for k, v := range vals {
		if v.IsEmpty() {
			continue
		}
		itemId, _ := strconv.Atoi(k)
		count := v.Int64()
		result[itemId] = count
	}

	//  写缓存（存原始，返回副本）
	itemCache.Set(key, result, cache.DefaultExpiration)
	return cloneMap(result), nil
}

func AddItem(userId uint64, itemId int, delta int64) (int64, error) {
	key := getItemKey(userId)
	newCount, err := ssdb.GetClient().HIncr(
		key,
		strconv.Itoa(itemId),
		delta,
	)
	if err != nil {
		return 0, err
	}

	// 删除本地缓存
	itemCache.Delete(key)
	// 广播
	err = redis.PublishMessage(cacheChanel.ItemChanel, key)
	if err != nil {
		log2.Get().Warn("Publish item cache delete err", zap.Error(err))
	}
	return newCount, nil
}

func ConsumeItem(userId uint64, itemMap map[int]int64) bool {
	for itemId, delta := range itemMap {
		_, err := AddItem(userId, itemId, -delta)
		if err != nil {
			return false
		}
	}
	return true
}

func cloneMap(src map[int]int64) map[int]int64 {
	dst := make(map[int]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// 监听Item
func Listening() {
	getMes := redis.GetRedisClient().Subscribe(redis.GetCtx(), cacheChanel.ItemChanel)
	go func() {
		for msg := range getMes.Channel() {
			itemCache.Delete(msg.Payload)
		}
	}()
}
