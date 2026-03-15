package crontab

import (
	"sync"
	"time"

	cron "github.com/robfig/cron/v3"
)

var (
	crontabOnce sync.Once
	crontab     *cron.Cron
)

// initCrontab 初始化全局实例
func initCrontab() {
	crontab = cron.New(cron.WithSeconds(), cron.WithLocation(time.Local))
}

// Get 获取全局实例
func Get() *cron.Cron {
	crontabOnce.Do(initCrontab)
	return crontab
}

// Start 执行全局实例，非阻塞
func Start() {
	Get().Start()
}

// AddFunc 添加任务
func AddFunc(spec string, cmd func()) (cron.EntryID, error) {
	return Get().AddFunc(spec, cmd)
}

// AddDelay 添加一次性延时任务
func AddDelay(delay time.Duration, job func()) cron.EntryID {
	targetTime := time.Now().Add(delay)
	schedule := &OnceSchedule{TargetTime: targetTime}
	return Get().Schedule(schedule, cron.FuncJob(job))
}

// OnceSchedule 自定义一次性调度器
type OnceSchedule struct {
	TargetTime time.Time
}

// Next ..
func (s *OnceSchedule) Next(t time.Time) time.Time {
	if t.Before(s.TargetTime) {
		return s.TargetTime
	}

	// 返回零时间表示不再执行
	return time.Time{}
}
