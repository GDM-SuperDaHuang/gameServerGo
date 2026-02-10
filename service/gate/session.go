package gate

import (
	"Server/service/common"
	"Server/service/config"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

// -------------------------------------- 变量 --------------------------------------

const (
	// PingCheckInterval 心跳检查间隔
	PingCheckInterval = 3 * time.Second
)

var sessionPool = common.NewPool(func() *Session {
	return &Session{}
})

// Session 客户端会话
type Session struct {
	//engine     engine.Engine
	conn       gnet.Conn
	remoteAddr string

	pingTime  time.Time
	readChan  chan struct{}
	closeChan chan struct{}

	// ready 是否准备好
	ready atomic.Bool
	// shareKey rc4 加密密钥
	shareKey string
	// player 绑定的玩家信息
	player *player
	// 请求指定版本号范围
	//version *pb_gate.Version

	closeOnce sync.Once
	lock      sync.RWMutex
}

type player struct {
	accountID    uint64
	realServerID uint32 // 角色当前所在的区服 id
	serverID     uint32 // 角色归属区服 id
	roleID       uint64 // 角色 id
}

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
func (s *Session) String() string {
	buffer := bytes.Get().Buffer(256)
	defer bytes.Get().Release(buffer)

	buffer.WriteString("address: ")
	buffer.WriteString(s.RemoteAddrString())

	if s.player != nil && s.player.accountID > 0 {
		buffer.WriteString(", accountID: ")
		buffer.WriteString(strconv.FormatUint(s.player.accountID, 10))
		buffer.WriteString(", serverID: ")
		buffer.WriteString(strconv.Itoa(int(s.player.serverID)))
		buffer.WriteString(", roleID: ")
		buffer.WriteString(strconv.FormatUint(s.player.roleID, 10))
	}

	return buffer.String()
}

// WriteCb 写入消息，发送，发送结束后无论成功还是失败，都会调用 cb
func (s *Session) WriteCb(b []byte, cb func()) error {
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
func (s *Session) RoleID() uint64 {
	if s.player != nil {
		return s.player.roleID
	}
	return 0
}

// Reset 重置
func (s *Session) Reset() {
	s.engine = nil
	s.conn = nil
	s.remoteAddr = ""
	s.readChan = nil
	s.closeChan = nil
	s.ready.Store(false)
	s.shareKey = ""
	if s.player != nil {
		playerPool.Put(s.player)
		s.player = nil
	}
	s.version = nil
}

// -------------------------------------- 内部 --------------------------------------

func newSession(engine engine.Engine, c gnet.Conn) *Session {
	s := sessionPool.Get()
	s.reset()

	s.engine = engine
	s.conn = c
	// 客户端请求并发数目前设置为 1
	s.readChan = make(chan struct{}, 1)
	s.closeChan = make(chan struct{})
	s.remoteAddr = s.RemoteAddrString()
	return s
}

func (s *Session) start() {
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

func (s *Session) close() {
	s.closeOnce.Do(func() {
		s.shutdown()

		// 关闭连接
		s.lock.Lock()
		defer s.lock.Unlock()

		close(s.closeChan)

		sessionPool.Put(s)
	})
}

// reset 重置 Session 状态
func (s *Session) reset() {
	s.engine = nil
	s.conn = nil
	s.remoteAddr = ""
	s.pingTime = time.Time{}
	s.readChan = nil
	s.closeChan = nil
	s.ready.Store(false)
	s.shareKey = ""
	s.player = nil
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
	if s.player == nil {
		return 0
	}
	return s.player.accountID
}

// RealServerID 玩家当前所处区服
func (s *Session) RealServerID() uint32 {
	if s.player == nil {
		return 0
	}
	return s.player.realServerID
}

func (s *Session) serverID() uint32 {
	if s.player == nil {
		return 0
	}
	return s.player.serverID
}

func (s *Session) roleID() uint64 {
	if s.player == nil {
		return 0
	}
	return s.player.roleID
}

var playerPool = bytes.NewPool(func() *player {
	return &player{}
})

// Reset 重置
func (p *player) Reset() {
	p.accountID = 0
	p.realServerID = 0
	p.serverID = 0
	p.roleID = 0
}

func (p *player) set(accountID uint64, realServerID uint32) {
	p.accountID = accountID
	p.realServerID = realServerID
	p.serverID = realServerID
}
