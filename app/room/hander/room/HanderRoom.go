package room

import (
	"context"
	"gameServer/app/room/hander/config"
	"gameServer/common/db/items"
	"gameServer/common/errorCode"
	"gameServer/pkg/logger/log2"
	"gameServer/protobuf/pbGo"
	"gameServer/protobuf/protoHandlerInit"
	"gameServer/service/common"
	"gameServer/service/services/node"

	"go.uber.org/zap"
)

type HandlerRoom struct { //必须大写,必须使用指针
}

// 开始匹配 1001
func (h *HandlerRoom) StartMatchHandler(_ context.Context, player *common.Player, req *pbGo.StartMatchReq, resp *pbGo.StartMatchResp) *common.ErrorInfo {
	userId := player.UserId
	// 1. 检查是否已在匹配中
	roomConfig := config.GetRoomConfigByRoomId(req.RoomType)
	if roomConfig == nil {
		log2.Get().Error("[StartMatchHandler] GetRoomConfigByRoomId false ", zap.Any("UserId", userId))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_GetConfigFailed,
		}
	}
	consume := roomConfig.Consume
	if consume == nil {
		log2.Get().Warn("[StartMatchHandler] GetRoomConfigByRoomId consume is null  ", zap.Any("UserId", userId))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_GetConfigFailed,
		}
	}
	//2. 检查是否满足条件
	ok := items.VerifyItem(userId, consume)
	if !ok {
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_ItemNotEnough,
		}
	}
	// 立马响应条件满足
	resp = &pbGo.StartMatchResp{}
	itemMap := make(map[uint64]uint64)
	for _, info := range req.ItemInfoList {
		itemMap[info.ItemId] = info.Count
	}
	// 进入匹配
	MatchPlayer(roomConfig, &PlayerInfo{
		heroId:  req.HeroId,
		itemMap: itemMap,
		player:  player,
	})
	return nil
}

// 取消匹配 1003
func (h *HandlerRoom) CancelMatchRespHandler(_ context.Context, player *common.Player, _ *pbGo.CancelMatchReq, resp *pbGo.CancelMatchResp) *common.ErrorInfo {
	ok := RemoveHasJoinPlayer(player)
	if !ok {
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_NotJoinRoom,
		}
	}

	resp = &pbGo.CancelMatchResp{}
	return nil
}

// 竞拍 1004
func (h *HandlerRoom) BetHandler(_ context.Context, player *common.Player, req *pbGo.BetReq, resp *pbGo.BetResp) *common.ErrorInfo {
	// 如果在房间中
	room := FindHasJoinedRoom(player.UserId)
	if room == nil || room.roomStatus == Matching {
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_NotJoinRoom,
		}
	}
	code := room.roundList.getCurRoundOp(player.UserId)
	if code > 0 {
		return &common.ErrorInfo{
			Code: code,
		}
	}
	// 正式操作

	consume := map[int]uint64{}
	for _, info := range req.BetInfo {
		consume[int(info.ItemId)] = info.Count
	}

	//2. 检查是否满足条件
	ok := items.VerifyItem(player.UserId, consume)
	if !ok {
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_ItemNotEnough,
		}
	}

	ok = PlayerOperate(&Operation{
		userId:    player.UserId,
		operation: opBet,
		consume:   consume,
	})
	if !ok {
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_NotJoinRoom,
		}
	}

	for _, one := range room.playerInfos.playerInfos {
		if one.player.UserId == player.UserId {
			continue
		}
		node.Push(one.player, protoHandlerInit.BetPush, &pbGo.BetResp{
			PlayerInfo: &pbGo.PlayerInfo{
				UserId: player.UserId,
			},
		})
	}

	// 立马响应条件满足
	resp.PlayerInfo = &pbGo.PlayerInfo{
		UserId: player.UserId,
	}

	return nil
}

// 使用道具 1006
func (h *HandlerRoom) UseItemHandler(_ context.Context, player *common.Player, req *pbGo.UseItemReq, resp *pbGo.UseItemResp) *common.ErrorInfo {
	room := FindHasJoinedRoom(player.UserId)
	item := req.Item
	if item == nil {
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_UseNullItem,
		}
	}
	msg, code := room.UserItem(player.UserId, int(item.ItemId), item.Count)
	if code > 0 {
		return &common.ErrorInfo{
			Code: code,
		}
	}
	resp.ChangeScreenInfo = msg
	return nil
}
