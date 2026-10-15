package itemAllocation

import (
	"gameServer/app/room/hander/config"
	commonConfig "gameServer/common/config"
	"gameServer/pkg/logger/log2"
	"gameServer/pkg/random/weight"

	"go.uber.org/zap"
	"golang.org/x/exp/rand"
)

func GetItems(idMap map[int]int) ([]*commonConfig.Item, map[int8]int) {
	random := weight.NewWeightedRandom()
	// 数量选择
	idWeight := random.SelectByWeight(idMap, 1)
	if len(idWeight) == 0 {
		log2.Get().Error("random.SelectByWeight(idMap, 1) len ==0 ")
		return nil, nil
	}
	id := idWeight[0]
	roomItemAllocationConfig := config.GetRoomItemAllocationConfigById(id)
	if roomItemAllocationConfig == nil {
		log2.Get().Error("GetRoomItemAllocationConfigById is null ,not found")
		return nil, nil
	}

	allItemConfig := config.GetAllItemConfig()
	if allItemConfig == nil {
		log2.Get().Error("GetAllItemConfig is null ,not found")
		return nil, nil
	}

	// 数量选择
	sumWeight := random.SelectByWeight(roomItemAllocationConfig.SumWeightMap, 1)
	if len(sumWeight) == 0 {
		log2.Get().Error("sumWeight len == 0", zap.Any("roomItemAllocationConfig", roomItemAllocationConfig.SumWeightMap))
		return nil, nil
	}
	sum := sumWeight[0]
	if sum <= 2 {
		log2.Get().Error("sum <= 2", zap.Any("sumWeight", sumWeight))
		return nil, nil
	}
	// 藏品选择
	itemList := random.SelectByWeight(roomItemAllocationConfig.ItemWeightMap, sum)
	if len(itemList) == 0 {
		log2.Get().Error("itemList is null ,not found", zap.Any("roomItemAllocationConfig.ItemWeightMap", roomItemAllocationConfig.ItemWeightMap), zap.Any("sum", sum))
	}

	res := make([]*commonConfig.Item, 0, len(itemList))
	for _, itemId := range itemList {
		item := config.GetItemConfigById(itemId)
		if item == nil {
			log2.Get().Error("GetItemConfigById config itemId is null")
			continue
		}
		res = append(res, item)
	}

	entryMap := make(map[int8]int)
	// 词条选择
	if roomItemAllocationConfig.IsEntry2 {
		entryWeight := random.SelectByWeightUnique(roomItemAllocationConfig.EntryMap2, 1)
		if len(entryWeight) == 0 {
			log2.Get().Error("GetItemConfigById entryWeight is null", zap.Int("roomItemAllocationConfig", id))
			return nil, nil
		}
		entryId := entryWeight[0]
		e := config.GetEntry2ById(entryId)
		if e == nil {
			log2.Get().Error("GetEntry2ById config entry is null", zap.Int("entryId", entryId))
			return nil, nil
		}
		entryList := random.SelectByWeightUnique(e.EntryMap, len(roomItemAllocationConfig.EntrySum2))
		for i, eId := range entryList {
			rId := int8(roomItemAllocationConfig.EntrySum2[i])
			entryMap[rId] = eId
		}
	} else {
		entryWeight := random.SelectByWeight(roomItemAllocationConfig.EntryMap1, 1)
		if len(entryWeight) == 0 {
			log2.Get().Error("roomItemAllocationConfig.EntryMap1   entryWeight len == 0 ")
			return nil, nil
		}
		entryConfig := config.GetEntry1ById(entryWeight[0])
		if entryConfig == nil {
			log2.Get().Error("GetEntry1ById is null ,not found")
			return nil, nil
		}
		entryMap = entryConfig.EntryMap
	}

	return res, entryMap
}

func GetItemSafety(idMap map[int]int) ([]*commonConfig.Item, map[int8]int) {
	items, m := GetItems(idMap)
	if len(items) == 0 {
		items = []*commonConfig.Item{}
		sum := rand.Intn(5) + 4
		for i := 0; i < sum; i++ {
			index := rand.Intn(len(items))
			item := items[index]
			items = append(items, item)
		}
	}
	if len(m) == 0 {
		m = make(map[int8]int)
	}
	return items, m
}

var (
	itemsAll = []*commonConfig.Item{
		{
			Id:       200007,
			ItemType: 3,
			Quality:  5,
			Area:     []uint32{3, 1},
			Price:    map[int]int64{6: 65610},
			UserType: 1,
		}, {
			Id:       200008,
			ItemType: 3,
			Quality:  5,
			Area:     []uint32{2, 4},
			Price:    map[int]int64{6: 61143},
			UserType: 1,
		}, {
			Id:       200009,
			ItemType: 3,
			Quality:  5,
			Area:     []uint32{3, 2},
			Price:    map[int]int64{6: 51801},
			UserType: 1,
		},

		{
			Id:       200071,
			ItemType: 4,
			Quality:  4,
			Area:     []uint32{2, 3},
			Price:    map[int]int64{6: 12581},
			UserType: 1,
		}, {
			Id:       200072,
			ItemType: 4,
			Quality:  4,
			Area:     []uint32{2, 3},
			Price:    map[int]int64{6: 11424},
			UserType: 1,
		}, {
			Id:       200073,
			ItemType: 4,
			Quality:  4,
			Area:     []uint32{2, 2},
			Price:    map[int]int64{6: 9880},
			UserType: 1,
		},

		{
			Id:       200121,
			ItemType: 5,
			Quality:  5,
			Area:     []uint32{1, 1},
			Price:    map[int]int64{6: 14332},
			UserType: 1,
		}, {
			Id:       200122,
			ItemType: 5,
			Quality:  5,
			Area:     []uint32{1, 1},
			Price:    map[int]int64{6: 12225},
			UserType: 1,
		}, {
			Id:       200123,
			ItemType: 5,
			Quality:  5,
			Area:     []uint32{1, 1},
			Price:    map[int]int64{6: 9245},
			UserType: 1,
		},

		{
			Id:       200301,
			ItemType: 8,
			Quality:  4,
			Area:     []uint32{3, 3},
			Price:    map[int]int64{6: 12045},
			UserType: 1,
		}, {
			Id:       200302,
			ItemType: 8,
			Quality:  4,
			Area:     []uint32{2, 3},
			Price:    map[int]int64{6: 11768},
			UserType: 1,
		}, {
			Id:       200303,
			ItemType: 8,
			Quality:  4,
			Area:     []uint32{2, 2},
			Price:    map[int]int64{6: 10380},
			UserType: 1,
		},
	}
)
