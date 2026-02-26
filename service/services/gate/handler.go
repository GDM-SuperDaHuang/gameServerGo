package gate

import (
	"fmt"
	"gameServer/pkg/config"
	"gameServer/protobuf/pbGo"
	"gameServer/service/common"
	"gameServer/service/common/proto"
	"slices"
	"time"
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

var uuid uint64 = 0

// loginHandler 登录
func (g *Gate) loginHandler(session *common.Session, message *common.Message) *common.Resp {
	uuid++
	fmt.Println("=====%d", message.Head.SN)
	//t := time.Now()
	//session.pingTime = t
	session.Player = &common.Player{
		RoleID: 1,
	}
	g.tcpServer.roles.Store(session.Player.RoleID, session)
	// 保存网关节点
	if !slices.Contains(session.Player.ServerIds, config.Get().NodeID()) {
		session.Player.ServerIds = append(session.Player.ServerIds, config.Get().NodeID())
	}
	return proto.Response1(&pbGo.LoginResp{
		Uuid: uuid,
		Name: "akkkkk",
		Role: &pbGo.User{
			Id: 123,
		},
	})

	//cliReq := &pb_gate.LoginReq{}
	//if err := proto.Unmarshal(message.Body, cliReq); err != nil {
	//	return proto.UnmarshalFailed(err)
	//}
	//
	//// 1. JWT 验证
	//m, err := g.loginJWT.Verify(cliReq.GetToken())
	//if err != nil {
	//	logger.Get().Error("[gate] jwt verify failed", zap.String("accessToken", cliReq.GetToken()), zap.Error(err))
	//	return proto.Error(pb_protocol.ErrorCode_LoginTokenVerifyFailed, err.Error())
	//}
	//uuid, found := m["uuid"]
	//if !found {
	//	return proto.Error(pb_protocol.ErrorCode_LoginTokenVerifyFailed, "uuid not found")
	//}
	//uuidF, ok := uuid.(float64)
	//if !ok {
	//	return proto.Error(pb_protocol.ErrorCode_LoginTokenVerifyFailed, "uuid invalid format")
	//}
	//if uint64(uuidF) != cliReq.GetUuid() {
	//	return proto.Errorf(pb_protocol.ErrorCode_LoginTokenVerifyFailed, "uuid not match, uuid(token): %f, uuid(req): %d", uuidF, cliReq.GetUuid())
	//}
	//
	//// 2. 区服验证
	//if cliReq.GetServerId() <= 0 {
	//	return proto.Error(pb_protocol.ErrorCode_ServerIDEmpty, "server id empty")
	//}
	//// TODO 验证区服开放时间，状态，白名单
	//
	//// 3. 绑定数据
	//player := session.playerPool.Get()
	//player.set(cliReq.GetUuid(), cliReq.GetServerId())
	//session.player = player
	//
	//// 4. 指定版本号
	//session.version = cliReq.GetVersion()
	//if session.version == nil {
	//	return proto.Error(pb_protocol.ErrorCode_ServiceVersionsNotFound, "")
	//}
	//
	//// 5. 允许后续发送到 gate 以外的服务
	//session.ready.Store(true)
	//
	//// 6. 从账号服拉取角色列表，meig
	//accountReq := &pb_account.AccountListReq{
	//	Uuid:     player.accountID,
	//	ServerId: player.serverID,
	//}
	//accountResp := &pb_account.AccountListResp{}
	//if code, err := g.protocolAccount(session, pb_protocol.MessageID_AccountList, accountReq, accountResp); err != nil {
	//	return proto.Error(code, err.Error())
	//}
	//roles := accountResp.GetList()
	//if len(roles) == 0 {
	//	return proto.Error(pb_protocol.ErrorCode_AccountListIsEmpty, "")
	//}
	//role := roles[0]
	//roleID := role.GetRoleId()
	//
	//session.player.roleID = roleID
	//g.tcpServer.roles.Store(roleID, session)
	//
	//lang := cliReq.GetLang()
	//
	//// 7. 角色创建
	//if role.IsNew {
	//	return g.gameCreateRole(session, role, lang)
	//}
	//
	//// 8. 进入角色服
	//return g.gameLoginRole(session, role, lang)
}
