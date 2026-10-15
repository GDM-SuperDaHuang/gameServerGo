package logic

import (
	"gameServer/app/room/hander/config"
	"gameServer/common/constValue"
	"gameServer/pkg/random/snowflake"
	"gameServer/service/common"
	"math/rand"
	"time"
)

const (
	baseValue int64 = 10000
)

var (
	robotGenerate = snowflake.NewNode(1)
)

type Robot struct {
	Uid       uint64
	robotType uint8
	roomType  uint32
}

func createOneRobot(roomConfig *config.Room, robotConfig *config.Robot) *MatchRequest {
	heroId := 3005
	l := len(robotConfig.CharacterList)
	if l > 0 {
		heroId = robotConfig.CharacterList[rand.Intn(l)]
	}
	robot := &PlayerInfo{
		Player: &common.Player{
			Uid: robotGenerate.Generate(),
		},
		HeroId:      heroId,
		playerType:  robotConfig.RobotType,
		robotConfig: robotConfig,
		UserItemMap: make(map[int]int8),
	}

	return &MatchRequest{
		player:     robot,
		roomConfig: roomConfig,
	}
}

// 游戏开始时调用
func (r *Room) startRobotActions(roomConfig *config.Room) {
	// 压测测试
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		// 定时撮合
		case <-ticker.C:
			if r == nil {
				ticker.Stop()
				return
			}

			if r.roomStatus == RoomStatusClose { //并发
				ticker.Stop()
				return
			}

			data := r.FindData()
			if data == nil {
				return
			}
			round := data.curRound

			if data.curRound == nil { // todo 为什么可能是null
				return
			}
			ps := data.playerInfos
			if ps == nil {
				return
			}
			for uid, p := range ps {
				if p.playerType > 0 {
					timeLen := round.ThinkTimeMap[uid]
					r.robotRoutine(p, roomConfig, round, timeLen)
				}
			}
		}
	}

}

func (r *Room) robotRoutine(robot *PlayerInfo, roomConfig *config.Room, curRound *Round, timeLen int64) {
	// 如果没有准备好
	if curRound.RoUid == 0 {
		return
	}
	now := time.Now().Unix()
	if curRound.creatTime+timeLen > now { //未来 > now
		return
	}
	goldValue := int64(0)
	if robot.Player.Uid == curRound.RoUid {
		goldValue = curRound.MK
	} else {
		v := rand.Int63n(7) + 1 //1~7
		goldValue = int64(float64(curRound.MK*v) / float64(10))
	}

	// 可能弃牌
	pUid := curRound.PUid
	isAbstain := false
	if pUid > 0 {
		if int64(r.playerInfos[pUid].robotBetConfig.AbstainRate) > rand.Int63n(baseValue) {
			isAbstain = true
		}
	}
	var robotOp *Operation
	if isAbstain {
		robotOp = &Operation{
			Uid:       robot.Player.Uid,
			goldValue: goldValue,
			operation: PlayerOpAbstain,
		}
	} else {
		robotOp = &Operation{
			Uid:       robot.Player.Uid,
			goldValue: goldValue,
			operation: PlayerOpBet,
		}
	}
	r.Action(roomConfig, robotOp)

	// 使用道具
	itemId := r.roItemMap[robot.Player.Uid][int(curRound.RoundIndex)]
	if itemId > 0 {
		robotOp2 := &Operation{
			Uid:       robot.Player.Uid,
			itemId:    itemId,
			operation: PlayerOpUseItem,
		}
		r.Action(roomConfig, robotOp2)
	}
}

// 第一回合
func (r *Room) getGoldValue(value int64, targetUid int64) int64 {
	targetValue := int64(0)
	value2 := (r.SumPrice * constValue.RoomValue1) / constValue.BaseValue10
	if value2 >= value {
		targetValue = value
	} else {
		targetValue = value2
	}
	robotBetConfig := r.playerInfos[targetUid].robotBetConfig
	k := robotBetConfig.KList[0]
	// 10000,12000
	vibration := rand.Int63n(k.Upper-k.Lower) + k.Lower //10000~12000
	goldValue := targetValue * vibration / baseValue
	return goldValue
}

// 之后回合
func (r *Room) getGoldValue2(lastRound *Round, timeout int) (int64, int64, map[int64]int64) {
	//获取上盘最大的值
	maxGoldValue := int64(0)
	targetUid := int64(0)

	thinkTimeMap := make(map[int64]int64) //机器人思考时间
	//
	playerSum := int64(0)
	sumBetTime := int64(0)
	creatTime := lastRound.creatTime * 1000
	for uid, p := range r.playerInfos {
		if p.playerType > 0 {
			thinkTimeMap[uid] = 0
			continue
		}
		op := lastRound.Op[uid]
		if op == nil {
			continue
		}
		if lastRound.Op[uid].operation == PlayerOpBet {
			playerSum++
			sumBetTime += op.isBet - creatTime
		}
		g := op.goldValue
		if g >= maxGoldValue {
			maxGoldValue = g
			targetUid = uid
		}
	}

	// 秒
	averageTime := int64(0)
	if playerSum > 0 {
		averageTime = sumBetTime / (1000 * playerSum)
	}

	for uid, _ := range thinkTimeMap {
		timeValue := int64(float64(averageTime) * (float64(rand.Int63n(constValue.ThinkTime2-constValue.ThinkTime1) + constValue.ThinkTime1)) / float64(constValue.BaseValue10)) //0.7~1.3
		if timeValue >= int64(timeout) {
			timeValue = timeValue - 5
		}
		thinkTimeMap[uid] = timeValue
	}

	if targetUid == 0 { //没有离开，弃牌的时候
		return 0, 0, thinkTimeMap
	}
	//取最大值
	value := (r.SumPrice * constValue.RoomValue2) / constValue.BaseValue10
	if value > maxGoldValue {
		maxGoldValue = value
	}

	// 选择一个策略
	robotBetConfig := r.playerInfos[targetUid].robotBetConfig
	index := int(lastRound.RoundIndex - 1)
	i := len(robotBetConfig.KList) - 1
	if index < 0 {
		return rand.Int63n(10000) + 20000, targetUid, thinkTimeMap
	} else if index > i {
		index = i
	}
	k := robotBetConfig.KList[index]

	// 10000,12000
	vibration := rand.Int63n(k.Upper-k.Lower) + k.Lower //10000~12000
	goldValue := maxGoldValue * vibration / baseValue
	return goldValue, targetUid, thinkTimeMap
}
