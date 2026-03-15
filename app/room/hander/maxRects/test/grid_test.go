package test

import (
	"fmt"
	"gameServer/app/room/hander/maxRects"
	"sort"
	"testing"
)

func TestRun(t *testing.T) {

	// 测试数据：包含各种尺寸的矩形
	items := []maxRects.Item{
		{Id: 1, Length: 2, Width: 4},
		{Id: 2, Length: 4, Width: 2},
		{Id: 3, Length: 4, Width: 3},
		{Id: 2, Length: 5, Width: 1},
		{Id: 5, Length: 6, Width: 2},
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Length*items[i].Width >
			items[j].Length*items[j].Width
	})
	result := maxRects.Pack(items)

	if err := maxRects.CheckBounds(10, result); err != nil {
		panic(err)
	}

	if err := maxRects.CheckOverlap(10, result); err != nil {
		panic(err)
	}

	maxRects.DebugPrintBoard(10, result)
	fmt.Println("═══════════════════════════════════════════════════════════════", result)
	fmt.Println("测试1：允许旋转（优化高度）")
	fmt.Println("═══════════════════════════════════════════════════════════════")
}
