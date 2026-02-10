package rpc

import (
	"Server/service/datapack"
	"Server/service/logger"
	"Server/service/proto"
	"Server/service/session"
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
)

// ForwardTarget 调用目标服务 Forward.go 中的 Dispatch 函数，也就是在各服务中，modules/xxx/action.go 中的各种 xxxHandler 函数
//
//   - targetID: 目标ID，如果是唯一服务，填 0。如果是多服务，填目标服务的ID，例如调用 game-1 的填 1
//   - req: 请求参数，请使用 BuildForwardReq 构建，内涵对象池，需要调用 ReleaseForwardReq 释放
//   - rpcclient: rpc 客户端
//   - versionMin: 最小版本号，填 0 表示不限制
//   - versionMax: 最大版本号，填 0 表示不限制
//func ForwardTarget(targetID uint32, req *pb_forward.ForwardReq, rpcclient pkgrpc.Client, versionMin, versionMax uint32) (pb_protocol.ErrorCode, []byte) {
//	resp := &pb_forward.ForwardResp{}
//
//	var err error
//	if targetID > 0 {
//		err = rpcclient.Wrap(targetID, versionMin, versionMax).Call(context.Background(), "Dispatch", req, resp)
//	} else {
//		err = rpcclient.Call(context.Background(), "Dispatch", req, resp)
//	}
//
//	if err != nil {
//		logger.Get().Error(
//			"ForwardTarget call Dispatch failed",
//			zap.String("target", rpcclient.Name()),
//			zap.Uint32("realServerID", req.RealServerId),
//			zap.Uint32("serverID", req.ServerId),
//			zap.Uint64("roleID", req.RoleId),
//			zap.Uint16("protocol", uint16(req.Protocol)),
//			zap.Error(err),
//		)
//
//		return proto.Errorf(
//			pb_protocol.ErrorCode_RemoteCallFailed,
//			"remote call to %s failed, realServerID: %d, serverID: %d, roleID: %d, protocol: %d, err: %s",
//			rpcclient.Name(), req.RealServerId, req.ServerId, req.RoleId, uint16(req.Protocol), err.Error(),
//		)
//	}
//
//	if resp.Code == pb_protocol.ErrorCode_Success && len(resp.Data) == 0 {
//		return proto.Response(nil)
//	}
//
//	if resp.Code != pb_protocol.ErrorCode_Success {
//		return proto.Error(resp.Code, resp.Devmsg)
//	}
//
//	return proto.ResponseBytes(resp.Data)
//}

func ForwardTarget1(session *session.Session, message *datapack.Message, rpcClient ClientInterface) *proto.Resp {
	// todo 可优化为对象池
	rpcReq := datapack.RpcMessage{
		Data:   message,
		Player: session.Player,
	}
	var rpcResp *proto.Resp
	var err error
	if targetID > 0 {
		err = rpcClient.Wrap(targetID, versionMin, versionMax).Call(context.Background(), "Dispatch", rpcReq, rpcResp)
	} else {
		err = rpcClient.Call(context.Background(), "Dispatch", rpcReq, rpcResp)
	}
	if err != nil {
		logger.Get().Error(
			"ForwardTarget call Dispatch failed",
			zap.String("target", rpcclient.Name()),
			zap.Uint32("realServerID", req.RealServerId),
			zap.Uint32("serverID", req.ServerId),
			zap.Uint64("roleID", req.RoleId),
			zap.Uint16("protocol", uint16(req.Protocol)),
			zap.Error(err),
		)

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

// ProtocolTarget 模拟协议调用
//
//   - rpcclient: rpc 客户端
//   - protocol: 协议ID
//   - req: 协议对应的请求参数
//   - resp: 协议对应的响应参数
//   - targetID: 目标ID，如果是唯一服务，填 0。如果是多服务，填目标服务的ID，例如调用 game-1 的填 1
//   - versionMin: 最小版本号，填 0 表示不限制
//   - versionMax: 最大版本号，填 0 表示不限制
func ProtocolTarget(protocol pb_protocol.MessageID, req, resp gproto.Message, rpcclient pkgrpc.Client, targetID, versionMin, versionMax, realServerID, serverID uint32, roleID uint64) (pb_protocol.ErrorCode, error) {
	// 组装消息，模拟调用
	b, err := proto.Marshal(req)
	if err != nil {
		return 0, err
	}

	forwardReq := BuildForwardReq(realServerID, serverID, roleID, protocol, b)
	defer ReleaseForwardReq(forwardReq)

	code, data := ForwardTarget(targetID, forwardReq, rpcclient, versionMin, versionMax)

	// data 是 pb_gate.Response 编码后的数据
	gateResp := proto.GateResp()
	defer proto.GateRespRelease(gateResp)
	err = proto.Unmarshal(data, gateResp)
	if err != nil {
		return pb_protocol.ErrorCode_ProtoUnarshalFailed, fmt.Errorf("protocolTarget: proto.Unmarshal failed, protocol: %d, err: %v", protocol, err)
	}

	if code != pb_protocol.ErrorCode_Success {
		return code, errors.New(gateResp.Devmsg)
	}

	if resp != nil && data != nil {
		// gateResp.Payload 是 resp 编码后的数据
		err = proto.Unmarshal(gateResp.Payload, resp)
		if err != nil {
			return code, err
		}
	}

	return code, nil
}
