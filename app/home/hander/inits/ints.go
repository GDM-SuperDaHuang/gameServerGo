package inits

import (
	"gameServer/app/home/hander/config"
	"gameServer/app/home/hander/rank"
	"gameServer/common/constValue"
	"gameServer/common/db/items"
	"gameServer/common/db/user"
	"gameServer/pkg/crontab"
	"gameServer/pkg/excel/reader"
	"gameServer/pkg/logger/log2"
	"strconv"

	"go.uber.org/zap"
)

// ┌───────────── 秒 (0-59)
// │ ┌───────────── 分钟 (0-59)
// │ │ ┌───────────── 小时 (0-23)
// │ │ │ ┌───────────── 日期 (1-31)
// │ │ │ │ ┌───────────── 月份 (1-12)
// │ │ │ │ │ ┌───────────── 星期 (0-6, 0=周日)
// │ │ │ │ │ │
// * * * * * *

// 示例: "*/5 * * * * *" 每 5 秒执行一次
// 示例: "0 */5 * * * *" 每 5 分钟执行一次
// 示例: "0 0 * * * *" 每小时 0 分 0 秒 执行一次
// 示例: "0 0 0 * * *" 每天 0小时 0 分 0 秒 执行一次

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

	//rankGold()
	crontab.Start()
	//crontab.AddFunc("0 0 0 * * *", func() { //每天晚上12点一次
	//	RankGold()
	//})
	crontab.AddFunc("*/5 * * * * *", func() { //每5s一次
		RankGold()
	})

}

func RankGold() {
	allUser, err := user.GetAllUserNoCache()
	if err != nil {
		log2.Get().Error("RankServer GetAllUserNoCache failed ", zap.Any("err:", err))
		return
	}
	if allUser == nil {
		return
	}
	if len(allUser) == 0 {
		return
	}

	server := rank.GetRankServer("")
	err = server.ClearRank()
	if err != nil {
		log2.Get().Error("RankServer ClearRank failed ", zap.Any("err:", err))
		return
	}
	for userId, _ := range allUser {
		count, err := items.GetItemNoCache(userId, constValue.GoldItemId)
		if err != nil {
			log2.Get().Error("RankServer GetItemNoCache failed ", zap.Any("err:", err))
			return
		}
		err = server.UpdateScore(strconv.FormatUint(userId, 10), count)
		if err != nil {
			return
		}
	}
}
