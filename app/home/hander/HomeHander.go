package hander

import (
	"context"
	"gameServer/app/home/hander/config"
	"gameServer/app/home/hander/logic"
	"gameServer/common/constValue"
	"gameServer/common/db/items"
	"gameServer/common/db/round"
	"gameServer/common/errorCode"
	"gameServer/pkg/elastic"
	"gameServer/pkg/logger/log2"
	"gameServer/protobuf/pbGo"
	"gameServer/service/common"
	"gameServer/service/node"

	"go.uber.org/zap"
)

type HomeHandler struct { //必须大写,必须使用指针
}

// 2001
func (h *HomeHandler) StartGameHandler(_ context.Context, player *common.Player, req *pbGo.StartGameReq, resp *pbGo.StartGameResp) *common.ErrorInfo {

	// 产生一个局id
	// todo redis 唯一Id++
	resp.RoundId = round.IncrRoundId(player.Uid)
	logic.UpdateRound(player.Uid, resp.RoundId, nil)
	// 错误做法
	//resp = &pbGo.ItemInfoResp{
	//	ItemList: itemList,
	//}
	return nil
}

// 2002
func (h *HomeHandler) DeathHandler(_ context.Context, player *common.Player, req *pbGo.DeathReq, resp *pbGo.DeathResp) *common.ErrorInfo {
	logic.UpdateRound(player.Uid, req.RoundId, nil)
	return nil
}

// 2002
func (h *HomeHandler) GameOverHandler(_ context.Context, player *common.Player, req *pbGo.DeathReq, resp *pbGo.DeathResp) *common.ErrorInfo {
	logic.GetRoundReward(player.Uid, req.RoundId, nil)
	return nil
}

// 2002
func (h *HomeHandler) BuyItemHandler(_ context.Context, player *common.Player, req *pbGo.BuyItemRep, resp *pbGo.BuyItemResp) *common.ErrorInfo {
	var errorInfo *common.ErrorInfo
	defer func() {
		elastic.Upload("BuyItemHandler", map[string]any{
			"uid": player.Uid,
			"req": req,
			"err": errorInfo,
		})
	}()
	if req.Item == nil {
		errorInfo = common.Error(errorCode.ErrorCode_ReqIsNull)
		return errorInfo
	}
	//item := config.GetItemConfigById(int(req.Item.ItemId))
	//if item == nil {
	//	log2.Get().Error("GetItemConfigById is null ", zap.Any("address", req.Item.ItemId))
	//	errorInfo = common.Error(errorCode.ErrorCode_GetConfigFailed)
	//	return errorInfo
	//}

	priceMap := make(map[int]int64)
	//for k, v := range item.Price {
	//	priceMap[k] = v * req.Item.Count
	//}
	//verify := items.VerifyItem(player.Uid, priceMap)
	//if !verify {
	//	log2.Get().Debug("BuyItem false ", zap.Int64("Uid", player.Uid), zap.Uint64("ItemId", req.Item.ItemId))
	//	errorInfo = common.Error(errorCode.ErrorCode_ItemNotEnough)
	//	return errorInfo
	//}
	flag := items.ConsumeItem(player.Uid, priceMap, constValue.Reward_Rid_BuyItem, "BuyItemHandler")
	if !flag {
		//log2.Get().Error("BuyItem ConsumeItem false ", zap.Int64("Uid", player.Uid), zap.Any("item.Price", item.Price))
		errorInfo = common.Error(errorCode.ErrorCode_ItemNotEnough)
		return errorInfo
	}
	items.RewardItem(player.Uid, map[int]int64{int(req.Item.ItemId): req.Item.Count}, constValue.Reward_Rid_BuyItem, "BuyItemHandler")
	pushInfo := &pbGo.LoginResp{
		Uid:         player.Uid,
		AccessToken: "asdsa",
	}
	node.Push(player, 4, pushInfo)
	return nil
}

// 热更新配置
func (h *HomeHandler) UpdateConfigHandler(_ context.Context, player *common.Player, req *pbGo.UpdateConfigRep, resp *pbGo.UpdateConfigResp) *common.ErrorInfo {
	err := config.UpdateConfig()
	if err != nil {
		log2.Get().Error("UpdateConfigHandler error ", zap.Error(err))
		return common.Error(errorCode.ErrorCode_UpdateConfigFailed)
	}
	log2.Get().Info("[home] [UpdateConfigHandler] UpdateConfigHandler", zap.Any("player", player), zap.Any("req", req))
	return nil
}
