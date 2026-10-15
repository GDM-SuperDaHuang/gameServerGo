package main

// Docker Swarm 全集群监控
//
// 部署位置: Swarm 管理节点 (manager), 以单副本运行, 挂载 /var/run/docker.sock。
// 原理: 只有 manager 的 dockerd 拥有全集群视图 (/nodes /services /tasks 全局可见)。
//   worker 节点的 dockerd 只能看到本机容器, 因此不能在每台机器各跑一个旧版 monitor。
//
// 监控覆盖 (默认全部, 不再按名字过滤):
//   1) Swarm Task 异常: failed / rejected / 退出码非 0 (实时事件 + 轮询兜底)
//   2) Swarm Service 降级: Running 副本数 < 期望副本数
//   3) Swarm Node 异常: down / disconnected / drain 下线的业务节点
//   4) 本机普通容器 die (兼容 infra 等非 swarm 容器)
//
// 推送: 飞书群机器人 (加签模式)
//
// 环境变量:
//   FEISHU_WEBHOOK  飞书 webhook (默认沿用旧地址)
//   FEISHU_SECRET   飞书加签密钥
//   POLL_INTERVAL   全量轮询间隔, 默认 30s (兜底, 防止事件丢失)
//   ALERT_COOLDOWN  同一故障重复告警冷却, 默认 10m (service正常变更通知无冷却)
//   MONITOR_INCLUDE 仅监控名字包含该子串的 service/task, 为空=全部监控
//   DEBUG_EVENTS    设为 1 则打印所有原始 event

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const dockerSocket = "/var/run/docker.sock"

var cfg = struct {
	webhook       string
	secret        string
	pollInterval  time.Duration
	alertCooldown time.Duration
	taskMaxAge    time.Duration
	include       string
	debugEvents   bool
}{
	webhook:       getenv("FEISHU_WEBHOOK", "https://open.feishu.cn/open-apis/bot/v2/hook/06bcb6d4-3325-4079-aa0c-47a04fdb8ebb"),
	secret:        getenv("FEISHU_SECRET", "GPWaLJePQ7l2VlzswwMEP"),
	pollInterval:  getenvDur("POLL_INTERVAL", 30*time.Second),
	alertCooldown: getenvDur("ALERT_COOLDOWN", 10*time.Minute),
	taskMaxAge:    getenvDur("TASK_MAX_AGE", 30*time.Minute),
	include:       os.Getenv("MONITOR_INCLUDE"),
	debugEvents:   os.Getenv("DEBUG_EVENTS") == "1",
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func getenvDur(k string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// ----------------------------------------------------------------------------
// docker unix-socket client (仅标准库, 不引入 docker SDK)
// ----------------------------------------------------------------------------

var dockerClient = &http.Client{
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", dockerSocket)
		},
	},
	Timeout: 30 * time.Second,
}

func dockerGet(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", "http://unix"+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := dockerClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker API %s -> %d %s", path, resp.StatusCode, string(b))
	}
	return b, nil
}

// ----------------------------------------------------------------------------
// swarm 数据结构 (只取用到的字段)
// ----------------------------------------------------------------------------

type swarmNode struct {
	ID          string `json:"ID"`
	Description struct {
		Hostname string `json:"Hostname"`
	} `json:"Description"`
	Spec struct {
		Availability string `json:"Availability"`
		Role         string `json:"Role"`
	} `json:"Spec"`
	Status struct {
		State string `json:"State"`
	} `json:"Status"`
	ManagerStatus *struct {
		Leader bool `json:"Leader"`
	} `json:"ManagerStatus"`
}

type swarmService struct {
	ID   string `json:"ID"`
	Spec struct {
		Name string `json:"Name"`
		Mode struct {
			Replicated *struct {
				Replicas *uint64 `json:"Replicas"`
			} `json:"Replicated"`
			Global *struct{} `json:"Global"`
		} `json:"Mode"`
		TaskTemplate struct {
			ContainerSpec struct {
				Image string `json:"Image"`
			} `json:"ContainerSpec"`
		} `json:"TaskTemplate"`
	} `json:"Spec"`
}

type swarmTask struct {
	ID           string `json:"ID"`
	ServiceID    string `json:"ServiceID"`
	Slot         int    `json:"Slot"`
	NodeID       string `json:"NodeID"`
	DesiredState string `json:"DesiredState"`
	Spec         struct {
		ContainerSpec struct {
			Image string `json:"Image"`
		} `json:"ContainerSpec"`
	} `json:"Spec"`
	Status struct {
		State           string `json:"State"`
		Err             string `json:"Err"`
		Timestamp       string `json:"Timestamp"`
		ContainerStatus *struct {
			ExitCode    int    `json:"ExitCode"`
			ContainerID string `json:"ContainerID"`
		} `json:"ContainerStatus"`
	} `json:"Status"`
}

// ----------------------------------------------------------------------------
// 告警去重 (冷却)
// ----------------------------------------------------------------------------

type alerter struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func newAlerter() *alerter { return &alerter{last: map[string]time.Time{}} }

// allow 同一 key 在 cooldown 内只告警一次 (仅故障告警用; service正常变更通知无冷却，每条都发)
func (a *alerter) allow(key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t, ok := a.last[key]; ok && time.Since(t) < cfg.alertCooldown {
		return false
	}
	a.last[key] = time.Now()
	// 顺手清理过期 key, 防止 map 无限增长
	for k, t := range a.last {
		if time.Since(t) > 2*cfg.alertCooldown {
			delete(a.last, k)
		}
	}
	return true
}

var alerts = newAlerter()

func matchInclude(name string) bool {
	if cfg.include == "" {
		return true
	}
	return strings.Contains(name, cfg.include)
}

// ----------------------------------------------------------------------------
// main
// ----------------------------------------------------------------------------

func main() {
	fmt.Println("======================================")
	fmt.Println("   Docker Swarm Monitor Started       ")
	fmt.Println("======================================")
	fmt.Printf("poll=%s cooldown=%s include=%q\n", cfg.pollInterval, cfg.alertCooldown, cfg.include)
	fmt.Printf("feishu webhook=...%s (只打印后8位, 用于对照飞书群机器人配置页确认是哪个群)\n", webhookSuffix(cfg.webhook))

	if err := checkSwarm(); err != nil {
		fmt.Println("[FATAL] swarm 检查失败:", err)
		fmt.Println("提示: 本监控必须部署在 Swarm 管理节点(manager)并挂载 /var/run/docker.sock")
		// 不直接退出, 继续重试, 避免 manager 选举期间反复 CrashLoop
	}

	// 事件流断线自动重连
	go func() {
		for {
			if err := listenSwarmEvents(); err != nil {
				fmt.Println("事件流断开, 5秒后重连:", err)
				time.Sleep(5 * time.Second)
			}
		}
	}()

	// 全量轮询兜底
	for {
		if err := pollOnce(); err != nil {
			fmt.Println("[poll] error:", err)
		}
		time.Sleep(cfg.pollInterval)
	}
}

func checkSwarm() error {
	b, err := dockerGet("/info")
	if err != nil {
		return err
	}
	var info struct {
		Swarm struct {
			ControlAvailable bool   `json:"ControlAvailable"`
			NodeID           string `json:"NodeID"`
		} `json:"Swarm"`
	}
	if err := json.Unmarshal(b, &info); err != nil {
		return err
	}
	if !info.Swarm.ControlAvailable {
		return fmt.Errorf("当前节点不是 swarm manager (ControlAvailable=false)")
	}
	fmt.Println("[ok] 当前节点为 swarm manager, NodeID =", info.Swarm.NodeID)
	return nil
}

// ----------------------------------------------------------------------------
// 实时事件流: 订阅全集群的 service / task / node / container 事件
//
// 说明: manager 上的 /events 会收到整个 swarm 的 task/service 状态流转,
// 这正是"部署在管理节点就能监控所有节点"的关键。worker 本机 die 事件
// 只在本机可见, 但会以 task failed/rejected 形式汇总到 manager。
// ----------------------------------------------------------------------------

func listenSwarmEvents() error {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", dockerSocket)
		},
	}
	client := &http.Client{Transport: tr}

	filters := `{"type":["service","task","node","container","network"],"event":["die","oom","fail","create","update","disconnect","remove","destroy"]}`
	reqURL := "http://unix/events?filters=" + url.QueryEscape(filters)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Println("已连接 Swarm events 流")

	decoder := json.NewDecoder(resp.Body)
	for {
		var event map[string]interface{}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				return fmt.Errorf("swarm stream closed")
			}
			return err
		}
		handleEvent(event)
	}
}

func handleEvent(event map[string]interface{}) {
	if cfg.debugEvents {
		b, _ := json.Marshal(event)
		fmt.Println("[event]", string(b))
	}

	typ, _ := event["Type"].(string)
	action, _ := event["Action"].(string)
	actor, _ := event["Actor"].(map[string]interface{})
	attr := map[string]interface{}{}
	if actor != nil {
		attr, _ = actor["Attributes"].(map[string]interface{})
	}
	name := strOf(attr, "name")
	if name == "" {
		name = strOf(actor, "ID")
	}

	switch typ {
	case "container":
		handleContainerEvent(action, attr, name)
	case "task":
		// task 事件只做触发器: 立刻全量 poll 一次定位失败 task,
		// 因为 event 本身字段不全 (没有 exitCode / Err)。
		if action == "update" || action == "fail" {
			go func() {
				// 稍等让 task Status 落库, 否则查到的是旧状态
				time.Sleep(3 * time.Second)
				if err := pollOnce(); err != nil {
					fmt.Println("[event-triggered poll] error:", err)
				}
			}()
		}
	case "node":
		// 节点 down / disconnect 立刻告警
		if action == "update" || action == "disconnect" || action == "remove" {
			text := fmt.Sprintf("🚨 Swarm节点事件\n节点: %s\n事件: %s %s\n时间: %s",
				name, typ, action, nowStr())
			if matchInclude(name) && alerts.allow("node-event:"+name+":"+action) {
				fmt.Println("[node event]", name, action)
				SendFeiShu(cfg.webhook, cfg.secret, text)
			}
		}
	case "service":
		if action == "update" || action == "remove" || action == "create" {
			fmt.Printf("[service event] %s %s\n", name, action)
			// 正常变更也通知 (ℹ️)，与故障告警 (🚨) 区分开:
			//   update = 发版/滚动更新/扩缩容/改配置，属正常操作
			//   create = 首次部署，属正常操作
			//   remove = 服务下线，需确认是否人为操作
			// 真正的故障 (副本不足/task失败) 会由 poll 的 checkServices/checkTasks 另行 🚨 告警。
			var reason string
			switch action {
			case "create":
				reason = "服务被创建(首次部署，正常操作)"
			case "remove":
				reason = "服务被删除/下线(请确认是否人为操作)"
			default:
				reason = "配置变更/发版/扩缩容/滚动更新(正常操作)"
			}
			text := fmt.Sprintf("ℹ️ Swarm服务变更通知(非故障)\n服务: %s\n事件: %s %s\n原因: %s\n时间: %s\n备注: 正常变更通知；若随后出现副本不足/任务异常会另行🚨告警",
				name, typ, action, reason, nowStr())
			if !matchInclude(name) {
				return
			}
			// 正常变更无冷却，每条 event 都推 (故障 🚨 告警仍有 ALERT_COOLDOWN 去重)
			if err := SendFeiShu(cfg.webhook, cfg.secret, text); err != nil {
				fmt.Println("[service event] Feishu send error:", err)
			}
		}
	}
}

// 兼容监控: manager 本机的普通容器 (infra 等非 swarm 容器) die 事件
func handleContainerEvent(action string, attr map[string]interface{}, name string) {
	if action != "die" && action != "oom" {
		return
	}
	// swarm task 容器名形如 stack_service.1.xxx, 同样走 task 链路告警,
	// 这里只处理非 swarm 容器, 避免重复告警。
	if _, isTask := attr["com.docker.swarm.task.id"]; isTask {
		return
	}
	if name == "" || !matchInclude(name) {
		// include 为空时 matchInclude=true, 即监控全部本机容器;
		// 若噪音大可设置 MONITOR_INCLUDE 过滤。
		if cfg.include != "" {
			return
		}
	}
	exitCode := strOf(attr, "exitCode")
	image := strOf(attr, "image")
	host := hostName()
	text := fmt.Sprintf("🚨 Docker容器退出(本机:%s)\n容器: %s\n镜像: %s\n事件: %s\n退出码: %s\n时间: %s",
		host, name, image, action, exitCode, nowStr())
	if alerts.allow("container:" + host + ":" + name + ":" + exitCode) {
		fmt.Printf("[container %s] %s exit=%s\n", action, name, exitCode)
		SendFeiShu(cfg.webhook, cfg.secret, text)
	}
}

// ----------------------------------------------------------------------------
// 全量轮询 (兜底 + 定时体检): nodes / services / tasks
// ----------------------------------------------------------------------------

func pollOnce() error {
	nodes, err := listNodes()
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}
	services, err := listServices()
	if err != nil {
		return fmt.Errorf("list services: %w", err)
	}
	tasks, err := listTasks()
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	nodeName := map[string]string{} // nodeID -> hostname
	for _, n := range nodes {
		nodeName[n.ID] = n.Description.Hostname
	}
	checkNodes(nodes)
	checkTasks(tasks, services, nodeName)
	checkServices(services, tasks)
	return nil
}

func listNodes() ([]swarmNode, error) {
	b, err := dockerGet("/nodes")
	if err != nil {
		return nil, err
	}
	var nodes []swarmNode
	if err := json.Unmarshal(b, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func listServices() ([]swarmService, error) {
	b, err := dockerGet("/services")
	if err != nil {
		return nil, err
	}
	var svcs []swarmService
	if err := json.Unmarshal(b, &svcs); err != nil {
		return nil, err
	}
	return svcs, nil
}

func listTasks() ([]swarmTask, error) {
	b, err := dockerGet("/tasks")
	if err != nil {
		return nil, err
	}
	var tasks []swarmTask
	if err := json.Unmarshal(b, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// 1) 节点检查
func checkNodes(nodes []swarmNode) {
	for _, n := range nodes {
		host := n.Description.Hostname
		if host == "" {
			host = n.ID[:12]
		}
		// down = 节点失联; Availability= drain/pause 也值得关注
		if n.Status.State != "ready" {
			text := fmt.Sprintf("🚨 Swarm节点异常\n节点: %s(%s)\n角色: %s\n状态: %s\n可用性: %s\n时间: %s",
				host, shortID(n.ID), n.Spec.Role, n.Status.State, n.Spec.Availability, nowStr())
			if alerts.allow("node-down:" + n.ID) {
				fmt.Println("[node down]", host, n.Status.State)
				SendFeiShu(cfg.webhook, cfg.secret, text)
			}
		}
	}
}

// 2) Task 检查: failed / rejected / 非0退出
func checkTasks(tasks []swarmTask, services []swarmService, nodeName map[string]string) {
	svcName := map[string]string{}
	for _, s := range services {
		svcName[s.ID] = s.Spec.Name
	}
	for _, t := range tasks {
		name := svcName[t.ServiceID]
		if name == "" {
			name = t.ServiceID
		}
		if !matchInclude(name) {
			continue
		}
		state := t.Status.State
		if state != "failed" && state != "rejected" && state != "orphaned" {
			continue
		}
		// 只告警新鲜的失败: swarm 会保留历史 task, 太旧的失败(如上次部署残留)
		// 反复告警只是噪音; 持续的故障会不断产生新 task, 照样能抓到。
		// 副本数不足的兜底由 checkServices 负责。
		if taskTooOld(t.Status.Timestamp) {
			continue
		}
		exitCode := -1
		if t.Status.ContainerStatus != nil {
			exitCode = t.Status.ContainerStatus.ExitCode
		}
		// rejected 通常是调度问题 (Err 有说明), exitCode 可能为空, 照样告警
		key := fmt.Sprintf("task:%s:%d:%s:%d", name, t.Slot, t.Status.Err, exitCode)
		if !alerts.allow(key) {
			continue
		}
		node := nodeName[t.NodeID]
		if node == "" {
			node = shortID(t.NodeID)
		}
		image := t.Spec.ContainerSpec.Image
		text := fmt.Sprintf(
			"🚨 Swarm任务异常\n服务: %s\nSlot: %d\n节点: %s\n状态: %s (期望: %s)\n退出码: %d\n错误: %s\n镜像: %s\n时间: %s",
			name, t.Slot, node, state, t.DesiredState, exitCode, t.Status.Err, image, nowStr(),
		)
		fmt.Printf("[task %s] %s.%d node=%s err=%s exit=%d\n", state, name, t.Slot, node, t.Status.Err, exitCode)
		SendFeiShu(cfg.webhook, cfg.secret, text)
	}
}

// 3) Service 检查: Running 副本不足
func checkServices(services []swarmService, tasks []swarmTask) {
	running := map[string]int{}
	starting := map[string]bool{}
	for _, t := range tasks {
		switch t.Status.State {
		case "running":
			running[t.ServiceID]++
		case "new", "allocated", "pending", "assigned", "accepted", "preparing", "ready", "starting":
			// 还有 task 处于启动中(刚部署/滚动更新), 此时副本不足是正常的
			starting[t.ServiceID] = true
		}
	}
	for _, s := range services {
		name := s.Spec.Name
		if !matchInclude(name) {
			continue
		}
		if s.Spec.Mode.Replicated == nil || s.Spec.Mode.Replicated.Replicas == nil {
			continue // global 模式跳过副本数检查
		}
		want := int(*s.Spec.Mode.Replicated.Replicas)
		got := running[s.ID]
		if got < want {
			if starting[s.ID] {
				fmt.Println("[service starting] ", name, got, "/", want)
				continue
			}
			if alerts.allow(fmt.Sprintf("svc-degraded:%s:%d/%d", name, got, want)) {
				text := fmt.Sprintf("🚨 Swarm服务副本不足\n服务: %s\n运行中: %d / 期望: %d\n时间: %s",
					name, got, want, nowStr())
				fmt.Println("[service degraded]", name, got, "/", want)
				SendFeiShu(cfg.webhook, cfg.secret, text)
			}
		}
	}
}

// ----------------------------------------------------------------------------
// 飞书推送 (加签模式)
// ----------------------------------------------------------------------------

func GenFeiShuSign(secret string, timestamp int64) (string, error) {
	// 官方文档算法 (自定义机器人-签名校验):
	//   key = "timestamp\nsecret", data = 空字符串, HMAC-SHA256 后 Base64。
	// 注意: 这和常规 key=secret 的用法不同, 按常规写法服务端回 19021。
	stringToSign := fmt.Sprintf("%v", timestamp) + "\n" + secret
	h := hmac.New(sha256.New, []byte(stringToSign))
	if _, err := h.Write([]byte{}); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func SendFeiShu(webhook, secret, text string) error {
	timestamp := time.Now().Unix()
	sign, err := GenFeiShuSign(secret, timestamp)
	if err != nil {
		return err
	}
	body := map[string]interface{}{
		"timestamp": fmt.Sprintf("%d", timestamp),
		"sign":      sign,
		"msg_type":  "text",
		"content": map[string]string{
			"text": text,
		},
	}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", webhook, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Feishu send failed:", err)
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	fmt.Println("Feishu response:", string(respBody))
	// 19021 = 加签失败或时间戳超1小时: 99% 是 webhook/secret 不配对，或宿主机时间不同步
	var fr struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &fr); err == nil && fr.Code != 0 {
		hint := ""
		if fr.Code == 19021 {
			hint = " (排查: 1.docker service inspect 看 FEISHU_WEBHOOK/FEISHU_SECRET 是否配对 2.宿主机执行 date/date -u 看时间是否偏差>1h 3.飞书后台是否开启了加签)"
		}
		return fmt.Errorf("feishu api code=%d msg=%s%s", fr.Code, fr.Msg, hint)
	}
	return nil
}

// ----------------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------------

func nowStr() string { return time.Now().Format("2006-01-02 15:04:05") }

// taskTooOld 判断 task 失败时间是否超出关注窗口, 解析失败时返回 false (照常告警)
func taskTooOld(ts string) bool {
	if ts == "" {
		return false
	}
	tm, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return false
	}
	return time.Since(tm) > cfg.taskMaxAge
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// webhookSuffix 只取 webhook 末尾几位打日志, 避免全地址泄露, 又能对照飞书后台确认群
func webhookSuffix(webhook string) string {
	if len(webhook) > 8 {
		return webhook[len(webhook)-8:]
	}
	return webhook
}

func strOf(m map[string]interface{}, k string) string {
	if m == nil {
		return ""
	}
	s, _ := m[k].(string)
	return s
}

func hostName() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
