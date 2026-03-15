package test

import (
	"fmt"
	"gameServer/common/db/item"
	"gameServer/pkg/cache/cacheX"
	"gameServer/pkg/cache/ssdb"
	"testing"
	"time"

	"github.com/seefan/gossdb/v2/conf"
)

type Role struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

type Role1 struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
}

func TestRun(t *testing.T) {
	cfg := &conf.Config{
		Host:        "127.0.0.1",
		Port:        8888,
		MinPoolSize: 20,
		MaxPoolSize: 100,
		Encoding:    true, // 支持非基本数据类型
		Password:    "123",
		// 更多配置可参考pkg文档
	}
	err := ssdb.Init(cfg)
	if err != nil {
		panic(err)
	}
	//panic(err)
	defer ssdb.Close()
	roleCache := cachex.NewCacheX[*Role](true, 10*time.Minute, 5*time.Minute)
	// 写入缓存

	// KV
	role := Role{Id: 1, Name: "战士1212", Level: 10}
	r, err := roleCache.Get("role:12")
	if err != nil {
		fmt.Println("读取角色失败1:")
		panic(err)
	}
	fmt.Println("读取角色:", r)

	err = roleCache.Set("user:role:2", &role, 3600)
	if err != nil {
		panic(err)
	}

	// 读取缓存
	r, err = roleCache.Get("user:role:1")
	if err != nil {
		fmt.Println("读取角色失败2:")
		panic(err)

	}
	r, err = roleCache.Get("user:role:2")
	if err != nil {
		fmt.Println("读取角色失败2:")
		panic(err)

	}
	fmt.Println("读取角色:", r)

	//
	data := make(map[uint64]item.ItemInfo)
	data[1] = item.ItemInfo{ItemSum: 1}
	data[2] = item.ItemInfo{ItemSum: 3}
	data[3] = item.ItemInfo{ItemSum: 6}
	data[4] = item.ItemInfo{ItemSum: 9}
	data[5] = item.ItemInfo{ItemSum: 4}
	data[6] = item.ItemInfo{ItemSum: 4}

	itemKey := "constValue:role:1"
	err = ssdb.GetClient().Set(itemKey, data)
	if err != nil {
		panic(err)
	}
	get, err := ssdb.GetClient().Get(itemKey)
	if err != nil {
		panic(err)
	}
	var dbItem map[uint64]item.ItemInfo
	err = get.As(&dbItem)
	if err != nil {
		panic(err)
	}
	fmt.Println(dbItem)
}

func TestHashRun(t *testing.T) {
	cfg := &conf.Config{
		Host:        "127.0.0.1",
		Port:        8888,
		MinPoolSize: 20,
		MaxPoolSize: 100,
		Encoding:    true, // 支持非基本数据类型
		Password:    "123",
		// 更多配置可参考pkg文档
	}
	err := ssdb.Init(cfg)
	if err != nil {
		panic(err)
	}
	defer ssdb.Close()
	key := "Role:1"
	role := Role{Id: 1, Name: "战士1212", Level: 10}
	role1 := Role1{Name: "yyy", Level: 100}

	key1 := "role"
	key2 := "role1"
	err = ssdb.GetClient().MultiHSet(key, map[string]interface{}{
		key1: role,
		key2: role1,
	})
	// 写入缓存
	if err != nil {
		fmt.Println("读取角色失败1:")
		panic(err)
	}

	dbVals, err := ssdb.GetClient().HGetAll(key)
	for k, dbvalue := range dbVals {
		if k == key1 {
			var r Role
			err = dbvalue.As(&r)
			if err != nil {
				panic(err)
			}
			fmt.Println("读取角色:", r)

		} else if k == key2 {
			var r Role1
			err = dbvalue.As(&r)
			if err != nil {
				panic(err)
			}
			fmt.Println("读取角色:", r)

		}
	}
	err = ssdb.GetClient().MultiHDel(key, key1, key2)
	if err != nil {
		panic(err)
	}

	dbVals, err = ssdb.GetClient().HGetAll(key)
	for k, dbvalue := range dbVals {
		if k == key1 {
			var r Role
			err = dbvalue.As(&r)
			if err != nil {
				panic(err)
			}
			fmt.Println("读取角色:", r)

		} else if k == key2 {
			var r Role1
			err = dbvalue.As(&r)
			if err != nil {
				panic(err)
			}
			fmt.Println("读取角色:", r)

		}
	}

}

func TestStringRun(t *testing.T) {
	key := GetItemKey(111)
	fmt.Println(key)
}

func GetItemKey(userId uint64) string {
	itemKey := "constValue:UserId:%d:" //道具表,KV, key:itemId

	return fmt.Sprintf(itemKey, userId)
}
