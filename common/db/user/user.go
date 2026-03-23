package user

import (
	"fmt"
	cachex "gameServer/pkg/cache/cacheX"
	"gameServer/pkg/cache/ssdb"
	"gameServer/pkg/random/salt"
	"gameServer/pkg/random/snowflake"
	"strconv"
	"time"
)

const (
	allUser     = "AllUser"
	userInfoKey = "UserInfo:UserId:%d"
	oLKey       = "OL:{Openid:%s,LoginType:%s}" //复合索引
	//oLCache       = cachex.NewCacheX[*OL](true, 2*time.Minute, 1*time.Minute)
)

var (
	olCache       = cachex.NewCacheX[*OL](true, 10*time.Minute, 5*time.Minute)
	userInfoCache = cachex.NewCacheX[*UserInfo](true, 10*time.Minute, 5*time.Minute)
	node          = snowflake.NewNode(1)
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

// 所有的userId
//type AllUser struct { //索引 Hash 结构
//	UserIdList []uint64 //游戏用户唯一id
//}

// 第三方信息
type OL struct { //key 索引
	UserId uint64 //游戏用户唯一id
}

// 第三方映射表
func GetOLKey(openid, loginType string) string {
	return fmt.Sprintf(oLKey, openid, loginType)
}

func getUserInfoKey(userId uint64) string {
	return fmt.Sprintf(userInfoKey, userId)
}

// 查 ol
func FindOL(openid, loginType string) (error, *OL) {
	ol, err := olCache.Get(GetOLKey(openid, loginType))
	if err != nil {
		return err, nil
	}
	return nil, ol
}

// 查表数据 userid
func FindUser(userId uint64) (*UserInfo, error) {
	userIdKey := getUserInfoKey(userId)
	userInfo, err := userInfoCache.Get(userIdKey)
	if err != nil {
		return nil, err
	}
	return userInfo, nil
}

// 获取所有的 userid
//func GetAllUser(userCache *cachex.CacheX[*AllUser]) (*AllUser, error) {
//	userList, err := userCache.Get(allUser)
//	if err != nil {
//		return nil, err
//	}
//	return userList, nil
//}

func GetAllUserNoCache() (map[uint64]struct{}, error) {
	dbVal, err := ssdb.GetClient().HGetAll(allUser)
	if err != nil {
		return nil, err
	}
	allUserMap := make(map[uint64]struct{}, len(dbVal))
	for dbUid, _ := range dbVal {
		userId, err := strconv.ParseUint(dbUid, 10, 64)
		if err != nil {
			return nil, err
		}
		allUserMap[userId] = struct{}{}
	}

	return allUserMap, nil
}

func CreateUser(openid, loginType string) (*UserInfo, error) {
	olKey := GetOLKey(openid, loginType)
	userId := node.Generate()
	ol := &OL{
		UserId: userId,
	}
	fLoginType, err := strconv.ParseUint(loginType, 10, 32)
	if err != nil {
		return nil, err
	}

	userInfo := &UserInfo{
		UserId:         ol.UserId,
		Salt:           salt.Lower(32),
		LoginType:      int32(fLoginType),
		Openid:         openid,
		CreatTimestamp: uint64(time.Now().UnixMilli()),
	}
	// 写入ol
	err = olCache.SetNX(olKey, ol)
	if err != nil {
		return nil, err
	}
	err = userInfoCache.SetNX(getUserInfoKey(userId), userInfo)
	if err != nil {
		return nil, err
	}

	// 4. 加入 AllUser（Hash模拟Set）
	err = ssdb.GetClient().HSet(allUser, strconv.FormatUint(ol.UserId, 10), 1)
	if err != nil {
		return nil, err
	}
	return userInfo, nil
}
