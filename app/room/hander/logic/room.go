package logic

import (
	"fmt"
	"gameServer/app/room/hander/config"
	"gameServer/app/room/hander/maxRects"
	"gameServer/common/constValue"
	"gameServer/common/db/items"
	"gameServer/common/errorCode"
	"gameServer/pkg/logger/log2"
	"gameServer/protobuf/pbGo"
	"gameServer/protobuf/protoHandlerInit"
	"gameServer/service/common"
	"gameServer/service/services/node"
	"sort"
	"time"

	"go.uber.org/zap"
	"golang.org/x/exp/rand"
)

const (
	RoomStatusWait  = 0
	RoomStatusPlay  = 1
	RoomStatusClose = 2

	PlayerStatusNormal      = 0
	PlayerStatusLeave       = 1
	PlayerStatusCancelMatch = 2

	PlayerOpWaiting = 0 //等待
	PlayerOpBet     = 1 //发牌
	PlayerOpAbstain = 2 //弃权
	PlayerOpUseItem = 3 //使用道具
)

var (
	tickerTest = time.NewTicker(1 * time.Second)
	endTime    = int64(0)
)

func TestInit() {

	// 按房间类型分桶
	//buckets := make(map[uint32][]*MatchRequest) //房间类型-匹配请求
	for {
		select {
		// 定时撮合
		case <-tickerTest.C:
			ttt := endTime - time.Now().Unix()
			fmt.Printf("剩余时间，===========: %d\n", ttt)
		}
	}
}

// ================= 玩家 =================
type PlayerInfo struct {
	//PlayerId uint64
	Player        *common.Player //网关
	playerType    uint8          //0:真实玩家，>0:机器人
	HeroId        uint32         //选择的人物
	ChoiceItemMap map[int]int64  //选择的道具
	UserItemMap   map[int]int8   //使用了道具

	robotConfig *config.Robot //机器人配置

	status uint8 // 0: 正常，1：已经离开

	//Send func(msg interface{}) // 推送函数（WebSocket等）
}

type PlayerInfos map[uint64]*PlayerInfo

// ================= 回合 =================

type Round struct {
	RoundIndex int8

	Op    map[uint64]*Operation //所有玩家的操作,userid -本局玩家的操作
	timer *time.Timer

	//timeOut   int64 //超时时间戳
	creatTime int64 //创建时间戳 秒
}

// ================= 房间 =================

type Room struct {
	roomId     int32
	roomType   uint32
	maxPlayer  int
	createTime int64

	playerInfos PlayerInfos
	roomStatus  int8

	roundList []*Round
	maxRound  uint8

	// 命令
	cmdChan chan interface{}

	closeChan chan int32 //通知管理器删除

	gridInfo *[]*maxRects.Placement //物品信息
}

// ================= 命令 =================

// 玩家加入
type JoinCmd struct {
	Player     *PlayerInfo
	roomConfig *config.Room
	Resp       chan error
}

// 玩家操作
type ActionCmd struct {
	UserId     uint64
	roomConfig *config.Room
	op         *Operation       //操作
	Resp       chan *ActionResp //成功,0,其他为错误码
}

type ActionResp struct {
	Code uint16
	Data *pbGo.UseItemResp
}

// 超时操作
type timeoutCmd struct {
	RoundIndex int8 //超时的轮index
	roomConfig *config.Room
}

// 房间销毁
type stop struct {
}

// 离开操作
type LeaveCmd struct {
	UserId uint64
}

// ================= 创建房间 =================

func NewRoom(roomId int32, cfg *config.Room, closeChan chan int32) *Room {

	// 分配藏品信息
	sum := int(cfg.ItemSum)
	itemList := make([]int, 0, sum)
	maxLen := len(cfg.ItemList)
	for i := 0; i < sum; i++ {
		index := rand.Intn(maxLen)
		itemList = append(itemList, cfg.ItemList[index])
	}

	gridInfo := make([]maxRects.Item, 0, len(itemList))
	for _, itemId := range itemList {
		info := config.GetItemConfigById(itemId)
		if info == nil {
			log2.Get().Error("GetItemConfigById failed ", zap.Any("itemId", itemId))
			continue
		}
		if info.Area == nil || len(info.Area) != 2 {
			log2.Get().Error("ItemConfig Area is null", zap.Any("itemId", itemId))
			continue
		}
		gridInfo = append(gridInfo, maxRects.Item{
			Id:     itemId,
			Length: info.Area[0],
			Width:  info.Area[1],
		})
	}
	//

	// 优化排序
	sort.Slice(gridInfo, func(i, j int) bool {
		return gridInfo[i].Length*gridInfo[i].Width >
			gridInfo[j].Length*gridInfo[j].Width
	})
	placements := maxRects.Pack(gridInfo)

	r := &Room{
		roomId:      roomId,
		roomType:    cfg.RoomType,
		maxPlayer:   cfg.CapacityLimit,
		createTime:  time.Now().Unix(),
		playerInfos: make(PlayerInfos),
		cmdChan:     make(chan interface{}, 100),
		maxRound:    cfg.RoundLimit,
		gridInfo:    &placements,
		closeChan:   closeChan,
	}

	go r.loop()

	return r
}

// ================= 主循环 =================
func (r *Room) loop() {
	for cmd := range r.cmdChan {

		switch c := cmd.(type) {
		case *JoinCmd:
			r.handleJoin(c)

		case *ActionCmd:
			r.handleAction(c)

		case *timeoutCmd:
			r.handleTimeout(c)

		case *LeaveCmd:
			r.handleLeave(c)
		case *stop:
			// 清空 channel（防止 goroutine 泄漏）
			for {
				select {
				case <-r.cmdChan:
				default:
					return
				}
			}
		}
	}
}

// ================= Join =================

func (r *Room) handleJoin(c *JoinCmd) {
	if len(r.playerInfos) >= r.maxPlayer {
		c.Resp <- fmt.Errorf("room full")
		return
	}

	r.playerInfos[c.Player.Player.UserId] = c.Player
	c.Resp <- nil

	//r.broadcast("player_join", c.Player.Player.UserId)

	// 满员开始
	if len(r.playerInfos) == r.maxPlayer {
		r.startGame(c.roomConfig)
	}
}

// ================= Action =================

func (r *Room) handleAction(c *ActionCmd) {
	resp := &ActionResp{
		Code: errorCode.ErrorCode_Success,
	}
	if r.roomStatus != RoomStatusPlay {
		resp.Code = errorCode.ErrorCode_NotJoinRoom
		c.Resp <- resp
		return
	}
	userId := c.UserId
	lastRound := r.getLastRound()
	if lastRound != nil {
		op, ok := lastRound.Op[userId]
		if ok && op.operation == PlayerOpAbstain {
			resp.Code = errorCode.ErrorCode_AlreadyAbstain
			c.Resp <- resp
			return
		}
	}

	round := r.getCurrentRound()
	_, ok := round.Op[userId]
	if !ok {
		round.Op[userId] = &Operation{}
	}
	if round.Op[userId].isBet {
		resp.Code = errorCode.ErrorCode_AlreadyBet
		c.Resp <- resp
		return
	}

	round.Op[userId].operation = c.op.operation

	if c.op.operation == PlayerOpBet { //竞拍
		round.Op[userId].isBet = true
		round.Op[userId].goldValue = c.op.goldValue

		//广播所有
		for _, one := range r.playerInfos {
			if one.Player.UserId == userId { // 排除本人
				continue
			}
			if one.playerType > 0 { // 排除机器人
				continue
			}
			if one.status == PlayerStatusLeave { //排除离开的
				continue
			}

			node.Push(one.Player, protoHandlerInit.BetPush, &pbGo.BetResp{
				PlayerInfo: &pbGo.PlayerInfo{
					UserId: userId,
				},
			})
		}

	} else if c.op.operation == PlayerOpAbstain { //弃拍
		round.Op[userId].operation = PlayerOpAbstain
	} else if c.op.operation == PlayerOpUseItem { //使用道具
		if r.playerInfos[userId].ChoiceItemMap == nil {
			resp.Code = errorCode.ErrorCode_ItemNotEnough
			c.Resp <- resp
			return
		}
		_, ok = r.playerInfos[userId].ChoiceItemMap[c.op.itemId]
		if !ok {
			resp.Code = errorCode.ErrorCode_ItemNotEnough
			c.Resp <- resp
			return
		}

		if c.op.itemId == 0 { //使用空
			resp.Code = errorCode.ErrorCode_UseNullItem
			c.Resp <- resp
			return
		}

		if r.playerInfos[userId].UserItemMap[c.op.itemId] > 0 { //重复使用
			resp.Code = errorCode.ErrorCode_ReusingItem
			c.Resp <- resp
			return
		}

		// 验证合法
		itemId := c.op.itemId
		item := config.GetItemConfigById(c.op.itemId)
		if item == nil {
			log2.Get().Error("GetItemConfigById failed ", zap.Any("itemId", itemId))
			resp.Code = errorCode.ErrorCode_ItemNotEnough
			c.Resp <- resp
			return
		}
		abilityId := item.Ability
		if abilityId <= 0 {
			resp.Code = errorCode.ErrorCode_ConfigError
			c.Resp <- resp
			return
		}
		ability := config.GetAbilityConfigById(abilityId)
		if ability == nil {
			log2.Get().Error("GetAbilityConfigById failed ", zap.Any("abilityId", abilityId))
			resp.Code = errorCode.ErrorCode_GetConfigFailed
			c.Resp <- resp
			return
		}

		var (
			changeScreenInfo = &pbGo.ScreenInfo{}
			data             = &pbGo.UseItemResp{}
			HintList         = make([]*pbGo.Hint, 0)
			h                = &pbGo.Hint{}
		)

		hint := r.applyAbility(ability, userId, 0)
		h.HintType = 2
		h.Id = hint.id
		h.Value = uint64(hint.Value)
		h.ValueType = hint.ValueType
		HintList = append(HintList, h)

		buildChangeScreenInfo(changeScreenInfo, *r.gridInfo, userId)

		//data.ItemInfoList =
		data.ChangeScreenInfo = changeScreenInfo
		data.HintList = HintList

		// 广播
		resp.Data = data

		//消耗
		ok = items.ConsumeItem(userId, map[int]int64{
			itemId: 1,
		})
		if !ok {
			resp.Code = errorCode.ErrorCode_GetConfigFailed
			c.Resp <- resp
			return
		}
		r.playerInfos[userId].UserItemMap[c.op.itemId] = round.RoundIndex
		round.Op[userId].itemId = c.op.itemId
	}
	c.Resp <- resp

	need := 0
	for _, info := range round.Op {
		if info.isBet || info.operation == PlayerOpAbstain {
			need++
		}
	}
	// 全员完成，提前推进`
	if need == len(r.playerInfos) { // 所有都操作了
		if round.timer != nil {
			round.timer.Stop() // 停止上一轮的计时
		}
		r.nextRound(c.roomConfig, round)
	}
}

// ================= 超时 =================
func (r *Room) handleTimeout(c *timeoutCmd) {
	if r.roomStatus != RoomStatusPlay {
		return
	}

	round := r.getCurrentRound()

	// 旧timer直接忽略
	if round.RoundIndex != c.RoundIndex {
		return
	}

	// 自动补齐未操作玩家
	for userId := range r.playerInfos {
		if round.Op[userId] == nil { // 弃权
			round.Op[userId] = &Operation{
				operation: PlayerOpAbstain,
			}
			continue
		}
		if round.Op[userId].isBet {
			continue
		}
		//op, ok := round.Op[userId]
		//if !ok {
		//	op = &Operation{}
		//	round.Op[userId] = op
		//}
		//round.Op[userId].operation = PlayerOpAbstain
	}

	r.nextRound(c.roomConfig, round)
}

// ================= 离开 =================
func (r *Room) handleLeave(c *LeaveCmd) {
	//if r.roomStatus == RoomStatusWait { //不可能
	//	delete(r.playerInfos, c.UserId)
	//} else { // 已经游戏中
	//}
	//log2.Get().Warn("handleLeave ============  ", zap.Any("roomId:=", r.roomId))
	r.playerInfos[c.UserId].status = PlayerStatusLeave
	// 如果房间没人了，可以关闭
	if len(r.playerInfos) == 0 {
		r.roomStatus = RoomStatusClose
	}
}

// ================= 开始游戏 =================
func (r *Room) startGame(roomConfig *config.Room) {
	r.roomStatus = RoomStatusPlay
	matchInfoPush := &pbGo.MatchInfoPush{
		RoomType: r.roomType,
	}

	// 如果道具不足？？ todo
	playerInfoList := make([]*pbGo.PlayerInfo, 0, r.maxPlayer)
	for userId, p := range r.playerInfos {
		playerInfoList = append(playerInfoList, &pbGo.PlayerInfo{
			UserId:  userId,
			HeroId:  p.HeroId,
			BetInfo: make([]*pbGo.ItemInfo, 0),
		})
		if p.playerType > 0 {
			continue
		}
		// 5. 匹配成功后扣道具
		items.ConsumeItem(userId, roomConfig.Consume)
	}
	matchInfoPush.PlayerInfoList = playerInfoList
	for _, p := range r.playerInfos {
		if p.playerType != 0 { //排除机器人
			continue
		}
		if p.status == PlayerStatusLeave { //离开的人
			continue
		}

		node.Push(p.Player, protoHandlerInit.MatchInfoPush, matchInfoPush)
	}

	r.roundList = []*Round{}
	r.nextRound(roomConfig, nil) //推送首轮
	go r.startRobotActions(roomConfig)
}

// ================= 下一回合 =================
func (r *Room) nextRound(roomConfig *config.Room, lastRound *Round) {

	// 全是机器人则结束
	earlyFinish := true
	// 至少还有一个真实玩家在玩
	for _, info := range r.playerInfos {
		if info.playerType == 0 && info.status == PlayerStatusNormal {
			earlyFinish = false
			break
		}
	}
	if !earlyFinish {
		earlyFinish = isEarlyFinish(roomConfig, lastRound)
	}
	if uint8(len(r.roundList)) >= r.maxRound || earlyFinish { //圆满或者提前结束
		r.finishGame(roomConfig)
		return
	}

	// 生成新的下一轮
	var (
		// 从1开始
		index = int8(len(r.roundList) + 1)
		round = &Round{
			creatTime:  time.Now().Unix(),
			RoundIndex: index,
			Op:         make(map[uint64]*Operation),
		}
	)

	if lastRound != nil {
		for userId, op := range lastRound.Op { //为弃权玩家填充默认操作
			if op.operation == PlayerOpAbstain {
				round.Op[userId] = &Operation{
					userId:    userId,
					operation: PlayerOpAbstain,
				}
			}
		}

	}

	r.roundList = append(r.roundList, round)

	r.pushRoundInfo(roomConfig, nil)

	// test
	now := time.Now().Unix()
	endTime = now + int64(roomConfig.Timeout)

	// 启动超时
	round.timer = time.AfterFunc(time.Duration(roomConfig.Timeout)*time.Second, func() {
		r.cmdChan <- &timeoutCmd{RoundIndex: index, roomConfig: roomConfig} //发送信息通知
	})
}

// 因为差距提前结束
func isEarlyFinish(roomConfig *config.Room, lastRound *Round) bool {
	if lastRound == nil {
		return false
	}
	if len(lastRound.Op) < roomConfig.CapacityLimit {
		return false
	}

	maxIndex := int8(len(roomConfig.EarlyTermination) - 1)
	if maxIndex < 0 {
		return false
	}

	// ❗判断是否达到最大回合,或者提前结束
	max1 := int64(0)
	max2 := int64(0)
	for _, op := range lastRound.Op {
		count := op.goldValue
		if count > max1 {
			// 当前值大于最大值，更新 max1 和 max2
			max2 = max1  // 原来的最大值变成第二大值
			max1 = count // 当前值成为新的最大值
		} else if count > max2 && count != max1 {
			// 当前值介于 max2 和 max1 之间，且不等于 max1
			max2 = count
		}
	}
	termination := 0                  //百分比
	index := lastRound.RoundIndex - 1 //
	if index > maxIndex {
		index = maxIndex
	}
	termination = roomConfig.EarlyTermination[index]
	if 100*int(max1)-int(max2)*(100+termination) >= 0 {
		return true
	}
	return false
}

// ================= 结束游戏 =================
func (r *Room) finishGame(roomConfig *config.Room) {
	r.roomStatus = RoomStatusClose

	// 停掉当前timer
	if len(r.roundList) > 0 {
		round := r.getCurrentRound()
		if round.timer != nil {
			round.timer.Stop()
		}
	}
	// 计算结果
	r.pushRoundInfo(roomConfig, r.calcResult())

	// 清除房间 todo
	r.cmdChan <- &stop{}    //必有,发送，停止主循环
	r.closeChan <- r.roomId //停止机器人,销毁房间
}

// 结算
func (r *Room) calcResult() *pbGo.RoomSettlementInfo {
	roomSettlementInfo := &pbGo.RoomSettlementInfo{}
	itemInfo := &pbGo.ItemInfo{}
	betInfo := make([]*pbGo.ItemInfo, 0, 1)
	maxV := int64(0)

	curRound := r.getCurrentRound()
	var (
		targetUserId uint64
		sumValue     = int64(0)
	)

	for uid, info := range curRound.Op {
		v := info.goldValue
		if v > maxV {
			itemInfo.ItemId = uint64(constValue.GoldItemId)
			itemInfo.Count = v
			targetUserId = uid
			maxV = v
		}
	}
	if targetUserId == 0 {
		return nil
	}
	betInfo = append(betInfo, itemInfo)
	heroId := r.playerInfos[targetUserId].HeroId
	// 利润
	for _, info := range *r.gridInfo {
		one := config.GetItemConfigById(info.Item.Id)
		if one == nil {
			log2.Get().Error("GetItemConfigById failed ", zap.Any("constValue", info.Item.Id))
			continue
		}
		for id, value := range one.Price {
			if id == constValue.GoldItemId {
				sumValue += value
			}
		}
	}

	// 响应
	roomSettlementInfo.PlayerInfo = &pbGo.PlayerInfo{
		UserId:  targetUserId,
		HeroId:  heroId,
		BetInfo: betInfo,
	}
	profit := &pbGo.ItemInfo{
		ItemId: uint64(constValue.GoldItemId),
		Count:  sumValue - itemInfo.Count,
	}
	roomSettlementInfo.Expenses = itemInfo
	roomSettlementInfo.Profit = profit

	items.RewardItem(targetUserId, map[int]int64{
		int(roomSettlementInfo.Profit.ItemId): sumValue,
	})

	items.ConsumeItem(targetUserId, map[int]int64{
		int(roomSettlementInfo.Expenses.ItemId): roomSettlementInfo.Expenses.Count,
	})
	return roomSettlementInfo
}

// ================= 工具 =================

func (r *Room) getCurrentRound() *Round {
	return r.roundList[len(r.roundList)-1]
}

func (r *Room) getLastRound() *Round {
	if len(r.roundList) >= 2 {
		return r.roundList[len(r.roundList)-2]
	}
	return nil
}

// ================= 广播 =================

//func (r *Room) broadcast(event string, data interface{}) {
//	msg := map[string]interface{}{
//		"event": event,
//		"data":  data,
//	}
//	for _, p := range r.playerInfos {
//		if p.Send != nil {
//			p.Send(msg)
//		}
//	}
//}

// ================= 对外接口 =================

func (r *Room) Join(p *PlayerInfo, cfg *config.Room) error {
	if r.roomStatus == RoomStatusClose {
		return fmt.Errorf("room is bull")
	}
	resp := make(chan error)
	r.cmdChan <- &JoinCmd{
		Player:     p,
		roomConfig: cfg,
		Resp:       resp,
	}
	return <-resp
}

func (r *Room) GettestG() []*maxRects.Placement {
	return *r.gridInfo
}

func (r *Room) Action(roomConfig *config.Room, op *Operation) *ActionResp {
	if r.roomStatus == RoomStatusClose {
		return &ActionResp{
			Code: errorCode.ErrorCode_NotJoinRoom,
		}
	}
	resp := make(chan *ActionResp)
	r.cmdChan <- &ActionCmd{
		UserId:     op.userId,
		op:         op,
		Resp:       resp,
		roomConfig: roomConfig,
	}
	return <-resp
}

func (r *Room) Leave(userId uint64) {
	//log2.Get().Warn("Leave start !!!========== ", zap.Int32("roomId:= ", r.roomId))
	if r.roomStatus == RoomStatusClose {
		return
	}
	r.cmdChan <- &LeaveCmd{UserId: userId}
}
