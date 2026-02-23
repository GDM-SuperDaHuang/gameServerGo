package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	//"github.com/docker/docker/api/types"
	//"github.com/docker/docker/api/types/events"
	//"github.com/docker/docker/client"
)

const (
	dingToken  = "你的钉钉access_token"
	dingSecret = "你的钉钉secret"
)

func main() {
	// Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("docker client error: %v", err)
	}

	ctx := context.Background()
	msgFilter := types.EventsOptions{
		Filters: client.NewListOptions(),
	}

	// 监听 Docker 事件
	eventOptions := client.EventsOptions{
		Filters: msgFilter.Filters,
	}

	messages, errs := cli.Events(ctx, eventOptions)

	log.Println("Monitor started, listening for container die events...")

	// 容器名称正则
	containerPattern := regexp.MustCompile(`node-(room|gate)-\d+`)

	for {
		select {
		case event := <-messages:
			if event.Type == "container" && event.Action == "die" {
				name := trimSlash(event.Actor.Attributes["name"])
				if containerPattern.MatchString(name) {
					exitCode := event.Actor.Attributes["exitCode"]
					if exitCode != "0" {
						sendDingMsg(name, exitCode)
					}
				}
			}
		case err := <-errs:
			if err != nil {
				log.Printf("Docker event error: %v", err)
				time.Sleep(time.Second)
			}
		}
	}
}

// 去掉前后的 /
func trimSlash(s string) string {
	return s
}

// 发送钉钉消息
func sendDingMsg(container, exitCode string) {
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	// 构造签名
	stringToSign := fmt.Sprintf("%s\n%s", timestamp, dingSecret)
	h := hmac.New(sha256.New, []byte(dingSecret))
	h.Write([]byte(stringToSign))
	signData := h.Sum(nil)
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(signData))

	webhook := fmt.Sprintf(
		"https://oapi.dingtalk.com/robot/send?access_token=%s&timestamp=%s&sign=%s",
		dingToken, timestamp, sign,
	)

	msg := fmt.Sprintf("【服务器异常】容器: %s 退出码: %s", container, exitCode)

	jsonData := fmt.Sprintf(`{"msgtype":"text","text":{"content":"%s"}}`, msg)
	resp, err := http.Post(webhook, "application/json", strings.NewReader(jsonData))
	if err != nil {
		log.Printf("Send dingding message failed: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("Alarm sent for container %s exitCode %s", container, exitCode)
}
