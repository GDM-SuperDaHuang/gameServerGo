package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// 默认 serverlist 地址，对应 gameServerGo/config/serverlist.toml 中的 address = ":18181"
const DefaultServerListURL = "http://127.0.0.1:18181"

// GateResp 对应 serverlist/main.go handleGetService 返回:
// {"code":0, "addr":"...", "version":...}
type GateResp struct {
	Code    int    `json:"code"`
	Addr    string `json:"addr"`
	Version int    `json:"version"`
}

// HealthResp 对应 serverlist/main.go handleHealth 返回:
// {"status":"ok", "time":...}
type HealthResp struct {
	Status string `json:"status"`
	Time   int64  `json:"time"`
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

// CheckHealth GET /health
func CheckHealth(baseURL string) (*HealthResp, error) {
	resp, err := newHTTPClient().Get(baseURL + "/health")
	if err != nil {
		return nil, fmt.Errorf("请求 /health 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("/health 非200: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var h HealthResp
	if err := json.Unmarshal(body, &h); err != nil {
		return nil, fmt.Errorf("解析 /health 响应失败: %w, body=%s", err, string(body))
	}
	return &h, nil
}

// GetGate GET /service/Gate
func GetGate(baseURL string) (*GateResp, error) {
	resp, err := newHTTPClient().Get(baseURL + "/service/smash1")
	if err != nil {
		return nil, fmt.Errorf("请求 /service/XXX 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("/service/Gate 非200: %d body=%s", resp.StatusCode, string(body))
	}
	var g GateResp
	if err := json.Unmarshal(body, &g); err != nil {
		return nil, fmt.Errorf("解析 /service/Gate 响应失败: %w, body=%s", err, string(body))
	}
	return &g, nil
}

func main() {

	fmt.Printf("serverlist 地址: %s\n", DefaultServerListURL)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("请选择: 1=获取Gate  2=健康检查  q=退出")
		fmt.Print("> ")
		if !scanner.Scan() {
			fmt.Println("\n输入关闭，退出")
			return
		}
		switch strings.TrimSpace(scanner.Text()) {
		case "1":
			g, err := GetGate(DefaultServerListURL)
			if err != nil {
				fmt.Println("获取 Gate 失败:", err)
				continue
			}
			fmt.Printf("Gate 响应: code=%d addr=%s version=%d\n", g.Code, g.Addr, g.Version)
			if g.Code != 0 || g.Addr == "" {
				fmt.Println("注意: code!=0 或 addr 为空，说明 serverlist 的 GateList 为空 / GateMap 未配置")
			}
		case "2":
			h, err := CheckHealth(DefaultServerListURL)
			if err != nil {
				fmt.Println("health 检查失败:", err)
				continue
			}
			fmt.Printf("health OK: %+v\n", *h)
		case "q", "Q", "quit", "exit", "0":
			fmt.Println("退出")
			return
		default:
			fmt.Println("无效输入，请输入 1 / 2 / q")
		}
	}
}
