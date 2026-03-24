package config

// 人物表
type Heros struct {
	//Test   []int             `excel:"test"`   // 自动解析逗号分隔: tag1,tag2,tag3
	Id     int               `excel:"id"`     //人物唯一id
	Lock   bool              `excel:"lock"`   //是否解锁
	RAsMap map[int8][]uint32 `excel:"rAsMap"` //回合-能力list
	Price  map[int]int64     `excel:"price"`  // 自动解析逗号分隔: tag1,tag2,tag3

}

// 道具表
type Item struct {
	Id       int           `excel:"id"`       // 道具唯一id
	ItemType int8          `excel:"itemType"` // 产品类型
	Quality  int8          `excel:"quality"`  // 品质
	Area     []uint32      `excel:"area"`     // 面积
	Price    map[int]int64 `excel:"price"`    // 价值
	UserType int           `excel:"userType"` // 使用类型
	Ability  uint32        `excel:"ability"`  // 能力
}

// 奖励表
type ReceiveAward struct {
	Id         int           `excel:"id"`         // 道具唯一id
	RewardMap  map[int]int64 `excel:"rewardMap"`  // 价值
	RewardType int           `excel:"rewardType"` // 奖励类型，1 登录初始化， 2：一次性奖励 3：每天奖励 4：每月
}

type NormalDistribution struct {
	P     float64 `excel:"p"`     // 道具唯一id
	upper int     `excel:"upper"` // 范围下限
	Lower int     `excel:"Lower"` // 范围上限
}

type item struct {
	id        int `excel:"id"`        // 道具唯一id
	itemValue int `excel:"itemValue"` // 价值
}
