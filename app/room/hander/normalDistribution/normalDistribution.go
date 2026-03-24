package normalDistribution

import (
	"fmt"
	"gameServer/common/config"
	"gameServer/common/constValue"
	"math"
	"sort"
	"time"

	"gonum.org/v1/gonum/stat/distuv"

	"golang.org/x/exp/rand"
)

type NormalDistribution struct {
	P     float64 // 概率
	Upper int64   // 上限
	Lower int64   // 下限
}

//type Item struct {
//	Id        int
//	ItemValue int
//}

// 主函数
func SampleItems(dists []NormalDistribution, items []*config.Item, n int64) []*config.Item {

	// 1. 选区间
	dist := pickDistribution(dists)

	if dist.Upper < dist.Lower {
		fmt.Print("")
	}
	// 2. 正态采样目标值
	target := sampleNormal(dist.Lower, dist.Upper)

	// 3. 构造组合
	return pickItems(items, n, target)
}

//////////////////////////////////////////////////////
// Step1：选区间
//////////////////////////////////////////////////////

func pickDistribution(dists []NormalDistribution) NormalDistribution {
	r := rand.Float64()
	sum := 0.0

	for _, d := range dists {
		sum += d.P
		if r <= sum {
			return d
		}
	}
	return dists[len(dists)-1]
}

//////////////////////////////////////////////////////
// Step2：正态采样
//////////////////////////////////////////////////////

func sampleNormal(lower, upper int64) int64 {

	//alpha := (float64(lower) - mu) / sigma
	//beta := (float64(upper) - mu) / sigma
	//
	//cdfA := normCDF(alpha)
	//cdfB := normCDF(beta)
	//
	//u := rand.Float64()*(cdfB-cdfA) + cdfA
	//x := mu + sigma*normInv(u)
	//
	//if x < float64(lower) {
	//	x = float64(lower)
	//} else if x > float64(upper) {
	//	x = float64(upper)
	//}
	//return int64(x)
	t := distuv.NewTriangle(float64(lower), float64(upper), float64((lower+upper)/2), rand.New(rand.NewSource(uint64(time.Now().UnixNano()))))
	return int64(t.Rand())
}

// 标准正态 CDF
func normCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

// 标准正态逆 CDF (近似)
func normInv(p float64) float64 {
	// 使用近似算法 (Beasley-Springer/Moro)
	// 这里只提供简单近似，可替换更高精度实现
	return math.Sqrt2 * math.Erfinv(2*p-1)
}

//////////////////////////////////////////////////////
// Step3：构造组合（核心）
//////////////////////////////////////////////////////

func pickItems(items []*config.Item, n int64, target int64) []*config.Item {

	// 按 value 排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].Price[constValue.GoldItemId] < items[j].Price[constValue.GoldItemId]
	})

	res := make([]*config.Item, 0, n)
	sum := int64(0)

	// 1. 初始：接近平均值选
	avg := target / n

	for i := 0; i < len(items) && int64(len(res)) < n; i++ {
		if abs(items[i].Price[constValue.GoldItemId]-avg) < avg {
			res = append(res, items[i])
			sum += items[i].Price[constValue.GoldItemId]
		}
	}

	// 2. 不够补随机
	for int64(len(res)) < n {
		idx := rand.Intn(len(items))
		res = append(res, items[idx])
		sum += items[idx].Price[constValue.GoldItemId]
	}

	// 3. 局部优化（非常关键）
	for iter := 0; iter < 100; iter++ {
		i := rand.Intn(int(n))
		j := rand.Intn(len(items))

		newSum := sum - res[i].Price[constValue.GoldItemId] + items[j].Price[constValue.GoldItemId]

		// 更接近 target 就接受
		if abs(newSum-target) < abs(sum-target) {
			res[i] = items[j]
			sum = newSum
		}
	}

	return res
}

//////////////////////////////////////////////////////
// 工具函数
//////////////////////////////////////////////////////

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// 按类型权重抽取 n 个
func WeightedSample(items []*config.Item, weights map[int8]int, n int) []*config.Item {

	// 1. 按类型分类
	typeMap := map[int8][]*config.Item{}
	for _, it := range items {
		typeMap[it.ItemType] = append(typeMap[it.ItemType], it)
	}

	// 2. 构建类型权重前缀和
	typeList := []int8{}
	cumWeight := []int{}
	total := 0
	for t, w := range weights {
		typeList = append(typeList, t)
		total += w
		cumWeight = append(cumWeight, total)
	}

	// 3. 抽 n 次
	res := make([]*config.Item, 0, n)
	for i := 0; i < n; i++ {
		r := rand.Intn(total) // 0 ~ total-1

		// 找出类型
		var selectedType int8
		for idx, cw := range cumWeight {
			if r < cw {
				selectedType = typeList[idx]
				break
			}
		}

		// 从该类型随机抽物品
		itemsOfType := typeMap[selectedType]
		if len(itemsOfType) == 0 {
			continue // 这个类型没有物品，跳过
		}
		it := itemsOfType[rand.Intn(len(itemsOfType))]
		res = append(res, it)
	}

	return res
}
