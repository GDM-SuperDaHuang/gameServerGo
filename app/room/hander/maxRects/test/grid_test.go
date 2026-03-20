package test

import (
	"fmt"
	"gameServer/app/room/hander/config"
	"gameServer/app/room/hander/maxRects"
	"gameServer/common/constValue"
	"gameServer/pkg/excel/reader"
	"gameServer/pkg/logger/log2"
	"sort"
	"testing"
	"time"

	"go.uber.org/zap"
	"golang.org/x/exp/rand"
)

func TestRun(t *testing.T) {

	//初始化
	rand.Seed(uint64(time.Now().UnixNano()))
	// 创建读取器（指向excels目录）
	r := reader.NewExcelReader("./testExcels")
	allData, err := r.ReadAllExcels()
	if err != nil {
		panic(err)
	}
	err = r.ReadSheetToStruct(allData, config.GetAllExcelConfig())
	if err != nil {
		panic(err)
	}
	excelConfig := config.GetAllExcelConfig()
	for _, info := range excelConfig {
		if info == nil {
			panic(err)
		}
	}

	///////////////////////////////////////////////////////////////////

	cfg := config.GetRoomConfigByRoomId(1)
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
	result := maxRects.Pack(gridInfo)

	//// 测试数据：包含各种尺寸的矩形
	//items := []maxRects.Item{
	//	{Id: 1, Length: 5, Width: 3},
	//	{Id: 2, Length: 3, Width: 3},
	//	{Id: 3, Length: 2, Width: 2},
	//	{Id: 4, Length: 1, Width: 3},
	//
	//	{Id: 5, Length: 2, Width: 2},
	//
	//	{Id: 6, Length: 2, Width: 2},
	//	{Id: 7, Length: 2, Width: 2},
	//	{Id: 8, Length: 2, Width: 2},
	//
	//	{Id: 9, Length: 2, Width: 2},
	//
	//	{Id: 10, Length: 3, Width: 1},
	//	{Id: 11, Length: 1, Width: 1},
	//	{Id: 12, Length: 1, Width: 1},
	//}
	//sort.Slice(items, func(i, j int) bool {
	//	return items[i].Length*items[i].Width >
	//		items[j].Length*items[j].Width
	//})
	//result := maxRects.Pack(items)

	if err := maxRects.CheckBounds(12, result); err != nil {
		panic(err)
	}

	if err := maxRects.CheckOverlap(12, result); err != nil {
		panic(err)
	}

	maxRects.DebugPrintBoard(12, result)
	fmt.Println("═══════════════════════════════════════════════════════════════", result)
	fmt.Println("测试1：允许旋转（优化高度）")
	fmt.Println("═══════════════════════════════════════════════════════════════")

	for _, info := range result {
		index := getIndexByXY(info.StartX, info.StartY)
		fmt.Printf("index: %v\n", index)
	}
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
