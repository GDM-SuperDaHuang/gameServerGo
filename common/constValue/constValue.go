package constValue

const (
	GoldItemId int64  = 6 //金币id
	WIDE       uint32 = 9

	BaseValue10 int64 = 10
)

var (
	ThinkTime1 int64 = 7
	ThinkTime2 int64 = 13

	MatchTime1 int64 = 8
	MatchTime2 int64 = 12

	RoomValue1 int64 = 5
	RoomValue2 int64 = 5

	TopN     int64 = 20
	TimeSpan int64 = 60 * 86400
)

const (
	Reward_Rid_EnterRoome = 1
	Reward_Rid_SettleRoom = 2
	Reward_Rid_home       = 3
	Reward_Rid_login      = 4

	Reward_Rid_Reg     = 5
	Reward_Rid_BuyItem = 6
	Reward_Rid_BuyHero = 7

	Item_Rid_Room_consume = 8
	Item_Rid_Background   = 9

	Reward_Rid_SignIn = 10
)
