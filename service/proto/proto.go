package proto

import (
	"Server/service/common"
	"errors"
	"fmt"
	//"server/api/pb/pb_gate"
	//"server/api/pb/pb_protocol"

	"google.golang.org/protobuf/proto"
)

type Resp struct {
	//engine  *engine.Engine
	Code uint16
	Flag uint16
	Body *[]byte
}

// ErrInvalidPBMessage 无效的 google protobuf 消息
var ErrInvalidPBMessage = errors.New("invalid pb message")

var gateRespPool = common.NewPool(func() *pb_gate.Response {
	return &pb_gate.Response{}
})

// GateResp ..
func GateResp() *pb_gate.Response {
	return gateRespPool.Get()
}

// GateRespRelease ..
func GateRespRelease(resp *pb_gate.Response) {
	gateRespPool.Put(resp)
}

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

// Response 响应给客户端
//   - in: proto.Message 格式
func Response(in any) (uint16, []byte) {
	if in == nil {
		return ErrorCode_Success, nil
	}

	b, err := Marshal(in)
	if err != nil {
		return MarshalFailed(err)
	}

	//gateResp := GateResp()
	//gateResp.Payload = b
	//b2, err := proto.Marshal(gateResp)
	//GateRespRelease(gateResp)
	//if err != nil {
	//	return MarshalFailed(err)
	//}

	return pb_protocol.ErrorCode_Success, b2
}

// 普通响应的返回
func Response1(in any) *Resp {
	resp := &Resp{}
	if in == nil {
		resp.code = ErrorCode_Success
		return resp
	}
	b, err := Marshal(in)
	if err != nil {
		resp.code = ErrorCode_ProtoMarshalFailed
		return resp
	}
	resp.data = &b
	return resp
}

// 带有加密信息的返回
func Response2(in any, flag uint16) *Resp {
	resp := &Resp{}
	if in == nil {
		return resp
	}
	b, err := Marshal(in)
	if err != nil {
		return MarshalFailed1(resp)
	}
	resp.data = &b
	resp.flag = flag
	return resp
}

// ResponseBytes 响应给客户端
//   - b: 由 proto.Message 编码后的格式
func ResponseBytes(b []byte) (uint16, []byte) {
	gateResp := GateResp()
	gateResp.Payload = b
	b2, err := proto.Marshal(gateResp)
	GateRespRelease(gateResp)
	if err != nil {
		return MarshalFailed(err)
	}

	return ErrorCode_Success, b2
}

// Error 发生错误
func Error(code uint16, devmsg string) (uint16, []byte) {
	gateResp := GateResp()
	gateResp.Devmsg = devmsg
	b, _ := proto.Marshal(gateResp)
	GateRespRelease(gateResp)
	return code, b
}

// Errorf 发生错误
func Errorf(code uint16, format string, args ...any) (uint16, []byte) {
	devmsg := fmt.Sprintf(format, args...)
	return Error(code, devmsg)
}
func Errorf1(code uint16) *Resp {
	resp := &Resp{
		Code: code,
	}
	return resp
}

// MarshalFailed 协议编码失败
func MarshalFailed(err error) (uint16, []byte) {
	return Error(ErrorCode_ProtoMarshalFailed, err.Error())
}

// UnmarshalFailed 协议解码失败
func UnmarshalFailed(err error) (uint16, []byte) {
	return Error(ErrorCode_ProtoUnarshalFailed, err.Error())
}
