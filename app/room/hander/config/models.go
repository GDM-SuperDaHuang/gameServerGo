package config

import (
	"errors"
	"gameServer/common/config"
	"gameServer/common/constValue"
	"sort"
	"sync"
)

// 能力配置表
type Ability struct {
	Id            uint32 `excel:"id"`            //能力id
	ShowItemCount int8   `excel:"showItemCount"` //0:为所有的数量,n:具体数量，1,2,3，4
	ShowValue     int8   `excel:"showValue"`     //0：不显示价值 1:总价值 2:平均价值，3:某物品数量
	ShowGridCount int8   `excel:"showGridCount"` //提示物品占用格子数量；显示类:0:不显示 1:平均格子数量，2：所有的格子数量
	ShowQuality   bool   `excel:"showQuality"`   //是否显示品质,显示类：0：不显示1：显示
	ShowContour   bool   `excel:"showContour"`   //是否显示轮廓，显示类,0:不显示,1：显
	ShowAll       bool   `excel:"showAll"`       //是否显示所有，显示类,0:不显示,1：显

	CArea          int8   `excel:"cArea"`          // 条件类,格子面积:-1:最大格子，0：随机,n:具体数量的格子
	CQualityType   int8   `excel:"cQualityType"`   // 条件类：-3：最高品质,-2: 所有品质,0:随机品质,1：白色,2：绿色,3,4,5,6
	CItemType      int8   `excel:"cItemType"`      // 物品类型，0：随机类型，n：具体道具类型，如，1,2,3,4
	CItemTypeOr    []int8 `excel:"cItemTypeOr"`    // 或关系，物品类型，0：随机类型，n：具体道具类型，如，1,2,3,4
	CItemTypeCount int8   `excel:"cItemTypeCount"` // -1:类型最少数量,0：不参与，随机类型，
	CPrice         int8   `excel:"cPriceCount"`    // 条件类 -1:价值最高的,0：不参与，随机类型，
	CPrePQ         int8   `excel:"cPrePQ"`         // 条件类 0：不限制 1:已经暴露品质的条件下 ,2:已经暴露轮廓的条件下
	Desc           string `excel:"desc"`           // 描述

}

// 房间配置
type Room struct {
	RoomType          uint32           `excel:"roomType"` //房间类型
	Consume           map[int]int64    `excel:"consume"`  // 消耗
	Limit             map[uint64]int64 `excel:"limit"`    // 限制
	ItemAllocationMap map[int]int      `excel:"itemAllocationMap"`

	CapacityLimit    int     `excel:"capacityLimit"`    // 人数限制
	Timeout          int     `excel:"timeout"`          // 操作超时
	RoundLimit       uint8   `excel:"roundLimit"`       // 最大局数
	EarlyTermination []int   `excel:"earlyTermination"` // 提取结束
	RobotList        []uint8 `excel:"robotList"`        // 机器人id list
}

type Robot struct {
	RobotType     uint8       `excel:"robotType"`     //机器人类型
	CharacterList []int       `excel:"characterList"` //人物
	JoinTimes     []int       `excel:"joinTimes"`     //加入时间
	NickNames     []string    `excel:"nickNames"`     //机器人名称
	ItemWeight    map[int]int `excel:"itemWeight"`    //道具权重
}

type RobotBet struct {
	Id          int           `excel:"id"`          //id
	IncomeRate  int           `excel:"incomeRate"`  //人物
	LossTime    int           `excel:"lossTime"`    //流失时间
	KList       []int64       `excel:"kList"`       //每轮k
	ItemWeight  map[int]int64 `excel:"itemWeight"`  //道具权重
	AbstainRate int           `excel:"abstainRate"` //弃权
}

type RobotBetF struct {
	Id          int           //id
	IncomeRate  int           //收益
	LossTime    int           //流失时间
	KList       []*K          //每轮k
	ItemWeight  map[int]int64 //道具权重
	AbstainRate int           //弃权
}

type K struct {
	Lower int64
	Upper int64
}

type RoomAllocation struct {
	Id            int         `excel:"id"`            //机器人类型
	ItemWeightMap map[int]int `excel:"itemWeightMap"` //
	SumWeightMap  map[int]int `excel:"sumWeightMap"`  //
	EntryMap1     map[int]int `excel:"entryMap1"`     // 词条
	EntryMap2     map[int]int `excel:"entryMap2"`     // 词条
	EntrySum2     []int       `excel:"entrySum2"`     // 数量
	IsEntry2      bool        `excel:"isEntry2"`      // 是否
}

type RobotGlobalParameters struct {
	TopN      int64   `excel:"topN"`      //记录数
	TimeSpan  int64   `excel:"timeSpan"`  //间隔
	ThinkTime []int64 `excel:"thinkTime"` //思考系数
	MatchTime []int64 `excel:"matchTime"` //匹配时间
	RoomValue []int   `excel:"roomValue"` //房价价值
}

type Entry1 struct {
	Id       int          `excel:"id"`       //id
	EntryMap map[int8]int `excel:"entryMap"` //词条
}
type Entry2 struct {
	Id       int         `excel:"id"`       //id
	EntryMap map[int]int `excel:"entryMap"` //词条
}

// 机器人道具
type RobotItem struct {
	Id         int         `excel:"id"`         //id
	ItemWeight map[int]int `excel:"itemWeight"` //机器人道具
	ItemSum    int         `excel:"itemSum"`
}

// 注册
var (
	configMu sync.RWMutex

	characterList             = []*config.Heros{}
	itemList                  = []*config.Item{}
	abilityList               = []*Ability{}
	roomList                  = []*Room{}
	robotList                 = []*Robot{}
	roomAllocationList        = []*RoomAllocation{}
	robotBetList              = []*RobotBet{}
	robotGlobalParametersList = []*RobotGlobalParameters{}
	entryList1                = []*Entry1{}
	entryList2                = []*Entry2{}

	robotItemList = []*RobotItem{}

	// key：sheet名字
	allStructMap = map[string]interface{}{}
)

func initConfig() error {
	all, ok := allStructMap["robotBet"]
	if !ok {
		return errors.New("robotBet not found")
	}
	prt := all.(*[]*RobotBet)
	infos := *prt
	ls := make([]*RobotBetF, 0, len(infos))
	for _, info := range infos {
		r := &RobotBetF{
			Id:          info.Id,
			IncomeRate:  info.IncomeRate,
			LossTime:    info.LossTime,
			KList:       make([]*K, 0, len(info.KList)/2),
			ItemWeight:  info.ItemWeight,
			AbstainRate: info.AbstainRate,
		}
		length := len(info.KList)
		if length%2 != 0 {
			return errors.New("KList Format errors")
		}
		// 0,1,2,3,4
		// 0,1,2,3,4,5,6,7,8,9
		n := length / 2
		for i := 0; i < n; i++ {
			index := i * 2
			r.KList = append(r.KList, &K{
				Lower: info.KList[index],
				Upper: info.KList[index+1],
			})
		}
		ls = append(ls, r)
	}
	allStructMap["robotBet"] = &ls

	//初始化全局配置
	robotGlobals, ok := allStructMap["robotGlobalParameters"]
	if !ok {
		return errors.New("KList Format errors")
	}

	robotGlobalsPrt := robotGlobals.(*[]*RobotGlobalParameters)
	robotGlobalsInfos := *robotGlobalsPrt
	if len(robotGlobalsInfos) == 0 {
		return errors.New("robotGlobalsInfos is null ")
	}
	parameters := robotGlobalsInfos[0]
	constValue.ThinkTime1 = parameters.ThinkTime[0]
	constValue.ThinkTime2 = parameters.ThinkTime[1]
	if constValue.ThinkTime1 >= constValue.ThinkTime2 {
		return errors.New("ThinkTime is errors ")
	}
	constValue.MatchTime1 = parameters.MatchTime[0]
	constValue.MatchTime2 = parameters.MatchTime[1]
	if constValue.MatchTime1 >= constValue.MatchTime2 {
		return errors.New("MatchTime is errors ")
	}
	constValue.TopN = parameters.TopN
	if constValue.TopN == 0 {
		return errors.New("TopN is errors ")
	}
	constValue.TimeSpan = parameters.TimeSpan * 86400
	if constValue.TimeSpan == 0 {
		return errors.New("TimeSpan is errors ")
	}
	return nil
}

func GetRobotBetById(id int) *RobotBetF {
	configMu.RLock()
	defer configMu.RUnlock()

	all, ok := allStructMap["robotBet"]
	if !ok {
		return nil
	}
	prt := all.(*[]*RobotBetF)
	infos := *prt

	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func GetRobotById(id uint8) *Robot {
	configMu.RLock()
	defer configMu.RUnlock()

	all, ok := allStructMap["robot"]
	if !ok {
		return nil
	}
	prt := all.(*[]*Robot)
	infos := *prt

	for _, info := range infos {
		if info.RobotType == id {
			return info
		}
	}
	return nil
}

func GetAllExcelConfig() map[string]interface{} {
	return allStructMap
}

func GetRoomConfigByRoomId(roomId uint32) *Room {
	configMu.RLock()
	defer configMu.RUnlock()

	roomInfo, ok := allStructMap["room"]
	if !ok {
		return nil
	}
	prt := roomInfo.(*[]*Room)
	rooms := *prt

	for _, room := range rooms {
		if room.RoomType == roomId {
			return room
		}
	}
	return nil
}

func GetAbilityConfigById(id uint32) *Ability {
	configMu.RLock()
	defer configMu.RUnlock()

	roomInfo, ok := allStructMap["ability"]
	if !ok {
		return nil
	}
	prt := roomInfo.(*[]*Ability)
	infos := *prt

	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func GetHeroConfigById(id int) *config.Heros {
	configMu.RLock()
	defer configMu.RUnlock()

	roomInfo, ok := allStructMap["heros"]
	if !ok {
		return nil
	}
	prt := roomInfo.(*[]*config.Heros)
	infos := *prt
	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func GetItemConfigById(id int) *config.Item {
	configMu.RLock()
	defer configMu.RUnlock()

	roomInfo, ok := allStructMap["item"]
	if !ok {
		return nil
	}
	prt := roomInfo.(*[]*config.Item)
	infos := *prt
	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func GetAllItemConfig() []*config.Item {
	configMu.RLock()
	defer configMu.RUnlock()

	roomInfo, ok := allStructMap["item"]
	if !ok {
		return nil
	}
	prt := roomInfo.(*[]*config.Item)
	infos := *prt
	return infos
}

func GetRoomItemAllocationConfigById(id int) *RoomAllocation {
	configMu.RLock()
	defer configMu.RUnlock()

	roomInfo, ok := allStructMap["roomAllocation"]
	if !ok {
		return nil
	}
	prt := roomInfo.(*[]*RoomAllocation)
	infos := *prt
	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func GetRobotBetId(incomeRate, lossTime int) *RobotBetF {
	configMu.RLock()
	defer configMu.RUnlock()

	roomInfo, ok := allStructMap["robotBet"]
	if !ok {
		return nil
	}
	prt := roomInfo.(*[]*RobotBetF)
	infos := *prt
	incomeRateMap := make(map[int]struct{})
	lossTimeMap := make(map[int]struct{})
	for _, info := range infos {
		incomeRateMap[info.IncomeRate] = struct{}{}
		lossTimeMap[info.LossTime] = struct{}{}
	}

	income, ok := FindLeftInterval(incomeRateMap, incomeRate)
	if ok {
		loss, ok := FindLeftInterval(lossTimeMap, lossTime)
		if ok {
			for _, info := range infos {
				if info.IncomeRate == income && info.LossTime == loss {
					return info
				}
			}
		}
	}

	return &RobotBetF{
		KList: []*K{
			{
				Lower: 9000,
				Upper: 10000,
			},
			{
				Lower: 9000,
				Upper: 10000,
			},
			{
				Lower: 9000,
				Upper: 10000,
			},
			{
				Lower: 9000,
				Upper: 10000,
			},
			{
				Lower: 9000,
				Upper: 10000,
			},
			{
				Lower: 9000,
				Upper: 10000,
			},
			{
				Lower: 9000,
				Upper: 10000,
			},
		},
	}
}

func GetEntry1ById(id int) *Entry1 {
	configMu.RLock()
	defer configMu.RUnlock()

	entryInfo, ok := allStructMap["entry1"]
	if !ok {
		return nil
	}
	prt := entryInfo.(*[]*Entry1)
	infos := *prt
	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func GetEntry2ById(id int) *Entry2 {
	configMu.RLock()
	defer configMu.RUnlock()

	entryInfo, ok := allStructMap["entry2"]
	if !ok {
		return nil
	}
	prt := entryInfo.(*[]*Entry2)
	infos := *prt
	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func GetRobotItemById(id int) *RobotItem {
	configMu.RLock()
	defer configMu.RUnlock()

	robotItemInfo, ok := allStructMap["robotItem"]
	if !ok {
		return nil
	}
	prt := robotItemInfo.(*[]*RobotItem)
	infos := *prt
	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func FindLeftInterval(incomeRateMap map[int]struct{}, target int) (int, bool) {
	configMu.RLock()
	defer configMu.RUnlock()

	if len(incomeRateMap) == 0 {
		return 0, false
	}

	// 提取 key 到切片
	keys := make([]int, 0, len(incomeRateMap))
	for k := range incomeRateMap {
		keys = append(keys, k)
	}

	// 排序
	sort.Ints(keys)

	// 二分查找左区间
	// 找到最后一个 < target 的元素
	idx := sort.Search(len(keys), func(i int) bool {
		return keys[i] >= target
	})

	// idx 是第一个 >= target 的位置
	// 左区间是 idx-1
	if idx == 0 {
		// target 小于所有值，没有左区间
		return 0, false
	}

	return keys[idx-1], true
}

func UpdateConfig() error {
	//configMu.Lock()         // 获取写锁（阻塞所有读写）
	//defer configMu.Unlock() // 函数返回时释放写锁
	//// 创建读取器（指向excels目录）
	//r := reader.NewExcelReader("./excels")
	//allData, err := r.ReadAllExcels()
	//if err != nil || len(allData) == 0 {
	//	return err
	//}
	//initAllStruct()
	//err = r.ReadSheetToStruct(allData, allStructMap)
	//if err != nil {
	//	return err
	//}
	//excelConfig := GetAllExcelConfig()
	//for _, info := range excelConfig {
	//	if info == nil {
	//		return err
	//	}
	//}
	//err = initConfig()
	//if err != nil {
	//	return err
	//}
	return nil
}

//func initAllStruct() {
//	allStructMap = map[string]interface{}{
//		"heros":                 &characterList,
//		"item":                  &itemList,
//		"ability":               &abilityList,
//		"room":                  &roomList,
//		"robot":                 &robotList,
//		"roomAllocation":        &roomAllocationList,
//		"robotBet":              &robotBetList,              //后替换为RobotBetF
//		"robotGlobalParameters": &robotGlobalParametersList, //
//		"entry1":                &entryList1,                //
//		"entry2":                &entryList2,                //
//		"robotItem":             &robotItemList,             //
//	}
//}
