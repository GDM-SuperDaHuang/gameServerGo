package logic

import (
	"context"
	"gameServer/app/room/hander/config"
	"gameServer/common/constValue"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/recoverSafe"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/exp/rand"
)

type RoomManager struct {
	mu         sync.RWMutex
	rooms      map[int32]*Room
	playerRoom map[int64]*Room //玩家房间,目前加入的房间,有可能是旧的，有可能正在玩的
	nextRoomId int32           //自增的房间id

	playerCancel map[int64]context.CancelFunc

	// ✅ 匹配队列（核心）
	matchQueue chan *MatchRequest

	roomCloseCh chan int32 // 房间回收通道,对应room的 closeChan
}

type MatchRequest struct {
	player     *PlayerInfo
	roomConfig *config.Room
	ctx        context.Context

	matchStartTime int64 //开始匹配的时间秒
}

func (rm *RoomManager) FindRoomByUid(Uid int64) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.playerRoom[Uid]
}

func (rm *RoomManager) delPlayerRoom(Uid int64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.playerRoom, Uid)
}

func NewRoomManager() *RoomManager {
	rm := &RoomManager{
		rooms:        make(map[int32]*Room),
		playerRoom:   make(map[int64]*Room),
		playerCancel: make(map[int64]context.CancelFunc),
		matchQueue:   make(chan *MatchRequest, 10000), // 高并发缓冲
		roomCloseCh:  make(chan int32, 10000),
	}

	recoverSafe.GoWorker("matchWorker", rm.matchWorker, recoverSafe.DefaultWorkerOpt)
	recoverSafe.GoWorker("roomRecycleWorker", rm.roomRecycleWorker, recoverSafe.DefaultWorkerOpt)
	return rm
}

// 房间销毁
func (rm *RoomManager) roomRecycleWorker() error {
	for roomId := range rm.roomCloseCh {
		room := rm.FindRoom(roomId)
		if room != nil {
			for Uid := range room.playerInfos {
				pRoom := rm.FindRoomByUid(Uid)
				if pRoom == nil {
					continue
				}
				if pRoom.roomId == room.roomId {
					rm.delPlayerRoom(Uid)
				}
			}
		}
		rm.DeleteRoom(roomId)
	}
	return nil
}

func (rm *RoomManager) matchWorker() error {
	buckets := make(map[uint32]map[int64]*MatchRequest) //房间类型-玩家Uid-匹配请求

	ticker := time.NewTicker(10 * time.Millisecond)

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
			if buckets[rt] == nil {
				buckets[rt] = make(map[int64]*MatchRequest)
			}
			buckets[rt][req.player.Player.Uid] = req

		// 定时撮合
		case <-ticker.C:
			var players []*MatchRequest
			for roomType, ms := range buckets { //1,2,3,
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

					// 压测测试
					// 至少10s 匹配机器人
					if now-minTime < rand.Int63n(constValue.MatchTime2-constValue.MatchTime1)+constValue.MatchTime1 { //随机 8~12 s
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

				flagFull := 0
				players = make([]*MatchRequest, 0, need)
				for _, req := range ms {
					players = append(players, req)
					flagFull++
					if flagFull >= need {
						break
					}
				}
				if flagFull < need {
					for _, ro := range robots {
						players = append(players, ro)
						flagFull++
						if flagFull >= need {
							break
						}
					}
				}
				if flagFull < need {
					continue
				}
				cancelUidList := rm.createRoomWithPlayers(roomConfig, players)
				if cancelUidList == nil {
					// 删除
					for _, info := range players {
						delete(buckets[roomType], info.player.Player.Uid)
					}
				} else {
					for _, uid := range cancelUidList {
						delete(buckets[roomType], uid)
					}
				}
			}
		}
	}
	return nil
}

func (rm *RoomManager) createRoomWithPlayers(cfg *config.Room, reqs []*MatchRequest) []int64 {
	flag := false
	var cancelUidList []int64

	//检查是否有取消的
	for _, req := range reqs {
		uid := req.player.Player.Uid
		if req.player.playerType == 0 {
			// ❗ 再次检查 cancel（非常关键）
			select {
			case <-req.ctx.Done():
				rm.cleanPlayer(uid) // 玩家可能取消
				flag = true
				if cancelUidList == nil {
					cancelUidList = make([]int64, 0)
				}
				cancelUidList = append(cancelUidList, uid)
				continue
			default:
			}
		}
	}

	if !flag {
		room := rm.CreateRoom(cfg)
		if room == nil {
			cancelUidList = make([]int64, 0)
			log2.Get().Error("CreateRoom err")
			for _, req := range reqs {
				uid := req.player.Player.Uid
				cancelUidList = append(cancelUidList, uid)
			}
			return cancelUidList
		}

		for _, req := range reqs {
			uid := req.player.Player.Uid
			err := room.Join(req.player, cfg) // 必定加成功的
			if err != nil {
				if cancelUidList == nil {
					cancelUidList = make([]int64, 0)
				}
				rm.cleanPlayer(uid)
				flag = true
				cancelUidList = append(cancelUidList, uid)
				continue
			}
			rm.mu.Lock()
			rm.playerRoom[uid] = room
			delete(rm.playerCancel, uid)
			rm.mu.Unlock()
		}
		if flag {
			room.cmdChan <- &stop{}
			return cancelUidList
		}
		log2.Get().Info("[createRoomWithPlayers]:create Room ", zap.Int32("roomId= ", room.roomId))
	}
	return cancelUidList
}

func (rm *RoomManager) cleanPlayer(uid int64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.playerCancel, uid)
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
