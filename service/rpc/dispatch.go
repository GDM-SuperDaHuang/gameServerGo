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
		return errors.New("req is nil")
	}

	if resp == nil {
		return errors.New("resp is nil")
	}

	protocolMethod, found := f.protocoles[req.Data.Head.Protocol]
	if !found {
		resp.Code = proto.ErrorCode_ProtocolNotFound
		return fmt.Errorf("protocol not found: %d", req.Data.Head.Protocol)
	}
	err := protocolMethod.Call(ctx, uint64(0), req, resp)
	return err
}
