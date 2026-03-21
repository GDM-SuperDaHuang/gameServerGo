package logic

import (
	"gameServer/app/room/hander/config"
	"gameServer/pkg/random/snowflake"
	"gameServer/service/common"
	"math/rand"
	"time"
)

var (
	robotGenerate = snowflake.NewNode(1)
)

type Robot struct {
	UserId    uint64
	robotType uint8
	roomType  uint32
}

func (rm *RoomManager) createRobots(n int, roomConfig *config.Room) []*MatchRequest {
	robots := make([]*PlayerInfo, 0, n)
	for i := 0; i < n; i++ {
		robot := &PlayerInfo{
			Player: &common.Player{
				UserId: robotGenerate.Generate(),
			},
			HeroId:     3005,
			playerType: 1,
		}
		robots = append(robots, robot)
	}

	requests := make([]*MatchRequest, 0, n)

	for _, ro := range robots {
		requests = append(requests, &MatchRequest{
			player:     ro,
			roomConfig: roomConfig,
		})
	}
	return requests
}

func createOneRobot(roomConfig *config.Room, robotConfig *config.Robot) *MatchRequest {
	robot := &PlayerInfo{
		Player: &common.Player{
			UserId: robotGenerate.Generate(),
		},
		HeroId:      3005,
		playerType:  1,
		robotConfig: robotConfig,
	}

	return &MatchRequest{
		player:     robot,
		roomConfig: roomConfig,
	}
}

// 游戏开始时调用
func (r *Room) startRobotActions(roomConfig *config.Room) {
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		// 定时撮合
		case <-ticker.C:
			if r == nil {
				ticker.Stop()
				return
			}

			if r.roomStatus == RoomStatusClose {
				ticker.Stop()
				return
			}

			round := r.getCurrentRound()
			for _, p := range r.playerInfos {
				if p.playerType > 0 {
					timeLen := rand.Intn(p.robotConfig.ThinkTimes[1]-p.robotConfig.ThinkTimes[0]) + p.robotConfig.ThinkTimes[1] //10~15
					r.robotRoutine(p, roomConfig, round, int64(timeLen))
				}
			}
		}
	}

}

func (r *Room) robotRoutine(robot *PlayerInfo, roomConfig *config.Room, curRound *Round, timeLen int64) {
	op := curRound.Op[robot.Player.UserId]
	if op != nil {
		return
	}

	//log2.Get().Info("robot robot start", zap.Int("startTime", startTime))
	now := time.Now().Unix()
	if curRound.creatTime+timeLen > now { //未来 > now
		return
	}

	robotOp := &Operation{
		userId:    robot.Player.UserId,
		goldValue: rand.Int63n(20000),
		isBet:     true,
		operation: PlayerOpBet,
	}
	r.Action(roomConfig, robotOp)
}

//func (r *Room) robotOp(roomConfig *config.Room) {
//	round := r.getCurrentRound()
//
//	var robotOp *Operation
//	for userId, info := range r.playerInfos {
//		if info.playerType == 0 {
//			continue
//		}
//		op := round.Op[userId]
//		if op != nil {
//			continue
//		}
//		robotOp = &Operation{
//			userId:    userId,
//			goldValue: rand.Int63n(20000),
//			isBet:     true,
//			operation: PlayerOpBet,
//		}
//	}
//	if robotOp == nil {
//		return
//	}
//
//	// 执行操作
//	r.Action(roomConfig, robotOp)
//}
