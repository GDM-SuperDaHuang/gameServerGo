package test

import (
	"gameServer/app/room/hander/config"
	"gameServer/app/room/hander/normalDistribution"
	"gameServer/common/constValue"
	"gameServer/pkg/excel/reader"
	"testing"
	"time"

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

	upperSum := int64(0)

	allItemConfig := config.GetAllItemConfig()

	normalDistribution.WeightedSample()

	for _, info := range allItemConfig {
		if info == nil {
			panic(err)
		}
		value := info.Price[constValue.GoldItemId]
		upperSum += value
	}

	normal := []normalDistribution.NormalDistribution{
		{
			P:     0.4,
			Upper: upperSum / 6,
			Lower: upperSum / 7,
		},
		{
			P:     0.3,
			Upper: upperSum / 5,
			Lower: upperSum / 6,
		},
		{
			P:     0.2,
			Upper: upperSum / 4,
			Lower: upperSum / 5,
		},
		{
			P:     0.1,
			Upper: upperSum / 3,
			Lower: upperSum / 4,
		},
	}

	const N = 1000000

	count1, count2, count3, count4 := 0, 0, 0, 0

	for i := 0; i < N; i++ {
		res := normalDistribution.SampleItems(normal, allItemConfig, 12)
		sum := int64(0)
		for _, it := range res {
			sum += it.Price[constValue.GoldItemId]
		}

		if sum >= upperSum/7 && sum < upperSum/6 {
			count1++
		} else if sum < upperSum/5 {
			count2++
		} else if sum < upperSum/4 {
			count3++
		} else if sum < upperSum/3 {
			count4++
		}
	}
	p1 := float64(count1) / N
	p2 := float64(count2) / N
	p3 := float64(count3) / N
	p4 := float64(count4) / N

	t.Logf("p1: %+v", p1)
	t.Logf("p2: %+v", p2)
	t.Logf("p3: %+v", p3)
	t.Logf("p4: %+v", p4)

}
