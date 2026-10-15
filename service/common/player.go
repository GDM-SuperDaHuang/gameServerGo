package common

import (
	"gameServer/pkg/bytes"
)

type Player struct {
	ServerIds []uint32 //玩家所链接的服务器,网关编号在1~999

	accountID    uint64
	realServerID uint32 // 角色当前所在的区服 id
	serverID     uint32 // 角色归属区服 id
	Uid          int64  // 角色 id
}

var PlayerPool = bytes.NewPool(func() *Player {
	return &Player{}
})

// Reset 重置
func (p *Player) Reset() {
	p.ServerIds = nil
	p.accountID = 0
	p.realServerID = 0
	p.serverID = 0
	p.Uid = 0
}

// 释放消息
func FreePlayer(p *Player) {
	PlayerPool.Put(p)
}
