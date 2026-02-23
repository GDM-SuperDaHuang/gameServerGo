package rpc

import (
	"context"
	"errors"
	"fmt"
	"gameServer/service/common"
	"gameServer/service/common/proto"
)

// Dispatch 网关发生的消息派遣
func (f *Forward) Dispatch(ctx context.Context, req *common.RpcMessage, resp *common.Resp) error {
	if req == nil {
		resp = &common.Resp{}
		resp.Code = proto.ErrorCode_ReqNull
		return errors.New("req is nil")
	}

	if resp == nil {
		resp = &common.Resp{}
		resp.Code = proto.ErrorCode_RespNull
		return errors.New("resp is nil")
	}

	if req.Player == nil || req.Player.RoleID == 0 {
		resp = &common.Resp{}
		resp.Code = proto.ErrorCode_RespNull
		return errors.New("player is nil")
	}

	// todo 检查角色存在，可以自动创建角色所包含的所有消息，如道具等。

	protocolMethod, found := f.protocoles[req.Data.Head.Protocol]
	if !found {
		resp.Code = proto.ErrorCode_ProtocolNotFound
		return fmt.Errorf("protocol not found: %d", req.Data.Head.Protocol)
	}

	err := protocolMethod.Call(ctx, req.Player.RoleID, req, resp)
	return err
}
