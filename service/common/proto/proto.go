package proto

import (
	"errors"
	"gameServer/service/common"
	"google.golang.org/protobuf/proto"
)

// ErrInvalidPBMessage 无效的 google protobuf 消息
var ErrInvalidPBMessage = errors.New("invalid pb message")

//var gateRespPool = common.NewPool(func() *pb_gate.Response {
//	return &pb_gate.Response{}
//})
//
//// GateResp ..
//func GateResp() *pb_gate.Response {
//	return gateRespPool.Get()
//}
//
//// GateRespRelease ..
//func GateRespRelease(resp *pb_gate.Response) {
//	gateRespPool.Put(resp)
//}

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

//// Response 响应给客户端
////   - in: proto.Message 格式
//func Response(in any) (uint16, []byte) {
//	if in == nil {
//		return ErrorCode_Success, nil
//	}
//
//	b, err := Marshal(in)
//	if err != nil {
//		return MarshalFailed(err)
//	}
//
//	//gateResp := GateResp()
//	//gateResp.Payload = b
//	//b2, err := proto.Marshal(gateResp)
//	//GateRespRelease(gateResp)
//	//if err != nil {
//	//	return MarshalFailed(err)
//	//}
//
//	return pb_protocol.ErrorCode_Success, b2
//}

// 普通响应的返回
func Response1(in any) *common.Resp {
	resp := &common.Resp{}
	if in == nil {
		resp.Code = ErrorCode_Success
		return resp
	}
	b, err := Marshal(in)
	if err != nil {
		resp.Code = ErrorCode_ProtoMarshalFailed
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
		resp.Code = ErrorCode_ProtoMarshalFailed
		return resp
	}
	resp.Body = b
	resp.Flag = flag
	return resp
}

// ResponseBytes 响应给客户端
//   - b: 由 proto.Message 编码后的格式
//func ResponseBytes(b []byte) (uint16, []byte) {
//	gateResp := GateResp()
//	gateResp.Payload = b
//	b2, err := proto.Marshal(gateResp)
//	GateRespRelease(gateResp)
//	if err != nil {
//		return MarshalFailed(err)
//	}
//
//	return ErrorCode_Success, b2
//}

// Error 发生错误
//func Error(code uint16, devmsg string) (uint16, []byte) {
//	gateResp := GateResp()
//	gateResp.Devmsg = devmsg
//	b, _ := proto.Marshal(gateResp)
//	GateRespRelease(gateResp)
//	return code, b
//}

// Errorf 发生错误
//
//	func Errorf(code uint16, format string, args ...any) (uint16, []byte) {
//		devmsg := fmt.Sprintf(format, args...)
//		return Error(code, devmsg)
//	}
func Errorf1(code uint16) *common.Resp {
	resp := &common.Resp{
		Code: code,
	}
	return resp
}

// MarshalFailed 协议编码失败
//func MarshalFailed(err error) (uint16, []byte) {
//	return Error(ErrorCode_ProtoMarshalFailed, err.Error())
//}
//
//// UnmarshalFailed 协议解码失败
//func UnmarshalFailed(err error) (uint16, []byte) {
//	return Error(ErrorCode_ProtoUnarshalFailed, err.Error())
//}
