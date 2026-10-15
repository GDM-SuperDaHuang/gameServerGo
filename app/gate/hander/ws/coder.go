package ws

import (
	"context"
	"fmt"

	"gameServer/pkg/config"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/recoverSafe"
	"gameServer/service/common"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"go.uber.org/zap"
)

// -------------------------------------- 变量 --------------------------------------
var (
	tickerTest = time.NewTicker(1 * time.Second)
)

func (wss *WsServer) TestInit222() {
	// 按房间类型分桶
	for {
		select {
		// 定时撮合
		case <-tickerTest.C:
			load1 := wss.TempCountInto.Load()
			load2 := wss.TempCountWai.Load()
			print("===", load1, load2)
		}
	}
}

// WebSocket 实现

type WsServer struct {
	//sessions     sync.Map     // map[string]*Session 所有玩家的ipPort-session
	roles        sync.Map     // map[uint64]*Session  所有玩家的Uid-session
	sessionCount atomic.Int32 //链接数

	TempCountInto atomic.Int32
	TempCountWai  atomic.Int32

	// qps 统计
	requestCount int64
	lastQPS      int64
}

// -------------------------------------- 外部 --------------------------------------
// Start 启动 WebSocket 服务器
func (wss *WsServer) StartWs() error {
	c := config.Get()
	addr := c.Node().GetString("address") // 192.168.1.102:17003
	http.HandleFunc("/", wss.handleWS)
	log2.Get().Info("[gate] websocket start", zap.String("addr", addr))

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log2.Get().Error("[gate] websocket server error", zap.Error(err))
			panic(err)
		}
	}()

	//wss.TestInit222()
	return nil
}

func (wss *WsServer) handleWS(w http.ResponseWriter, r *http.Request) {
	opts := &websocket.AcceptOptions{
		InsecureSkipVerify: true, //允许跨域
	}
	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		log2.Get().Error("websocket accept failed", zap.Error(err))
		return
	}
	addr := r.RemoteAddr
	wss.newSession(conn, addr)
}

func (wss *WsServer) newSession(conn *websocket.Conn, addr string) {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Session{
		Conn:       conn,
		Ctx:        ctx,
		Cancel:     cancel,
		RemoteAddr: addr,
		SendQueue:  make(chan []byte, 1024), // 队列长度可调
	}

	n := wss.sessionCount.Add(1)
	log2.Get().Debug("[gate.OnOpen] connected", zap.Int32("total", n), zap.String("address", addr))
	recoverSafe.GoSafe("user ws ReadLoop", func() error {
		wss.ReadLoop(s)
		return nil
	})

	recoverSafe.GoSafe("user ws ReadLoop", func() error {
		wss.WriteLoop(s)
		return nil
	})
}

func (wss *WsServer) ReadLoop(s *Session) {
	defer func() {
		s.Close()
		// 如果已登录，从 roles 中清理
		if s.Player != nil && s.Player.Uid != 0 {
			wss.DelSessionIfMatch(s.Player.Uid, s)
		}
	}()

	for {
		_, data, err := s.Conn.Read(s.Ctx)
		if err != nil {
			return
		}

		// 业务处理
		wss.forward(data, s)
	}
}

// 释放消息
func (wss *WsServer) WriteLoop(s *Session) {
	defer func() {
		s.Close() //todo
		// 如果已登录，从 roles 中清理
		if s.Player != nil && s.Player.Uid != 0 {
			wss.DelSessionIfMatch(s.Player.Uid, s)
		}
	}()

	for {
		select {
		case data := <-s.SendQueue:
			err := s.Conn.Write(s.Ctx, websocket.MessageBinary, data)
			if err != nil {
				return
			}
		case <-s.Ctx.Done():
			return
		}
	}
}

// 接口实现
func (wss *WsServer) FindSession(roleID int64) *Session {
	if roleID == 0 {
		return nil
	}

	v, found := wss.roles.Load(roleID)
	if !found {
		return nil
	}

	return v.(*Session)
}

func (wss *WsServer) DelSession(roleID int64) {
	if s := wss.FindSession(roleID); s != nil {
		s.Close()
		if s.Player != nil {
			common.PlayerPool.Put(s.Player)
			s.Player = nil
		}
	}
	// 用户下线时间
	//go dbUser.UpdateUserOutTimestamp(roleID)
	wss.roles.Delete(roleID)
}

// DelSessionIfMatch 仅当当前映射仍为 target 时才删除，避免旧连接误删新连接。
func (wss *WsServer) DelSessionIfMatch(roleID int64, target *Session) bool {
	if roleID == 0 || target == nil {
		return false
	}
	current := wss.FindSession(roleID)
	if current == nil || current != target {
		return false
	}
	wss.DelSession(roleID)
	return true
}

// ----------------------------------------------------------------------------//
// 监控 QPS
func (qs *WsServer) monitorQPS() {
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
func (qs *WsServer) GetQPS() int64 {
	return atomic.LoadInt64(&qs.lastQPS)
}
