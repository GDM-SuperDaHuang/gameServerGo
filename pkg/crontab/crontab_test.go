package crontab_test

import (
	"sync/atomic"
	"testing"
	"time"

	"server/pkg/crontab"

	"github.com/stretchr/testify/require"
)

func TestAddDelay(t *testing.T) {
	executed := atomic.Bool{}
	delay := time.Second
	startTime := time.Now()

	id := crontab.AddDelay(delay, func() {
		executed.Store(true)
	})

	crontab.Start()
	time.Sleep(time.Second * 2)

	require.True(t, executed.Load(), "延迟任务应该被执行")
	executionTime := time.Since(startTime)
	require.True(t, executionTime >= delay, "执行时间应该不小于设定的延迟时间")

	// 验证任务不会重复执行
	executed.Store(false)
	time.Sleep(time.Second)
	require.False(t, executed.Load(), "延迟任务不应该重复执行")

	// 清理任务
	crontab.Get().Remove(id)
}

func TestConcurrentAddFunc(t *testing.T) {
	var counter int32
	for range 10 {
		id, err := crontab.AddFunc("*/1 * * * * *", func() {
			atomic.AddInt32(&counter, 1)
		})
		require.Nil(t, err)
		defer crontab.Get().Remove(id)
	}

	crontab.Start()
	time.Sleep(time.Second * 2)

	count := atomic.LoadInt32(&counter)
	require.True(t, count >= 10, "所有任务应该至少执行一次")
}
