package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"gnetClient/protobuf/pbGo"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

const HeaderLen = 12

// =======================
// 压测开关: 启动后串行创建 UserlinkingNumber 条 ws 链接
// (上一条完成才拨下一条)，全部连完后再高并发请求
// IsLoginReq=true  -> 全部连完后并发 SendLoginReq
// IsBuyItemRep=true -> 收到登录响应后自动 SendBuyItemRep
// =======================
const WsURL = "ws://127.0.0.1:17003/ws"
const UserlinkingNumber = 1
const IsLoginReq = true
const IsBuyItemRep = true

// 1000 连接时每个包都 Printf 会卡死控制台，默认只打统计
const VerboseLog = false

// 协议号常量
const (
	ProtocolLogin   = 4
	ProtocolBuyItem = 2002
)

// =======================
// 协议结构 (保持和 TCP 版一致: 12字节头 + protobuf body)
// Len(2) | Flag(2) | SN(4) | Code(2) | Protocol(2) | Body
// =======================

type MessageHead struct {
	Len      uint16
	Flag     uint16
	SN       uint32
	Code     uint16
	Protocol uint16
}

type Message struct {
	Head *MessageHead
	Body []byte
}

// =======================
// 封包 (不变)
// =======================

func PackMessage(msg *Message) ([]byte, error) {
	buf := new(bytes.Buffer)

	msg.Head.Len = uint16(len(msg.Body))

	// 按网络字节序写入
	if err := binary.Write(buf, binary.BigEndian, msg.Head.Len); err != nil {
		return nil, fmt.Errorf("write len failed: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, msg.Head.Flag); err != nil {
		return nil, fmt.Errorf("write flag failed: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, msg.Head.SN); err != nil {
		return nil, fmt.Errorf("write sn failed: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, msg.Head.Code); err != nil {
		return nil, fmt.Errorf("write code failed: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, msg.Head.Protocol); err != nil {
		return nil, fmt.Errorf("write protocol failed: %w", err)
	}

	if _, err := buf.Write(msg.Body); err != nil {
		return nil, fmt.Errorf("write body failed: %w", err)
	}

	return buf.Bytes(), nil
}

// =======================
// 解包: WebSocket 下是一整帧 byte[]，直接解析，不再用 io.ReadFull 流式读
// =======================

func ParseMessage(data []byte) (*Message, error) {
	if len(data) < HeaderLen {
		return nil, fmt.Errorf("packet too short: %d < %d", len(data), HeaderLen)
	}

	reader := bytes.NewReader(data[:HeaderLen])
	head := &MessageHead{}

	if err := binary.Read(reader, binary.BigEndian, &head.Len); err != nil {
		return nil, fmt.Errorf("parse len failed: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &head.Flag); err != nil {
		return nil, fmt.Errorf("parse flag failed: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &head.SN); err != nil {
		return nil, fmt.Errorf("parse sn failed: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &head.Code); err != nil {
		return nil, fmt.Errorf("parse code failed: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &head.Protocol); err != nil {
		return nil, fmt.Errorf("parse protocol failed: %w", err)
	}

	if int(head.Len) != len(data)-HeaderLen {
		return nil, fmt.Errorf("body len mismatch: head.Len=%d, actual=%d", head.Len, len(data)-HeaderLen)
	}

	body := make([]byte, head.Len)
	copy(body, data[HeaderLen:])

	return &Message{
		Head: head,
		Body: body,
	}, nil
}

// =======================
// 全局统计 (多客户端共享，原子操作)
// =======================

var (
	loginRespCount   int64
	buyItemRespCount int64
	testRpcRespCount int64
	sendFailCount    int64
	dialFailCount    int64
	connectedCount   int64
)

func vprintf(format string, args ...interface{}) {
	if VerboseLog {
		fmt.Printf(format, args...)
	}
}

// =======================
// 消息处理器 (持有 WebSocket 连接，每个连接一个实例)
// =======================

type MessageHandler struct {
	id      int
	conn    *websocket.Conn
	sn      uint32
	snMu    sync.Mutex
	writeMu sync.Mutex
	closed  chan struct{}
	closeMu sync.Once
}

func NewMessageHandler(id int, conn *websocket.Conn) *MessageHandler {
	return &MessageHandler{
		id:     id,
		conn:   conn,
		sn:     1,
		closed: make(chan struct{}),
	}
}

func (h *MessageHandler) prefix() string {
	return fmt.Sprintf("[c-%d]", h.id)
}

func (h *MessageHandler) NextSN() uint32 {
	h.snMu.Lock()
	defer h.snMu.Unlock()
	sn := h.sn
	h.sn++
	return sn
}

// 统一发送入口: 封包 + BinaryMessage 发送 (写加锁，gorilla/websocket 不允许并发写)
func (h *MessageHandler) SendMessage(msg *Message) error {
	data, err := PackMessage(msg)
	if err != nil {
		return fmt.Errorf("pack message failed: %w", err)
	}

	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	h.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := h.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("ws write failed: %w", err)
	}
	return nil
}

func (h *MessageHandler) SendLoginReq(sn uint32, protocolId uint16) error {
	// 每个客户端用独立 Code，避免服务端当成同一账号互踢
	body, err := proto.Marshal(&pbGo.LoginReq{
		LoginType: 0,
		Code:      fmt.Sprintf("000_%d", h.id),
	})
	if err != nil {
		return fmt.Errorf("marshal login req failed: %w", err)
	}

	msg := &Message{
		Head: &MessageHead{
			Flag:     0,
			SN:       sn,
			Code:     0,
			Protocol: protocolId,
		},
		Body: body,
	}

	if err := h.SendMessage(msg); err != nil {
		atomic.AddInt64(&sendFailCount, 1)
		return err
	}

	vprintf("%s 发送请求: SN=%d, protocolId=%d\n", h.prefix(), sn, protocolId)
	return nil
}

func (h *MessageHandler) SendBuyItemRep(sn uint32, protocolId uint16) error {
	body, err := proto.Marshal(&pbGo.BuyItemRep{
		Item: &pbGo.ItemInfo{
			ItemId: 1001,
			Count:  1,
		},
	})
	if err != nil {
		return fmt.Errorf("marshal buy item req failed: %w", err)
	}

	msg := &Message{
		Head: &MessageHead{
			Flag:     0,
			SN:       sn,
			Code:     0,
			Protocol: protocolId,
		},
		Body: body,
	}

	if err := h.SendMessage(msg); err != nil {
		atomic.AddInt64(&sendFailCount, 1)
		return err
	}

	vprintf("%s 发送请求: SN=%d, protocolId=%d\n", h.prefix(), sn, protocolId)
	return nil
}

func (h *MessageHandler) SendKeyTestReq(sn uint32, protocolId uint16) error {
	body, err := proto.Marshal(&pbGo.KeyTestReq{
		TestReq: "1",
	})
	if err != nil {
		return fmt.Errorf("marshal test rpc req failed: %w", err)
	}

	msg := &Message{
		Head: &MessageHead{
			Flag:     0,
			SN:       sn,
			Code:     0,
			Protocol: protocolId,
		},
		Body: body,
	}

	if err := h.SendMessage(msg); err != nil {
		atomic.AddInt64(&sendFailCount, 1)
		return err
	}

	vprintf("%s 发送RPC请求: SN=%d, protocolId=%d\n", h.prefix(), sn, protocolId)
	return nil
}

func (h *MessageHandler) HandleMessage(msg *Message) {
	vprintf("%s [收到消息] SN=%d | Protocol=%d | Code=%d | BodyLen=%d\n", h.prefix(), msg.Head.SN, msg.Head.Protocol, msg.Head.Code, msg.Head.Len)

	switch msg.Head.Protocol {
	case ProtocolLogin: // 登录响应
		var resp pbGo.LoginResp
		if err := proto.Unmarshal(msg.Body, &resp); err != nil {
			fmt.Printf("%s 解析登录响应失败: %v\n", h.prefix(), err)
			return
		}
		atomic.AddInt64(&loginRespCount, 1)
		//vprintf("%s 登录响应: UUID=%d, ItemList=%s\n", h.prefix(), resp.UserId, resp.ItemList)

		// 自动链: 登录成功后自动发购买请求
		if IsBuyItemRep {
			if err := h.SendBuyItemRep(h.NextSN(), ProtocolBuyItem); err != nil {
				fmt.Printf("%s 自动发送购买请求失败: %v\n", h.prefix(), err)
			}
		}

	case ProtocolBuyItem: // 购买响应
		var resp pbGo.BuyItemResp
		if err := proto.Unmarshal(msg.Body, &resp); err != nil {
			fmt.Printf("%s 解析购买响应失败: %v\n", h.prefix(), err)
			return
		}
		atomic.AddInt64(&buyItemRespCount, 1)
		vprintf("%s 购买响应: %s\n", h.prefix(), resp.String())

	case 5: // 购买响应
		var resp pbGo.KeyTestResp
		if err := proto.Unmarshal(msg.Body, &resp); err != nil {
			fmt.Printf("%s 解析购买响应失败: %v\n", h.prefix(), err)
			return
		}
		atomic.AddInt64(&buyItemRespCount, 1)
		vprintf("%s 购买响应: %s\n", h.prefix(), resp.String())

	default:
		fmt.Printf("%s 未知协议: %d\n", h.prefix(), msg.Head.Protocol)
	}
}

// =======================
// 建连阶段: 串行拨号，一条完成才拨下一条 (主协程阻塞调用)
// 连接成功后启动心跳 + 独立读协程，读协程由 wg 跟踪
// =======================

func connectOne(id int, wg *sync.WaitGroup) (*MessageHandler, error) {
	prefix := fmt.Sprintf("[c-%d]", id)

	conn, _, err := websocket.DefaultDialer.Dial(WsURL, nil)
	if err != nil {
		atomic.AddInt64(&dialFailCount, 1)
		return nil, err
	}
	atomic.AddInt64(&connectedCount, 1)
	fmt.Printf("%s 连接成功 (%s)\n", prefix, WsURL)

	handler := NewMessageHandler(id, conn)
	registerClient(handler)

	// 防止大包攻击 + 保活
	conn.SetReadLimit(4 << 20) // 4MB
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 心跳: 定期发 Ping
	ticker := time.NewTicker(30 * time.Second)
	done := handler.closed
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				handler.writeMu.Lock()
				handler.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err := handler.conn.WriteMessage(websocket.PingMessage, nil)
				handler.writeMu.Unlock()
				if err != nil {
					fmt.Printf("%s 发送Ping失败: %v\n", prefix, err)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// 独立读协程: 每条链接一个
	wg.Add(1)
	go readLoop(handler, wg)

	return handler, nil
}

func readLoop(handler *MessageHandler, wg *sync.WaitGroup) {
	defer wg.Done()
	prefix := handler.prefix()
	conn := handler.conn
	defer func() {
		unregisterClient(handler)
		conn.Close()
		fmt.Printf("%s 连接关闭\n", prefix)
	}()

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-handler.closed:
				// 主动关闭，不打印错误
			default:
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					fmt.Printf("%s 读取失败: %v\n", prefix, err)
				} else {
					fmt.Printf("%s 服务器断开连接\n", prefix)
				}
			}
			return
		}
		// 续租读超时
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		if msgType != websocket.BinaryMessage {
			// 忽略 Text / Ping / Pong 等非二进制帧
			continue
		}
		msg, err := ParseMessage(data)
		if err != nil {
			fmt.Printf("%s 解析消息失败: %v\n", prefix, err)
			continue
		}
		handler.HandleMessage(msg)
	}
}

// 高并发请求阶段: 快照所有在线客户端，每个开协程同时发，全部发完才返回
func concurrentSend(send func(h *MessageHandler) error) {
	clientsMu.RLock()
	snapshot := make([]*MessageHandler, 0, len(clients))
	for _, h := range clients {
		snapshot = append(snapshot, h)
	}
	clientsMu.RUnlock()
	if len(snapshot) == 0 {
		fmt.Println("当前无在线连接")
		return
	}
	var reqWg sync.WaitGroup
	for _, h := range snapshot {
		reqWg.Add(1)
		go func(h *MessageHandler) {
			defer reqWg.Done()
			if err := send(h); err != nil {
				fmt.Printf("%s 发送失败: %v\n", h.prefix(), err)
			}
		}(h)
	}
	reqWg.Wait()
	fmt.Printf("高并发请求完成: %d 条连接\n", len(snapshot))
}

func (h *MessageHandler) Close() {
	h.closeMu.Do(func() {
		close(h.closed)
		// 尝试优雅关闭，写失败也无所谓
		h.writeMu.Lock()
		_ = h.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		h.writeMu.Unlock()
		_ = h.conn.Close()
	})
}

// =======================
// 在线客户端表 (用于手动广播 / 优雅关闭)
// =======================

var (
	clientsMu sync.RWMutex
	clients   = make(map[int]*MessageHandler)
)

func registerClient(h *MessageHandler) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	clients[h.id] = h
}

func unregisterClient(h *MessageHandler) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(clients, h.id)
}

func closeAllClients() {
	clientsMu.RLock()
	snapshot := make([]*MessageHandler, 0, len(clients))
	for _, h := range clients {
		snapshot = append(snapshot, h)
	}
	clientsMu.RUnlock()
	for _, h := range snapshot {
		h.Close()
	}
}

func printStats(tag string) {
	clientsMu.RLock()
	n := len(clients)
	clientsMu.RUnlock()
	fmt.Printf("[%s] 在线=%d 累计建连成功=%d loginResp=%d buyItemResp=%d testRpcResp=%d dialFail=%d sendFail=%d\n",
		tag, n,
		atomic.LoadInt64(&connectedCount),
		atomic.LoadInt64(&loginRespCount),
		atomic.LoadInt64(&buyItemRespCount),
		atomic.LoadInt64(&testRpcRespCount),
		atomic.LoadInt64(&dialFailCount),
		atomic.LoadInt64(&sendFailCount))
}

// 广播发送给所有在线客户端 (高并发: 每条连接一个协程同时发)
func broadcastSend(protocolInt int) {
	switch protocolInt {
	case ProtocolLogin:
		concurrentSend(func(h *MessageHandler) error {
			return h.SendLoginReq(h.NextSN(), uint16(protocolInt))
		})
	case 5:
		concurrentSend(func(h *MessageHandler) error {
			return h.SendKeyTestReq(h.NextSN(), uint16(protocolInt))
		})
	case ProtocolBuyItem:
		concurrentSend(func(h *MessageHandler) error {
			return h.SendBuyItemRep(h.NextSN(), uint16(protocolInt))
		})
	default:
		fmt.Printf("未知命令: %d\n", protocolInt)
	}
}

func main() {
	fmt.Printf("启动压测客户端: 目标=%s, 连接数=%d, IsLoginReq=%v, IsBuyItemRep=%v\n",
		WsURL, UserlinkingNumber, IsLoginReq, IsBuyItemRep)

	// Ctrl+C 优雅关闭
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	var wg sync.WaitGroup

	// 阶段1: 串行建连，一条完成才拨下一条
	for i := 1; i <= UserlinkingNumber; i++ {
		fmt.Printf("正在创建第 %d/%d 条链接...\n", i, UserlinkingNumber)
		if _, err := connectOne(i, &wg); err != nil {
			fmt.Printf("[c-%d] 连接失败: %v\n", i, err)
			continue
		}
		fmt.Printf("第 %d/%d 条链接完成\n", i, UserlinkingNumber)
	}
	printStats("建连完成")

	// 阶段2: 全部连完后，高并发请求
	if IsLoginReq {
		fmt.Println("全部链接完成，开始高并发发送登录请求...")
		broadcastSend(ProtocolLogin)
	}

	// 主循环：处理用户输入 (广播到所有连接)，同时监听中断
	// 输入协议号发送: 4=登录, 1000=TestRpc, 2002=购买 (回车即向全部连接广播)
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("输入协议号发送(广播到所有连接): 4=登录, 1000=TestRpc, 2002=购买")
	fmt.Print("> ")

	inputCh := make(chan string)
	go func() {
		for scanner.Scan() {
			inputCh <- scanner.Text()
		}
		close(inputCh)
	}()

	// 等所有客户端协程退出的信号
	allDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(allDone)
	}()

	// 定时打印统计
	statsTicker := time.NewTicker(10 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case <-allDone:
			fmt.Println("全部连接已关闭，程序退出")
			printStats("终报")
			return
		case <-interrupt:
			fmt.Println("\n收到中断，正在关闭全部连接...")
			closeAllClients()
			select {
			case <-allDone:
			case <-time.After(3 * time.Second):
			}
			printStats("终报")
			return
		case <-statsTicker.C:
			printStats("统计")
		case text, ok := <-inputCh:
			if !ok {
				fmt.Println("输入关闭，正在关闭全部连接后退出...")
				closeAllClients()
				select {
				case <-allDone:
				case <-time.After(3 * time.Second):
				}
				return
			}
			fields := strings.Fields(text)
			if len(fields) == 0 {
				fmt.Print("> ")
				continue
			}
			protocolInt, err := strconv.Atoi(fields[0]) // 第一个必须是数字
			if err != nil {
				fmt.Printf("输入无效，请输入数字协议号: %v\n", err)
				fmt.Print("> ")
				continue
			}
			broadcastSend(protocolInt)
			fmt.Print("> ")
		}
	}
}
