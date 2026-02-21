package common

import (
	"gameServer/service/config"
	"gameServer/service/logger"
	"net"
	"sync"
	"time"

	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

// -------------------------------------- 变量 --------------------------------------

const (
	// PingCheckInterval 心跳检查间隔
	PingCheckInterval = 3 * time.Second
)

type Player struct {
	ServerIds []uint32 //玩家所链接的服务器,网关编号在1~999

	accountID    uint64
	realServerID uint32 // 角色当前所在的区服 id
	serverID     uint32 // 角色归属区服 id
	RoleID       uint64 // 角色 id
}

var sessionPool = NewPool(func() *Session {
	return &Session{}
})

// Session 客户端会话
type Session struct {
	//engine     engine.Engine
	conn       gnet.Conn
	remoteAddr string

	pingTime time.Time
	//readChan  chan struct{}
	//closeChan chan struct{}

	// ready 是否准备好
	//ready atomic.Bool
	// shareKey rc4 加密密钥
	shareKey string
	// player 绑定的玩家信息
	Player *Player
	// 请求指定版本号范围
	//version *pb_gate.Version

	closeOnce sync.Once
	lock      sync.RWMutex
}

//type Player struct {
//	accountID    uint64
//	realServerID uint32 // 角色当前所在的区服 id
//	serverID     uint32 // 角色归属区服 id
//	RoleID       uint64 // 角色 id
//}

// -------------------------------------- 外部 --------------------------------------

// RemoteAddrString 客户端地址
func (s *Session) RemoteAddrString() string {
	if s.conn == nil || s.conn.RemoteAddr() == nil {
		return s.remoteAddr
	}
	return s.conn.RemoteAddr().String()
}

// RemoteAddr 客户端地址
func (s *Session) RemoteAddr() net.Addr {
	if s.conn != nil {
		return s.conn.RemoteAddr()
	}
	return nil
}

// String 输出信息
//func (s *Session) String() string {
//	buffer := bytes.Get().Buffer(256)
//	defer bytes.Get().Release(buffer)
//
//	buffer.WriteString("address: ")
//	buffer.WriteString(s.RemoteAddrString())
//
//	if s.player != nil && s.player.accountID > 0 {
//		buffer.WriteString(", accountID: ")
//		buffer.WriteString(strconv.FormatUint(s.player.accountID, 10))
//		buffer.WriteString(", serverID: ")
//		buffer.WriteString(strconv.Itoa(int(s.player.serverID)))
//		buffer.WriteString(", roleID: ")
//		buffer.WriteString(strconv.FormatUint(s.player.roleID, 10))
//	}
//
//	return buffer.String()
//}

// WriteCb 写入消息，发送，发送结束后无论成功还是失败，都会调用 cb
func (s *Session) WriteCb(b []byte, cb func()) error {
	// 测试
	//if len(b) > 0 {
	//	fmt.Printf("Send %d bytes:\n%s\n", len(b), hex.Dump(b))
	//}
	return s.conn.AsyncWrite(b, func(_ gnet.Conn, err error) error {
		if cb != nil {
			cb()
		}

		if err == net.ErrClosed {
			err = nil
		}
		return err
	})
}

// RoleID 绑定的角色 id
//func (s *Session) RoleID() uint64 {
//	if s.player != nil {
//		return s.player.roleID
//	}
//	return 0
//}

// Reset 重置
func (s *Session) Reset() {
	s.conn = nil
	s.remoteAddr = ""

	s.shareKey = ""
	if s.Player != nil {
		playerPool.Put(s.Player)
		s.Player = nil
	}
	//s.version = nil
}

// -------------------------------------- 内部 --------------------------------------

func NewSession(c gnet.Conn) *Session {
	s := sessionPool.Get()
	s.reset()

	s.conn = c
	// 客户端请求并发数目前设置为 1
	//s.readChan = make(chan struct{}, 1)
	//s.closeChan = make(chan struct{})
	s.remoteAddr = s.RemoteAddrString()
	return s
}

func (s *Session) Start() {
	s.pingTime = time.Now()

	n := 0
	maxN := getMaxRetries()

	for s.RemoteAddr() != nil {
		time.Sleep(PingCheckInterval)

		if !s.checkHeart() {
			if s.incrementAndCheckRetries(&n, maxN) {
				return
			}
		} else {
			n = 0
		}
	}
}

func getMaxRetries() int {
	if config.Get().IsDevelop() {
		return 1000
	}
	return 10
}

// 超时关闭
func (s *Session) incrementAndCheckRetries(n *int, maxN int) bool {
	*n++
	if *n > maxN {
		if err := s.conn.Close(); err != nil {
			logger.Get().Error("[gate.session.start] checkHeart failed then conn.Close failed", zap.Error(err))
		}
		return true
	}
	return false
}

func (s *Session) Close() {
	s.closeOnce.Do(func() {
		s.shutdown()

		// 关闭连接
		s.lock.Lock()
		defer s.lock.Unlock()

		//close(s.closeChan)

		sessionPool.Put(s)
	})
}

// reset 重置 Session 状态
func (s *Session) reset() {
	s.conn = nil
	s.remoteAddr = ""
	s.pingTime = time.Time{}
	//s.readChan = nil
	//s.closeChan = nil
	s.shareKey = ""
	s.Player = nil
}

// shutdown 关闭账号，但不关闭连接
//
// 表现为将玩家踢出游戏
func (s *Session) shutdown() {
	s.lock.Lock()
	defer s.lock.Unlock()

	logger.Get().Debug(
		"[gate.session.shutdown] exit",
		zap.String("address", s.RemoteAddrString()),
		zap.Uint64("account", s.accountID()),
		zap.Uint32("server", s.serverID()),
		zap.Uint64("role", s.roleID()),
	)

	// TODO 玩家已登录处理
}

func (s *Session) checkHeart() (isTimeout bool) {
	c := config.Get()
	if c.IsDevelop() {
		// 开发模式下，时间改大
		if s.pingTime.Add(5 * time.Minute).Before(time.Now()) {
			isTimeout = true
		}
		return
	}

	// 10 次都未收到心跳信息
	if s.pingTime.Add(PingCheckInterval * 10).Before(time.Now()) {
		isTimeout = true
	}

	return
}

func (s *Session) accountID() uint64 {
	if s.Player == nil {
		return 0
	}
	return s.Player.accountID
}

// RealServerID 玩家当前所处区服
func (s *Session) RealServerID() uint32 {
	if s.Player == nil {
		return 0
	}
	return s.Player.realServerID
}

func (s *Session) serverID() uint32 {
	if s.Player == nil {
		return 0
	}
	return s.Player.serverID
}

func (s *Session) roleID() uint64 {
	if s.Player == nil {
		return 0
	}
	return s.Player.RoleID
}

var playerPool = NewPool(func() *Player {
	return &Player{}
})

// Reset 重置
func (p *Player) Reset() {
	p.accountID = 0
	p.realServerID = 0
	p.serverID = 0
	p.RoleID = 0
}

func (p *Player) set(accountID uint64, realServerID uint32) {
	p.accountID = accountID
	p.realServerID = realServerID
	p.serverID = realServerID
}
