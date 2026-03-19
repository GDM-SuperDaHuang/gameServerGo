package roomF

import (
	"context"
	"gameServer/app/room/hander/config"
	"gameServer/pkg/logger/log2"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/exp/rand"
)

const (
	StateIdle     = 0
	StateMatching = 1
	StateInRoom   = 2
)

type RoomManager struct {
	mu         sync.RWMutex
	rooms      map[int32]*Room
	playerRoom map[uint64]*Room //玩家房间,目前加入的房间,有可能是旧的，有可能正在玩的
	nextRoomId int32            //自增的房间id

	playerCancel map[uint64]context.CancelFunc

	// ✅ 状态（强烈建议）
	playerState map[uint64]int

	// ✅ 匹配队列（核心）
	matchQueue chan *MatchRequest

	roomCloseCh chan int32 // 房间回收通道,对应room的 closeChan
}

type MatchRequest struct {
	player     *PlayerInfo
	roomConfig *config.Room
	ctx        context.Context

	matchStartTime int64 //开始匹配的时间秒
	//ver        int64 // ✅ 关键字段
}

func (rm *RoomManager) FindRoomByUserId(userId uint64) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.playerRoom[userId]
}

//	func (rm *RoomManager) updatePlayerRoom(userId uint64, room *Room) {
//		rm.mu.Lock()
//		defer rm.mu.Unlock()
//		rm.playerRoom[userId] = room
//	}
func (rm *RoomManager) delPlayerRoom(userId uint64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.playerRoom, userId)
}

func NewRoomManager() *RoomManager {
	rm := &RoomManager{
		rooms:        make(map[int32]*Room),
		playerRoom:   make(map[uint64]*Room),
		playerCancel: make(map[uint64]context.CancelFunc),
		playerState:  make(map[uint64]int),
		matchQueue:   make(chan *MatchRequest, 10000), // 高并发缓冲
		roomCloseCh:  make(chan int32, 1000),
	}
	//
	go rm.matchWorker()
	go rm.roomRecycleWorker()
	return rm
}

// 房间销毁
func (rm *RoomManager) roomRecycleWorker() {
	for roomId := range rm.roomCloseCh {
		room := rm.FindRoom(roomId)
		if room != nil {
			for userId := range room.playerInfos {
				//log2.Get().Warn("roomRecycleWorker delPlayerRoom Start !!!========== ", zap.Int32("roomId:= ", room.roomId), zap.Uint64("userId:= ", userId))
				pRoom := rm.FindRoomByUserId(userId)
				//log2.Get().Warn("roomRecycleWorker delPlayerRoom !!!========== ", zap.Int32("roomId:= ", rrr.roomId), zap.Uint64("userId:= ", userId))
				if pRoom.roomId == room.roomId {
					rm.delPlayerRoom(userId)
				}
			}
		}
		rm.DeleteRoom(roomId)
	}
}

func (rm *RoomManager) matchWorker() {

	// 按房间类型分桶
	//buckets := make(map[uint32][]*MatchRequest) //房间类型-匹配请求

	buckets := make(map[uint32]map[uint64]*MatchRequest) //房间类型-玩家userId-匹配请求

	ticker := time.NewTicker(50 * time.Millisecond)

	for {
		select {

		// 收请求
		case req := <-rm.matchQueue:
			// 已取消直接丢弃
			select {
			case <-req.ctx.Done(): //极端情况
				log2.Get().Info("roomManager matchQueue ctx closed")
				continue
			default:
			}
			rt := req.roomConfig.RoomType
			//buckets[rt] = append(buckets[rt], req)
			if buckets[rt] == nil {
				buckets[rt] = make(map[uint64]*MatchRequest)
			}
			buckets[rt][req.player.Player.UserId] = req

		// 定时撮合
		case <-ticker.C:
			var players []*MatchRequest
			for roomType, ms := range buckets {
				if len(ms) == 0 {
					continue
				}

				roomConfig := config.GetRoomConfigByRoomId(roomType)
				if roomConfig == nil {
					continue
				}

				need := roomConfig.CapacityLimit
				var robots []*MatchRequest
				// 不够一桌就继续等
				if len(ms) < need {
					now := time.Now().Unix()
					// 可能加入机器人
					minTime := now //最早匹配时间
					for _, info := range ms {
						if info.matchStartTime < minTime {
							minTime = info.matchStartTime
						}
					}

					// 至少10s 匹配机器人
					if now-minTime < 10 {
						continue
					}
					lack := need - len(ms)
					robots = make([]*MatchRequest, 0, lack)
					for _ = range lack {
						// 随机选择一个
						l := len(roomConfig.RobotList)
						if l == 0 {
							log2.Get().Error(" len(roomConfig.RobotList)==0")
							continue
						}
						robotType := roomConfig.RobotList[rand.Intn(len(roomConfig.RobotList))]
						robotConfig := config.GetRobotById(robotType)
						if robotConfig == nil {
							continue
						}
						robot := createOneRobot(roomConfig, robotConfig)
						robots = append(robots, robot)
					}
				}

				players = make([]*MatchRequest, 0, need)
				for _, req := range ms {
					players = append(players, req)
				}
				for _, ro := range robots {
					players = append(players, ro)
				}

				// 取一桌人
				//players := list[:need]
				//buckets[roomType] = list[need:]
				if len(players) < need {
					continue
				}

				rm.createRoomWithPlayers(roomConfig, players)
			}
			// 删除
			for _, info := range players {
				roomConfig := info.roomConfig
				delete(buckets[roomConfig.RoomType], info.player.Player.UserId)
			}

		}
	}
}

func (rm *RoomManager) createRoomWithPlayers(cfg *config.Room, reqs []*MatchRequest) {

	room := rm.CreateRoom(cfg)

	for _, req := range reqs {

		uid := req.player.Player.UserId

		if req.player.playerType == 0 {
			// ❗ 再次检查 cancel（非常关键）
			select {
			case <-req.ctx.Done():
				rm.cleanPlayer(uid)
				continue
			default:
			}
		}

		err := room.Join(req.player, cfg) // 必定加成功的
		if err != nil {
			rm.cleanPlayer(uid)
			continue
		}

		rm.mu.Lock()
		rm.playerRoom[uid] = room
		rm.playerState[uid] = StateInRoom
		delete(rm.playerCancel, uid)
		rm.mu.Unlock()
	}
	log2.Get().Info("[createRoomWithPlayers]:create Room ", zap.Int32("roomId= ", room.roomId))
}

func (rm *RoomManager) cleanPlayer(uid uint64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.playerCancel, uid)
	rm.playerState[uid] = StateIdle
}

func (rm *RoomManager) CreateRoom(cfg *config.Room) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.nextRoomId = rm.nextRoomId + 1
	room := NewRoom(rm.nextRoomId, cfg, roomManager.roomCloseCh)
	rm.rooms[rm.nextRoomId] = room
	return room
}

func (rm *RoomManager) DeleteRoom(roomId int32) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.rooms, roomId)
}

func (rm *RoomManager) FindRoom(roomId int32) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	r, ok := rm.rooms[roomId]
	if ok {
		return r
	}
	return nil
}
