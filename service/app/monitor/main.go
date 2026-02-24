package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"net/http"
	"strings"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/client"
)

func main() {
	cli, _ := client.NewClientWithOpts(client.FromEnv)
	ctx := context.Background()

	f := filters.NewArgs()
	f.Add("event", "die")

	messages, _ := cli.Events(ctx, types.EventsOptions{Filters: f})

	for msg := range messages {
		var m map[string]interface{}
		json.Unmarshal(msg, &m)

		name := m["Actor"].(map[string]interface{})["Attributes"].(map[string]interface{})["name"].(string)
		if name != "" && strings.Contains(name, "node") {
			exitCode := m["Actor"].(map[string]interface{})["Attributes"].(map[string]interface{})["exitCode"].(float64)
			if exitCode != 0 {
				sendDingTalk(name, int(exitCode))
			}
		}
	}
}

func sendDingTalk(container string, code int) {
	webhook := "https://oapi.dingtalk.com/robot/send?access_token=xxxx"
	data := fmt.Sprintf(`{"msgtype":"text","text":{"content":"容器:%s 退出码:%d"}}`, container, code)
	http.Post(webhook, "application/json", bytes.NewBuffer([]byte(data)))
}
