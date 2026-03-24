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

// NewRandomSelector 创建随机选择器
//func NewDefaultSelector() *DefaultSelector {
//	return &DefaultSelector{
//		Servers: make([]*serverInfo, 0),
//	}
//}

// Select 随机选择一个服务器，由 rpcx 调用
func (s *DefaultSelector) Select(ctx context.Context, servicePath, serviceMethod string, _ /** args */ any) string {

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
		// 假设 game-1 存在两个版本的进程
		// v1: 线上版本
		// v2: 新开发功能，发布前测试版本
		//
		// 当前登录的账号设置了白名单，必须进入 v2
		//
		// 此时，versionMin = 2, versionMax = 2
		// 就会匹配到 v2 版本
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
				// 写入返回数据
				//ctx = context.WithValue(ctx, share.ResMetaDataKey, map[string]string{
				//	"Id": strconv.Itoa(int(targetServer.Id)),
				//})
				return targetServer.Address
			}
		} else {
			if server.Id == oneServer.Id {
				//log2.Get().Info("version======", zap.Uint32("curVersion:", server.curVersion))
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

		//versionMax, _ = strconv.Atoi(metadata["versionMax"])
		roomStatus, _ = strconv.Atoi(metadata["roomStatus"])
	)

	// todo 对象池优化
	//v := clientMetadataPool.Get()
	return &serverInfo{
		Id:         uint32(id),
		curVersion: uint32(curVersion),
		groupId:    uint32(groupId), //？？？
		//maxVersion: uint32(versionMax),
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
		//case "maxVersion":
		//	t, _ := strconv.Atoi(value)
		//	out.curVersion = uint32(t)
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
