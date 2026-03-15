package utils

import "time"

// IsToday 判断时间戳是否在今天内 秒
func IsToday(timestamp int64) bool {
	// 将时间戳转换为时间
	t := time.Unix(timestamp, 0)

	// 获取当前时间
	now := time.Now()

	// 获取今天的开始时间（0点0分0秒）
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 获取明天的开始时间（即今天结束时间的下一秒）
	tomorrowStart := todayStart.Add(24 * time.Hour)

	// 判断时间戳是否在今天的时间范围内 [今天0点, 明天0点)
	return t.After(todayStart) || t.Equal(todayStart) && t.Before(tomorrowStart)
}
