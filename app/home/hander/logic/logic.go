package logic

import (
	"sync"

	"gameServer/common/constValue"
)

// 分片锁:把原来的全局一把大锁拆成 N 个分片,不同 uid 大概率落在不同分片,
// 互不阻塞.同分片内仍串行,分片数越大并行度越高(内存占用略增),32 是常用折中.
// 不用 sync.Map 的原因:此处 key 高频创建/删除且全是写操作,分片互斥 map 更快、代码更直观.
const shardCount = 32

type shard struct {
	mu      sync.Mutex                 // 本分片只有写操作,用 Mutex 即可;将来加只读查询时再换 RWMutex + RLock
	players map[int64]map[int64]*Round // uid -> roundId -> info
}

var shards [shardCount]shard

func init() {
	for i := range shards {
		shards[i].players = make(map[int64]map[int64]*Round)
	}
}

func getShard(uid int64) *shard {
	return &shards[uint64(uid)%shardCount]
}

type Round struct {
	ItemMap    map[int64]int64 // 本局累计的道具(怪物掉落等)
	MonsterIds []uint32        // 怪物id列表
}

// monsterGold 不同怪物给不同的钱,后续替换为配置表查询.
// todo: 接入怪物配置表后替换此处固定值
func monsterGold(monsterId uint32) int64 {
	_ = monsterId
	return 10
}

// UpdateRound 内存保存一局内的击杀/掉落信息.
// 如果出现相同的 uid+roundId 则追加(同一局多次上报累加),不存在则创建.
// 注意:只阻塞落到同分片的玩家,不同分片完全并行.
func UpdateRound(uid, roundId int64, addMonsterIds []uint32) {
	if uid == 0 || roundId == 0 {
		return
	}
	s := getShard(uid)
	s.mu.Lock()
	defer s.mu.Unlock()

	rounds := s.players[uid]
	if rounds == nil {
		rounds = make(map[int64]*Round)
		s.players[uid] = rounds
	}
	round := rounds[roundId]
	if round == nil {
		round = &Round{
			ItemMap:    make(map[int64]int64),
			MonsterIds: make([]uint32, 0),
		}
		rounds[roundId] = round
	}
	round.AddMonsters(addMonsterIds)
}

// TakeRound 取出并删除一局的内存快照(游戏结束后先删后奖).
// 加锁只做"摘除 entry",深拷贝在解锁后进行,临界区最短.
// 返回拷贝后的快照,调用方拿到快照后再做发奖,保证重复结算时取不到数据实现幂等.
// ok=false 表示该局不存在(未开始/已结算/已删除).
func TakeRound(uid, roundId int64) (*Round, bool) {
	if uid == 0 || roundId == 0 {
		return nil, false
	}
	s := getShard(uid)
	s.mu.Lock()
	rounds := s.players[uid]
	if rounds == nil {
		s.mu.Unlock()
		return nil, false
	}
	round := rounds[roundId]
	if round == nil {
		s.mu.Unlock()
		return nil, false
	}
	// 摘除 entry,之后该指针本协程独占
	delete(rounds, roundId)
	if len(rounds) == 0 {
		delete(s.players, uid)
	}
	s.mu.Unlock()

	// 锁外拷贝,不阻塞其他玩家
	snapshot := &Round{
		ItemMap:    make(map[int64]int64, len(round.ItemMap)),
		MonsterIds: append([]uint32(nil), round.MonsterIds...),
	}
	for k, v := range round.ItemMap {
		snapshot.ItemMap[k] = v
	}
	return snapshot, true
}

// DelRound 删除指定一局的内存(不需要快照时使用).
func DelRound(uid, roundId int64) {
	if uid == 0 || roundId == 0 {
		return
	}
	s := getShard(uid)
	s.mu.Lock()
	defer s.mu.Unlock()

	rounds := s.players[uid]
	if rounds == nil {
		return
	}
	delete(rounds, roundId)
	if len(rounds) == 0 {
		delete(s.players, uid)
	}
}

// DelPlayerInfo 删除该玩家所有局的内存(清档/异常兜底使用).
func DelPlayerInfo(uid int64) {
	if uid == 0 {
		return
	}
	s := getShard(uid)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.players, uid)
}

// AddMonsters 把一批击杀计入 Round(金币累计+怪物列表追加).
// 调用约束:传入的必须是分片锁保护中的 Round,或已摘除独占的快照,方法内部不再加锁.
func (r *Round) AddMonsters(monsterIds []uint32) {
	if r == nil || len(monsterIds) == 0 {
		return
	}
	if r.ItemMap == nil {
		r.ItemMap = make(map[int64]int64)
	}
	for _, monsterId := range monsterIds {
		r.ItemMap[constValue.GoldItemId] += monsterGold(monsterId)
	}
	r.MonsterIds = append(r.MonsterIds, monsterIds...)
}

// CalcRoundReward 根据一局快照 + 精准度计算最终奖励.
// 传入的应是 TakeRound 返回的独占快照,无需加锁.
// 返回: 待发放的 itemMap(itemId->count), 精准度额外金币.
func CalcRoundReward(round *Round, precision int32) (map[int64]int64, int64) {
	reward := make(map[int64]int64)
	if round == nil {
		return reward, 0
	}
	for k, v := range round.ItemMap {
		if v > 0 {
			reward[k] += v
		}
	}
	var precisionExtraGold int64
	if precision > 0 {
		// todo: 精准度加成规则后续按策划配置调整,目前为 precision * 10
		precisionExtraGold = int64(precision) * constValue.BaseValue10
		reward[constValue.GoldItemId] += precisionExtraGold
	}
	return reward, precisionExtraGold
}
