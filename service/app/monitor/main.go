package main

import (
	"bufio"
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
	"strconv"
	"strings"
	"time"
)

const (
	dockerSocket = "/var/run/docker.sock"
	webhook      = "https://open.feishu.cn/open-apis/bot/v2/hook/408051b8-7007-42c5-ba7c-35e630a34a99"
	secret       = "pFRIDCPGsx3EWKTqfLoWWc"
)

// https://open.feishu.cn/open-apis/bot/v2/hook/408051b8-7007-42c5-ba7c-35e630a34a99
// pFRIDCPGsx3EWKTqfLoWWc
func main() {
	fmt.Println("======================================")
	fmt.Println("   Docker HTTP Event Monitor Started  ")
	fmt.Println("======================================")

	for {
		err := listenDockerEvents()
		fmt.Println("Docker 连接断开，5秒后重连:", err)
		time.Sleep(5 * time.Second)
	}
}

func listenDockerEvents() error {

	// 创建一个使用 Unix Socket 的 HTTP Transport
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", dockerSocket)
		},
	}

	client := &http.Client{Transport: tr}

	// 构造 filters 参数
	filters := `{"event":["die"]}`
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

	fmt.Println("已连接 Docker events 流")

	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("docker stream closed")
			}
			return err
		}

		handleEvent(line)
	}
}

func handleEvent(data []byte) {

	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return
	}

	// 只处理 container 事件
	if event["Type"] != "container" {
		return
	}

	actor, ok := event["Actor"].(map[string]interface{})
	if !ok {
		return
	}

	attr, ok := actor["Attributes"].(map[string]interface{})
	if !ok {
		return
	}

	name, _ := attr["name"].(string)

	// 只监控名字包含 node 的容器
	if name == "" || !strings.Contains(name, "node") {
		return
	}

	exitCodeStr, _ := attr["exitCode"].(string)
	if exitCodeStr == "" {
		return
	}

	exitCode, err := strconv.Atoi(exitCodeStr)
	if err != nil {
		return
	}

	if exitCode == 0 {
		return
	}

	fmt.Printf("检测到异常容器退出: %s, code=%d\n", name, exitCode)

	text := fmt.Sprintf(
		"🚨 Docker容器异常退出\n容器: %s\n退出码: %d\n时间: %s",
		name,
		exitCode,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	SendFeiShu(webhook, secret, text)
}

// 飞书
/***
{
        "timestamp": "1599360473",        // 时间戳。
        "sign": "xxxxxxxxxxxxxxxxxxxxx",  // 得到的签名字符串。
        "msg_type": "text",
        "content": {
                "text": "request example"
        }
}
*/
func GenFeiShuSign(secret string, timestamp int64) (string, error) {
	//timestamp + key 做sha256, 再进行base64 encode
	stringToSign := fmt.Sprintf("%v", timestamp) + "\n" + secret

	var data []byte
	h := hmac.New(sha256.New, []byte(stringToSign))
	_, err := h.Write(data)
	if err != nil {
		return "", err
	}

	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	fmt.Println(signature)
	return signature, nil
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
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Println("Feishu response:", string(respBody))
	return nil
}
