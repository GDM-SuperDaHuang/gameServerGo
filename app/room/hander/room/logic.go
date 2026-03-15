package room

import (
	"fmt"
	"gameServer/app/room/hander/config"
	"gameServer/app/room/hander/maxRects"
	"gameServer/common/db/items"
	"gameServer/pkg/logger/log2"
	"gameServer/protobuf/pbGo"
	"gameServer/protobuf/protoHandlerInit"
	"gameServer/service/common"
	"gameServer/service/services/node"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
)

var (
	matchPool *ants.Pool
	// 全局房间管理
	roomManager = &RoomManager{ //roomId - room
		rooms: make(map[int32]*Room),
	}
	playerManager = &PlayerManager{ //UserId - roomId 已经加入的房间
		prMap: make(map[uint64]*JoinInfo),
	}

	// 房间状态
	Matching int8 = 0
	Playing  int8 = 1
	End      int8 = 2
)

type PlayerManager struct {
	lock  sync.RWMutex         //
	prMap map[uint64]*JoinInfo // <玩家id,房间id>
}

// 加入信息
type JoinInfo struct {
	roomId  int32 //
	timeout int64 // todo 可能不需要，加入时间戳，秒
}

// 房间信息
type Room struct {
	//lock        sync.RWMutex // 计算房间人数
	Id          int32        // 唯一Id,创建房间时候自动生成
	roomType    uint32       // 房间类型
	playerInfos *PlayerInfos //已经进入房间的玩家
	roomStatus  int8         //房间状态，0匹配中，1游戏中，(如果游戏结束则立马销毁该房间)
	//players     *players
	roundList *roundSafety //每局信息,玩家中途离开时还会保留数据

	CreatTime int64 //房间创建时间戳

	gridInfo *[]*maxRects.Placement //物品信息
}

// 尝试添加玩家
func (r *Room) tryAddPlayer(roomConfig *config.Room, playerInfo *PlayerInfo) bool {
	if r.roomStatus != Matching {
		return false
	}
	playerSumLimit := roomConfig.CapacityLimit

	//防止多人加入
	r.playerInfos.lock.Lock()
	defer r.playerInfos.lock.Unlock()

	if len(r.playerInfos.playerInfos) >= playerSumLimit {
		return false
	}

	r.playerInfos.addPlayerInfo0(playerInfo)

	playerManager.SetJoinInfo(playerInfo.player.UserId, &JoinInfo{
		roomId:  r.Id,
		timeout: time.Now().Unix() + roomConfig.Timeout,
	})
	return true
}

// 从已有的房间里面退出
func RemoveHasJoinPlayer(p *common.Player) bool {
	joinInfo, ok := playerManager.GetJoinInfo(p.UserId)
	if !ok {
		return false
	}
	room := roomManager.findRoom(joinInfo.roomId)
	if room == nil {
		return true
	}
	room.removerRoomPlayer(p.UserId)
	return true
}

// 寻找已经加入房间
func FindHasJoinedRoom(userId uint64) *Room {
	joinInfo, ok := playerManager.GetJoinInfo(userId)
	if !ok {
		return nil
	}
	return roomManager.findRoom(joinInfo.roomId)
}

// 匹配逻辑（放入 ants 协程池）,销毁没有处理
func MatchPlayer(roomConfig *config.Room, playerInfo *PlayerInfo) bool {

	addFlag := false
	err := matchPool.Submit(func() {
		// 去除脏数据
		room := FindHasJoinedRoom(playerInfo.player.UserId)
		if room != nil {
			if roomConfig.RoomType == room.roomType { //删除
				room.removerRoomPlayer(playerInfo.player.UserId) // 旧房间移除
			}
		}

		// 1 找房间
		room = roomManager.tryFindMatchRoom(roomConfig)

		// 2 没有则创建
		if room == nil {
			room = roomManager.CreateRoom(roomConfig)
		}

		// 3 加入房间
		addFlag = room.tryAddPlayer(roomConfig, playerInfo)

		log2.Get().Info("playerInfos match success", zap.Uint64("UserId", playerInfo.player.UserId), zap.Int32("roomId", room.Id))

		// 4 满员准备开始游戏
		if room.playerInfos.len() >= roomConfig.CapacityLimit {
			StartGame(room, roomConfig)
		}
	})

	if err != nil {
		log2.Get().Error("match submit error", zap.Error(err))
	}
	return addFlag
}

// 开始游戏
func StartGame(room *Room, roomConfig *config.Room) bool {

	// 通知玩家开始游戏

	// 防止作弊，再次计算所有玩家的门票
	consume := roomConfig.Consume
	var playersToRemove []uint64
	for _, info := range room.playerInfos.playerInfos {
		if info.playerType > 0 { //排除机器人
			continue
		}
		// 再次验证，匹配成功后扣道具
		ok := items.VerifyItem(info.player.UserId, consume)
		if !ok { // 失败 极端情况,匹配失败,防止作弊
			//node.Push(info, protoHandlerInit.MatchInfoPush, matchInfoPush)
			if playersToRemove == nil {
				playersToRemove = make([]uint64, 0)
			}
			playersToRemove = append(playersToRemove, info.player.UserId)
		}
	}

	// 移除玩家
	if len(playersToRemove) > 0 {
		for _, userId := range playersToRemove {
			room.removerRoomPlayer(userId)
		}
		return false
	}

	matchInfoPush := &pbGo.MatchInfoPush{
		RoomType: roomConfig.RoomType,
	}

	log2.Get().Info("room start game", zap.Int32("roomId", room.Id))

	playerInfoList := make([]*pbGo.PlayerInfo, 0, room.playerInfos.len())
	for _, info := range room.playerInfos.playerInfos {
		userId := info.player.UserId
		// 5. 匹配成功后扣道具
		items.ConsumeItem(userId, consume)
		playerInfoList = append(playerInfoList, &pbGo.PlayerInfo{
			UserId:  userId,
			HeroId:  info.heroId,
			BetInfo: make([]*pbGo.ItemInfo, 0),
		})
	}

	// 6. 推送匹配成功
	matchInfoPush.PlayerInfoList = playerInfoList
	for _, info := range room.playerInfos.playerInfos {
		if info.playerType > 0 {
			continue
		}
		node.Push(info.player, protoHandlerInit.MatchInfoPush, matchInfoPush)
	}
	room.roomStatus = Playing

	// 7.开始加载数据
	room.roundList = &roundSafety{
		roundList: make([]*round, 0),
	}
	nextRound := room.nextRound(roomConfig)
	// 7.1分配藏品信息
	sum := int(roomConfig.ItemSum)
	itemList := make([]int, 0, sum)
	maxLen := len(roomConfig.ItemList)
	for i := 0; i < sum; i++ {
		index := rand.Intn(maxLen)
		itemList = append(itemList, roomConfig.ItemList[index])
	}

	gridInfo := make([]maxRects.Item, 0, len(itemList))
	for _, itemId := range itemList {
		info := config.GetItemConfigById(itemId)
		if info == nil {
			log2.Get().Error("GetItemConfigById failed ", zap.Any("itemId", itemId))
			continue
		}
		if info.Area == nil || len(info.Area) != 2 {
			log2.Get().Error("ItemConfig Area is null", zap.Any("itemId", itemId))
			continue
		}
		gridInfo = append(gridInfo, maxRects.Item{
			Id:     itemId,
			Length: info.Area[0],
			Width:  info.Area[1],
		})
	}
	//

	// 优化排序
	sort.Slice(gridInfo, func(i, j int) bool {
		return gridInfo[i].Length*gridInfo[i].Width >
			gridInfo[j].Length*gridInfo[j].Width
	})
	placements := maxRects.Pack(gridInfo)
	room.gridInfo = &placements
	// 8.推送信息
	room.pushRoomInfo(nextRound, roomConfig, false)

	return true
}

// 房间超时移除超时玩家（定时器）
func StartRoomCleaner() {
	ticker := time.NewTicker(time.Second * 1)
	for range ticker.C {
		//now := time.Now().Unix()
		var destroyRoomIdList []int32
		for id, room := range roomManager.rooms {
			status := room.roomStatus
			if status == Playing { //游戏中
				roomConfig := config.GetRoomConfigByRoomId(room.roomType)
				if roomConfig == nil {
					continue
				}

				matchPool.Submit(func() {
					room.checkRoom(roomConfig) //检查玩家操作
					room.RobotPlayerOperate()  //机器人操作
				})
				continue

			} else if status == End { //结束
				log2.Get().Info("Destroy Room...", zap.Int32("roomId", id), zap.Int8("room.roomStatus", status))
				if destroyRoomIdList == nil {
					destroyRoomIdList = make([]int32, 0)
				}
				destroyRoomIdList = append(destroyRoomIdList, id)
			} else if status == Matching { //匹配中，可能会添加机器人
				room.robotMatchPlayer()
			}

			// todo 匹配超时，需要兜底??
			//for _, info := range room.playerInfos.playerInfos {
			//	joinInfo, ok := playerManager.GetJoinInfo(info.player.UserId)
			//	if !ok {
			//		if destroyRoomIdList == nil {
			//			destroyRoomIdList = make([]int32, 0)
			//		}
			//		destroyRoomIdList = append(destroyRoomIdList, id)
			//	}
			//	if joinInfo != nil && joinInfo.timeout < now { //todo,超时移除玩家
			//		room.removerRoomPlayer(info.player.UserId)
			//		delete(roomManager.rooms, id)
			//		log2.Get().Info("room timeout destroy", zap.Int32("roomId", id))
			//	}
			//}
		}

		//销毁房间
		for _, roomId := range destroyRoomIdList {
			DestroyRoom(roomId)
		}

	}
}

// 移除房间的某个玩家 todo
func (r *Room) removerRoomPlayer(userId uint64) {
	fmt.Print("removerRoomPlayer room.RLock \n")
	//r.lock.RLock()
	//defer r.lock.RUnlock()
	for _, info := range r.playerInfos.playerInfos {
		if info.player.UserId == userId {
			info.status = playerStatusLeave // 标记已经离开
		}
	}

	//移除
	playerManager.DelJoinInfo(userId)
	fmt.Print("removerRoomPlayer room.RUnlock \n")

}

// 移除整一个房间 todo
func DestroyRoom(roomId int32) {
	// 直接获取写锁，避免读锁和写锁的冲突
	roomManager.lock.Lock()
	defer roomManager.lock.Unlock()

	r := roomManager.rooms[roomId]
	if r == nil {
		return
	}
	delete(roomManager.rooms, roomId)
	// 释放锁后再处理玩家信息
	for _, info := range r.playerInfos.playerInfos {
		playerManager.DelJoinInfo(info.player.UserId)
	}
}

// 删除索引 i 处的元素
func removeIndex(s []*PlayerInfo, i int) []*PlayerInfo {
	// 检查索引边界
	if i < 0 || i >= len(s) {
		return s
	}
	// 将 i 之前的元素与 i 之后的元素拼接
	return append(s[:i], s[i+1:]...)
}
func removeIndex2(s []*common.Player, i int) []*common.Player {
	// 检查索引边界
	if i < 0 || i >= len(s) {
		return s
	}
	// 将 i 之前的元素与 i 之后的元素拼接
	return append(s[:i], s[i+1:]...)
}

// 写入时加锁
func (pm *PlayerManager) SetJoinInfo(userId uint64, info *JoinInfo) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pm.prMap[userId] = info
}

// 读取时加读锁
func (pm *PlayerManager) GetJoinInfo(userId uint64) (*JoinInfo, bool) {
	pm.lock.RLock()
	defer pm.lock.RUnlock()
	info, ok := pm.prMap[userId]
	return info, ok
}

// 删除
func (pm *PlayerManager) DelJoinInfo(userId uint64) {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	delete(pm.prMap, userId)
}

// 协程池
func InitMatchPool(poolSize int) error {
	var err error
	matchPool, err = ants.NewPool(
		poolSize,
		ants.WithPanicHandler(func(i any) {
			log2.Get().Error("[matchPool] panic", zap.Any("err", i))
		}),
	)

	return err
}

// 初始化配置表配置,协程池,房间监控
func InitRoomConfig() {
	err := InitMatchPool(5000)
	if err != nil {
		panic(err)
	}
	go StartRoomCleaner()
}
