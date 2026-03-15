package config

import (
	"gameServer/common/config"
	"math/rand"
)

// 能力配置表
type Ability struct {
	Id            uint32 `excel:"id"`            //能力id
	ShowItemCount int8   `excel:"showItemCount"` //0:为所有的数量,n:具体数量，1,2,3，4
	ShowPrice     int8   `excel:"showPrice"`     //0：不显示价值 1:总价值 2:平均价值
	ShowGridCount int8   `excel:"showGridCount"` //提示物品占用格子数量；显示类:0:不显示 1:平均格子数量，2：所有的格子数量
	ShowQuality   bool   `excel:"showQuality"`   //是否显示品质,显示类：0：不显示1：显示
	ShowContour   bool   `excel:"showContour"`   //是否显示轮廓，显示类,0:不显示,1：显
	ShowAll       bool   `excel:"showAll"`       //是否显示所有，显示类,0:不显示,1：显

	CArea          int8 `excel:"cArea"`          // 条件类,格子面积:-1:最大格子，0：随机,n:具体数量的格子
	CQualityType   int8 `excel:"cQualityType"`   // 条件类：-3：最高品质,-2: 所有品质,0:随机品质,1：白色,2：绿色,3,4,5,6
	CItemType      int8 `excel:"cItemType"`      // 物品类型，0：随机类型，n：具体道具类型，如，1,2,3,4
	CItemTypeCount int8 `excel:"cItemTypeCount"` // -1:类型最少数量,0：不参与，随机类型，
	CPrice         int8 `excel:"cPriceCount"`    // 条件类 -1:价值最高的,0：不参与，随机类型，
	CPrePQ         int8 `excel:"cPrePQ"`         // 条件类 0：不限制 1:已经暴露品质的条件下 ,2:已经暴露轮廓的条件下
}

// 房间配置
type Room struct {
	RoomType         uint32            `excel:"roomType"`         //房间类型
	Consume          map[int]uint64    `excel:"consume"`          // 消耗
	Limit            map[uint64]uint64 `excel:"limit"`            // 限制
	CapacityLimit    int               `excel:"capacityLimit"`    // 人数限制
	EntryList        []int             `excel:"entryList"`        // 词条集合
	EntryRound       []uint8           `excel:"entryRound"`       // 词条参与的局索引
	Timeout          int64             `excel:"timeout"`          // 操作超时
	RoundLimit       uint8             `excel:"roundLimit"`       // 最大局数
	ItemList         []int             `excel:"itemList"`         // 藏品id集合
	ItemSum          uint32            `excel:"itemSum"`          // 藏品随机数量
	EarlyTermination []int             `excel:"earlyTermination"` // 藏品随机数量
}

type Robot struct {
	RobotType     uint8    `excel:"robotType"`     //机器人类型
	RoomTypeList  []uint32 `excel:"roomTypeList"`  //房间
	CharacterList []uint32 `excel:"characterList"` //人物
	JoinTimes     []int    `excel:"joinTimes"`     //加入时间
	ThinkTimes    []int    `excel:"thinkTimes"`    //思考时间

	Vibration uint8 `excel:"vibration"` //波动百分比
}

// 词条
//type Entry struct {
//	RoomType      uint32 `excel:"id"` //词条唯一id
//	Ability int    `excel:"ability"`
//}

// 注册
var (
	characterList = []*config.Heros{}
	itemList      = []*config.Item{}
	abilityList   = []*Ability{}
	roomList      = []*Room{}
	robotList     = []*Robot{}

	allStructMap = map[string]interface{}{
		"heros":   &characterList,
		"item":    &itemList,
		"ability": &abilityList,
		"room":    &roomList,
		"robot":   &robotList,
	}
)

func GetAllExcelConfig() map[string]interface{} {
	return allStructMap
}

func GetRoomConfigByRoomId(roomId uint32) *Room {
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

//func GetEntryConfigById(id uint32) *Entry {
//	roomInfo, ok := allStructMap["ability"]
//	if !ok {
//		return nil
//	}
//	prt := roomInfo.(*[]*Entry)
//	infos := *prt
//	for _, info := range infos {
//		if info.RoomType == id {
//			return info
//		}
//	}
//	return nil
//}

func GetHeroConfigById(id uint32) *config.Heros {
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

func GetAllItemConfigById() []*config.Item {
	roomInfo, ok := allStructMap["item"]
	if !ok {
		return nil
	}
	prt := roomInfo.(*[]*config.Item)
	infos := *prt
	return infos
}

func GetRobotConfigByRoomType(roomType uint32) *Robot {
	all, ok := allStructMap["robot"]
	if !ok {
		return nil
	}
	prt := all.(*[]*Robot)
	infos := *prt

	robotTypeList := make([]*Robot, 0, 1)
	for _, info := range infos {
		for _, rt := range info.RoomTypeList {
			if rt == roomType {
				robotTypeList = append(robotTypeList, info)
				break
			}
		}
	}
	return robotTypeList[rand.Intn(len(robotTypeList))]
}
