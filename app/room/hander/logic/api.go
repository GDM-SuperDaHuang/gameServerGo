package logic

import (
	"fmt"
	"gameServer/app/room/hander/config"
	"gameServer/app/room/hander/maxRects"
	"gameServer/common/constValue"
	"gameServer/pkg/logger/log2"
	"gameServer/protobuf/pbGo"
	"time"

	"golang.org/x/exp/rand"
	"golang.org/x/tools/container/intsets"
)

const (
	roomHint = 1
	heroHint = 0
	itemHint = 2

	receiveAwardId = 2000
)

type Hint struct {
	HintType uint32  // 0：人物提示，1：词条提示
	Value    float64 //总价值,平均价值,的值等
}

type ElkOp struct {
	Uid       int64
	GoldValue int64 // 消耗的钱的值
	Operation int64 // 玩家操作， 0：等待玩家操作  1:发牌,2：弃权
	ItemId    int64 //使用道具,道具id

}

type ElkGoods struct {
	Uid  int64
	Good []*pbGo.Goods
}

var (
	endTime22 = int64(0)
)

func TestInit222() {

	// 按房间类型分桶
	for {
		select {
		// 定时撮合
		case <-tickerTest.C:
			ttt := endTime22 - time.Now().Unix()
			fmt.Printf("push ---剩余时间，===========: %d\n", ttt)
		}
	}
}

func (room *Room) ApplyAbilityTest(info *config.Ability, Uid int64, hintType uint32) *Hint {
	gridInfo := room.gridInfo
	var h = &Hint{}

	// 标志
	gInfo := *gridInfo

	count := 0
	qualityCount := 0
	for _, in := range gInfo {
		if in.Item.ItemType == 8 {
			count++
		}
		if in.Item.Quality == 4 {
			qualityCount++
		}
	}

	flag := map[int]struct{}{}
	for _, g := range gInfo {
		flag[g.Uid] = struct{}{}
	}
	// 条件类：
	// 前提条件过滤
	cPrePQ(gridInfo, flag, info.CPrePQ, Uid)
	//面积过滤
	flag = cArea(gridInfo, flag, info.CArea)
	// 品质过滤
	flag = cQualityType(gridInfo, flag, info.CQualityType)
	// 道具类型过滤
	flag = cItemType(gridInfo, flag, info.CItemType)
	// 道具或关系 类型过滤
	flag = cItemTypeOr(gridInfo, flag, info.CItemTypeOr)
	// 道具类型数量过滤
	flag = cItemTypeCount(gridInfo, flag, info.CItemTypeCount)
	// 道具的价值过滤
	flag = cPrice(gridInfo, flag, info.CPrice)

	//显示类
	// 显示数量过滤
	flag = showItemCount(Uid, gridInfo, flag, info.ShowItemCount, 1, info.ShowQuality, info.ShowContour, info.ShowAll)
	showPrice(gridInfo, flag, uint32(info.ShowValue), h)
	showGridCount(gridInfo, flag, uint32(info.ShowGridCount), h)
	h.HintType = hintType
	//h.id = info.Id
	return h
}

// 过滤器
// 显示数量
// 0:所有
func showItemCount(Uid int64, gridInfo *[]*maxRects.Placement, flag map[int]struct{}, showSum int8, curRoundIndex int8, q, c, a bool) map[int]struct{} {
	maxLen := int8(len(flag))
	if maxLen == 0 {
		return flag
	}

	if showSum == 0 { //不限制
		showSum = maxLen
	} else if maxLen < showSum {
		showSum = maxLen
	}

	// 收集所有的可能
	qLs := make([]int, 0, showSum)
	cLs := make([]int, 0, showSum)
	aLs := make([]int, 0, showSum)

	// 可选择的Q,C,A
	info := *gridInfo
	for _, p := range info {
		if p.ShowInfoMap == nil {
			p.ShowInfoMap = make(map[int64]*maxRects.ShowInfo)
		}

		showInfo := p.ShowInfoMap[Uid]
		if showInfo == nil {
			showInfo = &maxRects.ShowInfo{}
			p.ShowInfoMap[Uid] = showInfo
		}

		if _, ok := flag[p.Uid]; !ok {
			continue
		}

		// 收集
		if showInfo.Quality == nil {
			qLs = append(qLs, p.Uid)
		}
		if showInfo.Contour == 0 {
			cLs = append(cLs, p.Uid)
		}
		if showInfo.All == 0 {
			aLs = append(aLs, p.Uid)
		}

	}

	// 在进行随机选择
	qFlag := getFlag(qLs, showSum)
	cFlag := getFlag(cLs, showSum)
	aFlag := getFlag(aLs, showSum)
	qcFlag := flagUFlag(qFlag, cFlag, showSum)
	// 填充颜色

	if q && c {
		for _, p := range info {
			if _, ok := qcFlag[p.Uid]; !ok {
				continue
			}
			showInfo := p.ShowInfoMap[Uid]
			if showInfo.Quality == nil {
				index := randIndexByXY(p.StartX, p.EndX, p.StartY, p.EndY)
				showInfo.Quality = &maxRects.Quality{
					Index:       index,
					RoundIndex:  curRoundIndex,
					QualityType: p.Item.Quality,
				}
			}
			//c
			showInfo.Contour = curRoundIndex
		}
		return qcFlag
	} else if q {
		for _, p := range info {
			if _, ok := qFlag[p.Uid]; !ok {
				continue
			}

			showInfo := p.ShowInfoMap[Uid]
			if showInfo.Quality == nil {
				index := randIndexByXY(p.StartX, p.EndX, p.StartY, p.EndY)
				showInfo.Quality = &maxRects.Quality{
					Index:       index,
					RoundIndex:  curRoundIndex,
					QualityType: p.Item.Quality,
				}
			}
			p.ShowInfoMap[Uid] = showInfo

		}
		return qFlag

	} else if c {
		for _, p := range info {
			if _, ok := cFlag[p.Uid]; !ok {
				continue
			}
			showInfo := p.ShowInfoMap[Uid]
			showInfo.Contour = curRoundIndex
			p.ShowInfoMap[Uid] = showInfo
		}
		return cFlag
	} else if a {
		for _, p := range info {
			if _, ok := aFlag[p.Uid]; !ok {
				continue
			}
			showInfo := p.ShowInfoMap[Uid]
			showInfo.All = curRoundIndex
			p.ShowInfoMap[Uid] = showInfo

		}
		return aFlag
	}

	return flag
}

func getFlag(ls []int, showSum int8) map[int]struct{} {
	flag := make(map[int]struct{}, showSum)
	maxLen := len(ls)
	if maxLen == 0 {
		return flag
	}
	forCount := 1000
	repeatFlag := make(map[int]struct{}, showSum)
	for i := int8(0); i < showSum; i++ {
		for {
			forCount--
			index := rand.Intn(maxLen)

			if _, ok := repeatFlag[index]; !ok {
				repeatFlag[index] = struct{}{}
				uid := ls[index]
				flag[uid] = struct{}{}
				break
			}
			if len(flag) >= maxLen { //满了
				break
			}
			if forCount < 0 {
				log2.Get().Error("***可能无限循环***")
				break
			}
		}
	}
	return flag
}

func flagUFlag(f1, f2 map[int]struct{}, showSum int8) map[int]struct{} {
	flag := make(map[int]struct{}, len(f1))
	for k, v := range f1 {
		flag[k] = v
	}
	for k, v := range f2 {
		flag[k] = v
	}
	delCount := int8(len(flag)) - showSum
	if delCount > 0 {
		for k, _ := range flag {
			delete(flag, k)
			delCount--
			if delCount <= 0 {
				break
			}
		}
	}

	return flag
}

// 0：不显示价值
// 1:总价值
// 2:平均价值
func showPrice(gridInfo *[]*maxRects.Placement, flag map[int]struct{}, showType uint32, h *Hint) {
	if showType == 0 {
		return
	}
	//h.ValueType = showType
	info := *gridInfo

	sumValue := int64(0)
	for _, p := range info {
		_, ok := flag[p.Uid]
		if !ok {
			continue
		}
		price := p.Item.Price
		sumValue += price
	}
	if showType == 1 {
		h.Value = float64(sumValue)
	} else if showType == 2 {
		h.Value = float64(sumValue) / float64(len(info))
	} else if showType == 3 {
		h.Value = float64(len(info))
	}
}

// 0:不显示
// 1:平均格子数量，
// 2：所有的格子数量
func showGridCount(gridInfo *[]*maxRects.Placement, flag map[int]struct{}, showType uint32, h *Hint) {
	if showType == 0 {
		return
	}
	//h.ValueType = showType
	info := *gridInfo

	sumValue := int64(0)
	for _, p := range info {
		if _, ok := flag[p.Uid]; !ok {
			continue
		}
		tempArea := (p.EndX - p.StartX) * (p.EndY - p.StartY)
		sumValue += int64(tempArea)
	}

	if showType == 2 {
		h.Value = float64(sumValue)
	} else if showType == 1 {
		h.Value = float64(sumValue) / float64(len(info))
	}
}

// 面积
// -1:最大面积，n：具体面积
func cArea(gridInfo *[]*maxRects.Placement, flag map[int]struct{}, areaValue int8) map[int]struct{} {
	info := *gridInfo
	var (
		areaTypeMap = make(map[int8][]int) // 具体面积-info
	)
	if areaValue == 0 {
		return flag
	}

	for _, p := range info {
		if _, ok := flag[p.Uid]; !ok {
			continue
		}
		tempArea := getAreaByXY(p.StartX, p.EndX, p.StartY, p.EndY)
		ls, ok := areaTypeMap[tempArea]
		if !ok {
			ls = make([]int, 0)
		}
		ls = append(ls, p.Uid)
		areaTypeMap[tempArea] = ls
	}

	//重置
	if areaValue == -1 { //最大格子
		maxArea := int8(0)
		for area, _ := range areaTypeMap {
			if area >= maxArea {
				maxArea = area
			}
		}
		ls := areaTypeMap[maxArea]
		flag = make(map[int]struct{}, len(ls))
		for _, uid := range ls {
			flag[uid] = struct{}{}
		}
	} else { // 指定大小
		ls, ok := areaTypeMap[areaValue]
		if !ok {
			return nil
		}
		flag = make(map[int]struct{}, len(ls))
		for _, uid := range ls {
			flag[uid] = struct{}{}
		}
		return flag
	}

	return flag
}

// 品质
// -1：最高品质
// 0:随机品质
// 1：白色
// 2：绿色
func cQualityType(gridInfo *[]*maxRects.Placement, flag map[int]struct{}, qualityType int8) map[int]struct{} {
	info := *gridInfo
	var (
		typeMap map[int8][]int // 品质-info
	)

	if qualityType == 0 {
		return flag
	}

	for _, p := range info {
		_, ok := flag[p.Uid]
		if !ok {
			continue
		}
		if p.Item.Quality != qualityType {
			continue
		}

		if typeMap == nil {
			typeMap = make(map[int8][]int)
		}

		ls, ok := typeMap[p.Item.Quality]
		if !ok {
			ls = make([]int, 0)
		}
		ls = append(ls, p.Uid)
		typeMap[p.Item.Quality] = ls
	}
	if typeMap == nil {
		return nil
	}

	flag = make(map[int]struct{}, len(typeMap))
	for _, uid := range typeMap[qualityType] {
		flag[uid] = struct{}{}
	}

	return flag
}

// 品质
// -1：最高品质
// 0:随机品质
// 1：白色
// 2：绿色
func cQualityType1(gridInfo *[]*maxRects.Placement, flag map[int]struct{}, qualityTypeLs []int8) map[int]struct{} {
	info := *gridInfo
	var (
		typeLs []int // 品质-info
	)

	if qualityTypeLs == nil {
		return flag
	}

	qualityTypeFlag := make(map[int8]struct{}, len(qualityTypeLs))
	for _, q := range qualityTypeLs {
		qualityTypeFlag[q] = struct{}{}
	}

	for _, p := range info {
		_, ok := flag[p.Uid]
		if !ok {
			continue
		}

		quality := p.Item.Quality
		if _, ok = qualityTypeFlag[quality]; !ok {
			continue
		}

		if typeLs == nil {
			typeLs = make([]int, 0)
		}
		typeLs = append(typeLs, p.Uid)
	}

	if typeLs == nil {
		return nil
	}

	flag = make(map[int]struct{}, len(typeLs))
	for _, uid := range typeLs {
		flag[uid] = struct{}{}
	}

	return flag
}

// 道具类型
// 0：随机类型，
// n：具体道具类型，如，1,2,3,4
func cItemType(gridInfo *[]*maxRects.Placement, flag map[int]struct{}, itemType int8) map[int]struct{} {
	info := *gridInfo
	var (
		typeMap map[int8][]int // 类型-info
	)
	if itemType == 0 {
		return flag
	}

	for _, p := range info {
		_, ok := flag[p.Uid]
		if !ok {
			continue
		}
		if p.Item.ItemType != itemType {
			continue
		}
		if typeMap == nil {
			typeMap = make(map[int8][]int)
		}

		ls, ok := typeMap[p.Item.ItemType]
		if !ok {
			ls = make([]int, 0)
		}
		ls = append(ls, p.Uid)
		typeMap[p.Item.ItemType] = ls
	}

	if typeMap == nil {
		return nil
	}

	flag = make(map[int]struct{}, len(typeMap))
	for _, uid := range typeMap[itemType] {
		flag[uid] = struct{}{}
	}

	return flag
}

func cItemTypeOr(gridInfo *[]*maxRects.Placement, flag map[int]struct{}, itemType []int8) map[int]struct{} {
	info := *gridInfo
	var (
		typeMap map[int8][]int // 类型-info
	)
	if len(itemType) == 0 {
		return flag
	}

	for _, p := range info {
		_, ok := flag[p.Uid]
		if !ok {
			continue
		}

		f := false
		for _, t := range itemType {
			if p.Item.ItemType == t {
				f = true
				break
			}
		}
		if !f {
			continue
		}

		if typeMap == nil {
			typeMap = make(map[int8][]int)
		}

		ls, ok := typeMap[p.Item.ItemType]
		if !ok {
			ls = make([]int, 0)
		}
		ls = append(ls, p.Uid)
		typeMap[p.Item.ItemType] = ls
	}

	if typeMap == nil {
		return nil
	}

	flag = make(map[int]struct{}, len(typeMap))

	for _, ls := range typeMap {
		for _, uid := range ls {
			flag[uid] = struct{}{}
		}
	}

	return flag
}

// 道具类型 的 数量
// -1：数量最少的，
// 0：随机类型，
func cItemTypeCount(gridInfo *[]*maxRects.Placement, flag map[int]struct{}, itemType int8) map[int]struct{} {
	info := *gridInfo
	typeMap := make(map[int8][]int) // 类型-info
	if itemType == 0 {
		return flag
	}

	for _, p := range info {
		_, ok := flag[p.Uid]
		if !ok {
			continue
		}
		if itemType != -1 && p.Item.ItemType != itemType {
			continue
		}
		ls, ok := typeMap[p.Item.ItemType]
		if !ok {
			ls = make([]int, 0)
		}
		ls = append(ls, p.Uid)
		typeMap[p.Item.ItemType] = ls
	}

	if itemType == -1 {
		minLen := intsets.MaxInt
		var targetLs []int
		for _, ls := range typeMap {
			if len(ls) < minLen {
				minLen = len(ls)
				targetLs = ls
			}
		}

		flag = make(map[int]struct{}, len(typeMap))
		for _, l := range targetLs {
			flag[l] = struct{}{}
		}
		return flag

	}
	return flag
}

// 道具的价值
// -1:价值最高的
// 0：不参与，随机类型，
func cPrice(gridInfo *[]*maxRects.Placement, flag map[int]struct{}, itemType int8) map[int]struct{} {
	info := *gridInfo
	typeMap := make(map[int64][]int) // 价值-info
	if itemType == 0 {
		return flag
	}
	maxPrice := int64(0)
	for _, p := range info {
		_, ok := flag[p.Uid]
		if !ok {
			continue
		}
		price := p.Item.Price
		if typeMap[p.Item.Price] == nil {
			typeMap[price] = make([]int, 0, 1)
		}

		if maxPrice < price {
			maxPrice = price
			typeMap[maxPrice] = append(typeMap[maxPrice], p.Uid)
		} else if maxPrice == price {
			typeMap[maxPrice] = append(typeMap[maxPrice], p.Uid)
		}
	}

	ls := typeMap[maxPrice]
	flag = make(map[int]struct{}, len(ls))
	for _, uid := range ls {
		flag[uid] = struct{}{}
	}

	return flag
}

// 前提条件
// 0：不限制 1:已经暴露品质的条件下 ,2:已经暴露轮廓的条件下
func cPrePQ(gridInfo *[]*maxRects.Placement, flag map[int]struct{}, prePQType int8, Uid int64) {
	if prePQType == 0 {
		return
	}
	info := *gridInfo
	for _, p := range info {
		showInfo := p.ShowInfoMap[Uid]
		if showInfo == nil {
			showInfo = &maxRects.ShowInfo{}
		}
		if prePQType == 1 && showInfo.Quality != nil {
			flag[p.Uid] = struct{}{}
		} else if prePQType == 2 && showInfo.Contour > 0 {
			flag[p.Uid] = struct{}{}
		}
	}
}

// 推送信息
func (room *Room) pushRoundInfo(roomConfig *config.Room, roomSettlementInfo *pbGo.RoomSettlementInfo) {
	//curRound := room.getCurrentRound()
	//// 发送消息
	//var elkGoods []*ElkGoods
	//
	//uidList := make([]int64, 0)
	//for Uid, info := range room.playerInfos {
	//	uidList = append(uidList, Uid)
	//	if info.playerType > 0 { // 机器人
	//		continue
	//	}
	//	if info.status == PlayerStatusLeave { // 离开的人
	//		continue
	//	}
	//
	//	var (
	//		changeScreenInfo = &pbGo.ScreenInfo{}
	//		hintList         []*pbGo.Hint // todo
	//	)
	//
	//	//
	//
	//	//词条能力
	//	entryAbilityId, ok := room.entryMap[curRound.RoundIndex]
	//	if ok {
	//		entryAbility := config.GetAbilityConfigById(uint32(entryAbilityId))
	//		if entryAbility == nil {
	//			log2.Get().Error("GetAbilityConfigById failed ", zap.Any("entryAbility", entryAbilityId))
	//		} else {
	//			hint := room.applyAbility(entryAbility, Uid, roomHint)
	//			hintList = append(hintList, &pbGo.Hint{
	//				HintType: hint.HintType,
	//				Desc:     desc.GetDesc(entryAbility.Desc, hint.Value),
	//			})
	//		}
	//	}
	//
	//	// 人物的能力
	//	abilityList := make([]*config.Ability, 0)
	//	hero := config.GetHeroConfigById(info.HeroId)
	//	if hero == nil {
	//		log2.Get().Error("GetHeroConfigById failed, hero is null ", zap.Any("HeroId", info.HeroId))
	//	} else {
	//		abList, _ := hero.RAsMap[curRound.RoundIndex]
	//		for _, id := range abList {
	//			ability := config.GetAbilityConfigById(id)
	//			if ability == nil {
	//				log2.Get().Error("GetAbilityConfigById failed ", zap.Any("abilityId", id))
	//				continue
	//			}
	//			abilityList = append(abilityList, ability)
	//		}
	//	}
	//
	//	// 当前回合生效的能力，计算一个玩家,一个玩家一回合可能有多个能力
	//	for _, oneA := range abilityList {
	//		// 提示
	//		hint := room.applyAbility(oneA, Uid, heroHint)
	//		hintList = append(hintList, &pbGo.Hint{
	//			HintType: hint.HintType,
	//			Desc:     desc.GetDesc(oneA.Desc, hint.Value),
	//		})
	//	}
	//
	//	var (
	//		g        = room.gridInfo
	//		gridInfo = *g //所有物品
	//	)
	//
	//	//
	//
	//	// 填充上回合的玩家信息
	//	// 竞拍金额
	//	var betInfo []*pbGo.ItemInfo
	//	lastRound := room.getLastRound()
	//	if lastRound != nil {
	//
	//		for oneUid, pInfo := range lastRound.Op {
	//			betInfo = make([]*pbGo.ItemInfo, 0, 2)
	//			betInfo = append(betInfo, &pbGo.ItemInfo{
	//				ItemId: uint64(constValue.GoldItemId),
	//				Count:  pInfo.goldValue,
	//			})
	//			if pInfo.itemId > 0 {
	//				betInfo = append(betInfo, &pbGo.ItemInfo{
	//					ItemId: uint64(pInfo.itemId),
	//					Count:  1,
	//				})
	//			}
	//			// 玩家信息
	//			heroId := room.playerInfos[oneUid].HeroId
	//			changeScreenInfo.PlayerInfoList = append(changeScreenInfo.PlayerInfoList, &pbGo.PlayerInfo{
	//				Uid:  oneUid,
	//				HeroId:  uint32(heroId),
	//				BetInfo: betInfo,
	//			})
	//		}
	//	}
	//
	//	elkGoods = append(elkGoods, &ElkGoods{Uid: Uid, Good: changeScreenInfo.AllGoods})
	//	go node.Push(info.Player, protoHandlerInit.RoundInfoPush, &pbGo.RoundInfoPush{
	//		EndTimeOut:       curRound.creatTime + int64(roomConfig.Timeout),
	//		RoundIndex:       uint32(curRound.RoundIndex),
	//		IsFinish:         roomSettlementInfo != nil,
	//		ChangeScreenInfo: changeScreenInfo,
	//		HintList:         hintList,
	//		SettlementInfo:   roomSettlementInfo,
	//	})
	//	// 记录
	//	//go roomOneRecord(Uid)
	//	// 重置
	//	go reward.ResetReward(Uid, receiveAwardId)
	//}
	//round := make([]ElkOp, 0)
	//for uid, operation := range curRound.Op {
	//	round = append(round, ElkOp{
	//		Uid:    uid,
	//		GoldValue: operation.goldValue,
	//		Operation: int64(operation.operation),
	//		ItemId:    int64(operation.itemId),
	//	})
	//}
	//if roomSettlementInfo != nil {
	//	go func() {
	//		err := statistics.SaveRoomSettlementFromResult(
	//			context.Background(),
	//			room.roomId,
	//			room.roomType,
	//			curRound.RoundIndex,
	//			room.SumPrice,
	//			uidList,
	//			roomSettlementInfo,
	//			time.Now().Unix(),
	//		)
	//		if err != nil {
	//			log2.Get().Error("SaveRoomSettlementFromResult failed", zap.Int32("roomId", room.roomId), zap.Error(err))
	//		}
	//	}()
	//}
	//
	//elastic.Upload("Settlement", map[string]any{
	//	"roundIndex": curRound.RoundIndex,
	//	"uidList":    uidList,
	//	"roomType":   room.roomType,
	//	"round":      round,
	//
	//	"roomSettlementInfo": roomSettlementInfo,
	//	"goods":              elkGoods,
	//})
}

// 索引
func getIndexByXY(x, y uint32) uint32 {
	return y*constValue.WIDE + x
}
func getXYByIndex(index uint32) (x, y uint32) {
	x = index / constValue.WIDE
	y = index % constValue.WIDE
	return x, y
}

// 随机一个位置
func randIndexByXY(sx, ex, sy, ey uint32) uint32 {
	x := uint32(rand.Intn(int(ex-sx)) + int(sx)) //sx ~ ex
	y := uint32(rand.Intn(int(ey-sy)) + int(sy)) //0~2
	return getIndexByXY(x, y)
}

func getAreaByXY(sx, ex, sy, ey uint32) int8 {
	return int8((ex - sx) * (ey - sy))
}
