package room

import (
	"context"
	"gameServer/app/room/hander/config"
	"gameServer/app/room/hander/logic"
	"gameServer/common/constValue"
	"gameServer/common/db/items"
	"gameServer/common/errorCode"
	"gameServer/pkg/logger/log2"
	"gameServer/protobuf/pbGo"
	"gameServer/service/common"

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
	//resp = &pbGo.StartMatchResp{}
	itemMap := make(map[int]int64)
	for _, info := range req.ItemInfoList {
		itemMap[int(info.ItemId)] = info.Count
	}
	// 进入匹配
	logic.StartMatch(&logic.PlayerInfo{
		HeroId:        int(req.HeroId),
		ChoiceItemMap: itemMap,
		Player:        player,
		UserItemMap:   make(map[int]int8),
	}, roomConfig)
	return nil
}

// 取消匹配 1003
func (h *HandlerRoom) CancelMatchHandler(_ context.Context, player *common.Player, _ *pbGo.CancelMatchReq, resp *pbGo.CancelMatchResp) *common.ErrorInfo {
	logic.CancelMatch(player.UserId)
	//if !ok {
	//	return &common.ErrorInfo{
	//		Code: errorCode.ErrorCode_NotJoinRoom,
	//	}
	//}
	return nil
}

// 竞拍 1004
func (h *HandlerRoom) BetHandler(_ context.Context, player *common.Player, req *pbGo.BetReq, resp *pbGo.BetResp) *common.ErrorInfo {
	count := int64(0)
	// 如果在房间中
	for _, info := range req.BetInfo {
		if info.ItemId == uint64(constValue.GoldItemId) {
			count = info.Count
		}
	}
	code := logic.BetOp(player.UserId, count)
	if code > 0 {
		return &common.ErrorInfo{
			Code: code,
		}
	}

	// 立马响应条件满足
	resp.PlayerInfo = &pbGo.PlayerInfo{
		UserId: player.UserId,
	}

	return nil
}

// 使用道具 1006
func (h *HandlerRoom) UseItemHandler(_ context.Context, player *common.Player, req *pbGo.UseItemReq, resp *pbGo.UseItemResp) *common.ErrorInfo {
	respData, code := logic.UserItem(player.UserId, int(req.Item.ItemId), req.Item.Count)
	if code != 0 {
		return &common.ErrorInfo{
			Code: code,
		}
	}
	resp.ChangeScreenInfo = respData.ChangeScreenInfo
	resp.HintList = respData.HintList
	resp.ItemInfoList = respData.ItemInfoList
	return nil
}
