package room

// import (
//
//	"fmt"
//	"gameServer/app/room/hander/config"
//	"gameServer/common/constValue"
//	"gameServer/pkg/logger/log2"
//	"gameServer/pkg/random/snowflake"
//	"gameServer/service/common"
//	"math/rand"
//	"time"
//
//	"go.uber.org/zap"
//
// )
//
// var (
//
//	robotGenerate = snowflake.NewNode(1)
//	// 全局房间管理
//	robotManager = make(map[uint32][]*robot)
//
// )
//type robot struct {
//	//lock        sync.Mutex
//	robotType   uint8  //机器人类型
//	joinTime    int64  // 唯一Id,创建房间时候自动生成
//	roomType    uint32 // 房间类型
//	UserId      uint64 // 机器人id
//	heroId      uint32 //选择的人物
//	robotStatus int8   //房间状态，0空闲，1:匹配中，2游戏中
//}

//func creatRobot(roomConfig *config.Room) *robot {
//
//	robotConfig := config.GetRobotConfigByRoomType(roomConfig.RoomType)
//	if robotConfig == nil {
//		log2.Get().Error("creatRobot GetRobotConfigByRoomType failed ", zap.Any("roomConfig.RoomType", roomConfig.RoomType))
//		return nil
//	}
//	list := robotConfig.CharacterList
//	if len(list) == 0 {
//		log2.Get().Error("creatRobot robotConfig.CharacterList is null  ", zap.Any("roomConfig.RoomType", roomConfig.RoomType))
//		return nil
//	}
//
//	joinTimes := robotConfig.JoinTimes
//	if len(joinTimes) != 2 {
//		log2.Get().Error("creatRobot robotConfig.CharacterList is null  ", zap.Any("roomConfig.RoomType", roomConfig.RoomType))
//		return nil
//	}
//
//	joinTime := joinTimes[0] + rand.Intn(joinTimes[1]-joinTimes[0])
//
//	heroId := list[rand.Intn(len(list))]
//
//	if robotManager[roomConfig.RoomType] == nil {
//		robotManager[roomConfig.RoomType] = make([]*robot, 0)
//	}
//
//	one := &robot{
//		robotType: robotConfig.RobotType,
//		roomType:  roomConfig.RoomType,
//		joinTime:  int64(joinTime),
//		UserId:    robotGenerate.Generate(),
//		heroId:    heroId,
//	}
//	robotManager[roomConfig.RoomType] = append(robotManager[roomConfig.RoomType], one)
//	return one
//}

//
//func (r *Room) robotMatchPlayer() {
//	// 1 找房间
//	roomConfig := config.GetRoomConfigByRoomId(r.roomType)
//	if roomConfig == nil {
//		return
//	}
//
//	// 2 没有则创建
//	room := roomManager.tryFindMatchRoom(roomConfig)
//	if room == nil {
//		return
//	}
//
//	// 找机器人
//	ro := r.findRobotPlayer(roomConfig)
//	if ro == nil {
//		return
//	}
//	time1 := time.Now().Unix()
//	time2 := r.CreatTime + ro.joinTime
//	if time1 < time2 {
//		return
//	}
//
//	// 加入房间
//	//addFlag := MatchPlayer(roomConfig, &PlayerInfo{
//	//	heroId:     ro.heroId,
//	//	playerType: ro.robotType,
//	//	player: &common.Player{
//	//		UserId: robotGenerate.Generate(),
//	//	},
//	//})
//	//if addFlag {
//	//	ro.robotStatus = 1
//	//}
//
//	// 3 加入房间
//	room.tryAddPlayer(roomConfig, &PlayerInfo{
//		heroId:     ro.heroId,
//		playerType: ro.robotType,
//		player: &common.Player{
//			UserId: robotGenerate.Generate(),
//		},
//	})
//
//	//log2.Get().Info("playerInfos match success", zap.Uint64("UserId", playerInfo.player.UserId), zap.Int32("roomId", room.Id))
//
//	// 4 满员准备开始游戏
//	if room.playerInfos.len() >= roomConfig.CapacityLimit {
//		ro.robotStatus = 1
//		StartGame(room, roomConfig)
//	}
//
//	//room.addRobotPlayer(roomConfig, &PlayerInfo{
//	//	heroId:     ro.heroId,
//	//	playerType: ro.robotType,
//	//	player: &common.Player{
//	//		UserId: robotGenerate.Generate(),
//	//	},
//	//})
//}
//
//// 添加机器人
//func (r *Room) addRobotPlayer(roomConfig *config.Room, playerInfo *PlayerInfo) bool {
//	fmt.Print("addRobotPlayer room.Lock \n")
//
//	//r.lock.Lock()
//	//defer r.lock.Unlock()
//
//	if r.roomStatus != Matching {
//		return false
//	}
//	playerSumLimit := roomConfig.CapacityLimit
//
//	if r.playerInfos.len() >= playerSumLimit {
//		return false
//	}
//	r.playerInfos.AddPlayerInfo(playerInfo)
//
//	playerManager.SetJoinInfo(playerInfo.player.UserId, &JoinInfo{
//		roomId:  r.Id,
//		timeout: time.Now().Unix() + roomConfig.Timeout,
//	})
//	fmt.Print("addRobotPlayer room.unLock \n")
//	return true
//}
//
//// 查找空闲机器人
//func (r *Room) findRobotPlayer(roomConfig *config.Room) *robot {
//	for _, info := range robotManager[r.roomType] {
//		if info.robotStatus == 0 {
//			return info
//		}
//	}
//	return creatRobot(roomConfig)
//}
//
//func (r *Room) RobotPlayerOperate() {
//	robotConfig := config.GetRobotConfigByRoomType(r.roomType)
//	if robotConfig == nil {
//		log2.Get().Error("RobotPlayerOperate GetRobotConfigByRoomType failed ", zap.Any("roomConfig.RoomType", r.roomType))
//		return
//	}
//
//	//思考时间
//	curRound := r.roundList.getCurRound()
//	thinkTime := robotConfig.ThinkTimes
//	if len(thinkTime) != 2 {
//		log2.Get().Error("RobotPlayerOperate ThinkTimes len is err ", zap.Any("len=", len(thinkTime)))
//		return
//	}
//	time1 := time.Now().Unix()
//	time2 := int64(thinkTime[0] + rand.Intn(thinkTime[1]-thinkTime[0]))
//	if time1 < (curRound.creatTime + time2) {
//		return
//	}
//
//	//
//	//count := uint64(100)
//	//robotUseridMap := make(map[uint64]struct{})
//	//playerUseridList := make([]uint64, 0)
//	//opMap := make(map[uint64]struct{})
//
//	//for _, info := range r.playerInfos.playerInfos {
//	//	if info.playerType == 0 {
//	//		playerUseridList = append(playerUseridList, info.player.UserId)
//	//	}
//	//}
//	// 随机找一个玩家
//	//targetPlayerUserId := playerUseridList[rand.Intn(len(playerUseridList))]
//
//	// 结算某玩家的钱
//	//for _, info := range curRound.operations {
//	//	opMap[info.userId] = struct{}{}
//	//	if info.userId == targetPlayerUserId {
//	//		v, ok := info.consume[constValue.GoldItemId]
//	//		if ok {
//	//			count = v
//	//		} else {
//	//			count = uint64(rand.Intn(10000))
//	//		}
//	//	}
//	//}
//	r.playerInfos.lock.RLock()
//	defer r.playerInfos.lock.RUnlock()
//	for _, info := range r.playerInfos.playerInfos {
//
//		if info.playerType == 0 {
//			continue
//		}
//
//		if curRound.isReusingBet(info.player.UserId) {
//			continue
//		}
//
//		target := int64(rand.Intn(20000))
//		consume := map[int]int64{
//			constValue.GoldItemId: target,
//		}
//
//		op := &Operation{
//			userId:    info.player.UserId,
//			operation: opBet,
//			consume:   consume,
//		}
//
//		PlayerOperate(op)
//
//	}
//
//	//node.Push(info.player, protoHandlerInit.BetPush, &pbGo.BetResp{
//	//	PlayerInfo: &pbGo.PlayerInfo{
//	//		UserId: player.UserId,
//	//	},
//	//})
//
//	//for _, one := range room.playerInfos.playerInfos {
//	//	if one.player.UserId == player.UserId {
//	//		continue
//	//	}
//	//	if one.playerType > 0 {
//	//		continue
//	//	}
//	//	node.Push(one.player, protoHandlerInit.BetPush, &pbGo.BetResp{
//	//		PlayerInfo: &pbGo.PlayerInfo{
//	//			UserId: player.UserId,
//	//		},
//	//	})
//	//}
//
//	//count = uint64(rand.Intn(10000))
//	//
//	//// 随机让一个机器人发牌
//	//for userId, _ := range robotUseridMap { //非重复的机器人
//	//
//	//	vibration := rand.Intn(int(2 * robotConfig.Vibration))
//	//	vibration = vibration - int(robotConfig.Vibration)
//	//	target := (count * uint64(vibration)) / 100
//	//
//	//	target = uint64(rand.Intn(20000))
//	//	consume := map[int]uint64{
//	//		constValue.GoldItemId: target,
//	//	}
//	//
//	//	op := &Operation{
//	//		userId:    userId,
//	//		operation: opBet,
//	//		consume:   consume,
//	//	}
//	//	PlayerOperate(op)
//	//}
//
//}
