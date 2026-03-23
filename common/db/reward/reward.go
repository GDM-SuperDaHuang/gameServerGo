package reward

import (
	"fmt"
	"gameServer/pkg/cache/ssdb"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
)

var (
	//cache     = cachex.NewCacheX[[]*reward](true, 10*time.Minute, 5*time.Minute)
	dbKey       = "reward:UserId:%d"                        //道具表,<getItemKey,map[string(ItemId)]*Item>
	rewardCache = cache.New(20*time.Second, 10*time.Second) // <int,map[int]*Item>
)

type reward struct {
	Id        int
	Timestamp int64 //奖励时间戳,秒
}

func getKey(userId uint64) string {
	return fmt.Sprintf(dbKey, userId)
}

// 奖励
func SaveRewardInfo(userId uint64, id int) bool {
	key := getKey(userId)
	r := reward{Id: id, Timestamp: time.Now().Unix()}
	err := ssdb.GetClient().HSet(key, strconv.Itoa(id), r)
	if err != nil {
		return false
	}
	rewardCache.Delete(key)
	// 发布信息
	//err = redis.PublishMessage(cacheChanel.ItemChanel, getKey(userId))
	return true
}

func GetAllRewardInfo(userId uint64) map[int]*reward {
	key := getKey(userId)
	infos, ok := rewardCache.Get(key)
	if ok {
		if val, ok := infos.(map[int]*reward); ok {
			return val
		}
		return nil
	}
	val, err := ssdb.GetClient().HGetAll(key)
	if err != nil {
		return nil
	}
	all := make(map[int]*reward, len(val))
	for dbId, dbVal := range val {
		id, err := strconv.ParseInt(dbId, 10, 64)
		if err != nil {
			return nil
		}
		var r reward
		err = dbVal.As(&r)
		all[int(id)] = &r
	}
	rewardCache.Set(key, all, cache.DefaultExpiration)
	return all
}
