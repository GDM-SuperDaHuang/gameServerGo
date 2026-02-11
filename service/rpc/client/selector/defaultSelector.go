package selector

import (
	"context"
	"strconv"

	"github.com/smallnest/rpcx/share"
)

// DefaultSelector 用户回选择旧的版本，如果退出登录再次请求，回选择最新的版本
type DefaultSelector struct {
	servers []serverInfo
}
type serverInfo struct {
	Id      uint32
	Version uint32
	Address string
}

// NewRandomSelector 创建随机选择器
func NewDefaultSelector() *DefaultSelector {
	return &DefaultSelector{
		servers: make([]serverInfo, 0),
	}
}

// Select 随机选择一个服务器，由 rpcx 调用
func (s *DefaultSelector) Select(ctx context.Context, _ /** servicePath */, _ /** serviceMethod */ string, _ /** args */ any) string {
	if len(s.servers) == 0 {
		return ""
	}
	oneServer := GetServerInfo(ctx.Value(share.ResMetaDataKey).(map[string]string))
	if oneServer == nil {
		return ""
	}
	address := ""
	version := uint32(0)
	for _, server := range s.servers {
		// 假设 game-1 存在两个版本的进程
		// v1: 线上版本
		// v2: 新开发功能，发布前测试版本
		//
		// 当前登录的账号设置了白名单，必须进入 v2
		//
		// 此时，versionMin = 2, versionMax = 2
		// 就会匹配到 v2 版本
		// if server.id == metadata.id &&
		// 	(metadata.versionMax == 0 || server.version >= metadata.versionMin && server.version <= metadata.versionMax) {
		// 	return server.address
		// }
		if server.Id == oneServer.Id {
			return server.Address
		}
		if server.Version > version {
			address = server.Address
			server.Version = version
		}
	}
	return address
}

// UpdateServer 更新服务器列表，由 rpcx 调用
func (s *DefaultSelector) UpdateServer(servers map[string]string) {
	s.servers = make([]serverInfo, 0)
	for address := range servers {
		s.servers = append(s.servers, serverInfo{})
	}
}

func GetServerInfo(metadata map[string]string) *serverInfo {
	var (
		id, _         = strconv.Atoi(metadata["id"])
		versionMin, _ = strconv.Atoi(metadata["versionMin"])
		versionMax, _ = strconv.Atoi(metadata["versionMax"])
	)

	v := clientMetadataPool.Get()
	v.id = uint32(id)
	v.versionMin = uint32(versionMin)
	v.versionMax = uint32(versionMax)
	return v
}
