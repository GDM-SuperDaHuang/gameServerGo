package datapack

import (
	"Server/service/common"
	"crypto"
	"encoding/binary"
	"fmt"

	"go.uber.org/zap"
)

type deEnCode struct {
	// headLen 消息头长度
	//headLen int

	// whetherCompress 是否需要对消息负载 payload 进行压缩
	//whetherCompress bool

	// compressThreshold 压缩的阈值，当消息负载 payload 长度不小于该值时才会压缩
	//compressThreshold int

	// compress 压缩与解压器，默认 zip
	compress compress.Compress

	// whetherCrypto 是否需要对消息负载 payload 进行加密
	//whetherCrypto bool

	// whetherChecksum 是否启用校验值功能
	//whetherChecksum bool

	// order 默认使用大端模式
	order binary.ByteOrder

	// logger 日志
	logger *zap.Logger

	// emptyChecksum 空检验值，用于计算
	//emptyChecksum [ChecksumLength]byte
}

func NewLTD(
	whetherCompress bool,
	compressThreshold int,
	compress compress.Compress,
	whetherCrypto bool,
	whetherChecksum bool,
	logger *zap.Logger,
) Datapack {
	return &ltd{
		headLen:           calcHeadLen(whetherChecksum),
		whetherCompress:   whetherCompress,
		compressThreshold: compressThreshold,
		compress:          compress,
		whetherCrypto:     whetherCrypto,
		whetherChecksum:   whetherChecksum,
		order:             binary.BigEndian, // 默认使用大端,实例化
		logger:            logger,
		emptyChecksum:     [ChecksumLength]byte{},
	}
}

// HeadLen 消息头长度
func (l *deEnCode) HeadLen() int {
	return l.headLen
}

// Pack 封包
func (l *deEnCode) Pack(message *Message, cryptoHandler Crypto) (Callback, []byte, error) {
	body, flag, err := l.packBody(message, cryptoHandler)
	if err != nil {
		return nil, nil, err
	}

	estimatedSize := headLen
	if len(body) > 0 {
		estimatedSize += len(body)
	}

	// 固定大小的，可复用
	buffer := common.Get().Buffer(defaultBufferSize)

	if buffer.Cap() < estimatedSize {
		buffer.Grow(estimatedSize)
	}

	// 是否有校验值
	//if l.whetherChecksum {
	//	flag |= MessageFlagChecksum
	//}

	// 使用二进制写入优化
	head := make([]byte, headLen)
	index := 0

	// 消息体长度
	l.order.PutUint16(head[index:], uint16(len(body)))
	index += lenFieldLength

	//// 写入标记
	//if l.whetherChecksum {
	//	flag |= MessageFlagChecksum
	//}
	l.order.PutUint16(head[index:], flag)
	index += flagFieldLength

	// SN
	l.order.PutUint32(head[index:], message.Head.SN)
	index += snFieldLength

	// 错误码
	l.order.PutUint16(head[index:], message.Head.Code)
	index += codeFieldLength

	// 协议号
	l.order.PutUint16(head[index:], message.Head.Protocol)

	// 写入头部
	buffer.Write(head)

	// 校验值，先占位
	//if l.whetherChecksum {
	//	buffer.Write(l.emptyChecksum[:])
	//}

	// 负载
	if len(body) > 0 {
		buffer.Write(body)
	}

	allBytes := buffer.Bytes()

	// 计算校验值并填充
	//if l.whetherChecksum && (flag&MessageFlagNoEncrypt == 0) {
	//	checksumStartIndex := l.HeadLen() - ChecksumLength
	//	checksum := crypto.HmacMd5ByteToByte(allBytes, checksumKey)
	//	copy(allBytes[checksumStartIndex:], checksum)
	//}

	return func() {
		common.Get().Release(buffer)
	}, allBytes, nil
}

// Unpack 解包
func (l *deEnCode) Unpack(reader Reader) ([]*Message, error) {
	messages := make([]*Message, 0, 4)

	for {
		if err := l.hasCompleteMessage(reader); err != nil {
			if err == ErrIncompleteMessage {
				break
			}
			return nil, err
		}

		message, err := l.unpackOneMessage(reader)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	return messages, nil
}

func (l *deEnCode) packBody(message *Message, cryptoHandler Crypto) ([]byte, uint16, error) {
	// 负载
	body := message.Body
	if len(body) == 0 {
		return nil, 0, nil
	}

	var err error

	flag := message.Head.Flag

	// 压缩
	if l.whetherCompress && l.compress != nil && len(body) >= l.compressThreshold {
		compressed, err := l.compress.Compress(body)
		if err != nil {
			l.logger.Error("[pkg.datapack.packBody] compress failed",
				zap.String("message", message.String()),
				zap.String("err", err.Error()))
			return nil, 0, fmt.Errorf("compress failed: %w", err)
		}
		body = compressed
		flag |= MessageFlagCompress
	}

	// 加密
	if l.whetherCrypto && cryptoHandler != nil {
		body, err = cryptoHandler.Encrypt(body)
		if err != nil {
			l.logger.Error("[pkg.datapack.packBody] encrypt failed", zap.String("message", message.String()), zap.String("err", err.Error()))
			return nil, 0, err
		}

		flag |= MessageFlagEncrypt
	}

	return body, flag, nil
}
func (l *deEnCode) hasCompleteMessage(reader Reader) error {
	// 获取可读取的长度
	bufferLen := reader.InboundBuffered()

	// 如果小于头直接返回
	if bufferLen < headLen {
		return ErrIncompleteMessage
	}

	// 读取开头的2字节
	p, err := reader.Peek(lenFieldLength)
	if err != nil {
		return fmt.Errorf("peek message length: %w", ErrGetPayloadLen)
	}

	// 检查是否完整
	bodyLen := int(l.order.Uint16(p))
	if bufferLen < headLen+bodyLen {
		return ErrIncompleteMessage
	}

	return nil
}

func (l *deEnCode) unpackOneMessage(reader Reader) (*Message, error) {
	// 读取完整消息
	bodyLen, _ := reader.Peek(lenFieldLength)
	messageLen := headLen + int(l.order.Uint16(bodyLen))
	allBytes, err := reader.Next(messageLen)
	if err != nil {
		return nil, fmt.Errorf("read message: %w", ErrGetAllBytes)
	}

	// 解析消息头
	head, index, err := l.parseMessageHead(allBytes)
	if err != nil {
		return nil, err
	}

	// 解析消息体，解密,解压缩 todo 解压，加密没有处理
	payload, err := l.processMessageBody(allBytes[index:], head.Flag, nil, head.SN)
	if err != nil {
		return nil, err
	}

	return NewMessage(head.Flag, head.SN, head.Code, head.Protocol, payload), nil
}

// 解析头部
func (l *deEnCode) parseMessageHead(allBytes []byte) (*MessageHead, int, error) {
	index := lenFieldLength
	head := &MessageHead{
		Flag:     l.order.Uint16(allBytes[index:]),
		SN:       l.order.Uint32(allBytes[index+flagFieldLength:]),
		Code:     l.order.Uint16(allBytes[index+flagFieldLength+snFieldLength:]),
		Protocol: l.order.Uint16(allBytes[index+flagFieldLength+snFieldLength+codeFieldLength:]),
	}
	index += flagFieldLength + snFieldLength + codeFieldLength + protocolLength
	return head, index, nil
}

// 解析消息体，解密,解压缩
func (l *deEnCode) processMessageBody(payload []byte, flag uint16, cryptoHandler Crypto, sn uint32) ([]byte, error) {
	var err error

	// 解密
	if flag&MessageFlagEncrypt != 0 && cryptoHandler != nil {
		if payload, err = cryptoHandler.Decrypt(payload); err != nil {
			l.logger.Error("[pkg.datapack.Unpack] decrypt failed",
				zap.Uint32("sn", sn),
				zap.Error(err))
			return nil, ErrDecryptPayload
		}
	}

	// 解压
	if flag&MessageFlagCompress != 0 && l.compress != nil {
		if payload, err = l.compress.Uncompress(payload); err != nil {
			l.logger.Error("[pkg.datapack.Unpack] decompress failed",
				zap.Uint32("sn", sn),
				zap.Error(err))
			return nil, ErrDecompressPayload
		}
	}

	return payload, nil
}

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

// 释放消息
func FreeMessage(m *Message) {
	messagePool.Put(m)
}

// messagePool 消息池
var messagePool = common.NewPool(func() *Message {
	return &Message{
		Head: &MessageHead{},
		Body: nil,
	}
})

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
