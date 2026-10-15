package ws

import (
	"context"
	"encoding/binary"
	"gameServer/common/db/rooming"
	"gameServer/common/errorCode"
	"gameServer/pkg/compress"
	"gameServer/pkg/encrypt"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/utils"
	"gameServer/service/common"
	"gameServer/service/common/proto"
	"gameServer/service/rpc"
	"gameServer/service/rpc/client/selector"
	"strconv"
	"time"

	"github.com/smallnest/rpcx/share"
	"go.uber.org/zap"
)

var (
	compressor = compress.NewZlib()
)

func (wss *WsServer) forward(data []byte, s *Session) *common.Resp {

	// 1. 检查数据长度
	if len(data) < common.MessageHeadSize {
		return &common.Resp{
			Code: 1001, // 错误码：消息头不完整
			Body: []byte("message too short"),
		}
	}

	// 2. 解析消息头（大端序）
	head := &common.MessageHead{}
	head.Len = binary.BigEndian.Uint16(data[0:2])
	head.Flag = binary.BigEndian.Uint16(data[2:4])
	head.SN = binary.BigEndian.Uint32(data[4:8])
	head.Code = binary.BigEndian.Uint16(data[8:10])
	head.Protocol = binary.BigEndian.Uint16(data[10:12])

	// 3. 检查 Body 长度
	if len(data) < common.MessageHeadSize+int(head.Len) {
		return &common.Resp{
			Code: 1002, // 错误码：消息体不完整
			Body: []byte("body length mismatch"),
		}
	}

	// 4. 提取 Body(线上的可能是压缩/加密后的数据，Len 为线上长度)
	body := data[common.MessageHeadSize : common.MessageHeadSize+head.Len]

	// 4.1 按 Flag 解密/解压缩，只处理 Body 部分
	// Flag: 1=压缩, 2=加密, 3=压缩+加密(发送时先压缩后加密)
	// 接收时注意顺序：先解密，再解压缩，得到原数据
	// 压缩/解压只在 body >= 阈值时触发；加解密走会话级 LR-GAME-V1 通道
	plainBody, errCode, errMsg := decodeBody(s, head.Protocol, head.Flag, body)
	if errMsg != nil {
		log2.Get().Error("forward decodeBody failed",
			zap.Uint16("flag", head.Flag),
			zap.Uint16("protocol", head.Protocol),
			zap.Error(errMsg),
		)
		return &common.Resp{
			Code: errCode,
			Body: []byte(errMsg.Error()),
		}
	}
	body = plainBody
	head.Len = uint16(len(body)) // 更新为明文长度，内部流转使用明文

	// 5. 构造 Message 对象
	msg := &common.Message{
		Head: head,
		Body: body,
	}

	var resp *common.Resp
	// 7. 路由到对应的业务处理器
	if head.Protocol < 1000 {
		resp = wss.handleMessage(s, msg)
	} else {
		//转发到其它服务，需要 登录 准备好
		if s == nil || s.Player == nil {
			resp = proto.Errorf1(errorCode.ErrorCode_PushFailed)
		} else {
			resp = forwardTarget(s, msg, RpcGateClient)
		}
	}
	//msg.Head.Len = uint16(len(resp.Body))
	// 8. 构造响应消息并发送
	if resp != nil {
		respMsg := common.NewMessageResp(resp, msg)
		if CloseStatus == s.Send(encodeMessage(respMsg, s)) {
			if s.Player != nil && s.Player.Uid != 0 {
				wss.DelSession(s.Player.Uid)
			}
		}
		common.FreeMessage(respMsg)
	} else {
		return nil
	}

	return nil
}

// handleMessage 消息路由处理（示例框架）
func (wss *WsServer) handleMessage(s *Session, msg *common.Message) *common.Resp {
	// 根据 Protocol 路由到不同处理器
	switch msg.Head.Protocol {
	case 1: // 心跳协议
		return wss.heartHandler(s, msg)
	case 2: // 密钥交换
		return wss.keyExchangeHandle(s, msg)
	case 3: // 密钥验证
		return wss.KeyVerificationHandle(s, msg)
	case 4: // 登录协议
		return wss.loginHandler(s, msg)
	case 5: // 密钥测试
		return wss.KeyTestHandler(s, msg)
	case 100: // 热更新
		return wss.configHandler(s, msg)

	default:
		return proto.Errorf1(errorCode.ErrorCode_ProtocolNotFound)
	}
}

// decodeBody 按 Flag 还原 Body，只处理 Body 部分
// 顺序：先解密，再解压缩
// 压缩数据只在 len(body) >= CompressThreshold 时才解压
// 解密走会话级 LR-GAME-V1 通道(服务端用 client_write_key 解):
//   - 协议 2/3 (握手本身) 必须明文, 带加密位直接拒绝；
//   - 其它协议: 带加密位则用会话通道解密(未握手则失败)；
//     握手完成后仍走明文则拒绝(防降级)，要求加密。
func decodeBody(s *Session, protocol uint16, flag uint16, body []byte) ([]byte, uint16, error) {
	isHandshake := protocol == 2 || protocol == 3
	if isHandshake {
		if flag&common.FlagEncrypted != 0 {
			return nil, errorCode.ErrorCode_KeyExchangeFailed, encrypt.ErrNotVerified
		}
		if len(body) == 0 || flag == common.FlagNone {
			return body, 0, nil
		}
		// 握手包理论上不压缩, 下面仅兼容处理压缩位
		out := body
		if flag&common.FlagCompressed != 0 && len(out) >= common.CompressThreshold {
			dec, err := compressor.Uncompress(out)
			if err != nil {
				return nil, 1003, err
			}
			out = dec
		}
		return out, 0, nil
	}

	if len(body) == 0 && flag == common.FlagNone {
		return body, 0, nil
	}

	out := body

	// 1. 先解密
	if flag&common.FlagEncrypted != 0 {
		ch := s.GetSecure()
		if ch == nil {
			return nil, errorCode.ErrorCode_SecureChannelMissing, encrypt.ErrNotVerified
		}
		dec, err := ch.Decrypt(out)
		if err != nil {
			return nil, errorCode.ErrorCode_DecryptFailed, err
		}
		out = dec
	}

	// 2. 再解压缩(FlagCompressed 才触发)
	if flag&common.FlagCompressed != 0 {
		dec, err := compressor.Uncompress(out)
		if err != nil {
			return nil, errorCode.ErrorCode_UnCompressedFailed, err
		}
		out = dec
	}

	return out, 0, nil
}

// encodeMessage 将 Message 编码为字节流
// 顺序：先压缩，后加密，只处理 Body 部分
// body >= 阈值时：FlagNone 自动压缩；带压缩位则压缩；
// 加密走会话级 LR-GAME-V1 通道(服务端用 server_write_key 加密):
func encodeMessage(msg *common.Message, s *Session) []byte {
	body := msg.Body
	flag := msg.Head.Flag
	actualFlag := common.FlagNone
	if len(body) > 0 {
		wantCompress := flag&common.FlagCompressed != 0
		wantEncrypt := flag&common.FlagEncrypted != 0
		// body >= 阈值且发送方未指定 Flag 时，自动压缩
		if len(body) >= common.CompressThreshold {
			wantCompress = true
		}

		// 1. 先压缩
		if wantCompress {
			compressed, err := compressor.Compress(body)
			if err != nil {
				log2.Get().Error("encodeMessage compress failed", zap.Error(err))
			} else {
				body = compressed
				actualFlag |= common.FlagCompressed
			}
		}

		// 2. 后加密(会话通道，服务端视角 server_write_key)
		if wantEncrypt {
			if ch := s.GetSecure(); ch != nil {
				encrypted, err := ch.Encrypt(body)
				if err != nil {
					log2.Get().Error("encodeMessage secure encrypt failed", zap.Error(err))
				} else {
					body = encrypted
					actualFlag |= common.FlagEncrypted
				}
			} else {
				log2.Get().Error("encodeMessage secure channel missing, send plaintext")
			}
		}
	}

	bodyLen := uint16(len(body))
	buf := make([]byte, common.MessageHeadSize+bodyLen)

	// 写入消息头（大端序）
	binary.BigEndian.PutUint16(buf[0:2], bodyLen)
	binary.BigEndian.PutUint16(buf[2:4], actualFlag)
	binary.BigEndian.PutUint32(buf[4:8], msg.Head.SN)
	binary.BigEndian.PutUint16(buf[8:10], msg.Head.Code)
	binary.BigEndian.PutUint16(buf[10:12], msg.Head.Protocol)

	// 写入 Body(压缩/加密后的数据)
	copy(buf[12:], body)

	return buf
}

func forwardTarget(session *Session, message *common.Message, rpcClient rpc.ClientInterface) *common.Resp {
	rpcReq := common.RpcMessage{
		Data:   message,
		Player: session.Player,
	}
	var rpcResp = &common.Resp{}
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	protocolId := message.Head.Protocol
	groupId := utils.GetGroupIdByPb(int(protocolId))
	id := utils.GetServerId(groupId, session.Player.ServerIds) //本网关可能没有
	flag := false
	if id == 0 {
		//查询正在进行的游戏
		if protocolId >= 1000 && protocolId < 2000 { //room类型协议，可能正在进行游戏
			id = int(rooming.GetRoomingId(session.Player.Uid))
		}

		//分配一台服务器
		if id == 0 {
			s, ok := rpcClient.GetSelector().(*selector.DefaultSelector)
			if !ok {
				log2.Get().Error("转换失败")
				return proto.Errorf1(errorCode.ErrorCode_RemoteCallFailed)
			}
			ctx = context.WithValue(ctx, share.ResMetaDataKey, map[string]string{
				"id":      "0",
				"groupId": strconv.Itoa(int(groupId)),
			})
			addr := s.Select(ctx, "", "", nil)
			if addr != "" {
				for _, info := range s.Servers {
					if info.Address == addr {
						id = int(info.Id)
						break
					}
				}
			}
		}
		flag = true
	}

	ctx = context.WithValue(ctx, share.ResMetaDataKey, map[string]string{
		"id":      strconv.Itoa(id),
		"groupId": strconv.Itoa(int(groupId)),
	})
	err = rpcClient.Call(ctx, "Dispatch", rpcReq, rpcResp)

	if err != nil {
		log2.Get().Error("ForwardTarget call Dispatch failed",
			zap.Uint16("ProtocolId:", rpcReq.Data.Head.Protocol),
			zap.Int64("Uid:", rpcReq.Player.Uid),
			zap.Error(err),
		)
		for i := len(session.Player.ServerIds) - 1; i >= 0; i-- {
			if session.Player.ServerIds[i] == uint32(id) {
				session.Player.ServerIds = append(session.Player.ServerIds[:i], session.Player.ServerIds[i+1:]...)
			}
		}
		go rooming.DelRoomingId(session.Player.Uid)
		if rpcResp.Code > 0 {
			return proto.Errorf1(rpcResp.Code)
		}
		return proto.Errorf1(errorCode.ErrorCode_RemoteCallFailed)
	}

	// 更新 session
	if flag {
		// 排除相同的
		for _, serverId := range session.Player.ServerIds {
			if serverId == uint32(id) {
				return rpcResp
			}
		}
		if id == 0 {
			return rpcResp
		}
		session.Player.ServerIds = append(session.Player.ServerIds, uint32(id))
	}
	return rpcResp
}
