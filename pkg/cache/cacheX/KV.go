package cachex

import (
	"errors"
	"gameServer/pkg/cache/ssdb"

	"github.com/patrickmn/go-cache"
)

// KV结构

// Set 写入缓存，val 可以是基础类型或 struct，ttl 可选，单位秒
func (c *CacheX[T]) Set(key string, val T, ttl ...int64) error {
	//  是否写入 SSDB
	//if !c.dbFlag {
	//	return nil
	//}
	var ssdbTTL int64
	if len(ttl) > 0 {
		ssdbTTL = ttl[0]
		// 写入 SSDB
		if err := ssdb.GetClient().Set(key, val, ssdbTTL); err != nil {
			return err
		}
	} else {
		// 写入 SSDB
		if err := ssdb.GetClient().Set(key, val); err != nil {
			return err
		}
	}
	// 写入本地缓存
	c.Local.Set(key, val, cache.DefaultExpiration)
	return nil
}

// Get 读取缓存，先查本地 -> SSDB
func (c *CacheX[T]) Get(key string) (T, error) {
	var val T
	// 本地缓存
	if x, found := c.Local.Get(key); found {
		if val, ok := x.(T); ok {
			return val, nil
		}
		return val, errors.New("type mismatch in Local cache")
	}

	// SSDB
	dbVal, err := ssdb.GetClient().Get(key)
	if dbVal.IsEmpty() { //空数据
		return val, nil
	}
	if err != nil {
		return val, err
	}
	err = dbVal.As(&val)
	if err != nil {
		return val, err
	}
	// 写回本地缓存
	c.Local.Set(key, val, cache.DefaultExpiration)
	return val, nil
}

// Del 删除缓存
func (c *CacheX[T]) Del(key string) error {
	c.Local.Delete(key)
	//if !c.dbFlag {
	//	return nil
	//}
	err := ssdb.GetClient().Del(key)
	if err != nil {
		return err
	}
	return nil
}

// 删除缓存
//func (c *CacheX[T]) Delete(key string) {
//	c.Local.Delete(key)
//	ssdb.GetClient().Del(key)
//}

func (c *CacheX[T]) DeleteCache(key string) {
	c.Local.Delete(key)
}
