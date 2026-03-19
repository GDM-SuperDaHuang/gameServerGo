package roomF

import (
	"context"
	"gameServer/app/room/hander/config"
	"gameServer/common/constValue"
	"gameServer/common/db/items"
	"gameServer/common/errorCode"
	"gameServer/pkg/logger/log2"
	"gameServer/protobuf/pbGo"
	"time"

	"go.uber.org/zap"
)

var (
	roomManager *RoomManager
)

func StartMatch(player *PlayerInfo, roomConfig *config.Room) {
	uid := player.Player.UserId

	roomManager.mu.Lock()
	// 取消旧的
	if cancel, ok := roomManager.playerCancel[uid]; ok {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())

	roomManager.playerCancel[uid] = cancel
	room := roomManager.playerRoom[uid]
	//if room == nil {
	//	log2.Get().Warn("")
	//}
	if room != nil {
		room.Leave(uid)
	}
	roomManager.playerState[uid] = StateMatching

	roomManager.mu.Unlock()

	// ✅ 丢进队列（关键变化）
	roomManager.matchQueue <- &MatchRequest{
		player:         player,
		roomConfig:     roomConfig,
		ctx:            ctx,
		matchStartTime: time.Now().Unix(),
	}
}

func BetOp(userId uint64, count int64) uint16 {
	room := roomManager.FindRoomByUserId(userId)
	if room == nil {
		return errorCode.ErrorCode_NotJoinRoom
	}
	roomConfig := config.GetRoomConfigByRoomId(room.roomType)
	if roomConfig == nil {
		return errorCode.ErrorCode_GetConfigFailed
	}

	//2. 检查是否满足条件
	ok := items.VerifyItem(userId, map[int]int64{
		constValue.GoldItemId: count,
	})
	if !ok {
		return errorCode.ErrorCode_ItemNotEnough
	}

	resp := room.Action(roomConfig, &Operation{
		userId:    userId,
		operation: PlayerOpBet,
		goldValue: count,
	})
	if resp == nil {
		log2.Get().Error("[UserItem] room.Action of resp is null ", zap.Any("UserId", userId))
		return errorCode.ErrorCode_GetConfigFailed
	}
	return resp.Code
}

func UserItem(userId uint64, itemId int, count int64) (*pbGo.UseItemResp, uint16) {
	if count != 1 {
		return nil, errorCode.ErrorCode_UseItemNumErr
	}

	room := roomManager.FindRoomByUserId(userId)
	if room == nil {
		return nil, errorCode.ErrorCode_NotJoinRoom
	}
	roomConfig := config.GetRoomConfigByRoomId(room.roomType)
	if roomConfig == nil {
		return nil, errorCode.ErrorCode_GetConfigFailed
	}

	resp := room.Action(roomConfig, &Operation{
		userId:    userId,
		operation: PlayerOpUseItem,
		itemId:    itemId,
	})
	if resp == nil {
		log2.Get().Error("[UserItem] room.Action of resp is null ", zap.Any("UserId", userId))
		return nil, errorCode.ErrorCode_GetConfigFailed
	}
	return resp.Data, resp.Code
}

func CancelMatch(userId uint64) {
	roomManager.mu.Lock()
	defer roomManager.mu.Unlock()

	if cancel, ok := roomManager.playerCancel[userId]; ok {
		cancel() // ✅ 触发取消
		delete(roomManager.playerCancel, userId)
	}

	if roomManager.playerState[userId] == StateMatching {
		roomManager.playerState[userId] = StateIdle
	}

}
