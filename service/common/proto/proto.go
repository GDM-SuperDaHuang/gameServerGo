package proto

import (
	"errors"
	"gameServer/common/errorCode"
	"gameServer/service/common"

	"google.golang.org/protobuf/proto"
)

// ErrInvalidPBMessage 无效的 google protobuf 消息
var ErrInvalidPBMessage = errors.New("invalid pb message")

// Marshal 编码
func Marshal(in any) ([]byte, error) {
	m, ok := in.(proto.Message)
	if !ok {
		return nil, ErrInvalidPBMessage
	}
	return proto.Marshal(m)
}

// Unmarshal 解码
func Unmarshal(in []byte, out any) error {
	m, ok := out.(proto.Message)
	if !ok {
		return ErrInvalidPBMessage
	}

	return proto.Unmarshal(in, m)
}

// 普通响应的返回
func Response1(in any) *common.Resp {
	resp := &common.Resp{}
	if in == nil {
		resp.Code = errorCode.ErrorCode_Success
		return resp
	}
	b, err := Marshal(in)
	if err != nil {
		resp.Code = errorCode.ErrorCode_ProtoMarshalFailed
		return resp
	}
	resp.Body = b
	return resp
}

// 带有加密信息的返回
func Response2(in any, flag uint16) *common.Resp {
	resp := &common.Resp{}
	if in == nil {
		return resp
	}
	b, err := Marshal(in)
	if err != nil {
		resp.Code = errorCode.ErrorCode_ProtoMarshalFailed
		return resp
	}
	resp.Body = b
	resp.Flag = flag
	return resp
}

func Errorf1(code uint16) *common.Resp {
	resp := &common.Resp{
		Code: code,
	}
	return resp
}
