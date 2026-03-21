package weight

import (
	"sort"

	"golang.org/x/exp/rand"
)

type WeightPicker struct {
	ids    []int
	prefix []int64
	total  int64
}

func NewWeightPicker(m map[int]int64) *WeightPicker {
	wp := &WeightPicker{
		ids:    make([]int, 0, len(m)),
		prefix: make([]int64, 0, len(m)),
	}

	var sum int64 = 0
	for id, w := range m {
		if w <= 0 {
			continue
		}
		sum += w
		wp.ids = append(wp.ids, id)
		wp.prefix = append(wp.prefix, sum)
	}

	wp.total = sum
	return wp
}

// 抽1个
func (wp *WeightPicker) PickOne() int {
	if wp.total == 0 {
		return -1
	}

	r := rand.Int63n(wp.total) + 1

	// 二分查找
	idx := sort.Search(len(wp.prefix), func(i int) bool {
		return wp.prefix[i] >= r
	})

	return wp.ids[idx]
}

// 抽n个（可重复）
func (wp *WeightPicker) PickN(n int) []int {
	res := make([]int, 0, n)
	for i := 0; i < n; i++ {
		res = append(res, wp.PickOne())
	}
	return res
}

// 不重复抽取（无放回）
func PickNNoRepeat(m map[int]int64, n int) []int {
	res := make([]int, 0, n)

	// 拷贝一份
	tmp := make(map[int]int64)
	for k, v := range m {
		tmp[k] = v
	}

	for i := 0; i < n && len(tmp) > 0; i++ {
		wp := NewWeightPicker(tmp)
		id := wp.PickOne()
		if id == -1 {
			break
		}

		res = append(res, id)
		delete(tmp, id)
	}

	return res
}
