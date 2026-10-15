package inits

import (
	"context"
	"gameServer/app/home/hander/config"
	"gameServer/common/db/user"
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
	err = user.InitUid()
	if err != nil {
		panic(err)
	}

	// 索引建立
	err = user.EnsureIndexes(context.Background())
	if err != nil {
		panic(err)
	}
}
