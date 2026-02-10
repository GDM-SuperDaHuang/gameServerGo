package gate

import (
	"Server/service/logger"
	"Server/service/proto"
	"Server/service/session"
	"sync"
	"sync/atomic"
	//"server/api/pb/pb_protocol"
	//"server/internal/engine"
	//"server/pkg/datapack"
	//"server/pkg/logger"

	"Server/service/datapack"

	"github.com/panjf2000/ants/v2"
	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

// -------------------------------------- 变量 --------------------------------------

type tcpServer struct {
	*gnet.BuiltinEventEngine // 默认接口实现
	gate                     *Gate
	//engine                   Engine
	address   string
	multicore bool
	datapack  datapack.Datapack

	pool *ants.Pool //协程池

	sessions     sync.Map // map[string]*Session
	roles        sync.Map // map[uint64]*Session
	sessionCount atomic.Int32

	isTest bool
}

// -------------------------------------- 外部 --------------------------------------
// OnBoot 服务器启动时触发
// OnBoot fires when the engine is ready for accepting connections.
// The parameter engine has information and various utilities.
func (ts *tcpServer) OnBoot(_ gnet.Engine) (action gnet.Action) {
	//logger.Get().Info("[gate] tcp start", zap.String("address", ts.address), zap.Bool("multicore", ts.multicore))
	return
}

// OnShutdown fires when the engine is being shut down, it is called right after
// all event-loops and connections are closed.
func (ts *tcpServer) OnShutdown(_ gnet.Engine) {
	//logger.Get().Info("[gate] tcp close", zap.String("address", ts.address))
	ts.pool.Release()
}

// OnOpen 新连接建立时触发
// OnOpen fires when a new connection has been opened.
// The parameter out is the return value which is going to be sent back to the remote.
func (ts *tcpServer) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	remoteAddress := c.RemoteAddr().String()
	if _, exists := ts.sessions.Load(remoteAddress); exists {
		return
	}

	//s := newSession(ts.engine, c)
	//ts.sessions.Store(remoteAddress, s)
	//
	//n := ts.sessionCount.Add(1)
	//logger.Get().Debug("[gate.OnOpen] connected",
	//	zap.Int32("total", n),
	//	zap.String("address", remoteAddress))
	//
	//go s.start()

	return
}

// OnClose fires when a connection has been closed.
// The parameter err is the last known connection error.
func (ts *tcpServer) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	//n := ts.sessionCount.Add(-1)
	//logger.Get().Debug("[gate.OnOpen] disconnect",
	//	zap.Int32("remain", n),
	//	zap.String("address", c.RemoteAddr().String()))

	//if err != nil {
	//	switch err.Error() {
	//	case "EOF", "read: connection reset by peer",
	//		"connection reset by peer", "read: EOF":
	//		// Ignore common connection errors
	//	default:
	//		logger.Get().Error("Connection error", zap.Error(err))
	//	}
	//}
	//
	//// 尽量推送未完成的信息
	//if err := c.Flush(); err != nil {
	//	logger.Get().Error("Failed to flush connection", zap.Error(err))
	//}
	//
	//if s := ts.session(c); s != nil {
	//	ts.delete(s.RemoteAddrString())
	//	s.close()
	//} else {
	//	logger.Get().Warn("[gate.tcpServer.OnClose] session not found",
	//		zap.String("address", c.RemoteAddr().String()))
	//}

	return
}

// 有数据可读时触发（核心处理逻辑）
// OnTraffic fires when a local socket receives data from the remote.
func (ts *tcpServer) OnTraffic(c gnet.Conn) (action gnet.Action) {
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
			datapack.FreeMessage(message) //释放回内存池
		}
	})
	if err != nil {
		//logger.Get().Error("[gate.tcpServer.OnTraffic] ants.Submit failed",
		//	zap.String("session", session.String()),
		//	zap.Error(err))
	}

	return
}

// -------------------------------------- 内部 --------------------------------------

func (ts *tcpServer) session(c gnet.Conn) *session.Session {
	if c.RemoteAddr() == nil {
		return nil
	}

	address := c.RemoteAddr().String()
	v, found := ts.sessions.Load(address)
	if !found {
		return nil
	}

	return v.(*session.Session)
}

func (ts *tcpServer) findSession(roleID uint64) *session.Session {
	if roleID == 0 {
		return nil
	}

	v, found := ts.roles.Load(roleID)
	if !found {
		return nil
	}

	return v.(*session.Session)
}

func (ts *tcpServer) delete(address string) {
	sessionI, found := ts.sessions.Load(address)
	if !found {
		return
	}
	session := sessionI.(*session.Session)
	if session.player != nil && session.player.roleID > 0 {
		ts.roles.Delete(session.player.roleID)
	}
	ts.sessions.Delete(address)
}

func (ts *tcpServer) initPool(poolSize int) error {
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
func (ts *tcpServer) handleMessage(session *session.Session, message *datapack.Message) {
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
func (ts *tcpServer) write(session *session.Session, resp *proto.Resp, message *datapack.Message) error {
	respMessage := datapack.NewMessageResp(resp, message)
	defer datapack.FreeMessage(respMessage)

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

func (ts *tcpServer) InitPool(poolSize int) error {
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
