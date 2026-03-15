package config

import "gameServer/common/config"

// 注册
var (
	characterList    = []*config.Heros{}
	itemList         = []*config.Item{}
	receiveAwardList = []*config.ReceiveAward{}

	allStructMap = map[string]interface{}{
		"heros":        &characterList,
		"item":         &itemList,
		"receiveAward": &receiveAwardList,
	}
)

func GetReceiveAwardConfigById(id int) *config.ReceiveAward {
	infos, ok := allStructMap["receiveAward"]
	if !ok {
		return nil
	}
	prt := infos.(*[]*config.ReceiveAward)
	ls := *prt
	for _, info := range ls {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func GetAllExcelConfig() map[string]interface{} {
	return allStructMap
}

func GetHeroConfigById(id uint32) *config.Heros {
	all, ok := allStructMap["heros"]
	if !ok {
		return nil
	}
	prt := all.(*[]*config.Heros)
	infos := *prt
	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func GetItemConfigById(id int) *config.Item {
	roomInfo, ok := allStructMap["item"]
	if !ok {
		return nil
	}
	prt := roomInfo.(*[]*config.Item)
	infos := *prt
	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func GetAllItemConfigById() []*config.Item {
	roomInfo, ok := allStructMap["item"]
	if !ok {
		return nil
	}
	prt := roomInfo.(*[]*config.Item)
	infos := *prt
	return infos
}
