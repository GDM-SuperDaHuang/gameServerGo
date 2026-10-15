package room

import (
	"context"
	"gameServer/app/room/hander/config"
	"gameServer/app/room/hander/logic"
	"gameServer/common/db/rooming"
	"gameServer/protobuf/pbGo"
	"gameServer/service/common"
)

type HandlerRoom struct { //必须大写,必须使用指针
}

// 开始匹配 1001
func (h *HandlerRoom) StartMatchHandler(_ context.Context, player *common.Player, req *pbGo.StartMatchReq, resp *pbGo.StartMatchResp) *common.ErrorInfo {
	//var errorInfo *common.ErrorInfo
	//defer func() {
	//	elastic.Upload("BetHandler", map[string]any{
	//		"req": req,
	//		"err": errorInfo,
	//	})
	//}()
	////消除游戏房间记录
	//go rooming.DelRoomingId(player.Uid)
	//Uid := player.Uid
	//// 1. 检查是否已在匹配中
	roomConfig := config.GetRoomConfigByRoomId(req.RoomType)
	//if roomConfig == nil {
	//	log2.Get().Error("[StartMatchHandler] GetRoomConfigByRoomId false ", zap.Any("Uid", Uid), zap.Uint32("roomType", req.RoomType))
	//	errorInfo = common.Error(errorCode.ErrorCode_GetConfigFailed)
	//	return errorInfo
	//}
	//
	//if roomConfig.Consume == nil {
	//	log2.Get().Warn("[StartMatchHandler] GetRoomConfigByRoomId consume is null  ", zap.Any("Uid", Uid))
	//	errorInfo = common.Error(errorCode.ErrorCode_GetConfigFailed)
	//	return errorInfo
	//}
	//
	//itemMap := make(map[int]int64, len(req.ItemInfoList))
	//for _, info := range req.ItemInfoList {
	//	itemMap[int(info.ItemId)] = info.Count
	//}
	//consume := make(map[int]int64)
	//for i, i2 := range roomConfig.Consume {
	//	itemMap[i] = i2
	//}
	//
	//ok := items.VerifyItem(Uid, itemMap)
	//if !ok {
	//	errorInfo = common.Error(errorCode.ErrorCode_ItemNotEnough)
	//	return errorInfo
	//}
	//
	////2. 检查是否满足条件
	//ok = items.VerifyItem(Uid, consume)
	//if !ok {
	//	errorInfo = common.Error(errorCode.ErrorCode_ItemNotEnough)
	//	return errorInfo
	//}
	// 进入匹配
	logic.StartMatch(&logic.PlayerInfo{
		HeroId: int(req.HeroId),
		//ChoiceItemMap: itemMap,
		Player:      player,
		UserItemMap: make(map[int]int8),
	}, roomConfig)
	return nil
}

// 取消匹配 1003
func (h *HandlerRoom) CancelMatchHandler(_ context.Context, player *common.Player, _ *pbGo.CancelMatchReq, resp *pbGo.CancelMatchResp) *common.ErrorInfo {
	logic.CancelMatch(player.Uid)

	//消除游戏房间记录
	go rooming.DelRoomingId(player.Uid)
	return nil
}

// 竞拍 1004
func (h *HandlerRoom) BetHandler(_ context.Context, player *common.Player, req *pbGo.BetReq, resp *pbGo.BetResp) *common.ErrorInfo {
	var errorInfo *common.ErrorInfo
	//defer func() {
	//	elastic.Upload("BetHandler", map[string]any{
	//		"req":   req,
	//		"resp":  resp,
	//		"error": errorInfo,
	//	})
	//}()
	//
	//count := int64(0)
	//// 如果在房间中
	//for _, info := range req.BetInfo {
	//	if info.ItemId == uint64(constValue.GoldItemId) {
	//		count = info.Count
	//	}
	//}

	op := req.Operation
	code := logic.BetOp(player.Uid, 20, int8(op))
	if code > 0 {
		errorInfo = common.Error(code)
		return errorInfo
	}

	// 立马响应条件满足
	resp.PlayerInfo = &pbGo.PlayerInfo{
		Uid: player.Uid,
	}

	return nil
}

// 使用道具 1006
func (h *HandlerRoom) UseItemHandler(_ context.Context, player *common.Player, req *pbGo.UseItemReq, resp *pbGo.UseItemResp) *common.ErrorInfo {
	var errorInfo *common.ErrorInfo
	//defer func() {
	//	elastic.Upload("BetHandler", map[string]any{
	//		"req":  req,
	//		"err":  errorInfo,
	//		"resp": resp,
	//	})
	//}()
	//if req.Item == nil {
	//	errorInfo = common.Error(errorCode.ErrorCode_UseNullItem)
	//	return errorInfo
	//}
	respData, code := logic.UserItem(player.Uid, int(req.Item.ItemId), req.Item.Count)
	if code != 0 {
		errorInfo = common.Error(code)
		return errorInfo
	}
	//if respData.ChangeScreenInfo != nil {
	//	resp.ChangeScreenInfo = respData.ChangeScreenInfo
	//}
	//if respData.HintList != nil {
	//	resp.HintList = respData.HintList
	//}
	if respData.ItemInfoList != nil {
		resp.ItemInfoList = respData.ItemInfoList
	}
	return nil
}

// 热更新配置
func (h *HandlerRoom) UpdateConfigHandler(_ context.Context, player *common.Player, req *pbGo.UpdateConfigRep, resp *pbGo.UpdateConfigResp) *common.ErrorInfo {
	//err := config.UpdateConfig()
	//if err != nil {
	//	log2.Get().Error("UpdateConfigHandler error ", zap.Error(err))
	//	return common.Error(errorCode.ErrorCode_UpdateConfigFailed)
	//}
	//log2.Get().Info("[room] [UpdateConfigHandler] UpdateConfigHandler", zap.Any("player", player), zap.Any("req", req))
	return nil
}
