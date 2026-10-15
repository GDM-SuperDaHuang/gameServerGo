package user

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"gameServer/pkg/logger/log2"
	"gameServer/pkg/mongodb"
	"gameServer/pkg/random/salt"

	gocache "github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

const (
	userCollection = "users"

	userCacheTTL     = 5 * time.Minute
	userCacheCleanup = 10 * time.Minute

	userOpTimeout = 5 * time.Second
)

var (
	// 本地进程缓存：读穿透 + 写穿透。注意 go-cache 只在单进程内有效，
	// 多 gate/多实例部署下以 Mongo 为准，本地缓存只做读加速，写后会刷新/失效。
	localCache = gocache.New(userCacheTTL, userCacheCleanup)

	ErrUserNotFound = errors.New("user: not found")
	// ErrTokenExpired 快速登录凭证无效或过期（Redis key 缺失/TTL 到期）。
	ErrTokenExpired = errors.New("user: access token expired")
)

// warnToken 只在 logger 已初始化时打日志：单测/工具链场景下 log2.Get() 可能为 nil，
// 直接调用会 panic。这里做 nil 守卫，保证缓存/续期路径永不因日志崩进程。
func warnToken(msg string, fields ...zap.Field) {
	if l := log2.Get(); l != nil {
		l.Warn(msg, fields...)
	}
}

// UserInfo 用户信息。LoginType+Openid 决定一个用户Uid。
// 注意：快速登录凭证 AccessToken 不再存这里，统一存 Redis（见 common/db/user/token.go）：
// login:token:<SHA256(raw)[32]byte> -> uid 与 login:uid:<uid> -> SHA256(raw)[32]byte 双向映射，
// Redis 中只存哈希字节不明文存 raw，TTL 3 天滑动续期。
type UserInfo struct {
	Uid            int64  `bson:"uid" json:"uid"`                       //游戏用户唯一id
	Salt           string `bson:"salt" json:"salt"`                     //密码盐
	LoginType      int32  `bson:"loginType" json:"loginType"`           //平台类型
	Openid         string `bson:"openid" json:"openid"`                 //第三方登录id
	AvatarUrl      string `bson:"avatarUrl" json:"avatarUrl"`           //  用户第三方图片
	NickName       string `bson:"nickName" json:"nickName"`             //  用户第三方名称
	CreatTimestamp uint64 `bson:"creatTimestamp" json:"creatTimestamp"` //创建账号时间戳
	LoginTimestamp uint64 `bson:"loginTimestamp" json:"loginTimestamp"` //登录时间戳
	OutTimestamp   uint64 `bson:"outTimestamp" json:"outTimestamp"`     //退出登录时间戳
	OnlineDuration int64  `bson:"onlineDuration" json:"onlineDuration"` // 在线时长（秒）
	Status         int32  `bson:"status" json:"status"`                 //账号状态 0 正常，1 封号 等
	OpTime         int64  `bson:"opTime" json:"opTime"`                 //操作时间戳
	UpdatedAt      int64  `bson:"updatedAt" json:"updatedAt"`           //最后更新时间戳
}

// ---------------- key ----------------

func userCacheKey(uid int64) string { //<key,UserInfo>
	return fmt.Sprintf("user:uid:%d", uid)
}

func loginCacheKey(loginType int32, openid string) string { //<key,uid>
	return fmt.Sprintf("user:login:%d:%s", loginType, openid)
}

// clone 拷贝，防止调用方直接改缓存里的指针对象。
// UserInfo 现为纯值字段结构体，浅拷即深拷。
func cloneUser(u *UserInfo) *UserInfo {
	if u == nil {
		return nil
	}
	cp := *u
	return &cp
}

func getCacheByUID(uid int64) (*UserInfo, bool) {
	if v, ok := localCache.Get(userCacheKey(uid)); ok {
		if u, ok := v.(*UserInfo); ok && u != nil {
			return cloneUser(u), true
		}
	}
	return nil, false
}

func setCache(u *UserInfo) {
	if u == nil || u.Uid == 0 {
		return
	}
	c := cloneUser(u)
	localCache.Set(userCacheKey(u.Uid), c, gocache.DefaultExpiration)
	if u.Openid != "" {
		localCache.Set(loginCacheKey(u.LoginType, u.Openid), u.Uid, gocache.DefaultExpiration)
	}
}

//	主动失效本地缓存（跨实例写后、GM 封号等场景调用）。
//
// 注意：顺带踢掉该 uid 在 Redis 的 token（封号即时失效），Redis 不可用时只打日志不报错。
func InvalidateCache(u *UserInfo) {
	if u == nil {
		return
	}
	localCache.Delete(userCacheKey(u.Uid))
	if u.Openid != "" {
		localCache.Delete(loginCacheKey(u.LoginType, u.Openid))
	}
	if u.Uid != 0 {
		if err := DeleteAccessTokenByUid(u.Uid); err != nil && !errors.Is(err, redis.Nil) {
			warnToken("InvalidateCache delete token failed", zap.Int64("uid", u.Uid), zap.Error(err))
		}
	}
}

// EnsureIndexes 创建用户表唯一索引：uid 唯一，loginType+openid 唯一。启动时调用一次即可，幂等。
func EnsureIndexes(ctx context.Context) error {
	mdb := mongodb.GetDB()
	if mdb == nil {
		return errors.New("user: mongodb not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	coll := mdb.Collection(userCollection)
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "uid", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_uid"),
		},
		{
			Keys:    bson.D{{Key: "loginType", Value: 1}, {Key: "openid", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_login_openid"),
		},
	})
	return err
}

// ---------------- 查询 ----------------

// FindUser 按 uid 查询：本地缓存 -> Mongo，读穿透并回填缓存。
func FindUser(uid int64) (*UserInfo, error) {
	if uid == 0 {
		return nil, ErrUserNotFound
	}
	if u, ok := getCacheByUID(uid); ok {
		return u, nil
	}

	mdb := mongodb.GetDB()
	if mdb == nil {
		return nil, errors.New("user: mongodb not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), userOpTimeout)
	defer cancel()

	u := &UserInfo{}
	if err := mdb.Collection(userCollection).FindOne(ctx, bson.M{"uid": uid}).Decode(u); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	setCache(u)
	return cloneUser(u), nil
}

// 按 LoginType+Openid 查询（登录主路径）。
func FindByLogin(loginType int32, openid string) (*UserInfo, error) {
	// 先看 login->uid 映射缓存，再走 uid 缓存。
	if v, ok := localCache.Get(loginCacheKey(loginType, openid)); ok {
		if uid, ok := v.(int64); ok && uid != 0 {
			if u, ok := getCacheByUID(uid); ok {
				return u, nil
			}
		}
	}

	mdb := mongodb.GetDB()
	if mdb == nil {
		return nil, errors.New("user: mongodb not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), userOpTimeout)
	defer cancel()

	u := &UserInfo{}
	filter := bson.M{"loginType": loginType, "openid": openid}
	if err := mdb.Collection(userCollection).FindOne(ctx, filter).Decode(u); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	setCache(u)
	return cloneUser(u), nil
}

//	按客户端上传的原始 AccessToken 查询（快速登录）。
//
// token 存 Redis：login:token:<SHA256(raw)[32]byte> -> uid，TTL 到期/不存在即视为过期。
// 入参为明文，内部哈希后查询，Redis 中无明文。
func FindByToken(rawToken string) (*UserInfo, error) {
	if rawToken == "" {
		return nil, ErrUserNotFound
	}
	uid, err := GetUidByAccessToken(rawToken)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrTokenExpired
		}
		return nil, err
	}
	u, err := FindUser(uid)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Mongo 已删号但 Redis 残留：顺手清理僵尸 token。
			_ = DeleteAccessToken(rawToken, uid)
		}
		return nil, err
	}
	return u, nil
}

// 快速登录主入口：Redis 校验通过后滑动续期 TTL 并刷新 loginTimestamp。
func LoginByToken(rawToken string) (*UserInfo, error) {
	u, err := FindByToken(rawToken)
	if err != nil {
		return nil, err
	}
	err = TouchLogin(u.Uid)
	if err != nil {
		return nil, err
	}
	// 续租 AccessToken
	if rerr := TouchAccessTokenByUid(u.Uid); rerr != nil && !errors.Is(rerr, redis.Nil) {
		warnToken("TouchLogin refresh token ttl failed", zap.Int64("uid", u.Uid), zap.Error(rerr))
	}
	return u, nil
}

//	每次登录调用：刷新 loginTimestamp，并滑动续期 Redis token TTL（token 本体不变）。
//
// Redis 续期失败不阻断登录（只打日志），Mongo 时间戳更新为准。
func TouchLogin(uid int64) error {
	if uid == 0 {
		return ErrUserNotFound
	}
	now := time.Now()
	// todo优化
	_, err := UpdateFields(uid, bson.M{
		"loginTimestamp": uint64(now.Unix()),
	})
	if err != nil {
		return err
	}
	return nil
}

// 创建一个新AccessToken
func newRawAccessToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

//	新增用户：GenUid 发号 -> Mongo Insert -> Redis 落 token -> 回填本地缓存。
//
// 同一 LoginType+Openid 并发创建时，唯一索引冲突则回读已存在用户，保证幂等。
// 返回 rawToken 为明文快速登录凭证（仅本次返回，需下发客户端）。
func CreateUser(loginType int32, openid, nickName, avatarUrl string) (*UserInfo, string, error) {
	if openid == "" {
		return nil, "", errors.New("user: openid is empty")
	}

	// 已存在直接返回，避免重复发号。
	if u, err := FindByLogin(loginType, openid); err == nil && u != nil {
		return u, "", nil
	}

	uid := GenUid()
	if uid == 0 {
		return nil, "", errors.New("user: gen uid failed")
	}
	now := time.Now()
	u := &UserInfo{
		Uid:            int64(uid),
		Salt:           salt.Lower(32),
		LoginType:      loginType,
		Openid:         openid,
		AvatarUrl:      avatarUrl,
		NickName:       nickName,
		CreatTimestamp: uint64(now.Unix()),
		LoginTimestamp: uint64(now.Unix()),
		OpTime:         now.Unix(),
		UpdatedAt:      now.Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), userOpTimeout)
	defer cancel()

	mdb := mongodb.GetDB()
	if mdb == nil {
		return nil, "", errors.New("user: mongodb not initialized")
	}
	if _, err := mdb.Collection(userCollection).InsertOne(ctx, u); err != nil {
		// 并发下另一节点先插入：回读返回，保证 LoginType+Openid 唯一对应一个 Uid。
		if mongo.IsDuplicateKeyError(err) {
			if exist, gerr := FindByLogin(loginType, openid); gerr == nil && exist != nil {
				return exist, "", nil
			}
		}
		return nil, "", err
	}
	accessToken := newRawAccessToken()
	if err := SetAccessToken(accessToken, u.Uid); err != nil {
		return cloneUser(u), "", err
	}
	setCache(u)
	return cloneUser(u), accessToken, nil
}

//	登录主入口：存在则续期后返回，不存在则创建。
//
// 返回值 created=true 表示本次新建，此时 rawToken 为新签发的明文凭证；
// 老用户正常返回 rawToken=""（Redis 内 token 续期，凭证不变，客户端继续用已有 token）。
// 若老用户在 Redis 已无有效 token（如过期被清），则签发新 token 并返回（客户端需更新缓存）。
func GetOrCreateUser(loginType int32, openid, nickName, avatarUrl string) (*UserInfo, string, error) {
	u, err := FindByLogin(loginType, openid)
	var accessToken = ""
	if err == nil && u != nil { //查到用户
		terr := TouchLogin(u.Uid) //登陆时间
		if terr != nil {
			return nil, "", terr
		}
		// 创建新的accessToken
		accessToken = newRawAccessToken()
		if err = SetAccessToken(accessToken, u.Uid); err != nil {
			return u, "", err
		}
		// Redis 中只存 SHA-256 hex，不可逆推明文：有有效 token 时返回 ""，
		// 客户端继续用已有 token；仅无有效 token（过期被清）时签发新明文并返回。
		//if _, gerr := GetAccessTokenByUid(u.Uid); gerr != nil { // 查不到即无旧 token
		//	accessToken = newRawAccessToken()
		//	if err = SetAccessToken(accessToken, u.Uid); err != nil {
		//		return u, "", err
		//	}
		//}
		return u, accessToken, nil
	}

	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, "", err
	}
	u, raw, err := CreateUser(loginType, openid, nickName, avatarUrl)
	if err != nil {
		return nil, "", err
	}
	return u, raw, nil
}

// ---------------- 更新 ----------------

// UpdateFields 通用更新：Mongo $set + 本地缓存刷新（读-改-写回）。
// fields 的 key 为 bson tag；调用后自动维护 opTime/updatedAt。
func UpdateFields(uid int64, fields bson.M) (*UserInfo, error) {
	if uid == 0 {
		return nil, ErrUserNotFound
	}
	// 无更新
	if len(fields) == 0 {
		return FindUser(uid)
	}
	mdb := mongodb.GetDB()
	if mdb == nil {
		return nil, errors.New("user: mongodb not initialized")
	}

	now := time.Now().Unix()
	set := bson.M{}
	for k, v := range fields {
		set[k] = v
	}
	// 保护不可变键不被误改。
	delete(set, "uid")
	delete(set, "salt")
	delete(set, "_id")
	delete(set, "loginType")
	delete(set, "openid")
	delete(set, "creatTimestamp")
	// 兼容老调用方：token 已迁 Redis，忽略 Mongo 侧残留字段的写入。
	delete(set, "accessToken")
	delete(set, "accessTokenTime")
	set["opTime"] = now
	set["updatedAt"] = now

	ctx, cancel := context.WithTimeout(context.Background(), userOpTimeout)
	defer cancel()

	res := mdb.Collection(userCollection).FindOneAndUpdate(
		ctx,
		bson.M{"uid": uid},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After), //返回更新后的文档
	)
	u := &UserInfo{}
	if err := res.Decode(u); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) { //用户可能不存在
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	setCache(u)
	return cloneUser(u), nil
}

//	读-改-写更新：先读最新，再对内存对象打补丁，最后整体 $set 回写。
//
// 适合登录/登出这种多字段一起变的场景。
func UpdateUserInfo(uid int64, mutate func(*UserInfo)) (*UserInfo, error) {
	if mutate == nil {
		return FindUser(uid)
	}
	u, err := FindUser(uid)
	if err != nil {
		return nil, err
	}
	mutate(u)
	now := time.Now().Unix()
	u.OpTime = now
	u.UpdatedAt = now
	set := bson.M{
		"avatarUrl":      u.AvatarUrl,
		"nickName":       u.NickName,
		"loginTimestamp": u.LoginTimestamp,
		"outTimestamp":   u.OutTimestamp,
		"onlineDuration": u.OnlineDuration,
		"status":         u.Status,
		"opTime":         u.OpTime,
		"updatedAt":      u.UpdatedAt,
	}
	return UpdateFields(uid, set)
}

// UpdateUserNickName 更新昵称。
func UpdateUserNickName(uid int64, nickName string) error {
	_, err := UpdateFields(uid, bson.M{"nickName": nickName})
	return err
}

// 更新头像。
func UpdateUserAvatarUrl(uid int64, avatarUrl string) error {
	_, err := UpdateFields(uid, bson.M{"avatarUrl": avatarUrl})
	return err
}

// 登出打点：刷新 outTimestamp 并累加在线时长。
func UpdateUserOutTimestamp(uid int64) error {
	u, err := FindUser(uid)
	if err != nil {
		return err
	}
	now := uint64(time.Now().Unix())
	var delta int64
	if u.LoginTimestamp > 0 && now > u.LoginTimestamp {
		delta = int64(now - u.LoginTimestamp)
	}
	_, err = UpdateUserInfo(uid, func(info *UserInfo) {
		info.OutTimestamp = now
		info.OnlineDuration += delta
	})
	return err
}
