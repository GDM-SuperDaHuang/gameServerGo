package test

import (
	"fmt"
	"gameServer/app/room/hander/grid"
	"testing"
)

func TestRun(t *testing.T) {
	packer := grid.NewStripPacker()

	// 测试数据：包含各种尺寸的矩形
	items := []grid.Item{
		{Id: 1, Length: 6, Width: 2}, // 6×2 横向
		{Id: 2, Length: 6, Width: 2}, // 6×2
		{Id: 3, Length: 4, Width: 3}, // 4×3 较高
		{Id: 4, Length: 4, Width: 1}, // 4×1 扁的
		//{RoomType: 5, Length: 3, Width: 2}, // 3×2
		{Id: 6, Length: 3, Width: 2}, // 3×2
		//{RoomType: 7, Length: 2, Width: 4},  // 2×4 竖向高
		//{RoomType: 8, Length: 2, Width: 2},  // 2×2 正方形
		//{RoomType: 9, Length: 5, Width: 1},  // 5×1
		//{RoomType: 10, Length: 1, Width: 5}, // 1×5 竖向很高
		//{RoomType: 11, Length: 3, Width: 3}, // 3×3 正方形
		//{RoomType: 12, Length: 2, Width: 1}, // 2×1
	}

	//fmt.Println("═══════════════════════════════════════════════════════════════")
	//fmt.Println("测试1：允许旋转（优化高度）")
	//fmt.Println("═══════════════════════════════════════════════════════════════")
	//placements := packer.Pack(items, true)
	//grid.Visualize(placements, 10)
	//grid.PrintDetails(placements)

	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("测试2：禁止旋转")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	placements2 := packer.Pack(items, false)
	fmt.Println("", placements2)

	//grid.Visualize(placements2, 10)
	//grid.PrintDetails(placements2)
}
