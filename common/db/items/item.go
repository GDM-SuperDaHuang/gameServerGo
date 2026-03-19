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
	itemKey   = "item:UserId:%d"                          //道具表,<getItemKey,map[string(ItemId)]*Item>
)

type Item struct {
	ItemId   int
	ItemType int
	Count    int64
}

func getItemKey(userId uint64) string {
	return fmt.Sprintf(itemKey, userId)
}

// 获取所有的道具
func GetAllItems(userId uint64) (map[int]*Item, error) {
	cacheKey := getItemKey(userId)
	if v, ok := itemCache.Get(cacheKey); ok {
		return v.(map[int]*Item), nil
	}

	// miss -> 查询SSDB
	vals, err := ssdb.GetClient().HGetAll(cacheKey)
	if err != nil {
		return nil, err
	}
	items := make(map[int]*Item)
	for k, v := range vals {
		empty := v.IsEmpty()
		if empty { //空数据
			continue
		}
		itemId, _ := strconv.Atoi(k)
		var item Item
		err = v.As(&item)
		if err != nil {
			log2.Get().Error("dbItem as to Item err", zap.Error(err))
			return nil, err
		}
		items[itemId] = &item
	}
	itemCache.Set(cacheKey, items, cache.DefaultExpiration)

	return items, nil
}

// 验证自身道具是否充足
func VerifyItem(userId uint64, itemMap map[int]int64) bool {
	itemInfos, err := GetAllItems(userId)
	if err != nil {
		log2.Get().Warn("get ItemInfo false", zap.Any("err:", err))
		return false
	}
	for itemId, chanceValue := range itemMap {
		info, ok := itemInfos[itemId]
		if !ok {
			return false
		}
		if info.Count < chanceValue {
			return false
		}
	}
	return true
}

func GetItem(userId uint64, itemId int) (*Item, error) {
	items, err := GetAllItems(userId)
	if v, ok := items[itemId]; ok {
		return v, nil
	}
	return nil, err
}

func GetItemNoCache(userId uint64, itemId int) (*Item, error) {
	value, err := ssdb.GetClient().HGet(getItemKey(userId), strconv.Itoa(itemId))
	if err != nil {
		return nil, err
	}
	if value.IsEmpty() {
		return nil, nil
	}
	var item Item
	err = value.As(&item)
	if err != nil {
		return nil, err
	}
	return &item, err
}

func RewardItem(userId uint64, itemMap map[int]int64) bool {
	itemMapDb := make(map[string]interface{}) //db
	itemInfos, err := GetAllItems(userId)
	for itemId, chanceValue := range itemMap {
		_, ok := itemInfos[itemId]
		if !ok {
			itemInfos[itemId] = &Item{
				ItemId: itemId,
				Count:  chanceValue,
			}
		} else {
			itemInfos[itemId].Count += chanceValue
		}
		itemMapDb[strconv.Itoa(itemId)] = itemInfos[itemId]
	}
	//
	key := getItemKey(userId)
	err = ssdb.GetClient().MultiHSet(key, itemMapDb)
	if err != nil {
		return false
	}
	// 更新缓存
	itemCache.Set(key, itemInfos, cache.DefaultExpiration)

	// 发布信息
	err = redis.PublishMessage(cacheChanel.ItemChanel, getItemKey(userId))
	if err != nil {
		log2.Get().Warn("redis PublishCacheDelete item err", zap.Error(err))
	}
	return true
}

func ConsumeItem(userId uint64, itemMap map[int]int64) bool {
	itemInfos, err := GetAllItems(userId)
	itemMapDb := make(map[string]interface{}) // id - info
	for itemId, chanceValue := range itemMap {
		item, ok := itemInfos[itemId]
		if !ok {
			return false
		}
		if item == nil {
			return false
		}
		if item.Count < chanceValue {
			return false
		}
		item.Count -= chanceValue
		itemMapDb[strconv.Itoa(itemId)] = item
	}
	key := getItemKey(userId)
	err = ssdb.GetClient().MultiHSet(key, itemMapDb)
	if err != nil {
		return false
	}
	// 更新缓存
	itemCache.Set(key, itemInfos, cache.DefaultExpiration)

	// 发布信息
	err = redis.PublishMessage(cacheChanel.ItemChanel, getItemKey(userId))
	if err != nil {
		log2.Get().Warn("redis PublishCacheDelete item err", zap.Error(err))
	}
	return true
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
