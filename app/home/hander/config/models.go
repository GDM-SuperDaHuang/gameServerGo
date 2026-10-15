package config

import (
	"gameServer/common/config"
	"gameServer/pkg/excel/reader"
	"sync"
)

// 注册
var (
	// 读写锁 - 保护所有配置数据的并发访问
	configMu sync.RWMutex

	characterList    = []*config.Heros{}
	itemList         = []*config.Item{}
	receiveAwardList = []*config.ReceiveAward{}
	taskList         = []*config.Task{}
	progressList     = []*config.Progress{}
	giftPackageList  = []*config.GiftPackage{}
	signInList       = []*config.SignIn{}

	allStructMap = map[string]interface{}{
		"heros":        &characterList,
		"item":         &itemList,
		"receiveAward": &receiveAwardList,
		"task":         &taskList,
		"progress":     &progressList,
		"giftPackage":  &giftPackageList,
		"signIn":       &signInList,
	}
)

func GetAllExcelConfig() map[string]interface{} {
	return allStructMap
}

func GetItemConfigById(id int) *config.Item {
	configMu.RLock()
	defer configMu.RUnlock()

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

// UpdateTaskList 原子性更新任务列表（写操作）
func UpdateConfig() error {
	configMu.Lock()         // 获取写锁（阻塞所有读写）
	defer configMu.Unlock() // 函数返回时释放写锁
	// 创建读取器（指向excels目录）
	r := reader.NewExcelReader("./excels")
	allData, err := r.ReadAllExcels()
	if err != nil || len(allData) == 0 {
		return err
	}
	err = r.ReadSheetToStruct(allData, allStructMap)
	if err != nil {
		return err
	}
	excelConfig := GetAllExcelConfig()
	for _, info := range excelConfig {
		if info == nil {
			return err
		}
	}
	return nil
}
