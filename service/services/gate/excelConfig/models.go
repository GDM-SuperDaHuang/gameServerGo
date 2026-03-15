package excelConfig

import (
	"gameServer/common/config"
	"gameServer/pkg/excel/reader"
)

// 注册
var (
	characterList    = []*config.Heros{}
	receiveAwardList = []*config.ReceiveAward{}

	allStructMap = map[string]interface{}{
		"heros":        &characterList,
		"receiveAward": &receiveAwardList,
	}
)

func GetAllReceiveAwardConfig() []*config.ReceiveAward {
	infos, ok := allStructMap["receiveAward"]
	if !ok {
		return nil
	}
	prt := infos.(*[]*config.ReceiveAward)
	ls := *prt
	return ls
}

func GetInitLoginConfigReward() (map[int]uint64, []int) {
	initLoginInfo, ok := allStructMap["receiveAward"]
	if !ok {
		return nil, nil
	}
	loginsPtr := initLoginInfo.(*[]*config.ReceiveAward)
	logins := *loginsPtr // 解引用获取切片本身
	rewardMap := make(map[int]uint64)
	idList := make([]int, 0)

	for _, info := range logins {
		if info.RewardType != 1 {
			continue
		}
		idList = append(idList, int(info.Id))
		for k, v := range info.RewardMap {
			rewardMap[k] = v
		}
	}
	return rewardMap, idList
}

func GetInitCharacterListReward() []uint32 {
	initLoginInfo, ok := allStructMap["heros"]
	if !ok {
		return nil
	}
	ptr := initLoginInfo.(*[]*config.Heros)
	infos := *ptr

	idList := make([]uint32, 0)
	for _, info := range infos {
		if info.Lock {
			continue
		}
		idList = append(idList, info.Id)
	}
	return idList
}

// 初始化配置表配置
func init() {
	// 创建读取器（指向excels目录）
	r := reader.NewExcelReader("./excels")
	allData, err := r.ReadAllExcels()
	if err != nil || len(allData) == 0 {
		panic(err)
	}
	err = r.ReadSheetToStruct(allData, allStructMap)
	if err != nil {
		panic(err)
	}
	// 初始化redis
	//redis.NewRedisClient(config2.Get().RedisAddress())
	//items.Listening()
	//heros.Listening()
}
