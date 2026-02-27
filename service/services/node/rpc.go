package node

import (
	"context"
	"errors"
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log1"
	"gameServer/pkg/utils"
	"gameServer/service/common"
	"gameServer/service/rpc"
	rpcxServer "gameServer/service/rpc/server"
	"strconv"

	"github.com/smallnest/rpcx/share"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func (n *NodeServer) initRPC(f *rpc.Forward) error {
	SetNodeRPCClient(RPCNodeClients())

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

	log1.Get().Info("[initRPC] server start", zap.String("info", s.Output()))
	n.rpcServer = s

	return nil
}

// 主动推送给到网关到客户端
func Push(player *common.Player, protoId uint16, message proto.Message) {
	// todo 在线判断
	//player := online.Get(roleID)
	//if player == nil {
	//	// 不在线
	//	return
	//}
	if err := ToGate(player, protoId, message, RpcNodeClient); err != nil {
		log1.Get().Error("push to gate failed", zap.Uint64("roleID", player.UserId), zap.Int32("protocol", int32(protoId)), zap.Error(err))
	}
}

// ToGate 推送到网关，立即推送
//
//   - gateID: 网关 id
func ToGate(player *common.Player, protocol uint16, message proto.Message, rpcClient rpc.ClientInterface) error {
	if config.Get().IsTest() {
		return nil
	}
	body, err := proto.Marshal(message)
	if err != nil {
		log1.Get().Error("proto marshal failed", zap.Error(err))
		return err
	}
	// todo 可优化为对象池是否有问题
	pushMessage := common.NewMessage(0, 0, 0, protocol, body)
	rpcReq := common.RpcMessage{
		Data:   pushMessage,
		Player: player,
	}
	defer common.FreeMessage(pushMessage)
	id := utils.GetServerId(1, player.ServerIds) //获取网关id,网格为1组
	if id == 0 {
		return errors.New("server id is 0")
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, share.ResMetaDataKey, map[string]string{
		"id":      strconv.Itoa(id),
		"groupId": strconv.Itoa(1),
	})

	return rpcClient.Call(ctx, "Receive", rpcReq, nil)
}
