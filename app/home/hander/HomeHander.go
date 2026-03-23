package hander

import (
	"context"
	"gameServer/app/home/hander/config"
	"gameServer/app/home/hander/rank"
	"gameServer/common/constValue"
	"gameServer/common/db/heros"
	"gameServer/common/db/items"
	"gameServer/common/db/reward"
	"gameServer/common/errorCode"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/utils"
	"gameServer/protobuf/pbGo"
	"gameServer/service/common"
	"strconv"

	"go.uber.org/zap"
)

type HomeHandler struct { //必须大写,必须使用指针
}

// 2001
func (h *HomeHandler) GetItemInfoHandler(_ context.Context, player *common.Player, req *pbGo.ItemInfoRep, resp *pbGo.ItemInfoResp) *common.ErrorInfo {

	allItems, err := items.GetAllItems(player.UserId)
	if err != nil {
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_GetConfigFailed,
		}
	}
	itemList := make([]*pbGo.ItemInfo, 0, len(allItems))
	for id, count := range allItems {
		itemList = append(itemList, &pbGo.ItemInfo{
			ItemId: uint64(id),
			Count:  count,
		})
	}
	resp.ItemList = itemList
	// 错误做法
	//resp = &pbGo.ItemInfoResp{
	//	ItemList: itemList,
	//}
	return nil
}

// 2002
func (h *HomeHandler) BuyItemHandler(_ context.Context, player *common.Player, req *pbGo.BuyItemRep, resp *pbGo.BuyItemResp) *common.ErrorInfo {
	if req.Item == nil {
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_ReqIsNull,
		}
	}
	item := config.GetItemConfigById(int(req.Item.ItemId))
	if item == nil {
		log2.Get().Error("GetItemConfigById is null ", zap.Any("address", req.Item.ItemId))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_GetConfigFailed,
		}
	}

	verify := items.VerifyItem(player.UserId, item.Price)
	if !verify {
		log2.Get().Debug("BuyItem false ", zap.Uint64("UserId", player.UserId), zap.Uint64("ItemId", req.Item.ItemId))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_ItemNotEnough,
		}
	}
	flag := items.ConsumeItem(player.UserId, item.Price)
	if !flag {
		log2.Get().Error("BuyItem ConsumeItem false ", zap.Uint64("UserId", player.UserId), zap.Any("item.Price", item.Price))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_ItemNotEnough,
		}
	}
	items.RewardItem(player.UserId, map[int]int64{
		int(req.Item.ItemId): req.Item.Count,
	})

	// 错误做法
	//resp = &pbGo.ItemInfoResp{
	//	ItemList: itemList,
	//}
	return nil
}

// 2003
func (h *HomeHandler) BuyHeroHandler(_ context.Context, player *common.Player, req *pbGo.BuyHeroRep, resp *pbGo.BuyHeroResp) *common.ErrorInfo {
	hero := config.GetHeroConfigById(int(req.HeroId))
	if hero == nil {
		log2.Get().Error("GetHeroConfigById is null ", zap.Any("HeroId:", req.HeroId))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_GetConfigFailed,
		}
	}

	ls := heros.GetAllUnLockCharacter(player.UserId)
	if ls == nil {
		log2.Get().Debug("BuyHeroRepHandler GetAllUnLockCharacter fail", zap.Uint64("UserId", player.UserId))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_DBError,
		}
	}
	for _, info := range ls {
		if info.Id == int(req.HeroId) {
			return &common.ErrorInfo{
				Code: errorCode.ErrorCode_RepeatBuyHero,
			}
		}
	}

	verify := items.VerifyItem(player.UserId, hero.Price)
	if !verify {
		log2.Get().Debug("BuyHero false ", zap.Uint64("UserId", player.UserId), zap.Uint32("ItemId", req.HeroId))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_ItemNotEnough,
		}
	}
	ok := items.ConsumeItem(player.UserId, hero.Price)
	if !ok {
		log2.Get().Debug("BuyHeroRepHandler ConsumeItem fail", zap.Uint64("UserId", player.UserId))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_ItemNotEnough,
		}
	}

	ok = heros.UnLockCharacter(player.UserId, []int{int(req.HeroId)})
	if !ok {
		log2.Get().Debug("BuyHeroRepHandler ConsumeItem fail", zap.Uint64("UserId", player.UserId))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_DBError,
		}
	}

	heroInfoList := make([]*pbGo.HeroInfo, 0, len(ls))
	for _, info := range ls {
		heroInfoList = append(heroInfoList, &pbGo.HeroInfo{
			HeroId: uint32(info.Id),
			Unlock: true,
		})
	}

	resp.HeroInfoList = heroInfoList
	// 错误做法
	//resp = &pbGo.ItemInfoResp{
	//	ItemList: itemList,
	//}
	return nil
}

// 2004
func (h *HomeHandler) ReceiveAwardHandler(_ context.Context, player *common.Player, req *pbGo.ReceiveAwardReq, resp *pbGo.ReceiveAwardResp) *common.ErrorInfo {
	awardConfig := config.GetReceiveAwardConfigById(int(req.Id))

	if awardConfig == nil {
		log2.Get().Error("GetReceiveAwardConfigById is null ", zap.Any("id:", req.Id))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_GetConfigFailed,
		}
	}

	rewardInfo := reward.GetAllRewardInfo(player.UserId)
	timestamp := int64(0)
	if rewardInfo != nil { //直接奖励
		timestamp = rewardInfo[int(req.Id)].Timestamp
	}

	infos := make([]*pbGo.ItemInfo, 0)
	if awardConfig.RewardType == 1 || awardConfig.RewardType == 2 {
		if timestamp > 0 {
			return &common.ErrorInfo{
				Code: errorCode.ErrorCode_AlreadyReward,
			}
		}

		ok := reward.SaveRewardInfo(player.UserId, int(req.Id))
		if !ok {
			log2.Get().Error(" save SaveRewardInfo is false ", zap.Any("id:", req.Id))
			return &common.ErrorInfo{
				Code: errorCode.ErrorCode_DBError,
			}
		}
		ok = items.RewardItem(player.UserId, awardConfig.RewardMap)
		if !ok {
			log2.Get().Error(" RewardItem is false ", zap.Any("RewardMap:", awardConfig.RewardMap))
			return &common.ErrorInfo{
				Code: errorCode.ErrorCode_DBError,
			}
		}

		for id, count := range awardConfig.RewardMap {
			infos = append(infos, &pbGo.ItemInfo{
				ItemId: uint64(id),
				Count:  count,
			})
		}

	} else if awardConfig.RewardType == 3 { //每天
		rewardTimestamp := int64(0)
		if rewardInfo != nil {
			rewardTimestamp = rewardInfo[int(req.Id)].Timestamp
		}
		isToday := utils.IsToday(rewardTimestamp)
		if !isToday {
			ok := reward.SaveRewardInfo(player.UserId, int(req.Id))
			if !ok {
				log2.Get().Error(" save RewardDb is false ", zap.Any("id:", req.Id))
				return &common.ErrorInfo{
					Code: errorCode.ErrorCode_DBError,
				}
			}
			ok = items.RewardItem(player.UserId, awardConfig.RewardMap)
			if !ok {
				log2.Get().Error(" SaveRewardInfo is false ", zap.Any("RewardMap:", awardConfig.RewardMap))
				return &common.ErrorInfo{
					Code: errorCode.ErrorCode_DBError,
				}
			}

			for id, count := range awardConfig.RewardMap {
				infos = append(infos, &pbGo.ItemInfo{
					ItemId: uint64(id),
					Count:  count,
				})
			}

		} else {
			return &common.ErrorInfo{
				Code: errorCode.ErrorCode_AlreadyReward,
			}
		}
	}
	resp.ItemList = infos
	return nil
}

// 2005
func (h *HomeHandler) RankInfoHandler(_ context.Context, player *common.Player, req *pbGo.RankInfoReq, resp *pbGo.RankInfoResp) *common.ErrorInfo {
	//inits.RankGold()
	topUsers, topScores, err := rank.GetRankServer("").GetTopN(500)
	if err != nil {
		log2.Get().Error("GetRankServer fail", zap.Error(err))
		return &common.ErrorInfo{
			Code: errorCode.ErrorCode_GetConfigFailed,
		}
	}
	userInfoList := make([]*pbGo.UserInfo, 0, len(topUsers))
	for i, userIdStr := range topUsers {
		oneUserId, err := strconv.ParseUint(userIdStr, 10, 64)
		if err != nil {
			continue
		}
		oneScores := topScores[i]
		userInfoList = append(userInfoList, &pbGo.UserInfo{
			UserId: oneUserId,
			Index:  uint32(i),
			Item: &pbGo.ItemInfo{
				ItemId: uint64(constValue.GoldItemId),
				Count:  oneScores,
			},
		})
	}

	resp.UserInfoList = userInfoList
	return nil
}
