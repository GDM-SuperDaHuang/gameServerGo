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
	resp.RoundId = round.IncrRoundId(player.Uid)
	logic.UpdateRound(player.Uid, resp.RoundId, nil)
	// 错误做法
	//resp = &pbGo.ItemInfoResp{
	//	ItemList: itemList,
	//}
	return nil
}

// 2002 死亡上报:只做本局内存累加,不发奖
func (h *HomeHandler) DeathHandler(_ context.Context, player *common.Player, req *pbGo.DeathReq, resp *pbGo.DeathResp) *common.ErrorInfo {
	if req == nil || req.RoundId == 0 {
		return common.Error(errorCode.ErrorCode_ReqIsNull)
	}
	logic.UpdateRound(player.Uid, req.RoundId, req.MonsterId)
	return nil
}

// 2003 游戏结束:先删本局内存快照,再用快照发奖(重复请求因内存已删直接幂等返回)
func (h *HomeHandler) GameOverHandler(_ context.Context, player *common.Player, req *pbGo.GameOverReq, resp *pbGo.GameOverResp) *common.ErrorInfo {
	if req == nil || req.RoundId == 0 {
		return common.Error(errorCode.ErrorCode_ReqIsNull)
	}
	// 1. 先删后奖:取出并删除本局内存,拿到快照
	roundInfo, ok := logic.TakeRound(player.Uid, req.RoundId)
	if !ok {
		// 局不存在:未开始或已结算(内存已删),按重复结算处理
		return common.Error(errorCode.ErrorCode_AlreadyReward)
	}
	// 2. 结算时附带的最后一批击杀也计入快照
	if len(req.MonsterIdList) > 0 {
		roundInfo.AddMonsters(req.MonsterIdList)
	}
	// 3. 用快照计算奖励
	reward, precisionExtraGold := logic.CalcRoundReward(roundInfo, req.Precision)
	if len(reward) == 0 {
		resp.PrecisionExtraGold = int32(precisionExtraGold)
		resp.BillInfo = &pbGo.BillInfo{}
		return nil
	}
	// 4. 发奖
	itemMap := make(map[int]int64, len(reward))
	for itemId, count := range reward {
		if count <= 0 {
			continue
		}
		itemMap[int(itemId)] = count
	}
	if !items.RewardItem(player.Uid, itemMap, constValue.Reward_Rid_home, "GameOverHandler") {
		// 内存已删,发奖失败只打日志便于GM补单,不再写回内存(保证幂等)
		log2.Get().Error("[home] [GameOverHandler] RewardItem failed", zap.Int64("uid", player.Uid), zap.Int64("roundId", req.RoundId), zap.Any("reward", itemMap))
		return common.Error(errorCode.ErrorCode_DBError)
	}
	// 5. 回包
	resp.PrecisionExtraGold = int32(precisionExtraGold)
	for itemId, count := range itemMap {
		resp.ItemInfoList = append(resp.ItemInfoList, &pbGo.ItemInfo{
			ItemId: uint64(itemId),
			Count:  count,
		})
	}
	gold := reward[constValue.GoldItemId]
	resp.BillInfo = &pbGo.BillInfo{
		AllBonus: uint32(gold),
		YouBonus: uint32(gold),
	}
	return nil
}

// 2004
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
