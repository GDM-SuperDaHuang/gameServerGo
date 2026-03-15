package rank

import (
	"gameServer/pkg/cache/ssdb"
	"sync"
)

var (
	key              = "rankGoldCoin" //道具表,KV, key:itemId
	rankServerManger = &RankServerManger{
		rankMap: make(map[string]*Rank),
	}
)

func GetRankServer(rankType string) *Rank {
	rankServerManger.lock.Lock()
	defer rankServerManger.lock.Unlock()
	if rankType == "" {
		rankType = key
	}
	rank := rankServerManger.rankMap[rankType]
	if rank == nil {
		rankServerManger.rankMap[rankType] = &Rank{
			rankType: rankType,
		}
	}
	return rankServerManger.rankMap[rankType]
}

type Rank struct {
	rankType     string //排行榜类型
	userInfoList []*userInfo
}

type userInfo struct {
	userId uint64
	score  uint64
}

type RankServerManger struct {
	lock    sync.RWMutex //
	rankMap map[string]*Rank
}

// ==================== 基础操作 ====================

// UpdateScore 更新用户分数（如果不存在则添加）
func (r *Rank) UpdateScore(userId string, score int64) error {
	return ssdb.GetClient().ZSet(r.rankType, userId, score)
}

// GetScore 获取用户当前分数
func (r *Rank) GetScore(userId string) (int64, error) {
	return ssdb.GetClient().ZGet(r.rankType, userId)
}

// IncrScore 增加用户分数（支持负数，用于扣分）
func (r *Rank) IncrScore(userId string, delta int64) (int64, error) {
	return ssdb.GetClient().ZIncr(r.rankType, userId, delta)
}

// RemoveUser 从排行榜移除用户
func (r *Rank) RemoveUser(userId string) error {
	return ssdb.GetClient().ZDel(r.rankType, userId)
}

// UserExists 检查用户是否在排行榜中
func (r *Rank) UserExists(userId string) (bool, error) {
	return ssdb.GetClient().ZExists(r.rankType, userId)
}

// ==================== 排名查询（注意：大数据量时较慢，建议缓存） ====================

// GetRank 获取用户正序排名（第1名返回0）⚠️ 大数据量时慢
func (r *Rank) GetRank(userId string) (int64, error) {
	return ssdb.GetClient().ZRank(r.rankType, userId)
}

// GetReverseRank 获取用户倒序排名（第1名返回0）⚠️ 大数据量时慢
func (r *Rank) GetReverseRank(userId string) (int64, error) {
	return ssdb.GetClient().ZRRank(r.rankType, userId)
}

// ==================== 榜单查询（高效） ====================

// GetTopN 获取前N名（高分在前）- 最常用
func (r *Rank) GetTopN(n int64) ([]string, []int64, error) {
	// 从尾部取N个（分数最高的），倒序排列
	// offset=0, limit=n，因为是倒序，所以是高分在前
	return ssdb.GetClient().ZRRangeSlice(r.rankType, 0, n)
}

// GetBottomN 获取倒数N名（低分在前）
func (r *Rank) GetBottomN(n int64) ([]string, []int64, error) {
	return ssdb.GetClient().ZRangeSlice(r.rankType, 0, n)
}

// GetRangeByRank 根据排名区间获取（如第10-20名）
// start, end 从0开始，包含end
func (r *Rank) GetRangeByRank(start, end int64) ([]string, []int64, error) {
	limit := end - start + 1
	// 倒序取 = 高分在前
	return ssdb.GetClient().ZRRangeSlice(r.rankType, start, limit)
}

// ==================== 分数区间查询 ====================

// GetUsersByScoreRange 获取分数在 [minScore, maxScore] 区间的用户
func (r *Rank) GetUsersByScoreRange(minScore, maxScore interface{}, limit int64) ([]string, []int64, error) {
	// keyStart="" 表示从最开始，scoreStart=minScore, scoreEnd=maxScore
	return ssdb.GetClient().ZScan(r.rankType, "", minScore, maxScore, limit)
}

// CountByScoreRange 统计分数区间内的用户数量
func (r *Rank) CountByScoreRange(minScore, maxScore interface{}) (int64, error) {
	return ssdb.GetClient().ZCount(r.rankType, minScore, maxScore)
}

// ==================== 批量操作 ====================

// BatchUpdate 批量更新分数
func (r *Rank) BatchUpdate(scores map[string]int64) error {
	return ssdb.GetClient().MultiZSet(r.rankType, scores)
}

// BatchGetScores 批量获取用户分数
func (r *Rank) BatchGetScores(userIDs []string) (map[string]int64, error) {
	return ssdb.GetClient().MultiZGetArray(r.rankType, userIDs)
}

// BatchRemove 批量移除用户
func (r *Rank) BatchRemove(userIDs []string) error {
	return ssdb.GetClient().MultiZDel(r.rankType, userIDs...)
}

// ==================== 榜单管理 ====================

// GetTotalCount 获取榜单总人数
func (r *Rank) GetTotalCount() (int64, error) {
	return ssdb.GetClient().ZSize(r.rankType)
}

// ClearRank 清空整个排行榜
func (r *Rank) ClearRank() error {
	return ssdb.GetClient().ZClear(r.rankType)
}

// DeleteByRankRange 删除排名区间内的用户（如删除100名以后的）
func (r *Rank) DeleteByRankRange(start, end int64) error {
	return ssdb.GetClient().ZRemRangeByRank(r.rankType, start, end)
}

// DeleteByScoreRange 删除分数区间内的用户（如删除0分以下的）
func (r *Rank) DeleteByScoreRange(minScore, maxScore int64) error {
	return ssdb.GetClient().ZRemRangeByScore(r.rankType, minScore, maxScore)
}

// PopTopN 移除并返回前N名（用于颁奖后清空）
func (r *Rank) PopTopN(n int64) (map[string]int64, error) {
	return ssdb.GetClient().ZPopBack(r.rankType, n) // 尾部是高分
}

// PopBottomN 移除并返回后N名
func (r *Rank) PopBottomN(n int64) (map[string]int64, error) {
	return ssdb.GetClient().ZPopFront(r.rankType, n) // 头部是低分
}

// ==================== 多榜管理 ====================

// ListRanks 列出所有排行榜名称（支持前缀筛选）
func (r *Rank) ListRanks(prefixStart, prefixEnd string, limit int64) ([]string, error) {
	return ssdb.GetClient().ZList(prefixStart, prefixEnd, limit)
}

//// GetRankStats 获取榜单统计信息
//func (r *Rank) GetRankStats(rankName string) (*RankStats, error) {
//	total, err := ssdb.GetClient().ZSize(rankName)
//	if err != nil {
//		return nil, err
//	}
//
//	sum, err := ssdb.GetClient().ZSum(   "", "")
//	if err != nil {
//		return nil, err
//	}
//
//	avg, err := ssdb.GetClient().ZAvg(   "", "")
//	if err != nil {
//		return nil, err
//	}
//
//}
