package inits

import (
	"gameServer/app/room/hander/config"
	"gameServer/app/room/hander/room"
	"gameServer/pkg/excel/reader"
)

func init() {
	// 创建读取器（指向excels目录）
	r := reader.NewExcelReader("./excels")
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
	room.InitRoomConfig()
}
