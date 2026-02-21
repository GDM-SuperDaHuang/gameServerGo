package node

import (
	"context"
	"gameServer/service/common"
	"gameServer/service/config"
	"gameServer/service/logger"
	"gameServer/service/rpc"
	rpcxServer "gameServer/service/rpc/server"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func (n *NodeServer) initRPC(f *rpc.Forward) error {
	// 1. 服务端
	s, err := rpcxServer.NewServer(rpcxServer.BuildServerConfig())
	if err != nil {
		return err
	}
	if err = s.Start(); err != nil {
		return err
	}

	if err = s.Register(f); err != nil {
		return err
	}

	logger.Get().Info("[initRPC] server start", zap.String("info", s.Output()))

	n.rpcServer = s

	return nil
}

func Push(roleID uint64, protoId uint16, message proto.Message) {
	// todo 在线判断
	//player := online.Get(roleID)
	//if player == nil {
	//	// 不在线
	//	return
	//}
	player := &common.Player{}

	if err := ToGate(player, protoId, message); err != nil {
		logger.Get().Error("push to gate failed", zap.Uint64("roleID", roleID), zap.Int32("protocol", int32(protoId)), zap.Error(err))
	}
}

// ToGate 推送到网关，立即推送
//
//   - gateID: 网关 id
func ToGate(player *common.Player, protocol uint16, message proto.Message) error {
	if config.Get().IsTest() {
		return nil
	}
	body, err := proto.Marshal(message)
	if err != nil {
		logger.Get().Error("proto marshal failed", zap.Error(err))
		return err
	}
	// todo 可优化为对象池是否有问题
	pushMessage := common.NewMessage(0, 0, 0, protocol, body)
	rpcReq := common.RpcMessage{
		Data:   pushMessage,
		Player: player,
	}
	defer common.FreeMessage(pushMessage)
	return rpcClient.Call(context.Background(), "Receive", rpcReq, nil)
}
