package gate

import (
	"gameServer/common/db/heros"
	"gameServer/common/db/items"
	"gameServer/common/db/reward"
	"gameServer/common/db/user"
	"gameServer/common/errorCode"
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/loginSdk"
	"gameServer/pkg/utils"
	"gameServer/protobuf/pbGo"
	"gameServer/service/common"
	"gameServer/service/common/proto"
	"gameServer/service/services/gate/excelConfig"
	"slices"
	"time"

	"go.uber.org/zap"
)

var (
	//roleCache = cachex.NewCacheX[*user.UserInfo](true, 10*time.Minute, 5*time.Minute)
	//olCache   = cachex.NewCacheX[*user.OL](true, 10*time.Minute, 5*time.Minute)
	//allCache  = cachex.NewCacheX[*user.AllUser](true, 10*time.Minute, 5*time.Minute)

	//node = snowflake.NewNode(1)
	sum = 0
)

// heartHandler 心跳处理
func (g *Gate) heartHandler(session *common.Session, message *common.Message) *common.Resp {
	t := time.Now()
	session.PingTime = t
	return nil
	//return proto.Response1(&pbGo.HeartResp{
	//	Time:     t.Unix(),
	//	Location: t.Location().String(),
	//})
}

// loginHandler 登录
func (g *Gate) loginHandler(session *common.Session, message *common.Message) *common.Resp {
	sum++
	//fmt.Println("=====%d", message.Head.SN)
	//fmt.Println("sum=%d", sum)

	//t := time.Now()
	//session.pingTime = t

	cliReq := &pbGo.LoginReq{}
	if err := proto.Unmarshal(message.Body, cliReq); err != nil {
		log2.Get().Warn("loginHandler UnmarshalFailed false ", zap.Any("err", err))
		return proto.Errorf1(errorCode.ErrorCode_ProtoUnarshalFailed)
	}
	code := cliReq.Code
	if code == "" && cliReq.LoginType == 1 { //第三方登录
		log2.Get().Warn("loginHandler code is null ")
		return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
	}

	var (
		userInfo  *user.UserInfo
		ol        *user.OL
		err       error
		openid    *string
		loginType = string(cliReq.LoginType)
	)

	if cliReq.LoginType == 1 {
		respLogin := loginSdk.DouyinSendReq(loginSdk.AppId, code, loginSdk.Secret)
		if respLogin == nil {
			log2.Get().Warn("loginHandler DouyinSendReq failed ", zap.Any("respLogin", respLogin))
			return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
		}
		openid = respLogin.Openid
	} else if cliReq.LoginType == 0 { //游客登录
		openid = &code
		loginType = "0"
	}

	// ol 查询
	err, ol = user.FindOL(*openid, loginType)
	if err != nil {
		return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
	}
	if ol == nil {
		userInfo, err = user.CreateUser(*openid, loginType)
		if err != nil {
			log2.Get().Error("loginHandler AddUserToOL failed ", zap.Any("openid", openid))
			return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
		}
		userId := userInfo.UserId
		// 初始化奖励
		rewardMap, idList := excelConfig.GetInitLoginConfigReward()
		if rewardMap == nil || len(idList) == 0 {
			log2.Get().Error("user loginHandler GetInitLoginConfigReward failed ", zap.Any("userId:", userId))
			return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
		}
		for _, id := range idList {
			ok := reward.SaveRewardInfo(userId, id)
			if !ok {
				log2.Get().Error(" save SaveRewardInfo is false ", zap.Any("idList:", idList))
				return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
			}
		}

		ok := items.RewardItem(userId, rewardMap)
		if !ok {
			log2.Get().Error("user loginHandler InitLoginConfig failed ", zap.Any("userId:", userId))
			return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
		}

		characterList := excelConfig.GetInitCharacterListReward()
		if characterList == nil {
			log2.Get().Error("user loginHandler etInitCharacterListReward failed ", zap.Any("userId:", userId))

			return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
		}
		ok = heros.UnLockCharacter(userId, characterList)
		if !ok {
			log2.Get().Error("user loginHandler UnLockCharacter failed ", zap.Any("userId:", userId), zap.Any("characterList:", characterList))
			return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
		}

	} else {
		userInfo, err = user.FindUser(ol.UserId)
		if err != nil {
			log2.Get().Error("loginHandler FindUser failed ", zap.Any("UserId：", ol.UserId))
			return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
		}
	}

	// 响应
	var (
		itemList []*pbGo.ItemInfo
		heroList []*pbGo.HeroInfo
	)

	userId := userInfo.UserId

	// 自身道具
	allItems, err := items.GetAllItems(userId)
	if err != nil {
		log2.Get().Error("loginHandler GetAllItems failed ", zap.Any("err", err))
		return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
	}
	itemList = make([]*pbGo.ItemInfo, len(allItems))
	i := 0
	for id, count := range allItems {
		itemList[i] = &pbGo.ItemInfo{
			ItemId: uint64(id),
			Count:  count,
		}
		i++
	}
	// 自身解锁的人物
	characterList := heros.GetAllUnLockCharacter(userId)
	heroList = make([]*pbGo.HeroInfo, len(characterList))
	for ii, info := range characterList {
		heroList[ii] = &pbGo.HeroInfo{
			HeroId: uint32(info.Id),
			Unlock: true,
		}
	}
	// 奖励信息信息
	awardConfig := excelConfig.GetAllReceiveAwardConfig()
	if awardConfig == nil {
		log2.Get().Error("loginHandler GetAllReceiveAwardConfig failed ，awardConfig is null")
		return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
	}
	rewardInfo := reward.GetAllRewardInfo(userId)
	awardInfoList := make([]*pbGo.AwardInfo, 0)
	for _, info := range awardConfig {
		rewardType := info.RewardType
		timestamp := int64(0)
		if rewardInfo == nil { //直接奖励,显示
			timestamp = 0
		} else {
			for _, r := range rewardInfo {
				if r.Id == info.Id {
					timestamp = r.Timestamp
				}
			}
		}
		one := &pbGo.AwardInfo{
			Id: uint32(info.Id),
		}
		if rewardType == 1 {
			continue
		} else if rewardType == 2 {
			if timestamp > 0 { //已经奖励了
				one.IsReceived = true
			}
		} else if rewardType == 3 {
			isToday := utils.IsToday(timestamp)
			if isToday {
				one.IsReceived = true
			}
		}
		awardInfoList = append(awardInfoList, one)
	}

	// 保存会话
	player := common.PlayerPool.Get()
	player.UserId = userId
	session.Player = player
	g.tcpServer.roles.Store(session.Player.UserId, session)
	// 保存网关节点
	if !slices.Contains(session.Player.ServerIds, config.Get().NodeID()) {
		session.Player.ServerIds = append(session.Player.ServerIds, config.Get().NodeID())
	}

	return proto.Response1(&pbGo.LoginResp{
		AwardInfoList: awardInfoList,
		UserId:        userId,
		ItemList:      itemList,
		HeroList:      heroList,
	})
}
