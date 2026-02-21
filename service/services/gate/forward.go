package gate

import (
	"context"
	"fmt"
	"gameServer/service/utils"
	"strconv"

	"gameServer/service/common"
	"gameServer/service/common/proto"
	"gameServer/service/logger"
	"gameServer/service/rpc"
	"github.com/smallnest/rpcx/share"
	"go.uber.org/zap"
)

// 数据转发
func (g *Gate) forward(session *common.Session, message *common.Message) *common.Resp {
	protocol := int32(message.Head.Protocol)
	// 按照协议号 protocol 转发到对应的服务
	// protocol 范围: 0 ~ 65535

	// 并发控制
	// 1. 生产模式下，只允许心跳验证并发处理
	// 2. 开发模式下，全部都只能串行处理，方便调试
	//if protocol != int32(pb_protocol.MessageID_Heart) || config.Get().IsDevelop() {
	//	session.readChan <- struct{}{}
	//	defer func() {
	//		<-session.readChan
	//	}()
	//}

	// 本地处理: < 100
	if protocol < 100 {
		return g.forwardLocal(session, message)
	}

	//转发到其它服务，需要 登录 准备好
	if session.Player == nil {
		return proto.Errorf1(proto.ErrorCode_PushFailed)
	}

	// 远程rpc: 101 ~ 199
	return g.rpcForward(session, message)

	//if protocol < 199 {
	//	return g.rpcForward(session, message)
	//}
	//
	//// battle: 400 ~ 479
	//if protocol < 479 {
	//	return g.forwardBattle(session, message)
	//}
	//
	//// battlerecord: 480 ~ 499
	//if protocol < 499 {
	//	return g.forwardBattleRecord(session, message)
	//}
	//
	//// lang: 500 ~ 549
	//if protocol < 549 {
	//	return g.forwardLang(session, message)
	//}
	//
	//// unions: 600 - 799
	//if protocol >= 600 && protocol <= 799 {
	//	return g.forwardUnion(session, message)
	//}
	//
	//// game: 1001 - 39999
	//if protocol < 39999 {
	//	return g.forwardGame(session, message)
	//}
	//
	//// sdk: 40000 ~ 41999
	//if protocol < 41999 {
	//	return g.forwardSDK(session, message)
	//}

	//return proto.Errorf(pb_protocol.ErrorCode_ProtocolNotFound, "协议号: %d", protocol)
}

func (g *Gate) forwardLocal(session *common.Session, message *common.Message) *common.Resp {
	switch message.Head.Protocol {
	//case proto2.MessageID_Heart:
	//return g.heartHandler(session)
	//case proto.MessageID_SecretSharePubKey:
	//	return g.secretSharePubKeyHandler(session, message)
	//case proto.MessageID_SecretShareTest:
	//	return g.secretShareTestHandler(session, message)
	case 1:
		return g.loginHandler(session, message)
	}
	return proto.Errorf1(proto.ErrorCode_ProtocolNotFound)
}

func (g *Gate) rpcForward(session *common.Session, message *common.Message) *common.Resp {
	//return g.forwardTarget (session, message, nil)
	// 根据etcd创建客户端，进行调用
	return ForwardTarget(session, message, RPCClients())
}

func ForwardTarget(session *common.Session, message *common.Message, rpcClient rpc.ClientInterface) *common.Resp {
	// todo 可优化为对象池
	rpcReq := common.RpcMessage{
		Data:   message,
		Player: session.Player,
	}
	var rpcResp = &common.Resp{}
	var err error

	//if targetID > 0 {
	//	err = rpcClient.Wrap(targetID, versionMin, versionMax).Call(context.Background(), "Dispatch", rpcReq, rpcResp)
	//} else {
	//	err = rpcClient.Call(context.Background(), "Dispatch", rpcReq, rpcResp)
	//}
	ctx := context.Background()
	protocolId := message.Head.Protocol
	if protocolId >= 101 {
		groupId := utils.GetGroupIdByPb(int(protocolId))

		session.Player.ServerIds
		if protocolId >= 101 && protocolId <= 1000 {
			// todo
			ctx = context.WithValue(ctx, share.ResMetaDataKey, map[string]string{
				"id":      strconv.Itoa(10),
				"groupId": strconv.Itoa(groupId),
			})
		}

		// todo
		ctx = context.WithValue(ctx, share.ResMetaDataKey, map[string]string{
			"id": strconv.Itoa(10),
		})
		//固定
		//ctx = context.WithValue(ctx, "groupId", "1")
		//ctx = context.WithValue(ctx, "id", 10)
	} else if message.Head.Protocol > 1000 {
		ctx = context.WithValue(ctx, "groupId", "2")
	} else if message.Head.Protocol > 2001 {
		ctx = context.WithValue(ctx, "groupId", "3")
	}

	// 调用远程的Dispatch方法
	err = rpcClient.Call(ctx, "Dispatch", rpcReq, rpcResp)
	//err = rpcClient.Go(context.Background(), "Dispatch", rpcReq, rpcResp)

	if err != nil {
		//logger.Get().Error(
		//	"ForwardTarget call Dispatch failed",
		//	zap.String("target", rpcclient.Name()),
		//	zap.Uint32("realServerID", rpcReq.RealServerId),
		//	zap.Uint32("serverID", req.ServerId),
		//	zap.Uint64("roleID", req.RoleId),
		//	zap.Uint16("protocol", uint16(req.Protocol)),
		//	zap.Error(err),
		//)
		return proto.Errorf1(proto.ErrorCode_RemoteCallFailed)
	}

	//if resp.Code == pb_protocol.ErrorCode_Success && len(resp.Data) == 0 {
	//	return proto.Response(nil)
	//}
	//
	//if resp.Code != pb_protocol.ErrorCode_Success {
	//	return proto.Error(resp.Code, resp.Devmsg)
	//}

	return rpcResp
}

// Receive 网关接收其它服务的单个消息推送,必须实现，不然无法注册
func (g *Gate) Receive(_ context.Context, req *common.RpcMessage, resp *common.Resp) error {
	//找到对应的 session，写入消息
	session := g.tcpServer.findSession(req.Player.RoleID)
	if session == nil {
		logger.Get().Warn("[Receive] session not found", zap.Uint64("roleID", req.Player.RoleID))
		return fmt.Errorf("session not found, roleID: %d", req.Player.RoleID)
	}
	return g.tcpServer.write(session, resp, req.Data)
}
