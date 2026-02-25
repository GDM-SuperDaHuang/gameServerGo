package common

import "gameServer/pkg/bytes"

type MessageHead struct {
	// Len 消息体长度
	Len uint16 //2
	// Flag 标记
	Flag uint16 //2
	// SN 自增编号，由客户端发出，服务端原样返回。服务端主动发出的消息中 SN 值为 0
	SN uint32 //4
	// Code 错误码
	Code uint16 //2
	// Protocol 协议号
	Protocol uint16 //2
	// Checksum 校验值
	//Checksum [ChecksumLength]byte
}

type Message struct {
	Head *MessageHead
	Body []byte //protobuf
}

type RpcMessage struct {
	Data   *Message
	Player *Player
}

type Resp struct {
	//engine  *engine.Engine
	Code uint16
	Flag uint16
	Body []byte //pb
}

type ErrorInfo struct {
	//engine  *engine.Engine
	Code uint16
	Flag uint16
}

// ------------------------------------------- Message ---------------------------------------------------
// messagePool 消息池
var messagePool = bytes.NewPool(func() *Message {
	return &Message{
		Head: &MessageHead{},
		Body: nil,
	}
})

// NewMessage 创建消息
func NewMessage(flag uint16, sn uint32, code uint16, protocol uint16, payload []byte) *Message {
	m := messagePool.Get()
	m.Head.Len = uint16(len(payload))
	m.Head.Flag = flag
	m.Head.SN = sn
	m.Head.Code = code
	m.Head.Protocol = protocol
	m.Body = payload
	return m
}

func NewMessageResp(resp *Resp, message *Message) *Message {
	m := messagePool.Get()
	m.Head = &MessageHead{
		Len:      uint16(len(message.Body)),
		Flag:     resp.Flag,
		SN:       message.Head.SN,
		Code:     resp.Code,
		Protocol: message.Head.Protocol,
	}
	if resp.Body != nil {
		m.Body = resp.Body
	}

	return m
}

// 释放消息
func FreeMessage(m *Message) {
	messagePool.Put(m)
}

// Reset 重置
func (m *Message) Reset() {
	m.Head.Len = 0
	m.Head.Len = 0
	m.Head.Flag = 0
	m.Head.SN = 0
	m.Head.Code = 0
	m.Head.Protocol = 0
	m.Body = nil
}

// ------------------------------------------- Message ---------------------------------------------------

// ------------------------------------------- ErrorInfo ---------------------------------------------------
var errorInfoPool = bytes.NewPool(func() *ErrorInfo {
	return &ErrorInfo{}
})

// Reset 重置
func (e *ErrorInfo) Reset() {
	e.Code = 0
	e.Flag = 0
}

// 释放消息
func FreeErrorInfo(m *ErrorInfo) {
	errorInfoPool.Put(m)
}

// Error 报错
func Error(code uint16) *ErrorInfo {
	m := errorInfoPool.Get()
	m.Code = code
	return m
}

func ErrorF(code, flag uint16) *ErrorInfo {
	m := errorInfoPool.Get()
	m.Code = code
	m.Flag = flag
	return m
}

// ------------------------------------------- ErrorInfo ---------------------------------------------------
