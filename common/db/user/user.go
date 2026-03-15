package user

import (
	"fmt"
	cachex "gameServer/pkg/cache/cacheX"
	"gameServer/pkg/cache/ssdb"
)

var (
	allUser       = "AllUser"
	mainUserIdKey = "UserInfo:UserId:%d"
	oLKey         = "OL:{Openid:%s,LoginType:%s}" //复合索引
	//oLCache       = cachex.NewCacheX[*OL](true, 2*time.Minute, 1*time.Minute)
)

// 用户信息
type UserInfo struct {
	UserId         uint64 //游戏用户唯一id
	Salt           string //密码盐
	LoginType      int32  //平台类型
	Openid         string //第三方登录id
	CreatTimestamp uint64 //创建账号时间戳
	LoginTimestamp uint64 //登录时间戳
	OutTimestamp   uint64 //退出登录时间戳
	Status         int    //账号状态 0 正常，1 封号 等
}
type AllUser struct { //索引
	UserIdList []uint64 //游戏用户唯一id
}

// 第三方信息
type OL struct { //索引
	UserId uint64 //游戏用户唯一id
}

// 第三方映射表
func GetOLKey(openid, loginType string) string {
	return fmt.Sprintf(oLKey, openid, loginType)
}

// 用户基本数据
func getMainKey(userId uint64) string {
	return fmt.Sprintf(mainUserIdKey, userId)
}

// 建立表数据 第三方信息
func AddUserToOL(openid, loginType string, ol *OL, olCache *cachex.CacheX[*OL]) (error, *OL) {
	// 创建
	err := olCache.Set(GetOLKey(openid, loginType), ol)
	if err != nil {
		return err, nil
	}
	err = addUser(ol.UserId)
	if err != nil {
		return err, nil
	}
	return nil, ol
}

// 查 ol
func FindUserByOL(openid, loginType string, olCache *cachex.CacheX[*OL]) (error, *OL) {
	ol, err := olCache.Get(GetOLKey(openid, loginType))
	if ol == nil {
		return nil, nil
	}
	if err != nil {
		return err, nil
	}
	return nil, ol
}

// 添加一个用户 userid
func AddUser(userId uint64, userInfo *UserInfo, userCache *cachex.CacheX[*UserInfo]) error {
	userIdKey := getMainKey(userId)
	err := userCache.Set(userIdKey, userInfo)
	if err != nil {
		return err
	}
	return nil
}

// 查表数据 userid
func FindUser(userId uint64, userCache *cachex.CacheX[*UserInfo]) (*UserInfo, error) {
	userIdKey := getMainKey(userId)
	userInfo, err := userCache.Get(userIdKey)
	if err != nil {
		return nil, err
	}
	return userInfo, nil
}

// 获取所有的 userid
func GetAllUser(userCache *cachex.CacheX[*AllUser]) (*AllUser, error) {
	userList, err := userCache.Get(allUser)
	if err != nil {
		return nil, err
	}
	return userList, nil
}

func GetAllUserNoCache() (*AllUser, error) {
	dbVal, err := ssdb.GetClient().Get(allUser)
	if err != nil {
		return nil, err
	}
	var val AllUser
	err = dbVal.As(&val)
	if err != nil {
		return nil, err
	}
	return &val, nil
}

// KV结构，可能分布式，添加一个用户,不走缓存
func addUser(userId uint64) error {
	dbVal, err := ssdb.GetClient().Get(allUser)
	if err != nil {
		return err
	}
	if dbVal.IsEmpty() {
		val := &AllUser{
			UserIdList: make([]uint64, 0),
		}
		val.UserIdList = append(val.UserIdList, userId)
		// 写入 SSDB
		if err = ssdb.GetClient().Set(allUser, val); err != nil {
			return err
		}
	} else {
		var val AllUser
		err = dbVal.As(&val)
		if err != nil {
			return err
		}
		//防止重复
		for _, uid := range val.UserIdList {
			if uid == userId {
				return nil
			}
		}
		val.UserIdList = append(val.UserIdList, userId)
		if err = ssdb.GetClient().Set(allUser, val); err != nil {
			return err
		}
	}
	return nil
}
