package logic

var (
	allPlayerInfo = make(map[int64]map[int64]*Round) // <uid-RoundId,info>
)

type Round struct {
	ItemMap    map[int64]int64 // 获得道具
	MonsterIds []uint32        // 怪物id列表
}

// todo 内存保存,如果出现相同的uid则替换，覆盖
func UpdateRound(uid, RoundId int64, addMonsterIds []uint32) {
	if allPlayerInfo[uid] == nil {
		allPlayerInfo[uid] = make(map[int64]*Round)
	}
	if allPlayerInfo[uid][RoundId] == nil {
		allPlayerInfo[uid][RoundId] = &Round{
			ItemMap:    make(map[int64]int64),
			MonsterIds: make([]uint32, 0),
		}
	}
	if len(addMonsterIds) == 0 {
		return
	}

	for _, monsterId := range addMonsterIds {
		// 不同怪物有不同的钱
		print(monsterId)
		itemId := int64(0)
		itemValue := int64(0)
		allPlayerInfo[uid][RoundId].ItemMap[itemId] += itemValue
	}
	allPlayerInfo[uid][RoundId].MonsterIds = append(allPlayerInfo[uid][RoundId].MonsterIds, addMonsterIds...)
}

func GetRoundReward(uid, RoundId int64) {
	if allPlayerInfo[uid] == nil {
		return
	}

	round := allPlayerInfo[uid][RoundId]
	if round == nil {
		return
	}

	for _, monsterId := range round {
		// 不同怪物有不同的钱
		print(monsterId)
		addGold := int64(0)
		allPlayerInfo[uid][RoundId].goldValue += addGold
	}
	allPlayerInfo[uid][RoundId].MonsterIds = append(allPlayerInfo[uid][RoundId].MonsterIds, addMonsterIds...)
}

func DelPlayerInfo(uid int64) {
	delete(allPlayerInfo, uid)
}
