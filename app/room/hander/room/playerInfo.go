package room

import (
	"gameServer/service/common"
	"sync"
)

var (
	playerStatusNormal uint8 = 0 // 离开房间
	playerStatusLeave  uint8 = 1 // 离开房间
)

// 玩家信息
type PlayerInfo struct {
	//UserId     uint64 //
	heroId     uint32
	playerType uint8 //0:真实玩家，>0:机器人
	status     uint8 // 0: 正常，1：已经离开

	itemMap map[uint64]uint64 //选择的道具

	player *common.Player //网关
}

// 玩家信息
type PlayerInfos struct {
	lock        sync.RWMutex //
	playerInfos []*PlayerInfo
}

func (p *PlayerInfos) len() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return len(p.playerInfos)
}

// 防止多人读出来的值是相同的
func (p *PlayerInfos) isFull(max int) bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	if len(p.playerInfos) >= max {
		return true
	}
	return false
}

// 添加一个 玩家
func (p *PlayerInfos) AddPlayerInfo(playerInfo *PlayerInfo) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.addPlayerInfo0(playerInfo)
}

// 添加一个 玩家 无锁
func (p *PlayerInfos) addPlayerInfo0(playerInfo *PlayerInfo) {
	// 移除旧的，替换新的
	for _, info := range p.playerInfos {
		if info.player.UserId == playerInfo.player.UserId {
			info = playerInfo
			return
		}
	}
	// 不然添加新的
	p.playerInfos = append(p.playerInfos, playerInfo)
}

func (p *PlayerInfos) removePlayerInfo(i int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.playerInfos = removeIndex(p.playerInfos, i)
}

//type players struct {
//	lock    sync.RWMutex     //
//	players []*common.Player //网关
//}
//
//func (p *players) addPlayer(player *common.Player) {
//	p.lock.Lock()
//	defer p.lock.Unlock()
//	p.players = append(p.players, player)
//}
//func (p *players) removePlayer(i int) {
//	p.lock.Lock()
//	defer p.lock.Unlock()
//	p.players = removeIndex2(p.players, i)
//}
