package logic

type Operation struct {
	userId    uint64
	goldValue int64 // 消耗的钱的值
	operation int8  // 玩家操作， 0：等待玩家操作  1:发牌,2：弃权
	isBet     bool  //是否已经bet
	//ItemMap   map[uint64]int8 //使用的道具,道具id-轮数
	itemId int //使用道具,道具id
}
