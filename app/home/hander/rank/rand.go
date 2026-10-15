package rank

import (
	"sync"
)

var (
	key              = "rankGoldCoin" //道具表,KV, key:itemId
	rankServerManger = &RankServerManger{
		rankMap: make(map[string]*Rank),
	}
)

func GetRankServer(rankType string) *Rank {
	rankServerManger.lock.Lock()
	defer rankServerManger.lock.Unlock()
	if rankType == "" {
		rankType = key
	}
	rank := rankServerManger.rankMap[rankType]
	if rank == nil {
		rankServerManger.rankMap[rankType] = &Rank{
			rankType: rankType,
		}
	}
	return rankServerManger.rankMap[rankType]
}

type Rank struct {
	rankType     string //排行榜类型
	userInfoList []*userInfo
}

type userInfo struct {
	Uid   uint64
	score uint64
}

type RankServerManger struct {
	lock    sync.RWMutex //
	rankMap map[string]*Rank
}
