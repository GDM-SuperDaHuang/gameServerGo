package test

import (
	"fmt"
	"gameServer/app/room/hander/config"
	"gameServer/app/room/hander/logic"
	"gameServer/app/room/hander/maxRects"
	"gameServer/common/constValue"
	"gameServer/pkg/excel/reader"
	"sort"
	"strconv"
	"testing"
	"time"

	"golang.org/x/exp/rand"

	"github.com/charmbracelet/lipgloss"
)

var (
	// 定义样式
	cellStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Width(4).Height(2).
			Align(lipgloss.Center, lipgloss.Center)

	filledStyle = cellStyle.Copy().Background(lipgloss.Color("#7D56F4")).Foreground(lipgloss.Color("#FFF"))
	emptyStyle  = cellStyle.Copy().Foreground(lipgloss.Color("#666"))
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

	//cfg := config.GetRoomConfigByRoomId(1)
	//// 分配藏品信息
	//sum := int(cfg.ItemSum)
	//itemList := make([]int, 0, sum)
	//maxLen := len(cfg.ItemList)
	//for i := 0; i < sum; i++ {
	//	index := rand.Intn(maxLen)
	//	itemList = append(itemList, cfg.ItemList[index])
	//}

	//gridInfo := make([]maxRects.Item, 0, len(itemList))
	//for _, itemId := range itemList {
	//	info := config.GetItemConfigById(itemId)
	//	if info == nil {
	//		log2.Get().Error("GetItemConfigById failed ", zap.Any("itemId", itemId))
	//		continue
	//	}
	//	if info.Area == nil || len(info.Area) != 2 {
	//		log2.Get().Error("ItemConfig Area is null", zap.Any("itemId", itemId))
	//		continue
	//	}
	//	gridInfo = append(gridInfo, maxRects.Item{
	//		Id:     itemId,
	//		Length: info.Area[0],
	//		Width:  info.Area[1],
	//	})
	//}

	gridInfo := []maxRects.Item{
		{
			Id:     1,
			Length: 4,
			Width:  4,
		}, {
			Id:     1,
			Length: 4,
			Width:  3,
		}, {
			Id:     1,
			Length: 3,
			Width:  3,
		}, {
			Id:     1,
			Length: 3,
			Width:  3,
		},
		{
			Id:     1,
			Length: 3,
			Width:  3,
		},

		{
			Id:     1,
			Length: 2,
			Width:  2,
		}, {
			Id:     1,
			Length: 1,
			Width:  4,
		}, {
			Id:     1,
			Length: 1,
			Width:  1,
		}, {
			Id:     1,
			Length: 1,
			Width:  1,
		},

		{
			Id:     1,
			Length: 1,
			Width:  1,
		},
		{
			Id:     1,
			Length: 1,
			Width:  1,
		},
	}
	//

	// 优化排序
	sort.Slice(gridInfo, func(i, j int) bool {
		return gridInfo[i].Length*gridInfo[i].Width >
			gridInfo[j].Length*gridInfo[j].Width
	})
	result := maxRects.Pack(gridInfo)

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

	// 12x12数据
	data := make(map[[2]uint32]string)
	for _, info := range result {
		strW := info.EndX - info.StartX
		strH := info.EndY - info.StartY
		index := getIndexByXY(info.StartX, info.StartY)
		str := strconv.FormatUint(uint64(strW), 10) + "X" + strconv.FormatUint(uint64(strH), 10) + "\n " + strconv.FormatUint(uint64(index), 10)
		data[[2]uint32{info.StartX, info.StartY}] = str
	}

	// 渲染格子
	var rows []string
	for y := uint32(0); y < 6; y++ {
		var cols []string
		for x := uint32(0); x < 12; x++ {
			if v, ok := data[[2]uint32{x, y}]; ok {
				cols = append(cols, filledStyle.Render(v))
				index := getIndexByXY(x, y)
				fmt.Printf("index: %v\n", index)
			} else {
				cols = append(cols, emptyStyle.Render("·"))
			}
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cols...))
	}
	fmt.Println(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func TestRun2(t *testing.T) {

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

	for i := 0; i < 100000; i++ {
		roomConfig := config.GetRoomConfigByRoomId(uint32(rand.Intn(3) + 1))
		if roomConfig == nil {
			return
		}
		room := logic.NewRoom(1, roomConfig, nil)
		g := room.GettestG()
		flag := false
		for _, info := range g {
			index := getIndexByXY(info.StartX, info.StartY)
			if index == 0 {
				flag = true
				break
			}
		}
		if !flag {
			return
		}
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

// ///////////////////////////////////////////
// 存储数据: key=index, value=Data字符串
var gridData = make(map[int]string)

// XY转索引
func idx(x, y int) int {
	return y*12 + x
}
