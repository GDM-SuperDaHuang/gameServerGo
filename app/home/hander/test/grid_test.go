package test

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Item 矩形物品（可旋转）
type Item struct {
	Id     int
	Length int // 长度 1-6（在板子上的水平尺寸）
	Width  int // 宽度 1-6（在板子上的垂直尺寸，即层高）
}

// Placement 物品放置信息
type Placement struct {
	Item    Item
	Layer   int  // 所在层（从上到下 0开始）
	StartX  int  // 水平起始位置（0-9）
	StartY  int  // 垂直起始位置（累计高度）
	Rotated bool // 是否旋转（交换了Length和Width）
	EndX    int  // 水平结束位置
	EndY    int  // 垂直结束位置
}

// Layer 层信息（每层高度可能不同）
type Layer struct {
	Index     int
	Height    int // 该层的高度（由该层最高的物品决定）
	UsedWidth int // 已使用的水平宽度
	Items     []Placement
	BaseY     int // 该层底部的Y坐标（累计高度）
}

// StripPacker 二维装箱器
type StripPacker struct {
	BoardWidth int // 10
}

func NewStripPacker() *StripPacker {
	return &StripPacker{BoardWidth: 10}
}

// tryPlace 尝试将物品放入某层，返回是否成功及放置信息
func (sp *StripPacker) tryPlace(item Item, layer *Layer, allowRotate bool) (Placement, bool) {
	// 尝试不旋转
	if layer.UsedWidth+item.Length <= sp.BoardWidth && item.Width <= layer.Height {
		// 可以放入（高度适配，宽度够）
		return Placement{
			Item:    item,
			Layer:   layer.Index,
			StartX:  layer.UsedWidth,
			StartY:  layer.BaseY,
			Rotated: false,
			EndX:    layer.UsedWidth + item.Length - 1,
			EndY:    layer.BaseY + item.Width - 1,
		}, true
	}

	// 尝试旋转（如果允许且旋转后尺寸不同）
	if allowRotate && item.Length != item.Width {
		rotatedItem := Item{Id: item.Id, Length: item.Width, Width: item.Length}
		if layer.UsedWidth+rotatedItem.Length <= sp.BoardWidth && rotatedItem.Width <= layer.Height {
			return Placement{
				Item:    rotatedItem,
				Layer:   layer.Index,
				StartX:  layer.UsedWidth,
				StartY:  layer.BaseY,
				Rotated: true,
				EndX:    layer.UsedWidth + rotatedItem.Length - 1,
				EndY:    layer.BaseY + rotatedItem.Width - 1,
			}, true
		}
	}

	return Placement{}, false
}

// Pack 紧凑优先装箱（从上到下，从左到右）
// 策略：FFD + 可变高度层 + 旋转优化
func (sp *StripPacker) Pack(items []Item, allowRotate bool) []Placement {
	// 1. 按面积降序，面积相同按高度降序
	sort.Slice(items, func(i, j int) bool {
		areaI := items[i].Length * items[i].Width
		areaJ := items[j].Length * items[j].Width
		if areaI != areaJ {
			return areaI > areaJ
		}
		// 面积相同，优先放高度大的（减少层数）
		maxDimI := max(items[i].Length, items[i].Width)
		maxDimJ := max(items[j].Length, items[j].Width)
		return maxDimI > maxDimJ
	})

	layers := []Layer{}
	placements := []Placement{}

	for _, item := range items {
		placed := false

		// 2. 从上到下找第一个能放下的层
		for i := range layers {
			placement, ok := sp.tryPlace(item, &layers[i], allowRotate)
			if ok {
				placements = append(placements, placement)
				layers[i].UsedWidth += placement.Item.Length
				layers[i].Items = append(layers[i].Items, placement)
				placed = true
				break // 关键：找到第一个就停，保证从上到下紧凑
			}
		}

		// 3. 所有层都放不下，新建一层
		if !placed {
			newLayerIdx := len(layers)
			baseY := 0
			if newLayerIdx > 0 {
				prevLayer := layers[newLayerIdx-1]
				baseY = prevLayer.BaseY + prevLayer.Height
			}

			// 确定新层高度：取物品的最小高度（优先不旋转，除非旋转后更矮且放得下）
			useHeight := item.Width
			useLength := item.Length
			rotated := false

			if allowRotate && item.Width > item.Length && item.Length <= sp.BoardWidth {
				// 旋转后更矮，且宽度（旋转后的长度）能放下
				useHeight = item.Length
				useLength = item.Width
				rotated = true
			}

			// 创建新层
			newLayer := Layer{
				Index:     newLayerIdx,
				Height:    useHeight,
				UsedWidth: useLength,
				BaseY:     baseY,
				Items:     []Placement{},
			}

			placement := Placement{
				Item:    Item{Id: item.Id, Length: useLength, Width: useHeight},
				Layer:   newLayerIdx,
				StartX:  0,
				StartY:  baseY,
				Rotated: rotated,
				EndX:    useLength - 1,
				EndY:    baseY + useHeight - 1,
			}

			newLayer.Items = append(newLayer.Items, placement)
			layers = append(layers, newLayer)
			placements = append(placements, placement)
		}
	}

	return placements
}

// Visualize 可视化 - 显示ID和位置（考虑不同高度）
func Visualize(placements []Placement, boardWidth int) {
	if len(placements) == 0 {
		fmt.Println("没有物品")
		return
	}

	// 找出总高度和层数
	maxY := 0
	layerMap := make(map[int][]Placement)
	for _, p := range placements {
		if p.EndY > maxY {
			maxY = p.EndY
		}
		layerMap[p.Layer] = append(layerMap[p.Layer], p)
	}
	totalHeight := maxY + 1

	// 构建网格（Y轴从上到下打印，所以反转）
	// grid[y][x] = 物品ID或0
	grid := make([][]int, totalHeight)
	for i := range grid {
		grid[i] = make([]int, boardWidth)
	}

	// 填充网格
	for _, p := range placements {
		for y := p.StartY; y <= p.EndY; y++ {
			for x := p.StartX; x <= p.EndX; x++ {
				grid[y][x] = p.Item.Id
			}
		}
	}

	// 打印标题
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║        2D紧凑装箱可视化 (从上到下 ← 从左到右)                 ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ 板子尺寸: %d × %d (宽×高)                                   ║\n", boardWidth, totalHeight)
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("图例: [RoomType] 物品占据  · 空位")
	fmt.Println()

	// 打印列号
	fmt.Print("    ")
	for i := 0; i < boardWidth; i++ {
		fmt.Printf(" %d  ", i)
	}
	fmt.Println(" Y轴")

	// 构建边框
	cellWidth := 4
	horizLine := strings.Repeat("─", cellWidth)
	topBorder := "  ┌" + strings.Repeat(horizLine+"┬", boardWidth-1) + horizLine + "┐"
	midBorder := "  ├" + strings.Repeat(horizLine+"┼", boardWidth-1) + horizLine + "┤"
	bottomBorder := "  └" + strings.Repeat(horizLine+"┴", boardWidth-1) + horizLine + "┘"

	fmt.Println(topBorder)

	// 从上往下打印（Y轴从上到下）
	for y := totalHeight - 1; y >= 0; y-- {
		// ID行
		fmt.Printf("%2d│", y)
		for x := 0; x < boardWidth; x++ {
			if grid[y][x] != 0 {
				fmt.Printf("%2d │", grid[y][x])
			} else {
				fmt.Printf(" · │")
			}
		}
		fmt.Println()

		// 分隔线（除了最底层）
		if y > 0 {
			fmt.Println(midBorder)
		}
	}

	fmt.Println(bottomBorder)
	fmt.Print("X轴")
	for i := 0; i < boardWidth; i++ {
		fmt.Printf(" %d  ", i)
	}
	fmt.Println()

	// 打印统计
	printStatistics(placements, totalHeight, boardWidth)
}

// 打印统计信息
func printStatistics(placements []Placement, totalHeight, boardWidth int) {
	totalArea := 0
	usedArea := 0

	// 计算每层使用情况
	layerStats := make(map[int]struct {
		height    int
		usedWidth int
		area      int
	})

	for _, p := range placements {
		itemArea := p.Item.Length * p.Item.Width
		totalArea += itemArea
		usedArea += itemArea

		stats := layerStats[p.Layer]
		stats.area += itemArea
		if p.Item.Width > stats.height {
			stats.height = p.Item.Width
		}
		if p.EndX+1 > stats.usedWidth {
			stats.usedWidth = p.EndX + 1
		}
		layerStats[p.Layer] = stats
	}

	boardArea := boardWidth * totalHeight

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      统计信息                              ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ 物品总数:     %3d                                          ║\n", len(placements))
	fmt.Printf("║ 物品总面积:   %3d                                          ║\n", totalArea)
	fmt.Printf("║ 板子总面积:   %3d (%d×%d)                                   ║\n",
		boardArea, boardWidth, totalHeight)
	fmt.Printf("║ 空间利用率:   %5.1f%%                                       ║\n",
		float64(totalArea)/float64(boardArea)*100)
	fmt.Println("╠════════════════════════════════════════════════════════════╣")

	// 每层详情
	fmt.Println("║ 每层详情:                                                  ║")
	for i := 0; i < len(layerStats); i++ {
		stats := layerStats[i]
		fillRate := float64(stats.area) / float64(boardWidth*stats.height) * 100
		bar := strings.Repeat("█", stats.usedWidth) +
			strings.Repeat("░", 10-stats.usedWidth)
		fmt.Printf("║   层%2d: 高=%d [%s] 填充率=%.0f%%                      ║\n",
			i+1, stats.height, bar, fillRate)
	}
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
}

// 打印详细清单
func PrintDetails(placements []Placement) {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    物品放置详情                                  ║")
	fmt.Println("╠══════╦══════╦═══════╦════════╦════════╦════════╦══════════════╣")
	fmt.Println("║  RoomType  ║ 尺寸 ║ 旋转  ║  层号  ║ X坐标  ║ Y坐标  ║    范围      ║")
	fmt.Println("╠══════╬══════╬═══════╬════════╬════════╬════════╬══════════════╣")

	// 按层、X坐标排序
	sort.Slice(placements, func(i, j int) bool {
		if placements[i].Layer != placements[j].Layer {
			return placements[i].Layer < placements[j].Layer
		}
		return placements[i].StartX < placements[j].StartX
	})

	for _, p := range placements {
		rotated := "  -  "
		if p.Rotated {
			rotated = " 90° "
		}
		dim := fmt.Sprintf("%d×%d", p.Item.Length, p.Item.Width)
		if p.Rotated {
			dim = fmt.Sprintf("%d×%d(原%d×%d)",
				p.Item.Length, p.Item.Width,
				p.Item.Width, p.Item.Length)
		}
		fmt.Printf("║ %3d  ║ %-4s ║ %s ║   %2d   ║  %2d~%2d ║  %2d~%2d ║ (%2d,%2d)      ║\n",
			p.Item.Id, dim, rotated, p.Layer+1,
			p.StartX, p.EndX, p.StartY, p.EndY,
			p.StartX, p.StartY)
	}
	fmt.Println("╚══════╩══════╩═══════╩════════╩════════╩════════╩══════════════╝")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TestRun(t *testing.T) {
	packer := NewStripPacker()

	// 测试数据：包含各种尺寸的矩形
	items := []Item{
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
		{Id: 11, Length: 3, Width: 3}, // 3×3 正方形
		//{RoomType: 12, Length: 2, Width: 1}, // 2×1
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("测试1：允许旋转（优化高度）")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	placements := packer.Pack(items, true)
	Visualize(placements, 10)
	PrintDetails(placements)

	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("测试2：禁止旋转")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	placements2 := packer.Pack(items, false)
	Visualize(placements2, 10)
	PrintDetails(placements2)
}
