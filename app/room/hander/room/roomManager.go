package room

//
//import (
//	"gameServer/app/room/hander/config"
//	"sync"
//	"time"
//)
//
//// 房间管理
//type RoomManager struct {
//	lock       sync.RWMutex    //
//	rooms      map[int32]*Room // <房间id,房间信息>
//	nextRoomId int32           //自增的房间id
//}
//
//// 创建房间
//func (rm *RoomManager) CreateRoom(roomConfig *config.Room) *Room {
//
//	rm.lock.Lock()
//	defer rm.lock.Unlock()
//
//	rm.nextRoomId++
//
//	room := &Room{
//		Id:         rm.nextRoomId,
//		roomType:   roomConfig.RoomType,
//		roomStatus: Matching,
//		CreatTime:  time.Now().Unix(),
//		playerInfos: &PlayerInfos{
//			playerInfos: make([]*PlayerInfo, 0),
//		},
//		//players:     &players{},
//	}
//
//	rm.rooms[room.Id] = room
//
//	return room
//}
//
//// 尝试寻找可加入房间
//func (rm *RoomManager) tryFindMatchRoom(roomConfig *config.Room) *Room {
//	rm.lock.RLock()
//	defer rm.lock.RUnlock()
//	for _, r := range rm.rooms {
//
//		if r.roomType != roomConfig.RoomType {
//			continue
//		}
//
//		if r.roomStatus != Matching { //中途退出就不让再次加入了
//			continue
//		}
//
//		//if !r.playerInfos.isFull(roomConfig.CapacityLimit) {
//		//	return r
//		//}
//		if r.playerInfos.len() < roomConfig.CapacityLimit {
//			return r
//		}
//		//if len(r.playerInfos.playerInfos) < roomConfig.CapacityLimit {
//		//	return r
//		//}
//	}
//
//	return nil
//}
//
//func (rm *RoomManager) findRoom(roomId int32) *Room {
//	// 防止多人进入
//	rm.lock.RLock()
//	defer rm.lock.RUnlock()
//	r := rm.rooms[roomId]
//	return r
//}
