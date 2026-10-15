package inits

import (
	"gameServer/app/home/hander/config"
	"gameServer/pkg/crontab"
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
func Init() {
	err := config.UpdateConfig()
	if err != nil {
		panic(err)
	}
	crontab.Start()
	crontab.AddFunc("0 0 0 * * *", func() { //每天晚上12点一次
		RankGold()
	})
	crontab.AddFunc("0 * * * * *", func() { //每分钟一次
		clearOutOfRangeData()
	})

	// 数据库批处理
	//registry := Batch.NewRegistry()
	//registry.Register(
	//	Batch.DirtyHero, func(uid uint64) Batch.Storage {
	//		return heros.NewStorage(uid)
	//	})
	//
	//worker := Batch.NewWorker(registry)
	//crontab.AddFunc("*/3 * * * * *", func() { //每3秒一次
	//	Batch.StartPersist(worker)
	//})
}

func clearOutOfRangeData() {
}

func RankGold() {

}
