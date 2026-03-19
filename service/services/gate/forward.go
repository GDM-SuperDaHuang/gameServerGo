package gate

import (
	"context"
	"fmt"
	"gameServer/common/errorCode"
	"gameServer/pkg/cache/ssdb"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/utils"
	"gameServer/service/common"
	"gameServer/service/common/proto"
	"gameServer/service/rpc"
	"gameServer/service/rpc/client/selector"
	"strconv"
	"time"

	"github.com/smallnest/rpcx/share"
	"go.uber.org/zap"
)

// 数据转发
func (g *Gate) forward(session *common.Session, message *common.Message) *common.Resp {
	protocol := int32(message.Head.Protocol)
	// 按照协议号 protocol 转发到对应的服务
	// protocol 范围: 0 ~ 65535

	// 并发控制
	//1. 生产模式下，只允许心跳验证并发处理
	//2. 开发模式下，全部都只能串行处理，方便调试
	if protocol != 1 {
		session.ReadChan <- struct{}{}
		defer func() {
			<-session.ReadChan
		}()
	}
	// 本地处理: < 1000
	if protocol < 1000 {
		return g.forwardLocal(session, message)
	}

	//转发到其它服务，需要 登录 准备好
	if session.Player == nil {
		return proto.Errorf1(errorCode.ErrorCode_PushFailed)
	}

	// 远程rpc: 101 ~ 199
	return g.rpcForward(session, message)
}

func (g *Gate) forwardLocal(session *common.Session, message *common.Message) *common.Resp {
	switch message.Head.Protocol {
	case 1:
		return g.heartHandler(session, message)
	//case proto.MessageID_SecretSharePubKey:
	//	return g.secretSharePubKeyHandler(session, message)
	//case proto.MessageID_SecretShareTest:
	//	return g.secretShareTestHandler(session, message)
	case 4:
		fmt.Println("=======4=============")
		return g.loginHandler(session, message)
	}
	return proto.Errorf1(errorCode.ErrorCode_ProtocolNotFound)
}

func (g *Gate) rpcForward(session *common.Session, message *common.Message) *common.Resp {
	//return g.forwardTarget (session, message, nil)
	// 根据etcd创建客户端，进行调用
	return ForwardTarget(session, message, RpcGateClient)
}

func ForwardTarget(session *common.Session, message *common.Message, rpcClient rpc.ClientInterface) *common.Resp {
	start := time.Now().UnixMilli()
	rpcReq := common.RpcMessage{
		Data:   message,
		Player: session.Player,
	}
	var rpcResp = &common.Resp{}
	var err error

	ctx := context.Background()
	protocolId := message.Head.Protocol
	groupId := utils.GetGroupIdByPb(int(protocolId))
	id := utils.GetServerId(groupId, session.Player.ServerIds) //本网关可能没有
	flag := false
	if id == 0 {
		if protocolId >= 1000 && protocolId < 2000 { //room类型协议，可能正在进行游戏

			roleID := strconv.FormatUint(session.Player.UserId, 10)
			value, err := ssdb.GetClient().Get("RoleID:" + roleID)
			if err == nil && value.String() != "" {
				id = value.Int()
			}
		}

		s, ok := rpcClient.GetSelector().(*selector.DefaultSelector)
		if !ok {
			log2.Get().Error("转换失败")
			return proto.Errorf1(errorCode.ErrorCode_RemoteCallFailed)
		}
		ctx = context.WithValue(ctx, share.ResMetaDataKey, map[string]string{
			"id":      "0",
			"groupId": strconv.Itoa(int(groupId)),
		})
		addr := s.Select(ctx, "", "", nil)
		if addr != "" {
			for _, info := range s.Servers {
				if info.Address == addr {
					id = int(info.Id)
					break
				}
			}
		}
		flag = true
	}

	ctx = context.WithValue(ctx, share.ResMetaDataKey, map[string]string{
		"id":      strconv.Itoa(id),
		"groupId": strconv.Itoa(int(groupId)),
	})
	// 调用远程的Dispatch方法
	//start := time.Now() // 记录开始时间

	err = rpcClient.Call(ctx, "Dispatch", rpcReq, rpcResp)
	//err = rpcClient.Go(context.Background(), "Dispatch", rpcReq, rpcResp)

	// 计算耗时
	end := time.Now().UnixMilli()
	fmt.Printf("gate:函数运行时间: %v\n", end-start)

	if err != nil {
		log2.Get().Error("ForwardTarget call Dispatch failed",
			zap.Uint16("ProtocolId:", rpcReq.Data.Head.Protocol),
			zap.Uint64("userId:", rpcReq.Player.UserId),
			zap.Error(err),
		)
		//todo 如果失败，也更新session,删除id
		for i := len(session.Player.ServerIds) - 1; i >= 0; i-- {
			if session.Player.ServerIds[i] == uint32(id) {
				session.Player.ServerIds = append(session.Player.ServerIds[:i], session.Player.ServerIds[i+1:]...)
			}
		}
		return proto.Errorf1(errorCode.ErrorCode_RemoteCallFailed)
	}

	// 更新 session
	if flag {
		//if !flagRoom { //非房间类型
		//	m, ok := ctx.Value(share.ResMetaDataKey).(map[string]string)
		//	if ok {
		//		id, _ = strconv.Atoi(m["id"])
		//	}
		//}
		//m, ok := ctx.Value(share.ResMetaDataKey).(map[string]string)
		//if ok {
		//	id, _ = strconv.Atoi(m["id"])
		//}

		// 排除相同的
		for _, serverId := range session.Player.ServerIds {
			if serverId == uint32(id) {
				return rpcResp
			}
		}
		if id == 0 {
			return rpcResp
		}
		session.Player.ServerIds = append(session.Player.ServerIds, uint32(id))
	}
	return rpcResp
}

// Receive 网关接收其它服务的单个消息推送,必须实现，不然无法注册，rpcx协程
func (g *Gate) Receive(_ context.Context, req *common.RpcMessage, resp *common.Resp) error {
	//找到对应的 session，写入消息
	session := g.tcpServer.findSession(req.Player.UserId)
	if session == nil {
		log2.Get().Warn("[Receive] session not found", zap.Uint64("roleID", req.Player.UserId))
		return fmt.Errorf("session not found, roleID: %d", req.Player.UserId)
	}
	// todo 需要修改room 信息?
	//if req.Data.Head.Protocol == 1010 { //离开room
	//	groupId := utils.GetGroupIdByPb(int(req.Data.Head.Protocol))
	//	id := utils.GetServerId(groupId, session.Player.ServerIds) //本网关可能没有
	//	for i := len(session.Player.ServerIds) - 1; i >= 0; i-- {
	//		if session.Player.ServerIds[i] == uint32(id) {
	//			session.Player.ServerIds = append(session.Player.ServerIds[:i], session.Player.ServerIds[i+1:]...)
	//		}
	//	}
	//	//for index, fid := range session.Player.ServerIds {
	//	//	if fid == uint32(id) {
	//	//		session.Player.ServerIds = append(session.Player.ServerIds[:index], session.Player.ServerIds[index+1:]...)
	//	//	}
	//	//}
	//}
	resp.Body = req.Data.Body
	if req.Data.Head.Protocol == 1005 {
		log2.Get().Info(" ==================  1005    ===", zap.Uint64("userId:", req.Player.UserId))
	}
	return g.tcpServer.write(session, resp, req.Data)
}
