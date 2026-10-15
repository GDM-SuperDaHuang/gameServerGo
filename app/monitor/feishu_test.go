package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"
)

// 纯本地单测: 验证加签算法本身正确 (不联网, 默认 go test 就会跑)
func TestGenFeiShuSign(t *testing.T) {
	secret := "test-secret-123"
	ts := int64(1760080000)

	s1, err := GenFeiShuSign(secret, ts)
	if err != nil {
		t.Fatalf("GenFeiShuSign error: %v", err)
	}
	if s1 == "" {
		t.Fatal("sign 为空")
	}

	// 确定性: 同输入必同输出
	s2, _ := GenFeiShuSign(secret, ts)
	if s1 != s2 {
		t.Fatalf("同一输入签名不一致: %q vs %q", s1, s2)
	}

	// 敏感性: 换 timestamp / 换 secret 必须变 (否则加签无意义)
	s3, _ := GenFeiShuSign(secret, ts+1)
	if s1 == s3 {
		t.Fatal("timestamp 变化但签名没变, 加签逻辑异常")
	}
	s4, _ := GenFeiShuSign(secret+"x", ts)
	if s1 == s4 {
		t.Fatal("secret 变化但签名没变, 加签逻辑异常")
	}

	// 与官方文档算法交叉验证 (key="timestamp\nsecret", data=空, HMAC-SHA256+Base64)
	mac := hmac.New(sha256.New, []byte(fmt.Sprintf("%d\n%s", ts, secret)))
	mac.Write([]byte{})
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if s1 != want {
		t.Fatalf("算法与飞书文档不一致:\n got=%q\nwant=%q", s1, want)
	}
}

// 实链路测试: 真发一条消息到飞书群, 验证 webhook/secret 配对 + 时间戳有效
// 真实联网, 直接跑即可:
//
//	go test ./app/monitor/ -run TestSendFeiShuLive -v -count=1
//
// 如需覆盖默认配置: $env:FEISHU_WEBHOOK="..."; $env:FEISHU_SECRET="..."
// CI 如不想实发, 设 FEISHU_TEST_SEND=0 跳过。
func TestSendFeiShuLive(t *testing.T) {
	if os.Getenv("FEISHU_TEST_SEND") == "0" {
		t.Skip("FEISHU_TEST_SEND=0, 跳过实发测试")
	}

	webhook := getenv("FEISHU_WEBHOOK", cfg.webhook)
	secret := getenv("FEISHU_SECRET", cfg.secret)
	if webhook == "" || secret == "" {
		t.Fatal("webhook/secret 为空, 请检查 FEISHU_WEBHOOK/FEISHU_SECRET")
	}

	// 先校验本机时间, 飞书要求 timestamp 与服务器差 <1h, 否则必报 19021
	now := time.Now()
	t.Logf("本机时间: %s (unix=%d)", now.Format("2006-01-02 15:04:05"), now.Unix())
	t.Logf("webhook=%s...%s", webhook[:min(40, len(webhook))], webhook[max(0, len(webhook)-8):])

	text := fmt.Sprintf("✅ monitor飞书链路测试(非故障)\n时间: %s\n用途: 验证 webhook/secret 配对有效\n看到此条=链路正常",
		now.Format("2006-01-02 15:04:05"))
	if err := SendFeiShu(webhook, secret, text); err != nil {
		t.Fatalf("飞书实发失败: %v", err)
	}
	t.Log("飞书实发成功, 请去群里确认收到测试消息")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
