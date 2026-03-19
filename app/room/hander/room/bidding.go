package room

//
//import (
//	"gameServer/app/room/hander/config"
//	"gameServer/app/room/hander/maxRects"
//	"gameServer/common/constValue"
//	"gameServer/common/db/items"
//	"gameServer/common/errorCode"
//	"gameServer/pkg/logger/log2"
//	"gameServer/protobuf/pbGo"
//	"gameServer/protobuf/protoHandlerInit"
//	"gameServer/service/services/node"
//	"math/rand"
//	"time"
//
//	"go.uber.org/zap"
//	"golang.org/x/tools/container/intsets"
//)
//
//var (
//	opAbstain int8 = -1
//	opWaiting int8 = 0
//	opBet     int8 = 1
//)
//
//// 玩家操作
//type Operation struct {
//	userId    uint64
//	consume   map[int]int64 // 消耗的道具，或在竞拍的钱
//	operation int8          // 玩家操作，-1：弃权  0：等待玩家操作  1:发牌
//}
//
//type Hint struct {
//	id        uint32 // 提示id
//	HintType  uint32 // 0：人物提示，1：词条提示
//	Value     int64  //总价值,平均价值,的值等
//	ValueType uint32 //0：没有值，1：总价值,2:平均价值等,的值等
//}
//
//// 玩家操作
//func PlayerOperate(op *Operation) bool {
//	room := FindHasJoinedRoom(op.userId)
//	if room == nil {
//		return false
//	}
//	if room.roomStatus != Playing { //游戏开始才能操作
//		return false
//	}
//
//	matchPool.Submit(func() {
//		if room.roundList == nil || room.roundList.len() == 0 {
//			return
//		}
//
//		//当前局
//		curRound := room.roundList.getCurRound()
//		curRound.addOp(op)
//	})
//	return true
//}
//
//// 下一局或者创建一局
//func (room *Room) nextRound(roomConfig *config.Room) *round {
//	//log2.Get().Warn("============= nextRound start==========================", zap.Int32("roomId=", room.Id), zap.String("=========================", "======="))
//
//	// 0,1,2,3,4
//	index := room.roundList.len()
//	nowTime := time.Now().Unix()
//	ops := make([]*Operation, 0, roomConfig.CapacityLimit)
//	curRound := &round{
//		operations: ops,
//		index:      uint8(index),
//		timeOut:    nowTime + roomConfig.Timeout,
//		creatTime:  nowTime,
//	}
//
//	if len(room.roundList.roundList) == 0 { //首轮
//		for _, info := range room.playerInfos.playerInfos {
//			ops = append(ops, &Operation{
//				userId:    info.player.UserId,
//				operation: opWaiting,
//			})
//		}
//	} else {
//		lastRound := room.roundList.getCurRound()
//		for _, info := range lastRound.operations {
//			op := &Operation{
//				userId:  info.userId,
//				consume: make(map[int]int64),
//			}
//			if info.operation == opBet {
//				info.operation = opWaiting
//			} else {
//				info.operation = opAbstain
//				info.consume = nil
//			}
//			op.operation = info.operation
//			ops = append(ops, op)
//		}
//
//	}
//
//	curRound.operations = ops
//	room.roundList.addRound(curRound)
//	//log2.Get().Warn("=============nextRound==========================", zap.Int32("roomId=", room.Id), zap.Uint8("Roundindex=", curRound.index), zap.String("=============================================", "======="))
//	return curRound
//}
//
//// 玩家超时操作检查
//func (room *Room) checkRoom(roomConfig *config.Room) {
//	if room.roomStatus != Playing { //游戏中的房间才检查
//		return
//	}
//	now := time.Now().Unix()
//	curRound := room.roundList.getCurRound()
//	curRound.lock.RLock()
//	defer curRound.lock.RUnlock()
//
//	opSum := 0
//	abstainSum := 0
//
//	// 判断是否所有玩家操作完成
//	for _, op := range curRound.operations {
//		if op.operation == opWaiting && now > curRound.timeOut {
//			op.operation = opAbstain // 弃权
//			opSum++
//			abstainSum++
//		} else if op.operation == opBet { //竞拍
//			opSum++
//		} else if op.operation == opAbstain { //弃权直接跳过
//			opSum++
//		}
//	}
//
//	room.lock.Lock()
//	defer room.lock.Unlock()
//	// 判断是否所有玩家操作完成,开启下一局
//	if opSum == len(curRound.operations) && room.roomStatus == Playing { //是否开启下一轮
//		finish := false
//		if opSum-abstainSum <= 1 || uint8(room.roundList.len()) >= roomConfig.RoundLimit { //剩下最多一人，或者最后轮数时，直接结束比赛
//			finish = true
//		}
//
//		// 是否提前结束
//		index := curRound.index
//		termination := 0
//		// 3 : 2+1
//		if len(roomConfig.EarlyTermination)-1 >= int(index) { //不判断
//			termination = roomConfig.EarlyTermination[index]
//		}
//		max1 := int64(0)
//		max2 := int64(0)
//		for _, op := range curRound.operations {
//			if op.consume != nil {
//				for _, count := range op.consume {
//					if count > max1 {
//						// 当前值大于最大值，更新 max1 和 max2
//						max2 = max1  // 原来的最大值变成第二大值
//						max1 = count // 当前值成为新的最大值
//					} else if count > max2 && count != max1 {
//						// 当前值介于 max2 和 max1 之间，且不等于 max1
//						max2 = count
//					}
//				}
//			}
//		}
//		if termination > 0 {
//			if 100*int(max1)-int(max2)*(100+termination) >= 0 {
//				finish = true
//			}
//		}
//
//		// 产生下一轮 推送
//		if !finish {
//			curRound = room.nextRound(roomConfig)
//		}
//		isFinish := room.pushRoomInfo(curRound, roomConfig, finish)
//		log2.Get().Info("=============curRound==========================", zap.Int32("roomId=", room.Id), zap.Bool("isFinish=", isFinish), zap.Uint8("Roundindex=", curRound.index), zap.String("=========================", "======="))
//		//if !isFinish {
//		//	// 产生下一轮
//		//	room.nextRound(roomConfig)
//		//}
//	}
//
//}
//
//// 推送信息
//func (room *Room) pushRoomInfo(curRound *round, roomConfig *config.Room, isFinish bool) bool {
//
//	var (
//		roomSettlementInfo *pbGo.RoomSettlementInfo
//		sumValue           = int64(0)
//		targetPlayer       *Operation
//	)
//
//	// 结算处理
//	if isFinish {
//		roomSettlementInfo = &pbGo.RoomSettlementInfo{}
//		itemInfo := &pbGo.ItemInfo{}
//		betInfo := make([]*pbGo.ItemInfo, 0, 1)
//
//		maxV := int64(0)
//		for _, info := range curRound.operations {
//			for k, v := range info.consume {
//				if k == constValue.GoldItemId {
//					if v > maxV {
//						itemInfo.ItemId = uint64(k)
//						itemInfo.Count = v
//						targetPlayer = info
//						maxV = v
//					}
//				}
//			}
//		}
//
//		if targetPlayer != nil {
//			heroId := uint32(0)
//			for _, info := range room.playerInfos.playerInfos {
//				if info.player.UserId == targetPlayer.userId {
//					heroId = info.heroId
//				}
//			}
//			betInfo = append(betInfo, itemInfo)
//
//			// 利润
//			for _, info := range *room.gridInfo {
//				one := config.GetItemConfigById(info.Item.Id)
//				if one == nil {
//					log2.Get().Error("GetItemConfigById failed ", zap.Any("constValue", info.Item.Id))
//					continue
//				}
//				for id, value := range one.Price {
//					if id == constValue.GoldItemId {
//						sumValue += value
//					}
//				}
//			}
//
//			// 响应
//			roomSettlementInfo.PlayerInfo = &pbGo.PlayerInfo{
//				UserId:  targetPlayer.userId,
//				HeroId:  heroId,
//				BetInfo: betInfo,
//			}
//			profit := &pbGo.ItemInfo{
//				ItemId: uint64(constValue.GoldItemId),
//				Count:  sumValue - itemInfo.Count,
//			}
//			roomSettlementInfo.Expenses = itemInfo
//			roomSettlementInfo.Profit = profit
//		}
//
//	}
//
//	// 发送消息
//	for _, info := range room.playerInfos.playerInfos {
//		if info.playerType > 0 {
//			continue
//		}
//		if info.status == playerStatusLeave {
//			log2.Get().Info("=======发送消息终止！！！========", zap.Int32("roomId=", room.Id), zap.Bool("isFinish=", isFinish), zap.Uint8("Roundindex=", curRound.index), zap.String("=========================", "======="))
//			continue
//		}
//
//		var (
//			changeScreenInfo = &pbGo.ScreenInfo{}
//			hintList         []*pbGo.Hint // todo
//			userId           = info.player.UserId
//		)
//
//		// last
//
//		//词条能力
//		for _, index := range roomConfig.EntryRound {
//			if curRound.index == index {
//
//				hintList = make([]*pbGo.Hint, 0, 2)
//
//				// 需要排重
//				entryAbilityId := uint32(roomConfig.EntryList[rand.Intn(len(roomConfig.EntryList))])
//				//entry := config.GetEntryConfigById(entryId)
//				//if entry == nil {
//				//	log2.Get().Error("GetEntryConfigById failed ", zap.Any("entryId", entryId))
//				//	return
//				//}
//				//词条能力
//				entryAbility := config.GetAbilityConfigById(entryAbilityId)
//				if entryAbility == nil {
//					log2.Get().Error("GetAbilityConfigById failed ", zap.Any("entryAbility", entryAbilityId))
//					continue
//				}
//				hint := room.applyAbility(entryAbility, userId, 1)
//				hintList = append(hintList, &pbGo.Hint{
//					Id:        entryAbilityId,
//					HintType:  hint.HintType,
//					Value:     uint64(hint.Value),
//					ValueType: hint.ValueType,
//				})
//				// todo 开始，计算每一个玩家
//				break
//			}
//		}
//
//		// 人物的能力
//		abilityList := make([]*config.Ability, 0)
//		hero := config.GetHeroConfigById(info.heroId)
//		if hero == nil {
//			log2.Get().Error("GetHeroConfigById failed, hero is null ", zap.Any("heroId", info.heroId))
//		} else {
//			for roundIndex, abilitys := range hero.RAsMap {
//				if roundIndex != curRound.index { //排除不生效的回合
//					continue
//				}
//				for _, id := range abilitys {
//					ability := config.GetAbilityConfigById(id)
//					if ability == nil {
//						log2.Get().Error("GetAbilityConfigById failed ", zap.Any("abilityId", id))
//						continue
//					}
//					abilityList = append(abilityList, ability)
//				}
//			}
//		}
//
//		// 当前回合生效的能力，计算一个玩家,一个玩家一回合可能有多个能力
//		for _, oneA := range abilityList {
//			// 提示
//			hint := room.applyAbility(oneA, userId, 0)
//			hintList = append(hintList, &pbGo.Hint{
//				Id:        oneA.Id,
//				HintType:  hint.HintType,
//				Value:     uint64(hint.Value),
//				ValueType: hint.ValueType,
//			})
//		}
//
//		var (
//			g        = room.gridInfo
//			gridInfo = *g //所有物品
//		)
//		buildChangeScreenInfo(changeScreenInfo, gridInfo, userId)
//
//		//
//
//		// 填充上回合的玩家信息
//		// 竞拍金额
//		var betInfo []*pbGo.ItemInfo
//		lastRound := room.roundList.getLastRound()
//		if lastRound != nil {
//			betInfo = make([]*pbGo.ItemInfo, 0)
//			for _, pInfo := range lastRound.operations {
//				for id, v := range pInfo.consume {
//					if id == constValue.GoldItemId {
//						betInfo = append(betInfo, &pbGo.ItemInfo{
//							ItemId: uint64(id),
//							Count:  v,
//						})
//					}
//				}
//			}
//		}
//
//		// 玩家信息
//		heroId := uint32(0)
//		for _, playerInfo := range room.playerInfos.playerInfos {
//			if playerInfo.player.UserId == userId {
//				heroId = playerInfo.heroId
//			}
//		}
//
//		changeScreenInfo.PlayerInfoList = append(changeScreenInfo.PlayerInfoList, &pbGo.PlayerInfo{
//			UserId:  userId,
//			HeroId:  heroId,
//			BetInfo: betInfo,
//		})
//		log2.Get().Info("=============RoundInfoPush=End=========================", zap.Int32("roomId=", room.Id), zap.Bool("isFinish=", isFinish), zap.Uint8("Roundindex=", curRound.index), zap.String("=========================", "======="))
//		node.Push(info.player, protoHandlerInit.RoundInfoPush, &pbGo.RoundInfoPush{
//			EndTimeOut:       curRound.timeOut,
//			RoundIndex:       uint32(curRound.index + 1),
//			IsFinish:         isFinish,
//			ChangeScreenInfo: changeScreenInfo,
//			HintList:         hintList,
//			SettlementInfo:   roomSettlementInfo,
//		})
//	}
//
//	// 结算奖励
//	if isFinish {
//		if targetPlayer != nil {
//			items.RewardItem(targetPlayer.userId, map[int]int64{
//				int(roomSettlementInfo.Profit.ItemId): sumValue,
//			})
//
//			items.ConsumeItem(targetPlayer.userId, map[int]int64{
//				int(roomSettlementInfo.Expenses.ItemId): roomSettlementInfo.Expenses.Count,
//			})
//		}
//		room.roomStatus = End
//	}
//
//	return isFinish
//
//}
//
//func buildChangeScreenInfo(changeScreenInfo *pbGo.ScreenInfo, gridInfo []*maxRects.Placement, userId uint64) {
//	changeScreenInfo.GridList = make([]*pbGo.Grid, 0)
//	changeScreenInfo.AllGoods = make([]*pbGo.Goods, 0)
//
//	for _, p := range gridInfo {
//		showInfo := p.ShowInfoMap[userId]
//		if showInfo == nil {
//			continue
//		}
//		// 品质变化
//		quality := showInfo.Quality
//		if quality == nil {
//			continue
//		}
//		for index, qInfo := range quality {
//			changeScreenInfo.GridList = append(changeScreenInfo.GridList, &pbGo.Grid{
//				IndexId:     index,
//				ShowQuality: uint32(qInfo.RoundIndex),
//				QualityType: uint32(qInfo.QualityType),
//			})
//		}
//		// 轮廓
//		changeScreenInfo.AllGoods = append(changeScreenInfo.AllGoods, &pbGo.Goods{
//			ItemId:      uint32(p.Item.Id),
//			Index:       getIndexByXY(p.StartX, p.StartY),
//			ShowAll:     uint32(showInfo.All),
//			ShowContour: uint32(showInfo.Contour),
//		})
//	}
//}
//
//// 应用能力
//func (room *Room) applyAbility(info *config.Ability, userId uint64, hintType uint32) *Hint {
//	gridInfo := room.gridInfo
//	curRound := room.roundList.getCurRound()
//	var h = &Hint{}
//
//	// 标志
//	gInfo := *gridInfo
//	flag := map[int]struct{}{}
//	for _, g := range gInfo {
//		flag[g.Uid] = struct{}{}
//	}
//	// 条件类：
//	// 前提条件过滤
//	cPrePQ(gridInfo, flag, info.CPrePQ, userId)
//	//面积过滤
//	flag = cArea(gridInfo, flag, info.CArea)
//	// 品质过滤
//	flag = cQualityType(gridInfo, flag, info.CQualityType)
//	// 道具类型过滤
//	flag = cItemType(gridInfo, flag, info.CItemType)
//	// 道具类型数量过滤
//	flag = cItemTypeCount(gridInfo, flag, info.CItemTypeCount)
//	// 道具的价值过滤
//	flag = cPrice(gridInfo, flag, info.CPrice)
//
//	//显示类
//	// 显示数量过滤
//	flag = showItemCount(flag, info.ShowItemCount)
//	// todo 以后会不会叠加？？
//	showPrice(gridInfo, flag, uint32(info.ShowPrice), h)
//	showGridCount(gridInfo, flag, uint32(info.ShowGridCount), h)
//	showQCA(gridInfo, flag, info.ShowQuality, info.ShowContour, info.ShowAll, userId, int8(curRound.index))
//	h.HintType = hintType
//
//	return h
//}
//
//// 使用功能道具
//func (room *Room) UserItem(userId uint64, ItemId int, count int64) (*pbGo.ScreenInfo, uint16) {
//	if count != 1 {
//		return nil, errorCode.ErrorCode_UseItemNumErr
//	}
//	reusing := room.roundList.isReusing(userId, ItemId)
//	if reusing {
//		return nil, errorCode.ErrorCode_ReusingItem
//	}
//	curRound := room.roundList.getCurRound()
//	for _, p := range curRound.operations {
//		if p.userId == userId {
//			if p.consume == nil {
//				p.consume = make(map[int]int64)
//			}
//			p.consume[ItemId] = count
//			item := config.GetItemConfigById(ItemId)
//			if item == nil {
//				log2.Get().Error("GetItemConfigById failed ", zap.Any("itemId", ItemId))
//				return nil, errorCode.ErrorCode_GetConfigFailed
//			}
//			abilityId := item.Ability
//			if abilityId <= 0 {
//				return nil, errorCode.ErrorCode_ConfigError
//			}
//			ability := config.GetAbilityConfigById(abilityId)
//			if ability == nil {
//				log2.Get().Error("GetAbilityConfigById failed ", zap.Any("abilityId", abilityId))
//				return nil, errorCode.ErrorCode_GetConfigFailed
//			}
//			room.applyAbility(ability, userId, 0)
//
//			changeScreenInfo := &pbGo.ScreenInfo{}
//			buildChangeScreenInfo(changeScreenInfo, *room.gridInfo, userId)
//			return changeScreenInfo, 0
//		}
//	}
//	return nil, errorCode.ErrorCode_Success
//}
