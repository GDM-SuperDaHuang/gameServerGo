package items

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gameServer/pkg/mongodb"

	gocache "github.com/patrickmn/go-cache"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	itemKey        = "item:uid:%d"
	itemCollection = "user_items"
)

const (
	itemOpTimeout = 5 * time.Second

	itemLocalTTL     = 5 * time.Minute
	itemLocalCleanup = 10 * time.Minute
)

var (
	localCache = gocache.New(itemLocalTTL, itemLocalCleanup)

	ErrItemNotEnough = errors.New("items: not enough")
	ErrItemNotFound  = errors.New("items: not found")
)

// UserItems 玩家道具背包：itemId -> 数量。
type UserItems struct {
	Uid       int64         `bson:"uid" json:"uid"`
	Items     map[int]int64 `bson:"items" json:"items"`
	UpdatedAt int64         `bson:"updatedAt,omitempty" json:"updatedAt"`
}

// ---------------- key / cache ----------------

func itemCacheKey(uid int64) string {
	return fmt.Sprintf(itemKey, uid)
}

func itemField(itemId int) string {
	return fmt.Sprintf("items.%d", itemId)
}

func cloneItems(u *UserItems) *UserItems {
	if u == nil {
		return nil
	}
	cp := &UserItems{Uid: u.Uid, UpdatedAt: u.UpdatedAt}
	if u.Items != nil {
		cp.Items = make(map[int]int64, len(u.Items))
		for k, v := range u.Items {
			cp.Items[k] = v
		}
	}
	return cp
}

func getLocal(uid int64) (*UserItems, bool) {
	if v, ok := localCache.Get(itemCacheKey(uid)); ok {
		if u, ok := v.(*UserItems); ok && u != nil {
			return cloneItems(u), true
		}
	}
	return nil, false
}

func setLocal(u *UserItems) {
	if u == nil || u.Uid == 0 {
		return
	}
	if u.Items == nil {
		u.Items = map[int]int64{}
	}
	localCache.Set(itemCacheKey(u.Uid), cloneItems(u), gocache.DefaultExpiration)
}

// InvalidateCache 主动失效某玩家道具缓存（GM 补单/删档等场景调用）。
func InvalidateCache(uid int64) {
	localCache.Delete(itemCacheKey(uid))
}

// ---------------- index ----------------

// EnsureIndexes 创建 user_items 唯一索引：uid 唯一。启动时调用一次即可，幂等。
func EnsureIndexes(ctx context.Context) error {
	mdb := mongodb.GetDB()
	if mdb == nil {
		return errors.New("items: mongodb not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	coll := mdb.Collection(itemCollection)
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "uid", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_uid"),
	})
	return err
}

// ---------------- read ----------------

// GetItems 查询玩家整个背包：本地缓存 -> Mongo，读穿透并回填。
// 写路径（Consume/Reward）直接走 Mongo 原子 $inc，正确性不依赖缓存，
// 本地缓存只做读加速，与 user 模型保持一致。
func GetItems(uid int64) (*UserItems, error) {
	if uid == 0 {
		return nil, errors.New("items: empty uid")
	}
	if u, ok := getLocal(uid); ok {
		return u, nil
	}

	mdb := mongodb.GetDB()
	if mdb == nil {
		return nil, errors.New("items: mongodb not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), itemOpTimeout)
	defer cancel()
	u := &UserItems{}
	if err := mdb.Collection(itemCollection).FindOne(ctx, bson.M{"uid": uid}).Decode(u); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			empty := &UserItems{Uid: uid, Items: map[int]int64{}}
			setLocal(empty)
			return cloneItems(empty), nil
		}
		return nil, err
	}
	if u.Items == nil {
		u.Items = map[int]int64{}
	}
	setLocal(u)
	return cloneItems(u), nil
}

// GetItemCount 查询单个道具数量，背包不存在返回 0。
func GetItemCount(uid int64, itemId int) (int64, error) {
	u, err := GetItems(uid)
	if err != nil {
		return 0, err
	}
	if u.Items == nil {
		return 0, nil
	}
	return u.Items[itemId], nil
}

// ---------------- write (Mongo 原子) ----------------
// applyMapAtomic 一次 Mongo 原子操作增减多个道具（单文档多字段 $inc，天然原子）。
// positive=true 时为发放（允许 upsert）；false 时为消耗（每个道具都必须数量充足）。
func applyMapAtomic(uid int64, itemMap map[int]int64, positive bool) (*UserItems, error) {
	if uid == 0 {
		return nil, errors.New("items: empty uid")
	}
	if len(itemMap) == 0 {
		return GetItems(uid)
	}
	mdb := mongodb.GetDB()
	if mdb == nil {
		return nil, errors.New("items: mongodb not initialized")
	}
	inc := bson.M{}
	filter := bson.M{"uid": uid}
	for id, d := range itemMap {
		if d <= 0 {
			continue
		}
		if positive {
			inc[itemField(id)] = d
		} else {
			inc[itemField(id)] = -d
			filter[itemField(id)] = bson.M{"$gte": d} //防止多扣，必须 >=d
		}
	}
	if len(inc) == 0 {
		return GetItems(uid)
	}
	ctx, cancel := context.WithTimeout(context.Background(), itemOpTimeout)
	defer cancel()
	now := time.Now().Unix()
	update := bson.M{
		"$inc": inc,
		"$set": bson.M{"updatedAt": now},
	}
	var res *mongo.SingleResult
	if positive {
		update["$setOnInsert"] = bson.M{"uid": uid}
		res = mdb.Collection(itemCollection).FindOneAndUpdate(ctx, filter, update,
			options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After))
	} else {
		res = mdb.Collection(itemCollection).FindOneAndUpdate(ctx, filter, update,
			options.FindOneAndUpdate().SetReturnDocument(options.After))
	}
	u := &UserItems{}
	if err := res.Decode(u); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			if !positive {
				return nil, ErrItemNotEnough
			}
			return nil, err
		}
		return nil, err
	}
	if u.Items == nil {
		u.Items = map[int]int64{}
	}
	setLocal(u)
	return cloneItems(u), nil
}

// ConsumeItem 原子消耗一組道具：全部充足才一次扣完，否则任何道具都不扣，返回 false。
func ConsumeItem(uid int64, itemMap map[int]int64, rid int, tips string) bool {
	_ = rid
	_ = tips
	if len(itemMap) == 0 {
		return true
	}
	if _, err := applyMapAtomic(uid, itemMap, false); err != nil {
		return false
	}
	return true
}

// RewardItem 原子发放一組道具：一次 $inc 全部到账，Mongo 失败才返回 false。
func RewardItem(uid int64, itemMap map[int]int64, rid int, tips string) bool {
	_ = rid
	_ = tips
	if len(itemMap) == 0 {
		return true
	}
	if _, err := applyMapAtomic(uid, itemMap, true); err != nil {
		return false
	}
	return true
}
