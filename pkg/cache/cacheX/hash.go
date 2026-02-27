package cachex

import (
	"errors"
	"gameServer/pkg/cache/ssdb"

	"github.com/patrickmn/go-cache"
	"github.com/seefan/gossdb/v2"
)

// Hash结构
// client.HSet("user:1001", "gold", 1000)
// client.HSet("user:1001", "level", 20)

// todo Set 写入缓存
func (c *CacheX[T]) HSet(name, key string, val T) error {
	// 写入本地缓存
	c.local.Set(name+key, val, cache.DefaultExpiration) //重置刷新时间策列
	//  是否写入 SSDB
	if !c.dbFlag {
		return nil
	}

	// 写入 SSDB
	if err := ssdb.GetClient().HSet(name, key, val); err != nil {
		return err
	}
	return nil
}

// todo Get 读取缓存，先查本地 -> SSDB
func (c *CacheX[T]) HGet(name, key string) (T, error) {

	var zero T
	// 本地缓存
	if x, found := c.local.Get(name + key); found {
		if val, ok := x.(T); ok {
			return val, nil
		}
		return zero, errors.New("type mismatch in local cache")
	}
	if !c.dbFlag {
		return zero, nil
	}

	// SSDB
	var val T
	dbVal, err := ssdb.GetClient().HGet(name, key)
	if err != nil {
		return zero, err
	}
	err = dbVal.As(&val)
	if err != nil {
		return zero, err
	}
	// 写回本地缓存
	c.local.Set(name+key, val, cache.DefaultExpiration)
	return val, nil
}

// Del 删除缓存
func (c *CacheX[T]) HDel(name, key string) error {
	c.local.Delete(key)
	if !c.dbFlag {
		return nil
	}
	return gossdb.Client().HDel(name, key)
}
