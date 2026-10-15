package ws

import (
	"context"
	"gameServer/pkg/encrypt"
	"gameServer/pkg/logger/log2"
	"gameServer/service/common"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
)

const (
	NormalStatus = 0
	CloseStatus  = 1
	ErrorStatus  = 2
)

type Session struct {
	Conn   *websocket.Conn
	Ctx    context.Context
	Cancel context.CancelFunc

	RemoteAddr string

	SendQueue chan []byte // 写队列（关键）
	closeOnce sync.Once
	//Done      chan struct{} // 关闭信号
	closed int32 // 新增：原子关闭标记

	Player *common.Player

	// LR-GAME-V1 安全通道状态(每个连接独立)。
	// 握手完成前为 nil；KeyExchange 完成后暂存 handshake；
	// KeyVerification 通过后建成 secure 并置 secureOK=1。
	secureMu  sync.Mutex
	handshake *encrypt.ServerHandshake
	secure    *encrypt.SecureChannel
	secureOK  atomic.Bool
}

func (s *Session) Send(data []byte) int8 {
	// 快速失败：如果已关闭，直接返回 false
	if atomic.LoadInt32(&s.closed) >= 1 {
		return CloseStatus
	}

	select {
	case s.SendQueue <- data:
		return NormalStatus
	default:
		log2.Get().Error("[Session] send SendQueue is full")
		// 队列满，说明客户端慢 or 异常
		return ErrorStatus
	}
}

func (s *Session) Close() {
	s.closeOnce.Do(func() {
		atomic.StoreInt32(&s.closed, 1)
		s.Cancel()

		_ = s.Conn.Close(websocket.StatusNormalClosure, "closed")

		close(s.SendQueue)
	})
}

// SetHandshake 暂存握手状态(允许重复发起, 后者覆盖前者并重置通道)。
func (s *Session) SetHandshake(h *encrypt.ServerHandshake) {
	s.secureMu.Lock()
	defer s.secureMu.Unlock()
	s.handshake = h
	s.secure = nil
	s.secureOK.Store(false)
}

// GetHandshake 取握手状态。
func (s *Session) GetHandshake() *encrypt.ServerHandshake {
	s.secureMu.Lock()
	defer s.secureMu.Unlock()
	return s.handshake
}

// EstablishSecure 握手验证通过后建成安全通道。
func (s *Session) EstablishSecure(ch *encrypt.SecureChannel) {
	s.secureMu.Lock()
	defer s.secureMu.Unlock()
	s.secure = ch
	s.secureOK.Store(true)
}

// GetSecure 取已建成的通道, 未建成返回 nil。
func (s *Session) GetSecure() *encrypt.SecureChannel {
	if s == nil || !s.secureOK.Load() {
		return nil
	}
	s.secureMu.Lock()
	defer s.secureMu.Unlock()
	return s.secure
}

// SecureReady 是否已完成密钥验证。
func (s *Session) SecureReady() bool {
	return s != nil && s.secureOK.Load()
}
