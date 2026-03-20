package maxRects

import (
	"fmt"
	"gameServer/common/constValue"
)

type Item struct {
	Id     int
	Length uint32 // 水平尺寸
	Width  uint32 // 垂直尺寸
}

type Placement struct {
	Uid         int
	Item        Item
	Layer       int
	StartX      uint32
	StartY      uint32
	EndX        uint32
	EndY        uint32
	Rotated     bool
	ShowInfoMap map[uint64]*ShowInfo //显示类型,<userId,显示信息<类型:是否已经提示>> 0:不显示，1:显示品质，2:显示轮廓，3:显示所有

}

type rect struct {
	x uint32
	y uint32
	w uint32
	h uint32
}

type ShowInfo struct {
	Quality map[uint32]*Quality //<位置,所在变化时轮数索引>
	Contour int8                //0:不显示，发现变化时，所在轮数索引 1~5
	All     int8                //0:不显示，发现变化时，所在轮数索引 1~5
}

type Quality struct {
	RoundIndex  int8 //<位置,所在变化时轮数索引> 1~5
	QualityType int8 //类型
}

func Pack(items []Item) []*Placement {

	freeRects := []rect{
		{0, 0, constValue.WIDE, ^uint32(0) / 2},
	}

	var result []*Placement

	uid := 0
	for _, item := range items {

		bestY := ^uint32(0)
		bestX := ^uint32(0)

		var best rect
		rotated := false

		tryPlace := func(w, h uint32, rot bool) {

			for _, fr := range freeRects {

				if w <= fr.w && h <= fr.h {

					if fr.y < bestY || (fr.y == bestY && fr.x < bestX) {

						bestY = fr.y
						bestX = fr.x
						best = rect{fr.x, fr.y, w, h}
						rotated = rot
					}
				}
			}
		}

		tryPlace(item.Length, item.Width, false)

		if item.Length != item.Width {
			tryPlace(item.Width, item.Length, true)
		}

		node := best

		p := &Placement{
			Uid:     uid,
			Item:    item,
			StartX:  node.x,
			StartY:  node.y,
			EndX:    node.x + node.w,
			EndY:    node.y + node.h,
			Layer:   int(node.y),
			Rotated: rotated,
		}

		result = append(result, p)

		freeRects = splitFreeRects(freeRects, node)
		freeRects = pruneRects(freeRects)
		uid++
	}

	return result
}

func splitFreeRects(freeRects []rect, used rect) []rect {

	var newRects []rect

	for _, fr := range freeRects {

		if !intersect(fr, used) {
			newRects = append(newRects, fr)
			continue
		}

		if used.x > fr.x {
			newRects = append(newRects, rect{
				fr.x,
				fr.y,
				used.x - fr.x,
				fr.h,
			})
		}

		if used.x+used.w < fr.x+fr.w {
			newRects = append(newRects, rect{
				used.x + used.w,
				fr.y,
				fr.x + fr.w - (used.x + used.w),
				fr.h,
			})
		}

		if used.y > fr.y {
			newRects = append(newRects, rect{
				fr.x,
				fr.y,
				fr.w,
				used.y - fr.y,
			})
		}

		if used.y+used.h < fr.y+fr.h {
			newRects = append(newRects, rect{
				fr.x,
				used.y + used.h,
				fr.w,
				fr.y + fr.h - (used.y + used.h),
			})
		}
	}

	return newRects
}

func intersect(a, b rect) bool {

	return !(b.x >= a.x+a.w ||
		b.x+b.w <= a.x ||
		b.y >= a.y+a.h ||
		b.y+b.h <= a.y)
}

func pruneRects(rects []rect) []rect {

	var result []rect

	for i := 0; i < len(rects); i++ {

		contained := false

		for j := 0; j < len(rects); j++ {

			if i == j {
				continue
			}

			if containedIn(rects[i], rects[j]) {
				contained = true
				break
			}
		}

		if !contained {
			result = append(result, rects[i])
		}
	}

	return result
}

func containedIn(a, b rect) bool {

	return a.x >= b.x &&
		a.y >= b.y &&
		a.x+a.w <= b.x+b.w &&
		a.y+a.h <= b.y+b.h
}

func DebugPrintBoard(width uint32, placements []*Placement) {

	var maxY uint32

	for _, p := range placements {
		if p.EndY > maxY {
			maxY = p.EndY
		}
	}

	height := maxY

	board := make([][]rune, height)

	for y := uint32(0); y < height; y++ {
		board[y] = make([]rune, width)
		for x := uint32(0); x < width; x++ {
			board[y][x] = '.'
		}
	}

	for _, p := range placements {

		char := rune('A' + (p.Item.Id % 26))

		for y := p.StartY; y < p.EndY; y++ {
			for x := p.StartX; x < p.EndX; x++ {

				if board[y][x] != '.' {
					board[y][x] = 'X'
				} else {
					board[y][x] = char
				}
			}
		}
	}

	// 打印 X 轴
	fmt.Print("   ")
	for x := uint32(0); x < width; x++ {
		fmt.Print(x % 10)
	}
	fmt.Println()

	// 从上往下打印
	for y := height; y > 0; y-- {

		yy := y - 1

		fmt.Printf("%3d", yy)

		for x := uint32(0); x < width; x++ {
			fmt.Printf("%c", board[yy][x])
		}

		fmt.Println()
	}
}

func CheckOverlap(width uint32, placements []*Placement) error {

	board := map[[2]uint32]int{}

	for _, p := range placements {

		for y := p.StartY; y < p.EndY; y++ {
			for x := p.StartX; x < p.EndX; x++ {

				key := [2]uint32{x, y}

				if id, ok := board[key]; ok {
					return fmt.Errorf(
						"overlap: constValue %d with constValue %d at (%d,%d)",
						id,
						p.Item.Id,
						x,
						y,
					)
				}

				board[key] = p.Item.Id
			}
		}
	}

	return nil
}

func CheckBounds(width uint32, placements []*Placement) error {

	for _, p := range placements {

		if p.EndX > width {
			return fmt.Errorf("constValue %d out of board width", p.Item.Id)
		}

		if p.StartX >= p.EndX {
			return fmt.Errorf("constValue %d invalid X range", p.Item.Id)
		}

		if p.StartY >= p.EndY {
			return fmt.Errorf("constValue %d invalid Y range", p.Item.Id)
		}
	}

	return nil
}
