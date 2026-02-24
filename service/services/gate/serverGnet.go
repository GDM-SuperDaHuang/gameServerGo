package gate

import (
	"encoding/hex"
	"fmt"
	"gameServer/service/common"
	"gameServer/service/logger"
	datapack2 "gameServer/service/services/gate/datapack"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

// -------------------------------------- 变量 --------------------------------------

// gnet 实现
type gNetServer struct {
	*gnet.BuiltinEventEngine // 默认接口实现
	gate                     *Gate
	//engine                   Engine
	address   string
	multicore bool
	datapack  datapack2.Datapack

	pool *ants.Pool //协程池

	sessions     sync.Map     // map[string]*Session 所有玩家的ipPort-session
	roles        sync.Map     // map[uint64]*Session  所有玩家的userid-session
	sessionCount atomic.Int32 //链接数

	isTest bool

	// qps 统计
	requestCount int64
	lastQPS      int64
}

// -------------------------------------- 外部 --------------------------------------
// OnBoot 服务器启动时触发
// OnBoot fires when the engine is ready for accepting connections.
// The parameter engine has information and various utilities.
func (ts *gNetServer) OnBoot(_ gnet.Engine) (action gnet.Action) {
	// 启动 QPS 监控协程
	go ts.monitorQPS()
	logger.Get().Info("[gate] tcp start", zap.String("address", ts.address), zap.Bool("multicore", ts.multicore))
	return
}

// OnShutdown fires when the engine is being shut down, it is called right after
// all event-loops and connections are closed.
func (ts *gNetServer) OnShutdown(_ gnet.Engine) {
	logger.Get().Info("[gate] tcp close", zap.String("address", ts.address))
	ts.pool.Release()
}

// OnOpen 新连接建立时触发
// OnOpen fires when a new connection has been opened.
// The parameter out is the return value which is going to be sent back to the remote.
func (ts *gNetServer) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	remoteAddress := c.RemoteAddr().String()
	if _, exists := ts.sessions.Load(remoteAddress); exists {
		return
	}

	s := common.NewSession(c)
	ts.sessions.Store(remoteAddress, s)

	n := ts.sessionCount.Add(1)
	logger.Get().Debug("[gate.OnOpen] connected",
		zap.Int32("total", n),
		zap.String("address", remoteAddress))

	go s.Start()

	return
}

// OnClose fires when a connection has been closed.
// The parameter err is the last known connection error.
// 无论怎么样退出都清除玩家的 session 数据
func (gn *gNetServer) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	n := gn.sessionCount.Add(-1)
	logger.Get().Debug("[gate.OnOpen] disconnect",
		zap.Int32("remain", n),
		zap.String("address", c.RemoteAddr().String()))

	if err != nil {
		switch err.Error() {
		case "EOF", "read: connection reset by peer",
			"connection reset by peer", "read: EOF":
			// Ignore common connection errors
		default:
			logger.Get().Error("Connection error", zap.Error(err))
		}
	}

	// 尽量推送未完成的信息
	if err = c.Flush(); err != nil {
		logger.Get().Error("Failed to flush connection", zap.Error(err))
	}

	if s := gn.session(c); s != nil {
		gn.delete(s.RemoteAddrString())
		s.Close()
	} else {
		logger.Get().Warn("[gate.tcpServer.OnClose] session not found",
			zap.String("address", c.RemoteAddr().String()))
	}

	return
}

// 有数据可读时触发（核心处理逻辑）
func (ts *gNetServer) OnTraffic(c gnet.Conn) (action gnet.Action) {
	// 收到消息二进制打印
	rawBuf := make([]byte, 1024) // 或者适当大小
	n, _ := c.Read(rawBuf)
	if n > 0 {
		fmt.Printf("Recv raw bytes: %d\n%s\n", n, hex.Dump(rawBuf[:n]))
	}

	// 打印
	err, g, done := ts.PrintfBuffered(c)
	if done {
		return g
	}

	session := ts.session(c)
	if session == nil {
		//logger.Get().Error("[gate.tcpServer.OnTraffic] session not found",
		//	zap.String("remote address", c.RemoteAddr().String()))
		return gnet.Close
	}

	// 解码
	messages, err := ts.datapack.Unpack(c)
	if err != nil {
		//logger.Get().Error("[gate.tcpServer.OnTraffic] Unpack failed",
		//	zap.String("session", session.String()),
		//	zap.Error(err))
		return gnet.Close
	}

	// 提交任务到池中执行
	err = ts.pool.Submit(func() {
		for _, message := range messages {
			ts.handleMessage(session, message)
			common.FreeMessage(message) //释放回内存池
		}
	})
	if err != nil {
		//logger.Get().Error("[gate.tcpServer.OnTraffic] ants.Submit failed",
		//	zap.String("session", session.String()),
		//	zap.Error(err))
	}

	// qps 每处理一个请求，计数器 +1
	atomic.AddInt64(&ts.requestCount, 1)
	return
}

func (ts *gNetServer) PrintfBuffered(c gnet.Conn) (error, gnet.Action, bool) {
	// 当前缓冲区已有的数据长度
	n := c.InboundBuffered()
	if n == 0 {
		return nil, 0, true
	}

	// 只查看，不读取（不会清除缓冲区数据）
	buf, err := c.Peek(n)
	if err != nil {
		fmt.Println("peek error:", err)
		return nil, gnet.Close, true
	}
	// 打印十六进制
	fmt.Printf("recv (%d bytes): % X\n", n, buf)
	return err, 0, false
}

// -------------------------------------- 内部 --------------------------------------

func (ts *gNetServer) session(c gnet.Conn) *common.Session {
	if c.RemoteAddr() == nil {
		return nil
	}

	address := c.RemoteAddr().String()
	v, found := ts.sessions.Load(address)
	if !found {
		return nil
	}

	return v.(*common.Session)
}

func (ts *gNetServer) findSession(roleID uint64) *common.Session {
	if roleID == 0 {
		return nil
	}

	v, found := ts.roles.Load(roleID)
	if !found {
		return nil
	}

	return v.(*common.Session)
}

// 清空sessions
func (ts *gNetServer) delete(address string) {
	sessionI, found := ts.sessions.Load(address)
	if !found {
		return
	}
	session := sessionI.(*common.Session)
	if session.Player != nil && session.Player.RoleID > 0 {
		ts.roles.Delete(session.Player.RoleID)
	}
	ts.sessions.Delete(address)
}

func (ts *gNetServer) initPool(poolSize int) error {
	pool, err := ants.NewPool(
		poolSize,
		ants.WithPanicHandler(func(i any) {
			logger.Get().Error("[web.ants] panic", zap.Any("err", i))
		}),
	)
	if err != nil {
		return err
	}

	ts.pool = pool
	return nil
}

// Create response handler closure
func (ts *gNetServer) handleMessage(session *common.Session, message *common.Message) {
	//if ts.isTest {
	//	logger.Get().Info(
	//		"req <----",
	//		zap.Uint64("role", session.RoleID()),
	//		zap.Uint16("protocol", message.Head.Protocol),
	//		zap.Uint32("sn", message.Head.SN),
	//	)
	//}

	// 返回结果
	resp := ts.gate.forward(session, message)
	// request->response 模式
	_ = ts.write(session, resp, message)
}

// write 写入数据，发送到客户端
func (ts *gNetServer) write(session *common.Session, resp *common.Resp, message *common.Message) error {
	respMessage := common.NewMessageResp(resp, message)
	defer common.FreeMessage(respMessage)

	// 分享秘钥时不加密，也不验证校验值
	//if protocol == pb_protocol.MessageID_SecretSharePubKey {
	//	resp.Head.Flag = resp.Head.Flag &^ datapack.MessageFlagEncrypt //按位清除
	//}

	cb, b, err := ts.datapack.Pack(respMessage, nil)
	if err != nil {
		//logger.Get().Error("[gate.tcpServer.OnTraffic] pack message failed",
		//	zap.String("session", session.String()),
		//	zap.Error(err))
		return err
	}

	if len(b) > 0 {
		if ts.isTest {
			//logger.Get().Info(
			//	"resp ---->",
			//	zap.Uint64("role", session.RoleID()),
			//	zap.Uint16("protocol", uint16(protocol)),
			//	zap.Uint32("sn", sn),
			//	zap.Int32("code", int32(code)),
			//	zap.Int("size", len(b)),
			//)
		}

		if err := session.WriteCb(b, cb); err != nil {
			//logger.Get().Error("[gate.tcpServer.OnTraffic] session write failed",
			//	zap.String("session", session.String()),
			//	zap.Error(err))
			return err
		}
	}

	return nil
}

func (ts *gNetServer) InitPool(poolSize int) error {
	pool, err := ants.NewPool(
		poolSize,
		ants.WithPanicHandler(func(i any) {
			logger.Get().Error("[web.ants] panic", zap.Any("err", i))
		}),
	)
	if err != nil {
		return err
	}

	ts.pool = pool
	return nil
}

// ----------------------------------------------------------------------------//
// 监控 QPS
func (qs *gNetServer) monitorQPS() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 获取当前计数并归零
		count := atomic.SwapInt64(&qs.requestCount, 0)
		atomic.StoreInt64(&qs.lastQPS, count)

		fmt.Printf("当前 QPS: %d\n", count)
	}
}

// 获取当前 QPS
func (qs *gNetServer) GetQPS() int64 {
	return atomic.LoadInt64(&qs.lastQPS)
}
