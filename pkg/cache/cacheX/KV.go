package cachex

import (
	"gameServer/pkg/cache/ssdb"

	"github.com/patrickmn/go-cache"
	"github.com/seefan/gossdb/v2"
)

// KV结构

// Set 写入缓存，val 可以是基础类型或 struct，ttl 可选，单位秒
func (c *CacheX[T]) Set(key string, val T, ttl ...int64) error {
	// 写入本地缓存
	c.local.Set(key, val, cache.DefaultExpiration)
	//  是否写入 SSDB
	if !c.dbFlag {
		return nil
	}
	var ssdbTTL int64
	if len(ttl) > 0 {
		ssdbTTL = ttl[0]
	}
	// 写入 SSDB
	if err := ssdb.GetClient().Set(key, val, ssdbTTL); err != nil {
		return err
	}
	return nil
}

// Get 读取缓存，先查本地 -> SSDB
func (c *CacheX[T]) Get(key string) (T, error) {
	var val T
	// 本地缓存
	//if x, found := c.local.Get(key); found {
	//	if val, ok := x.(T); ok {
	//		return val, nil
	//	}
	//	return val, errors.New("type mismatch in local cache")
	//}
	//if !c.dbFlag {
	//	return val, nil
	//}

	// SSDB
	dbVal, err := ssdb.GetClient().Get(key)
	if err != nil {
		return val, err
	}
	err = dbVal.As(&val)
	if err != nil {
		return val, err
	}
	// 写回本地缓存
	c.local.Set(key, val, cache.DefaultExpiration)
	return val, nil
}

// Del 删除缓存
func (c *CacheX[T]) Del(key string) error {
	c.local.Delete(key)
	if !c.dbFlag {
		return nil
	}
	return gossdb.Client().Del(key)
}
