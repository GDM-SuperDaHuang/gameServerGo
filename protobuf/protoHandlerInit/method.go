package protoHandlerInit

// 必须大写
var ProtoIdToMethodMap = map[uint16]string{
	1000: "TestHandler", //协议id -- 处理方法
	// 房间游戏
	1001: "StartMatchHandler", //开始匹配
	//1002: "MatchInfoPush",
	1003: "CancelMatchHandler", //取消匹配
	1004: "BetHandler",         //竞拍
	// 1005: roundInfoPush //推送竞拍
	1006: "UseItemHandler", //道具使用竞拍

	2001: "GetItemInfoHandler",
	2002: "BuyItemHandler",
	2003: "BuyHeroHandler",
	2004: "ReceiveAwardHandler",
	2005: "RankInfoHandler",

	1099: "TestMaxHandler",
}

// 推送信息 协议id
const (
	MatchInfoPush uint16 = 1002
	BetPush       uint16 = 1004
	RoundInfoPush uint16 = 1005
)
