package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// mockServer 模拟 D:\Work\gameServerGo\app\serverlist\main.go 的行为，
// 用于本地单测（不需要启动真实 serverlist / etcd）。
func mockServer(t *testing.T, gateCode int, gateAddr string, gateVersion int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"time":   time.Now().Unix(),
		})
	})
	mux.HandleFunc("/service/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// 真实服务只有 path == "Gate" 时才回包，其他 path 直接 200 空 body（服务端 bug）
		if r.URL.Path == "/service/Gate" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    gateCode,
				"addr":    gateAddr,
				"version": gateVersion,
			})
			return
		}
	})
	return httptest.NewServer(mux)
}

func TestCheckHealth(t *testing.T) {
	srv := mockServer(t, 0, "192.168.1.102:17002", 1)
	defer srv.Close()

	h, err := CheckHealth(srv.URL)
	if err != nil {
		t.Fatalf("CheckHealth 失败: %v", err)
	}
	if h.Status != "ok" {
		t.Fatalf("期望 status=ok, 实际=%q", h.Status)
	}
	if h.Time <= 0 {
		t.Fatalf("期望 time>0, 实际=%d", h.Time)
	}
	t.Logf("health 响应: %+v", *h)
}

func TestGetGate(t *testing.T) {
	srv := mockServer(t, 0, "192.168.1.102:17002", 2)
	defer srv.Close()

	g, err := GetGate(srv.URL)
	if err != nil {
		t.Fatalf("GetGate 失败: %v", err)
	}
	if g.Code != 0 {
		t.Fatalf("期望 code=0, 实际=%d", g.Code)
	}
	if g.Addr != "192.168.1.102:17002" {
		t.Fatalf("期望 addr=192.168.1.102:17002, 实际=%q", g.Addr)
	}
	if g.Version != 2 {
		t.Fatalf("期望 version=2, 实际=%d", g.Version)
	}
	t.Logf("Gate 响应: %+v", *g)
}

func TestGetGate_EmptyAddr(t *testing.T) {
	// 模拟 GateList 为空 / GateMap 未配置时服务端返回 code=500 addr=""
	srv := mockServer(t, 500, "", 0)
	defer srv.Close()

	g, err := GetGate(srv.URL)
	if err != nil {
		t.Fatalf("GetGate 失败: %v", err)
	}
	if g.Code != 500 {
		t.Fatalf("期望 code=500, 实际=%d", g.Code)
	}
	if g.Addr != "" {
		t.Fatalf("期望 addr 为空, 实际=%q", g.Addr)
	}
}

// TestAgainstRealServer 集成测试：请求真实运行的 serverlist。
// 默认地址 http://127.0.0.1:18181，可用环境变量 SERVERLIST_URL 覆盖。
// 真实服务未启动时自动 skip，不影响 CI。
func TestAgainstRealServer(t *testing.T) {
	base := os.Getenv("SERVERLIST_URL")
	if base == "" {
		base = DefaultServerListURL
	}

	h, err := CheckHealth(base)
	if err != nil {
		t.Skipf("真实 serverlist 不可达 (%s): %v，先启动 gameServerGo/app/serverlist 再跑集成测试", base, err)
	}
	t.Logf("真实 /health: %+v", *h)
	if h.Status != "ok" {
		t.Errorf("真实 /health status 期望 ok, 实际 %q", h.Status)
	}

	g, err := GetGate(base)
	if err != nil {
		t.Fatalf("真实 /service/Gate 请求失败: %v", err)
	}
	t.Logf("真实 /service/Gate: code=%d addr=%s version=%d", g.Code, g.Addr, g.Version)
	if g.Code != 0 || g.Addr == "" {
		t.Logf("提示: code=%d addr=%q，说明服务端 GateList 为空或 GateMap 未配置（检查 etcd + serverlist.toml gateMap）", g.Code, g.Addr)
	}
}
