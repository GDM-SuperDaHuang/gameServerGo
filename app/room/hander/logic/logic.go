package logic

import (
	"context"
	"gameServer/app/room/hander/config"
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
	uid := player.Player.Uid

	roomManager.mu.Lock()
	// 取消旧的
	if cancel, ok := roomManager.playerCancel[uid]; ok {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())

	roomManager.playerCancel[uid] = cancel
	room := roomManager.playerRoom[uid]
	if room != nil {
		room.Leave(uid)
	}

	roomManager.mu.Unlock()

	// 丢进队列
	roomManager.matchQueue <- &MatchRequest{
		player:         player,
		roomConfig:     roomConfig,
		ctx:            ctx,
		matchStartTime: time.Now().Unix(),
	}
}

func BetOp(Uid int64, count int64, op int8) uint16 {
	room := roomManager.FindRoomByUid(Uid)
	if room == nil {
		return errorCode.ErrorCode_NotJoinRoom
	}

	roomConfig := config.GetRoomConfigByRoomId(room.roomType)
	//if roomConfig == nil {
	//	return errorCode.ErrorCode_GetConfigFailed
	//}
	//
	//if op == 0 {
	//	op = PlayerOpBet
	//	if count == 0 {
	//		return errorCode.ErrorCode_BetZero
	//	}
	//} else if op == 1 {
	//	op = PlayerOpAbstain
	//}

	//2. 检查是否满足条件
	//ok := items.VerifyItem(Uid, map[int]int64{int(constValue.GoldItemId): count})
	//if !ok {
	//	return errorCode.ErrorCode_ItemNotEnough
	//}
	resp := room.Action(roomConfig, &Operation{
		Uid:       Uid,
		operation: op,
		goldValue: count,
	})
	if resp == nil {
		log2.Get().Error("[UserItem] room.Action of resp is null ", zap.Any("Uid", Uid))
		return errorCode.ErrorCode_GetConfigFailed
	}
	return resp.Code
}

func UserItem(Uid int64, itemId int, count int64) (*pbGo.UseItemResp, uint16) {
	//if count != 1 {
	//	return nil, errorCode.ErrorCode_UseItemNumErr
	//}

	room := roomManager.FindRoomByUid(Uid)
	if room == nil {
		return nil, errorCode.ErrorCode_NotJoinRoom
	}
	roomConfig := config.GetRoomConfigByRoomId(room.roomType)
	if roomConfig == nil {
		return nil, errorCode.ErrorCode_GetConfigFailed
	}

	resp := room.Action(roomConfig, &Operation{
		Uid:       Uid,
		operation: PlayerOpUseItem,
		itemId:    itemId,
	})
	if resp == nil {
		log2.Get().Error("[UserItem] room.Action of resp is null ", zap.Any("Uid", Uid))
		return nil, errorCode.ErrorCode_GetConfigFailed
	}
	return resp.Data, resp.Code
}

func CancelMatch(Uid int64) {
	roomManager.mu.Lock()
	defer roomManager.mu.Unlock()

	if cancel, ok := roomManager.playerCancel[Uid]; ok {
		cancel() // ✅ 触发取消
		delete(roomManager.playerCancel, Uid)
	}
	room := roomManager.playerRoom[Uid]
	if room != nil {
		room.Leave(Uid)
	}
}
