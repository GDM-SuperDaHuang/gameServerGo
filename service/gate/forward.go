package gate

import (
	"Server/service/datapack"
	"Server/service/proto"
	"Server/service/rpc"
)

// 数据转发
func (g *Gate) forward(session *Session, message *datapack.Message) *proto.Resp {
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

	// 转发到其它服务，需要 session 准备好
	//if !session.ready.Load() {
	//	return pb_protocol.ErrorCode_SessionNotReady, nil
	//}

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

func (g *Gate) forwardLocal(session *Session, message *datapack.Message) *proto.Resp {
	switch message.Head.Protocol {
	case proto.MessageID_Heart:
		return g.heartHandler(session)
		//case proto.MessageID_SecretSharePubKey:
		//	return g.secretSharePubKeyHandler(session, message)
		//case proto.MessageID_SecretShareTest:
		//	return g.secretShareTestHandler(session, message)
		//case proto.MessageID_Login:
		//	return g.loginHandler(session, message)
	}
	return proto.Errorf1(proto.ErrorCode_ProtocolNotFound)
}

func (g *Gate) rpcForward(session *Session, message *datapack.Message) *proto.Resp {
	return g.forwardTarget(0, session, message, accountrpc.RPCClients())
}

func (g *Gate) forwardTarget(targetID uint32, session *Session, message *datapack.Message, rpcclient pkgrpc.Client) (pb_protocol.ErrorCode, []byte) {
	forwardReq := rpc.BuildForwardReq(
		session.RealServerID(),
		session.serverID(),
		session.roleID(),
		pb_protocol.MessageID(message.Head.Protocol),
		message.Body,
	)
	defer internalrpc.ReleaseForwardReq(forwardReq)

	return internalrpc.ForwardTarget(targetID, forwardReq, rpcclient, session.version.GameVersionMin, session.version.GameVersionMax)
}
