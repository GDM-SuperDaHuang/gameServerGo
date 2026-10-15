package selector

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"github.com/smallnest/rpcx/share"
)

// DefaultSelector 选择特定版本，如果没有选择最新的版本，（如果退出登录再次请求，回选择最新的版本,否则用户选择旧的版本）
type DefaultSelector struct {
	mu      sync.RWMutex //可能并发
	Servers []*serverInfo
}
type serverInfo struct {
	Id      uint32
	groupId uint32
	Address string

	maxVersion uint32
	curVersion uint32
	roomStatus uint8
	open       bool
}

// Select 随机选择一个服务器，由 rpcx 调用
func (s *DefaultSelector) Select(ctx context.Context, servicePath, serviceMethod string, _ /** args */ any) string {
	s.mu.RLock()         // 加读锁
	defer s.mu.RUnlock() // 方法退出时释放

	if len(s.Servers) == 0 {
		return ""
	}
	m, ok := ctx.Value(share.ResMetaDataKey).(map[string]string)
	if !ok {
		return ""
	}
	// 选择一台目标机器
	oneServer := getServerInfo(m)
	if oneServer.groupId == 1 && oneServer.Id == 0 { //远程到来的，空网关直接返回
		return ""
	}
	address := ""
	for _, server := range s.Servers {
		if oneServer.Id == 0 { //没有数据,则按平均分配服务器
			if server.groupId == oneServer.groupId && server.curVersion == server.maxVersion { //没有则返回最大版本
				serverList := make([]*serverInfo, 0) //相同组的所有版本
				for _, tempServer := range s.Servers {
					if server.groupId == tempServer.groupId && tempServer.curVersion == tempServer.maxVersion {
						serverList = append(serverList, tempServer)
					}
				}
				// 从 serverList 选择一个
				targetServer := serverList[rand.Intn(len(serverList))]
				return targetServer.Address
			}
		} else {
			if server.Id == oneServer.Id {
				return server.Address
			}
		}

	}

	return address
}

// UpdateServer 更新服务器列表，由 rpcx 调用
func (s *DefaultSelector) UpdateServer(servers map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(servers) == 0 {
		fmt.Println("Servers is empty")
		return
	}

	// 更新版本
	s.Servers = make([]*serverInfo, 0)
	groupIdMaxVersionMap := make(map[uint32]uint32) //各个服务的最大版本
	for address, metadata := range servers {
		serverMetadata := parseServerMetadata(metadata, address)
		if groupIdMaxVersionMap[serverMetadata.groupId] <= serverMetadata.curVersion {
			groupIdMaxVersionMap[serverMetadata.groupId] = serverMetadata.curVersion
		}
		// 排重
		flag := false
		for _, info := range s.Servers {
			if info.Id == serverMetadata.Id {
				flag = true
				break
			}
		}
		if !flag {
			s.Servers = append(s.Servers, serverMetadata)
		}
	}
	for _, info := range s.Servers { //todo？？？
		info.maxVersion = groupIdMaxVersionMap[info.groupId]
	}
}

func getServerInfo(metadata map[string]string) *serverInfo {
	var (
		id, _         = strconv.Atoi(metadata["id"])
		curVersion, _ = strconv.Atoi(metadata["curVersion"])
		groupId, _    = strconv.Atoi(metadata["groupId"])
		roomStatus, _ = strconv.Atoi(metadata["roomStatus"])
	)

	return &serverInfo{
		Id:         uint32(id),
		curVersion: uint32(curVersion),
		groupId:    uint32(groupId), //？？？
		roomStatus: uint8(roomStatus),
	}
}

func parseServerMetadata(metadata, address string) *serverInfo {
	out := &serverInfo{
		Address: address,
	}

	l1 := strings.Split(metadata, "&")
	for _, v := range l1 {
		key, value, _ := strings.Cut(v, "=")
		switch key {
		case "id":
			t, _ := strconv.Atoi(value)
			out.Id = uint32(t)
		case "groupId":
			t, _ := strconv.Atoi(value)
			out.groupId = uint32(t)
		case "version":
			t, _ := strconv.Atoi(value)
			out.curVersion = uint32(t)
		case "roomStatus":
			t, _ := strconv.Atoi(value)
			out.roomStatus = uint8(t)
		}
	}

	return out
}
