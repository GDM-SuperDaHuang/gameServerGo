package selector

import (
	"context"
	"fmt"
	"github.com/smallnest/rpcx/share"
	"strconv"
	"strings"
)

// DefaultSelector 用户回选择旧的版本，如果退出登录再次请求，回选择最新的版本
type DefaultSelector struct {
	servers []*serverInfo
}
type serverInfo struct {
	id      uint32
	groupId uint32
	address string

	maxVersion uint32
	curVersion uint32
	roomStatus uint8
}

// NewRandomSelector 创建随机选择器
//func NewDefaultSelector() *DefaultSelector {
//	return &DefaultSelector{
//		servers: make([]*serverInfo, 0),
//	}
//}

// Select 随机选择一个服务器，由 rpcx 调用
func (s *DefaultSelector) Select(ctx context.Context, servicePath, serviceMethod string, _ /** args */ any) string {
	if len(s.servers) == 0 {
		return ""
	}
	m, ok := ctx.Value(share.ResMetaDataKey).(map[string]string)
	//id, _ := ctx.Value("Id").(uint32)
	//m, ok := ctx.Value(share.ReqMetaDataKey).(map[string]string)
	if !ok {
		return ""
	}
	// 选择一台目标机器
	oneServer := getServerInfo(m)
	address := ""
	// 如果是房间类型，未结束之前选择的都是旧服务器，
	//if servicePath == "room" {
	//	if oneServer.RoomStatus == 1 {
	//		return address
	//	} else { //返回最新版本
	//		return address
	//	}
	//}
	//version := uint32(0)
	for _, server := range s.servers {
		// 假设 game-1 存在两个版本的进程
		// v1: 线上版本
		// v2: 新开发功能，发布前测试版本
		//
		// 当前登录的账号设置了白名单，必须进入 v2
		//
		// 此时，versionMin = 2, versionMax = 2
		// 就会匹配到 v2 版本
		//if server.id == metadata.id &&
		//	(metadata.versionMax == 0 || server.version >= metadata.versionMin && server.version <= metadata.versionMax) {
		//	return server.address
		//}
		if server.id == oneServer.id {
			return server.address
		}
	}

	return address
}

// UpdateServer 更新服务器列表，由 rpcx 调用
func (s *DefaultSelector) UpdateServer(servers map[string]string) {
	if len(servers) == 0 {
		fmt.Println("servers is empty")
		return
	}
	s.servers = make([]*serverInfo, 0)
	for address, metadata := range servers {
		s.servers = append(s.servers, parseServerMetadata(metadata, address))
	}
}

func getServerInfo(metadata map[string]string) *serverInfo {
	var (
		id, _         = strconv.Atoi(metadata["id"])
		versionMin, _ = strconv.Atoi(metadata["versionMin"])
		versionMax, _ = strconv.Atoi(metadata["versionMax"])
		roomStatus, _ = strconv.Atoi(metadata["roomStatus"])
	)

	// todo 对象池优化
	//v := clientMetadataPool.Get()
	return &serverInfo{
		id:         uint32(id),
		curVersion: uint32(versionMin),
		maxVersion: uint32(versionMax),
		roomStatus: uint8(roomStatus),
	}
}

func parseServerMetadata(metadata, address string) *serverInfo {
	out := &serverInfo{
		address: address,
	}

	l1 := strings.Split(metadata, "&")
	for _, v := range l1 {
		key, value, _ := strings.Cut(v, "=")
		switch key {
		case "id":
			t, _ := strconv.Atoi(value)
			out.id = uint32(t)
		case "groupId":
			t, _ := strconv.Atoi(value)
			out.groupId = uint32(t)
		case "maxVersion":
			t, _ := strconv.Atoi(value)
			out.curVersion = uint32(t)
		case "curVersion":
			t, _ := strconv.Atoi(value)
			out.groupId = uint32(t)
		case "roomStatus":
			t, _ := strconv.Atoi(value)
			out.groupId = uint32(t)
		}
	}

	return out
}
