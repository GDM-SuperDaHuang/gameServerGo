package logic

import (
	"fmt"
	"gameServer/app/room/hander/config"
	"gameServer/app/room/hander/maxRects"
	"gameServer/common/constValue"
	"gameServer/common/db/items"
	"gameServer/common/db/rooming"
	"gameServer/common/db/user"
	"gameServer/common/errorCode"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/recoverSafe"
	"gameServer/protobuf/pbGo"
	"gameServer/protobuf/protoHandlerInit"
	"gameServer/service/common"
	"gameServer/service/node"
	"sync/atomic"
	"time"

	tomlConfig "gameServer/pkg/config"

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

// ================= 玩家 =================
type PlayerInfo struct {
	Player        *common.Player //网关
	playerType    uint8          //0:真实玩家，>0:机器人
	HeroId        int            //选择的人物
	ChoiceItemMap map[int]int64  //选择的道具
	UserItemMap   map[int]int8   //使用了道具
	status        uint8          // 0: 正常，1：已经离开

	robotConfig    *config.Robot     //机器人配置 目前只是为了名字
	robotBetConfig *config.RobotBetF //机器人出价配置

}

type PlayerInfos map[int64]*PlayerInfo

// ================= 回合 =================

type Round struct {
	RoundIndex int8

	Op    map[int64]*Operation //所有玩家的操作,Uid -本局玩家的操作
	timer *time.Timer

	creatTime int64 //创建时间戳 秒

	MK    int64 //机器人参考价值
	RoUid int64 //交互机器人id
	PUid  int64 //参考玩家id

	ThinkTimeMap map[int64]int64 //机器人思考时间
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

	entryMap map[int8]int // 轮数-id

	roItemMap map[int64]map[int]int // 机器人道具 uid-轮数-道具id

	// 命令
	cmdChan chan interface{}

	closeChan chan int32 //通知管理器删除

	gridInfo *[]*maxRects.Placement //物品信息

	SumPrice int64 //房间总价值

	closing int32 // 0: running, 1: closing/closed
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
	Uid        int64
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
	Uid int64
}

// 查询操作
type FindCmd struct {
	Resp chan *FindResp //成功,0,其他为错误码
}

type FindResp struct {
	curRound    *Round
	playerInfos PlayerInfos
}

// ================= 创建房间 =================

func NewRoom(roomId int32, cfg *config.Room, closeChan chan int32) *Room {

	// 分配藏品信息
	//ls, entryMap := itemAllocation.GetItems(cfg.ItemAllocationMap)
	//if len(ls) == 0 {
	//	log2.Get().Error("")
	//}
	//
	//gridInfo := make([]maxRects.Item, 0, len(ls))
	//sumPrice := int64(0)
	//for _, info := range ls {
	//	price := info.Price[int(constValue.GoldItemId)]
	//	gridInfo = append(gridInfo, maxRects.Item{
	//		Id:       info.Id,
	//		Length:   info.Area[0],
	//		Width:    info.Area[1],
	//		Quality:  info.Quality,
	//		ItemType: info.ItemType,
	//		Price:    price,
	//	})
	//	sumPrice += price
	//}
	//
	//// 优化排序
	//sort.Slice(gridInfo, func(i, j int) bool {
	//	return gridInfo[i].Length*gridInfo[i].Width >
	//		gridInfo[j].Length*gridInfo[j].Width
	//})
	//placements := maxRects.Pack(gridInfo)
	//if len(placements) == 0 {
	//	log2.Get().Error("maxRects.Pack(gridInfo)", zap.Any("gridInfo", gridInfo))
	//}

	r := &Room{
		roomId: roomId,
		//roomType:    cfg.RoomType,
		//maxPlayer:   cfg.CapacityLimit,
		//createTime:  time.Now().Unix(),
		//playerInfos: make(PlayerInfos),
		//entryMap:    make(map[int8]int),
		//roItemMap:   make(map[int64]map[int]int),
		//cmdChan:     make(chan interface{}, 100),
		//maxRound:    cfg.RoundLimit,
		//gridInfo:    &placements,
		//closeChan:   closeChan,
		//SumPrice:    sumPrice,
	}
	//r.entryMap = entryMap
	recoverSafe.GoWorker("room.loop", r.loop3, recoverSafe.DefaultWorkerOpt)

	return r
}

func (r *Room) notifyClosed() {
	// 非阻塞通知，避免在退出路径因为通道拥堵卡死 loop
	select {
	case r.closeChan <- r.roomId:
	default:
		recoverSafe.GoSafe("room.close.notify", func() error {
			select {
			case r.closeChan <- r.roomId:
			case <-time.After(2 * time.Second):
				log2.Get().Warn("room close notify timeout", zap.Int32("roomId", r.roomId))
			}
			return nil
		})
	}
}

func (r *Room) requestClose() {
	if !atomic.CompareAndSwapInt32(&r.closing, 0, 1) {
		return
	}
	r.roomStatus = RoomStatusClose
	// stop 指令统一驱动 loop 退出；如果当下缓冲满，异步补发，避免阻塞调用方
	select {
	case r.cmdChan <- &stop{}:
	default:
		recoverSafe.GoSafe("room.stop.enqueue", func() error {
			select {
			case r.cmdChan <- &stop{}:
			case <-time.After(2 * time.Second):
				log2.Get().Warn("room stop enqueue timeout", zap.Int32("roomId", r.roomId))
			}
			return nil
		})
	}
	r.notifyClosed()
}

func (r *Room) isClosing() bool {
	return atomic.LoadInt32(&r.closing) == 1 || r.roomStatus == RoomStatusClose
}

func (r *Room) replyClosed(pending interface{}) {
	switch c := pending.(type) {
	case *JoinCmd:
		if c.Resp != nil {
			select {
			case c.Resp <- fmt.Errorf("room closed"):
			default:
			}
		}
	case *ActionCmd:
		if c.Resp != nil {
			select {
			case c.Resp <- &ActionResp{Code: errorCode.ErrorCode_NotJoinRoom}:
			default:
			}
		}
	case *FindCmd:
		if c.Resp != nil {
			select {
			case c.Resp <- nil:
			default:
			}
		}
	}
}

func (r *Room) loop3() error {
	for cmd := range r.cmdChan {

		// 单独处理 stop 命令，直接退出循环
		if _, ok := cmd.(*stop); ok {
			log2.Get().Info("room stop", zap.Any("roomId", r.roomId))
			// 清空 channel 中剩余的命令（可选，防止 goroutine 泄漏）
			for {
				select {
				case pending := <-r.cmdChan:
					r.replyClosed(pending)
				default:
					log2.Get().Info("default stop")
					return nil
				}
			}
		}

		func() {
			defer func() {

				if err := recover(); err != nil {
					log2.Get().Error("room cmd panic", zap.Any("panic", err))
					//兜底返回
					switch c := cmd.(type) {
					case *JoinCmd:
						c.Resp <- fmt.Errorf("panic")
					case *ActionCmd:
						c.Resp <- &ActionResp{Code: errorCode.ErrorCode_Panic}
					case *FindCmd:
						c.Resp <- nil
					}
				}
			}()

			switch c := cmd.(type) {
			case *JoinCmd:
				r.handleJoin(c)
			case *ActionCmd:
				r.handleAction(c)
			case *timeoutCmd:
				r.handleTimeout(c)
			case *LeaveCmd:
				r.handleLeave(c)
			case *FindCmd:
				r.handleFind(c)
			}
		}()
	}
	return nil
}

// ================= Join =================

func (r *Room) handleJoin(c *JoinCmd) {
	if len(r.playerInfos) >= r.maxPlayer {
		c.Resp <- fmt.Errorf("room full")
		return
	}

	r.playerInfos[c.Player.Player.Uid] = c.Player
	c.Resp <- nil

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
	//Uid := c.Uid
	//playerInfo, ok := r.playerInfos[Uid]
	//if !ok || playerInfo == nil {
	//	resp.Code = errorCode.ErrorCode_NotJoinRoom
	//	c.Resp <- resp
	//	return
	//}
	//lastRound := r.getLastRound()
	//if lastRound != nil {
	//	op, ok := lastRound.Op[Uid]
	//	if ok && op.operation == PlayerOpAbstain {
	//		resp.Code = errorCode.ErrorCode_AlreadyAbstain
	//		c.Resp <- resp
	//		return
	//	}
	//}
	//
	//round := r.getCurrentRound()
	//_, exists := round.Op[Uid]
	//if !exists {
	//	round.Op[Uid] = &Operation{
	//		Uid: Uid,
	//	}
	//}
	//
	//if c.op.operation == PlayerOpBet && round.Op[Uid].isBet > 0 { //禁止重复竞拍
	//	resp.Code = errorCode.ErrorCode_AlreadyBet
	//	c.Resp <- resp
	//	return
	//}
	//
	//round.Op[Uid].operation = c.op.operation
	//
	//isRobot := playerInfo.playerType > 0
	//
	//if c.op.operation == PlayerOpBet { //竞拍
	//
	//	round.Op[Uid].isBet = time.Now().UnixMilli()
	//	round.Op[Uid].goldValue = c.op.goldValue
	//
	//	//广播所有
	//	for _, one := range r.playerInfos {
	//		if one.Player.Uid == Uid { // 排除本人
	//			continue
	//		}
	//		if one.playerType > 0 { // 排除机器人
	//			continue
	//		}
	//		if one.status == PlayerStatusLeave { //排除离开的
	//			continue
	//		}
	//
	//		go node.Push(one.Player, protoHandlerInit.BetPush, &pbGo.BetResp{
	//			PlayerInfo: &pbGo.PlayerInfo{
	//				Uid: Uid,
	//			},
	//		})
	//	}
	//
	//} else if c.op.operation == PlayerOpAbstain { //弃拍
	//	round.Op[Uid].operation = PlayerOpAbstain
	//} else if c.op.operation == PlayerOpUseItem && isRobot {
	//	if c.op.operation == PlayerOpUseItem && round.Op[Uid].itemId > 0 { //禁止重复竞拍
	//		resp.Code = errorCode.ErrorCode_ReusingItem
	//		c.Resp <- resp
	//		return
	//	}
	//	playerInfo.UserItemMap[c.op.itemId] = round.RoundIndex
	//	round.Op[Uid].itemId = c.op.itemId
	//} else if c.op.operation == PlayerOpUseItem && !isRobot { //真人使用道具
	//	if playerInfo.ChoiceItemMap == nil {
	//		resp.Code = errorCode.ErrorCode_ItemNotEnough
	//		c.Resp <- resp
	//		return
	//	}
	//	_, ok = playerInfo.ChoiceItemMap[c.op.itemId]
	//	if !ok {
	//		resp.Code = errorCode.ErrorCode_ItemNotEnough
	//		c.Resp <- resp
	//		return
	//	}
	//
	//	if c.op.itemId == 0 { //使用空
	//		resp.Code = errorCode.ErrorCode_UseNullItem
	//		c.Resp <- resp
	//		return
	//	}
	//
	//	if playerInfo.UserItemMap[c.op.itemId] > 0 { //重复使用
	//		resp.Code = errorCode.ErrorCode_ReusingItem
	//		c.Resp <- resp
	//		return
	//	}
	//
	//	// 验证合法
	//	itemId := c.op.itemId
	//	item := config.GetItemConfigById(c.op.itemId)
	//	if item == nil {
	//		log2.Get().Error("GetItemConfigById failed ", zap.Any("itemId", itemId))
	//		resp.Code = errorCode.ErrorCode_ItemNotEnough
	//		c.Resp <- resp
	//		return
	//	}
	//	abilityId := item.Ability
	//	if abilityId <= 0 {
	//		resp.Code = errorCode.ErrorCode_ConfigError
	//		c.Resp <- resp
	//		return
	//	}
	//	ability := config.GetAbilityConfigById(abilityId)
	//	if ability == nil {
	//		log2.Get().Error("GetAbilityConfigById failed ", zap.Any("abilityId", abilityId))
	//		resp.Code = errorCode.ErrorCode_GetConfigFailed
	//		c.Resp <- resp
	//		return
	//	}
	//
	//	var (
	//		changeScreenInfo = &pbGo.ScreenInfo{}
	//		data             = &pbGo.UseItemResp{}
	//		HintList         = make([]*pbGo.Hint, 0)
	//		h                = &pbGo.Hint{}
	//	)
	//
	//	//hint := r.applyAbility(ability, Uid, itemHint)
	//	//h.HintType = 2
	//	//h.Desc = desc.GetDesc(ability.Desc, hint.Value)
	//	HintList = append(HintList, h)
	//
	//	//data.ItemInfoList =
	//	data.ChangeScreenInfo = changeScreenInfo
	//	data.HintList = HintList
	//
	//	// 广播
	//	resp.Data = data
	//
	//	// 道具校验
	//	ok = items.VerifyItem(Uid, map[int]int64{itemId: 1})
	//	if !ok {
	//		resp.Code = errorCode.ErrorCode_ItemNotEnough
	//		c.Resp <- resp
	//		return
	//	}
	//
	//	//消耗
	//	go items.ConsumeItem(Uid, map[int]int64{itemId: 1}, constValue.Item_Rid_Room_consume, "roomUserItem")
	//	if !ok {
	//		resp.Code = errorCode.ErrorCode_DBError
	//		c.Resp <- resp
	//		return
	//	}
	//	playerInfo.UserItemMap[c.op.itemId] = round.RoundIndex
	//	round.Op[Uid].itemId = c.op.itemId
	//}
	//c.Resp <- resp
	//
	////下一轮推进
	//need := 0
	//for _, info := range round.Op {
	//	if info.isBet > 0 || info.operation == PlayerOpAbstain {
	//		need++
	//	}
	//}
	//
	//// 全员完成，提前推进`
	//if need == len(r.playerInfos) { // 所有都操作了
	//	r.nextRound(c.roomConfig, round)
	//}
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
	for Uid := range r.playerInfos {
		if round.Op[Uid] == nil { // 弃权
			round.Op[Uid] = &Operation{
				Uid:       Uid,
				operation: PlayerOpAbstain,
			}
			continue
		}
	}

	r.nextRound(c.roomConfig, round)
}

// ================= 离开 =================
func (r *Room) handleLeave(c *LeaveCmd) {
	info, ok := r.playerInfos[c.Uid]
	if !ok || info == nil {
		return
	}
	info.status = PlayerStatusLeave
	//roomOneRecord(c.Uid)

	isClose := true
	for _, info := range r.playerInfos {
		if info.status != PlayerStatusLeave {
			isClose = false
		}
	}
	// 如果房间没人了，可以关闭
	if isClose {
		r.requestClose()
	}
}

// 查询信息
func (r *Room) handleFind(c *FindCmd) {
	// 拷贝
	round := &Round{}
	currentRound := r.getCurrentRound()
	round.RoundIndex = currentRound.RoundIndex
	round.Op = make(map[int64]*Operation)
	round.MK = currentRound.MK
	round.RoUid = currentRound.RoUid
	round.PUid = currentRound.PUid
	round.ThinkTimeMap = make(map[int64]int64)
	for k, v := range currentRound.Op {
		round.Op[k] = v
	}
	round.creatTime = currentRound.creatTime

	for k, v := range currentRound.ThinkTimeMap {
		round.ThinkTimeMap[k] = v
	}

	ps := make(map[int64]*PlayerInfo)
	for uid, info := range r.playerInfos {
		ps[uid] = info
	}

	resp := &FindResp{
		curRound:    round,
		playerInfos: ps,
	}
	c.Resp <- resp
}

// ================= 开始游戏 =================
func (r *Room) startGame(roomConfig *config.Room) {
	r.roomStatus = RoomStatusPlay
	matchInfoPush := &pbGo.MatchInfoPush{
		RoomType: r.roomType,
	}

	// 如果道具不足？？ todo
	playerInfoList := make([]*pbGo.PlayerInfo, 0, r.maxPlayer)
	nickNameMap := make(map[string]struct{})
	for Uid, p := range r.playerInfos {
		avatarUrl := ""
		nickName := ""
		if p.playerType > 0 {
			// 防止机器人重名
			for i := 0; i < 1000; i++ {
				index := rand.Intn(len(p.robotConfig.NickNames))
				str := p.robotConfig.NickNames[index]
				if _, ok := nickNameMap[str]; !ok {
					nickName = str
					nickNameMap[str] = struct{}{}
					break
				}
			}
			if nickName == "" {
				nickName = fmt.Sprintf("dx%d", Uid)
			}
		} else {
			userInfo, err := user.FindUser(Uid)
			if err == nil && userInfo != nil {
				avatarUrl = userInfo.AvatarUrl
				nickName = userInfo.NickName
			}
		}

		playerInfoList = append(playerInfoList, &pbGo.PlayerInfo{
			Uid:       Uid,
			HeroId:    uint32(p.HeroId),
			BetInfo:   make([]*pbGo.ItemInfo, 0),
			AvatarUrl: avatarUrl,
			NickName:  nickName,
		})
		if p.playerType > 0 {
			continue
		}
		// 5. 匹配成功后扣道具
		consume := make(map[int]int64)
		for i, i2 := range roomConfig.Consume {
			consume[i] = i2
		}
		go items.ConsumeItem(Uid, consume, constValue.Reward_Rid_EnterRoome, "enterRoome")
	}
	//matchInfoPush.PlayerInfoList = playerInfoList
	for _, p := range r.playerInfos {
		if p.playerType != 0 { //排除机器人
			continue
		}
		if p.status == PlayerStatusLeave { //离开的人
			continue
		}

		go node.Push(p.Player, protoHandlerInit.MatchInfoPush, matchInfoPush)
	}
	//isRobotGo := false
	for _, info := range r.playerInfos {
		// 初始k
		if info.playerType == 0 {

			//记录正在游戏
			go rooming.SaveRooming(info.Player.Uid, tomlConfig.Get().NodeID())
		} else { //机器人

		}
	}

	r.roundList = []*Round{}
	r.nextRound(roomConfig, nil) //推送首轮

	// 如果有机器人则开启
	//if isRobotGo {
	//	recoverSafe.GoSafe("startRobotWorker", func() error {
	//		r.startRobotActions(roomConfig)
	//		return nil
	//	})
	//}
}

// ================= 下一回合 =================
func (r *Room) nextRound(roomConfig *config.Room, lastRound *Round) {
	//if lastRound != nil && lastRound.timer != nil {
	//	lastRound.timer.Stop() // 停止上一轮的计时
	//}
	//isFinish := r.isFinish(roomConfig, lastRound)
	//if isFinish { //圆满结束,提前结束,加时赛结束
	//	r.finishGame(roomConfig)
	//	return
	//}
	//
	//roUid := int64(0)
	//for uid, p := range r.playerInfos {
	//	if p.playerType > 0 {
	//		roUid = uid
	//		break
	//	}
	//}
	//
	// 生成新的下一轮
	var (
		// 从1开始
		index = int8(len(r.roundList) + 1)
		round = &Round{
			creatTime:  time.Now().Unix(),
			RoundIndex: index,
			//Op:           make(map[int64]*Operation),
			//MK:           0,
			//RoUid:     roUid,
			//ThinkTimeMap: make(map[int64]int64),
		}
	)

	//if round.RoundIndex == 1 {
	//	value, targetUid := r.findMaxPlayer()
	//	round.PUid = targetUid
	//	round.MK = r.getGoldValue(value, targetUid)
	//	thinkTime := int64(roomConfig.Timeout - 5)
	//	if thinkTime <= 0 {
	//		thinkTime = 5
	//	}
	//	for uid, p := range r.playerInfos {
	//		if p.playerType == 0 {
	//			continue
	//		}
	//		round.ThinkTimeMap[uid] = rand.Int63n(thinkTime)
	//	}
	//} else {
	//	round.MK, round.PUid, round.ThinkTimeMap = r.getGoldValue2(lastRound, roomConfig.Timeout)
	//}
	//
	//if lastRound != nil {
	//	for Uid, op := range lastRound.Op { //为弃权玩家填充默认操作
	//		if op.operation == PlayerOpAbstain {
	//			round.Op[Uid] = &Operation{
	//				Uid:    Uid,
	//				operation: PlayerOpAbstain,
	//			}
	//		}
	//	}
	//
	//}
	//
	//r.roundList = append(r.roundList, round)
	//
	//r.pushRoundInfo(roomConfig, nil)
	//
	//// test
	//now := time.Now().Unix()
	//endTime = now + int64(roomConfig.Timeout)

	// 启动超时
	round.timer = time.AfterFunc(time.Duration(roomConfig.Timeout)*time.Second, func() {
		if r.isClosing() {
			return
		}
		select {
		case r.cmdChan <- &timeoutCmd{RoundIndex: index, roomConfig: roomConfig}: //发送信息通知
		default:
			log2.Get().Warn("drop timeout cmd because room channel is full", zap.Int32("roomId", r.roomId), zap.Int8("roundIndex", index))
		}
	})
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

// ================= 对外接口 =================

func (r *Room) Join(p *PlayerInfo, cfg *config.Room) error {
	if r.isClosing() {
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

func (r *Room) Action(roomConfig *config.Room, op *Operation) *ActionResp {
	if r.isClosing() {
		return &ActionResp{
			Code: errorCode.ErrorCode_NotJoinRoom,
		}
	}
	resp := make(chan *ActionResp)
	r.cmdChan <- &ActionCmd{
		Uid:        op.Uid,
		op:         op,
		Resp:       resp,
		roomConfig: roomConfig,
	}
	return <-resp
}

func (r *Room) FindData() *FindResp {
	if r.isClosing() {
		return &FindResp{}
	}
	resp := make(chan *FindResp)
	r.cmdChan <- &FindCmd{
		Resp: resp,
	}
	return <-resp
}

func (r *Room) Leave(Uid int64) {
	if r.isClosing() {
		return
	}
	r.cmdChan <- &LeaveCmd{Uid: Uid}
}
