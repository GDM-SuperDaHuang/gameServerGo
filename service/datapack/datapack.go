package datapack

import (
	"errors"
)

type Datapack interface {
	// HeadLen 消息头长度
	HeadLen() int

	//  封包
	Pack(message *Message, cryptoHandler Crypto) (Callback, []byte, error)

	//  解包
	Unpack(reader Reader) ([]*Message, error) //reader == c gnet.Conn
}
type Callback func()

// Reader from github.com/panjf2000/gnet/v2
type Reader interface {
	// Next returns a slice containing the next n bytes from the buffer,
	// advancing the buffer as if the bytes had been returned by Read.
	// Calling this method has the same effect as calling Peek and Discard.
	// If the amount of the available bytes is less than requested, a pair of (0, io.ErrShortBuffer)
	// is returned.
	//
	// Note that the []byte buf returned by Next() is not allowed to be passed to a new goroutine,
	// as this []byte will be reused within event-loop.
	// If you have to use buf in a new goroutine, then you need to make a copy of buf and pass this copy
	// to that new goroutine.
	Next(n int) (buf []byte, err error)

	// Peek returns the next n bytes without advancing the inbound buffer, the returned bytes
	// remain valid until a Discard is called. If the amount of the available bytes is
	// less than requested, a pair of (0, io.ErrShortBuffer) is returned.
	//
	// Note that the []byte buf returned by Peek() is not allowed to be passed to a new goroutine,
	// as this []byte will be reused within event-loop.
	// If you have to use buf in a new goroutine, then you need to make a copy of buf and pass this copy
	// to that new goroutine.
	Peek(n int) (buf []byte, err error)

	// Discard advances the inbound buffer with next n bytes, returning the number of bytes discarded.
	Discard(n int) (discarded int, err error)

	// InboundBuffered returns the number of bytes that can be read from the current buffer.
	InboundBuffered() (n int)
}

// Crypto 加密与解密接口
type Crypto interface {
	// Encrypt 加密
	Encrypt(in []byte) ([]byte, error)

	// Decrypt 解密
	Decrypt(in []byte) ([]byte, error)
}

// Message 消息
type Message struct {
	Head *MessageHead
	Body []byte
}

// MessageHead 消息头
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

// 消息头各字段的长度
const (
	headLen         = lenFieldLength + flagFieldLength + snFieldLength + codeFieldLength + protocolLength
	lenFieldLength  = 2 //消息体长度大小
	flagFieldLength = 2
	snFieldLength   = 4
	codeFieldLength = 2
	protocolLength  = 2

	// 对象池大小
	defaultBufferSize = 1024
)

// Flag标志
const (
	// 0000 啥也不干

	//  消息体经过压缩
	MessageFlagCompress = uint16(0x0001)

	//  消息体被加密
	MessageFlagEncrypt = uint16(0x0010)

	//// MessageFlagChecksum 消息开启校验值检查
	//MessageFlagChecksum = uint16(0x0100)
	//
	//// MessageFlagNoEncrypt 消息体不加密
	//MessageFlagNoEncrypt = uint16(0x1000)
)

var (
	// ErrGetPayloadLen 获取负载长度失败
	ErrGetPayloadLen = errors.New("get payload length failed")

	// ErrGetAllBytes 获取所有内容失败
	ErrGetAllBytes = errors.New("get all bytes failed")

	// ErrVerifyChecksum 校验失败
	ErrVerifyChecksum = errors.New("verify checksum failed")

	// ErrNoChecksumFlag 无校验标记
	ErrNoChecksumFlag = errors.New("no checksum flag")

	// ErrDecryptPayload 解密负载失败
	ErrDecryptPayload = errors.New("payload decrypt failed")

	// ErrDecompressPayload 解压负载失败
	ErrDecompressPayload = errors.New("payload decompress failed")

	// ErrIncompleteMessage 不是完整消息
	ErrIncompleteMessage = errors.New("incomplete message")
)
