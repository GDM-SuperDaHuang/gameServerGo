package weight

import (
	"time"

	"golang.org/x/exp/rand"
)

// WeightedRandom 按权重随机选择
type WeightedRandom struct {
	r *rand.Rand
}

// NewWeightedRandom 创建新的权重随机选择器
func NewWeightedRandom() *WeightedRandom {
	return &WeightedRandom{
		r: rand.New(rand.NewSource(uint64(time.Now().UnixNano()))),
	}
}

// SelectByWeight 根据权重选择 n 个 ID（可重复）
// weights: map[id]weight，weight 越大被选中的概率越高
// n: 需要选择的数量
// 返回: 选中的 ID 集合
func (wr *WeightedRandom) SelectByWeight(weights map[int]int, n int) []int {
	if n <= 0 || len(weights) == 0 {
		return []int{}
	}

	// 计算总权重
	totalWeight := 0
	for _, w := range weights {
		if w > 0 {
			totalWeight += w
		}
	}

	if totalWeight == 0 {
		return []int{}
	}

	result := make([]int, 0, n)

	for i := 0; i < n; i++ {
		// 生成 [0, totalWeight) 的随机数
		r := wr.r.Intn(totalWeight)

		// 根据权重选择
		cumulative := 0
		for id, weight := range weights {
			if weight <= 0 {
				continue
			}
			cumulative += weight
			if r < cumulative {
				result = append(result, id)
				break
			}
		}
	}

	return result
}

// SelectByWeightUnique 根据权重选择 n 个不重复的 ID
// 如果 n > 可用ID数量，返回所有可用ID
func (wr *WeightedRandom) SelectByWeightUnique(weights map[int]int, n int) []int {
	if n <= 0 || len(weights) == 0 {
		return []int{}
	}

	// 过滤有效权重并排序（便于确定性结果）
	type item struct {
		id     int
		weight int
	}

	var items []item
	totalWeight := 0

	for id, w := range weights {
		if w > 0 {
			items = append(items, item{id: id, weight: w})
			totalWeight += w
		}
	}

	if len(items) == 0 {
		return []int{}
	}

	// 如果请求数量超过可用数量，调整n
	if n > len(items) {
		n = len(items)
	}

	result := make([]int, 0, n)
	remaining := make([]item, len(items))
	copy(remaining, items)
	remainingTotal := totalWeight

	for len(result) < n && remainingTotal > 0 {
		// 在剩余项中按权重选择
		r := wr.r.Intn(remainingTotal)
		cumulative := 0

		for i, it := range remaining {
			cumulative += it.weight
			if r < cumulative {
				result = append(result, it.id)

				// 移除已选中的项
				remainingTotal -= it.weight
				remaining = append(remaining[:i], remaining[i+1:]...)
				break
			}
		}
	}

	return result
}

// SelectByWeightWithRecord 按权重选择，并返回每个ID被选中的次数统计
func (wr *WeightedRandom) SelectByWeightWithRecord(weights map[int]int, n int) ([]int, map[int]int) {
	result := wr.SelectByWeight(weights, n)

	// 统计次数
	counts := make(map[int]int)
	for _, id := range result {
		counts[id]++
	}

	return result, counts
}
