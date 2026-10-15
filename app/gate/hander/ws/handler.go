package ws

import (
	"context"
	"errors"
	"gameServer/app/gate/hander/excelConfig"
	"gameServer/common/db/user"
	"gameServer/common/errorCode"
	"gameServer/pkg/config"
	"gameServer/pkg/encrypt"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/loginSdk"
	"gameServer/protobuf/pbGo"
	"gameServer/service/common"
	"gameServer/service/common/proto"
	"slices"
	"strconv"

	"github.com/smallnest/rpcx/share"
	"go.uber.org/zap"
)

// heartHandler 心跳处理 1
func (wss *WsServer) heartHandler(s *Session, message *common.Message) *common.Resp {
	return proto.Response1(nil)
}

// loginHandler 登录 4
// 登录分两条路径（互斥）：
//  1. 快速登录：cliReq.AccessToken 非空时优先走，Redis login:token:<SHA256(raw)[32]byte> -> uid 再查用户信息，TTL 3 天滑动续期（Redis 无明文，只存字节）
//  2. 第三方/游客登录：code 换 openid 后 loginType, openid 查库，
//     存在则续期（Mongo loginTimestamp + Redis TTL），不存在则 GenUid 建号并签发新 token（存 Redis）。
func (wss *WsServer) loginHandler(s *Session, message *common.Message) *common.Resp {
	cliReq := &pbGo.LoginReq{}
	if err := proto.Unmarshal(message.Body, cliReq); err != nil {
		log2.Get().Warn("loginHandler UnmarshalFailed false ", zap.Any("err", err))
		return proto.Errorf1(errorCode.ErrorCode_ProtoUnarshalFailed)
	}

	var (
		userInfo     *user.UserInfo
		respRawToken string // 本次要下发给客户端的明文凭证；老用户/快速登录为空或回显，客户端继续用旧 token
	)

	// ---------- 1. 快速登录：AccessToken 只查 Redis，再按 uid 查用户信息 ----------
	if len(cliReq.AccessToken) > 0 {
		u, err := user.LoginByToken(cliReq.AccessToken)
		if err != nil {
			if errors.Is(err, user.ErrTokenExpired) {
				log2.Get().Warn("loginHandler fast login token expired")
			} else {
				log2.Get().Warn("loginHandler fast login failed", zap.Error(err))
			}
			return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
		}
		if u.Status == 1 {
			log2.Get().Warn("loginHandler user banned", zap.Int64("uid", u.Uid))
			return proto.Errorf1(errorCode.ErrorCode_UserStatusBan)
		}
		userInfo = u
		// token 本体不变（TouchLogin 只续期 Redis TTL），直接回显请求 token，
		// 客户端继续用它做下次快速登录。
		respRawToken = cliReq.AccessToken
	} else {
		// ---------- 2. 第三方/游客登录：code -> openid -> GetOrCreateUser ----------
		var (
			openid    string
			loginType = cliReq.LoginType
		)

		switch cliReq.LoginType {
		case 1: // 抖音登录
			if cliReq.Code == "" {
				log2.Get().Warn("loginHandler code is null ")
				return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
			}
			respLogin := loginSdk.DouyinSendReq(cliReq.Code)
			if respLogin == nil || respLogin.Openid == nil || *respLogin.Openid == "" {
				log2.Get().Warn("loginHandler DouyinSendReq failed ", zap.Any("respLogin", respLogin))
				return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
			}
			openid = *respLogin.Openid
		case 2: // 微信登录
			if cliReq.Code == "" {
				log2.Get().Warn("loginHandler code is null ")
				return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
			}
			respLogin := loginSdk.WxLoginReq(cliReq.Code)
			if respLogin == nil || respLogin.OpenID == "" {
				log2.Get().Warn("loginHandler WxLoginReq failed ", zap.Any("respLogin", respLogin))
				return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
			}
			openid = respLogin.OpenID
		case 0: // 游客登录：客户端生成的设备码即 openid
			if cliReq.Code == "" {
				log2.Get().Warn("loginHandler guest code is null ")
				return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
			}
			openid = cliReq.Code
		default:
			log2.Get().Warn("loginHandler unknown loginType", zap.Int32("loginType", cliReq.LoginType))
			return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
		}

		if openid == "" {
			log2.Get().Warn("loginHandler openid is null ", zap.Int32("loginType", cliReq.LoginType))
			return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
		}

		// 可能会创建用户
		u, rawToken, err := user.GetOrCreateUser(loginType, openid, cliReq.NickName, cliReq.AvatarUrl)
		if err != nil {
			if errors.Is(err, user.ErrUserNotFound) {
				log2.Get().Warn("loginHandler GetOrCreateUser not found", zap.Error(err))
				return proto.Errorf1(errorCode.ErrorCode_LoginFailed)
			}
			log2.Get().Error("loginHandler GetOrCreateUser failed", zap.Error(err),
				zap.Int32("loginType", loginType), zap.String("openid", openid))
			return proto.Errorf1(errorCode.ErrorCode_DBError)
		}
		if u.Status == 1 {
			log2.Get().Warn("loginHandler user banned", zap.Int64("uid", u.Uid))
			return proto.Errorf1(errorCode.ErrorCode_UserStatusBan)
		}
		userInfo = u
		// 新建用户时 rawToken 为新签发的明文凭证（已落 Redis），需下发客户端；
		// 老用户 TouchLogin 续期 Redis TTL 但凭证不变，rawToken 为 ""（Redis 无有效 token 时会签发新 token 并下发），
		// 客户端无新 token 时继续用已有 token。
		respRawToken = rawToken
	}

	// 响应
	var (
		Uid int64
	)

	// 保存会话
	Uid = userInfo.Uid
	if s.Player != nil && s.Player.Uid == Uid {
		// 同连接刷新：复用，不Get/Store
	} else {
		var oldPlayer *common.Player
		var oldUid int64
		if s.Player != nil {
			oldUid, oldPlayer = s.Player.Uid, s.Player
			s.Player = nil
			if oldUid != 0 && oldUid != Uid {
				wss.roles.CompareAndDelete(oldUid, s) // 清换号残留
			}
		}

		if old := wss.FindSession(Uid); old != nil && old != s {
			wss.DelSessionIfMatch(Uid, old) // 踢异地连接
		}
		player := common.PlayerPool.Get()
		player.Uid = Uid
		s.Player = player
		if oldPlayer != nil {
			common.FreePlayer(oldPlayer) // 同连接串行，无在途RPC，可立即归还
		}
		wss.roles.Store(Uid, s)
	}

	//player := common.PlayerPool.Get()
	//player.Uid = Uid
	//s.Player = player
	//
	//wss.TempCountWai.Add(1)
	//// 重连替换：先尝试清理当前 Uid 的旧连接，避免旧连接退出时误删新连接。
	//if old := wss.FindSession(Uid); old != nil && old != s {
	//	wss.TempCountInto.Add(1)
	//	wss.DelSessionIfMatch(Uid, old)
	//}
	//wss.roles.Store(s.Player.Uid, s)

	// 保存网关节点
	if !slices.Contains(s.Player.ServerIds, config.Get().NodeID()) {
		s.Player.ServerIds = append(s.Player.ServerIds, config.Get().NodeID())
	}

	resq := &pbGo.LoginResp{
		Uid:         Uid,
		AccessToken: respRawToken,
		//ItemList: itemList,
		//HeroList: heroList,
	}

	//elasticMap := make(map[string]interface{})
	//elasticMap["req"] = cliReq
	//elasticMap["resq"] = resq
	//elastic.Upload("loginHandler", elasticMap)
	return proto.Response1(resq)
}

// keyExchangeHandle 密钥交换 2 (明文).
// 客户端: shared = X25519(client_private, server_public)
// 服务端: shared = X25519(server_private, client_public)
// salt = client_random || server_random, HKDF-SHA256 派生 client/server_write_key 各 32B + nonce prefix 各 4B。
func (wss *WsServer) keyExchangeHandle(s *Session, message *common.Message) *common.Resp {
	cliReq := &pbGo.KeyExchangeReq{}
	if err := proto.Unmarshal(message.Body, cliReq); err != nil {
		log2.Get().Warn("keyExchangeHandle UnmarshalFailed", zap.Any("err", err))
		return proto.Errorf1(errorCode.ErrorCode_ProtoUnarshalFailed)
	}
	if len(cliReq.ClientPublicKey) != encrypt.KeySize {
		log2.Get().Warn("keyExchangeHandle invalid client_public_key len", zap.Int("len", len(cliReq.ClientPublicKey)))
		return proto.Errorf1(errorCode.ErrorCode_KeyExchangeFailed)
	}
	if len(cliReq.ClientRandom) < encrypt.MinRandomSize || len(cliReq.ClientRandom) > encrypt.MaxRandomSize {
		log2.Get().Warn("keyExchangeHandle invalid client_random len", zap.Int("len", len(cliReq.ClientRandom)))
		return proto.Errorf1(errorCode.ErrorCode_KeyExchangeFailed)
	}

	h, err := encrypt.NewServerHandshake(cliReq.ClientPublicKey, cliReq.ClientRandom)
	if err != nil {
		log2.Get().Warn("keyExchangeHandle NewServerHandshake failed", zap.Error(err))
		return proto.Errorf1(errorCode.ErrorCode_KeyExchangeFailed)
	}
	s.SetHandshake(h)

	resq := &pbGo.KeyExchangeResp{
		ServerPublicKey: h.ServerPublicKey(),
		ServerRandom:    h.ServerRandom(),
	}
	return proto.Response1(resq)
}

// KeyVerificationHandle 密钥验证 3 (明文).
// 客户端 confirm_mac = HMAC-SHA256(client_write_key, "LR-GAME-V1 client confirm" || client_random||server_random||client_pub||server_pub)
// 服务端回 confirm_mac = HMAC-SHA256(server_write_key, "LR-GAME-V1 server confirm" || 同上 transcript)。
// 验证通过后建成按会话 AES-256-GCM 双向通道: 服务端加密用 server_write_key, 解密用 client_write_key。
func (wss *WsServer) KeyVerificationHandle(s *Session, message *common.Message) *common.Resp {
	cliReq := &pbGo.KeyVerificationReq{}
	if err := proto.Unmarshal(message.Body, cliReq); err != nil {
		log2.Get().Warn("KeyVerificationHandle UnmarshalFailed", zap.Any("err", err))
		return proto.Errorf1(errorCode.ErrorCode_ProtoUnarshalFailed)
	}
	h := s.GetHandshake()
	if h == nil {
		log2.Get().Warn("KeyVerificationHandle handshake missing, exchange first")
		return proto.Errorf1(errorCode.ErrorCode_SecureChannelMissing)
	}

	if len(cliReq.ConfirmMac) != 32 {
		log2.Get().Warn("KeyVerificationHandle invalid confirm_mac len", zap.Int("len", len(cliReq.ConfirmMac)))
		return proto.Errorf1(errorCode.ErrorCode_KeyVerifyFailed)
	}

	if !h.VerifyClient(cliReq.ConfirmMac) {
		log2.Get().Warn("KeyVerificationHandle VerifyClient failed")
		return proto.Errorf1(errorCode.ErrorCode_KeyVerifyFailed)
	}

	ch, err := h.BuildServerChannel()
	if err != nil {
		log2.Get().Error("KeyVerificationHandle BuildServerChannel failed", zap.Error(err))
		return proto.Errorf1(errorCode.ErrorCode_KeyVerifyFailed)
	}
	s.EstablishSecure(ch)

	resq := &pbGo.KeyVerificationResp{
		ConfirmMac: h.ServerConfirm(),
	}
	return proto.Response1(resq)
}

// KeyTestHandler 密钥测试 5
func (wss *WsServer) KeyTestHandler(session *Session, message *common.Message) *common.Resp {
	cliReq := &pbGo.KeyTestReq{}
	if err := proto.Unmarshal(message.Body, cliReq); err != nil {
		log2.Get().Warn("configHandler UnmarshalFailed false ", zap.Any("err", err))
		return proto.Errorf1(errorCode.ErrorCode_ProtoUnarshalFailed)
	}

	resq := &pbGo.KeyTestResp{
		TestResp: "阿斯蒂芬静安寺平均分怕是积分qawsdfasdsadsafasdgvasasdddd/*/*-++/+-//*nmawsdfasfasdgvasasdddd/*/*-++/+-//*nmawsdfasdsadasdfasdfas/*/++dfadsadasdfasdfas/*/++dfasasdddadsafasdgvasasdddd/*/*-++/+-//*nmawsdfasdsadasdfasdfas/*/+ddddasfawq+/-+/*+rtewyrhtjhgkasdflaadsafasdgvasasdddd/*/*-++/+-//*nmawsdfasdsadasdfasdfas/*/+skflkashfljkashfjhsakjfddddddddddvgbfasdgvasasdddd/*/*-++/+-//*nmawsdfasdsadasdfasdfas/*/++dfanmqawsdfasdddddddafasdgvasasdddd/*/*-++/+-//*nmawsdfasdsadasdfasdfas/*/++dfasdflaskflkashfljkashfjhsakjfddddddddddvgbnmwsqadsafasdgvasasdddd/*/*-++/+-//*nmawsdfasdsadasdfasdfas/*/+awsdfasdddddddasdflaskflkashfljkashfjhsakjfddddddddddvgbnmdfaqawsdfaadsafasdgvasasdddd/*/*-++/+-//*nmawsdfasdsadasdfasdfas/*/+sdddddddasdflaskflkashfljkashfjhsakjfddddddddddvgbnmsdddddd+/*+/+/*+/*+dasdflaskflkashadsafasdgvasasdddd/*/*-++/+-//*nmawsdfasdsadasdfasdfas/*/+fljkashfjhsak+//jfdddddddd+-*+/ddvgbnmlaskflka+/*++/+/*+/*+/+/*+/********ddddvgbnm",
	}

	return proto.Response2(resq, common.FlagEncrypted)
}

// heartHandler 热更新
func (wss *WsServer) configHandler(session *Session, message *common.Message) *common.Resp {
	cliReq := &pbGo.UpdateConfigRep{}
	if err := proto.Unmarshal(message.Body, cliReq); err != nil {
		log2.Get().Warn("configHandler UnmarshalFailed false ", zap.Any("err", err))
		return proto.Errorf1(errorCode.ErrorCode_ProtoUnarshalFailed)
	}

	ctx := context.Background()
	id := int(cliReq.Id)
	groupId := int(cliReq.GroupId)
	data := cliReq.ExtraData
	if data > 0 { //仅仅更新本地
		err := excelConfig.UpdateConfig()
		if err != nil {
			log2.Get().Error("configHandler UpdateConfig failed ", zap.Any("err", err))
			return proto.Errorf1(errorCode.ErrorCode_UpdateConfigFailed)
		}
		return proto.Response1(&pbGo.UpdateConfigResp{})
	} else { //更新远程
		ctx = context.WithValue(ctx, share.ResMetaDataKey, map[string]string{
			"id":      strconv.Itoa(id),
			"groupId": strconv.Itoa(int(groupId)),
		})
		// 调用远程的Dispatch方法
		rpcReq := common.RpcMessage{
			Data:   message,
			Player: session.Player,
		}
		var rpcResp = &common.Resp{}
		err := RpcGateClient.Call(ctx, "Dispatch", rpcReq, rpcResp)
		if err != nil {
			log2.Get().Warn("configHandler RpcGateClient false ", zap.Any("err", err))
			return proto.Errorf1(errorCode.ErrorCode_RemoteCallFailed)
		}
		return rpcResp
	}
}
