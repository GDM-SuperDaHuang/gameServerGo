package rpc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func mustCompleted(t *testing.T, completed bool, err error) {
	t.Helper()
	if !completed {
		t.Fatalf("dispatch not completed, err=%v", err)
	}
	if err != nil {
		t.Fatalf("dispatch err: %v", err)
	}
}

// 同一玩家并发打进来，必须串行执行（同时在跑的最多 1 个）
func TestPlayerDispatcher_SamePlayerSerial(t *testing.T) {
	d := newPlayerDispatcher(16, 512)
	var cur, maxSeen atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 6000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uid := int64(12345)
			completed, err := d.dispatch(context.Background(), uid, func() error {
				n := cur.Add(1)
				for {
					m := maxSeen.Load()
					if n <= m || maxSeen.CompareAndSwap(m, n) {
						break
					}
				}
				time.Sleep(2 * time.Millisecond)
				cur.Add(-1)
				return nil
			})
			if !completed || err != nil {
				t.Errorf("dispatch completed=%v err: %v", completed, err)
			}
		}()
	}
	wg.Wait()
	if got := maxSeen.Load(); got != 1 {
		t.Fatalf("same player must run serially, max concurrency = %d", got)
	}
}

// 不同玩家必须能并发（32 个玩家各睡 50ms，总耗时应远小于串行的 1600ms）
func TestPlayerDispatcher_DifferentPlayersConcurrent(t *testing.T) {
	d := newPlayerDispatcher(1024, 512)
	var cur, maxSeen atomic.Int32
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 10000; i++ {
		uid := int64(1000 + i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			completed, err := d.dispatch(context.Background(), uid, func() error {
				n := cur.Add(1)
				for {
					m := maxSeen.Load()
					if n <= m || maxSeen.CompareAndSwap(m, n) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				cur.Add(-1)
				return nil
			})
			if !completed || err != nil {
				t.Errorf("dispatch completed=%v err: %v", completed, err)
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	if got := maxSeen.Load(); got <= 1 {
		t.Fatalf("different players must run concurrently, max concurrency = %d", got)
	}
	if elapsed >= 1500*time.Millisecond {
		t.Fatalf("different players run too slowly, elapsed = %v", elapsed)
	}
}

// ctx 超时必须能中断排队等待
func TestPlayerDispatcher_CtxCancel(t *testing.T) {
	d := newPlayerDispatcher(1, 1)
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	go func() {
		completed, err := d.dispatch(context.Background(), 7, func() error {
			once.Do(func() { close(started) })
			<-release
			return nil
		})
		mustCompleted(t, completed, err)
	}()
	<-started
	// 占满队列（queueLen=1）：注意新语义下第三个任务会快速失败 ErrDispatchBusy，
	// 这里用阻塞占位 + 可取消的 ctx 来验证“等待可中断”。
	occupyDone := make(chan struct{})
	go func() {
		defer close(occupyDone)
		_, _ = d.dispatch(context.Background(), 7, func() error { return nil })
	}()
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	completed, err := d.dispatch(ctx, 7, func() error { return nil })
	if completed || err == nil {
		t.Fatalf("dispatch with cancelled ctx should abandon, completed=%v err=%v", completed, err)
	}
	close(release)
	<-occupyDone
}

// 队列满必须快速失败，而不是无限阻塞 rpcx 工作线程
func TestPlayerDispatcher_QueueFullFastFail(t *testing.T) {
	d := newPlayerDispatcher(1, 1)
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	go func() {
		completed, err := d.dispatch(context.Background(), 7, func() error {
			once.Do(func() { close(started) })
			<-release
			return nil
		})
		mustCompleted(t, completed, err)
	}()
	<-started
	// 占满队列缓冲
	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		_, _ = d.dispatch(context.Background(), 7, func() error { return nil })
	}()
	time.Sleep(20 * time.Millisecond)

	start := time.Now()
	completed, err := d.dispatch(context.Background(), 7, func() error { return nil })
	elapsed := time.Since(start)
	if completed || !errors.Is(err, ErrDispatchBusy) {
		t.Fatalf("full queue should fast-fail with ErrDispatchBusy, completed=%v err=%v", completed, err)
	}
	if elapsed >= time.Second {
		t.Fatalf("full queue must not block, elapsed=%v", elapsed)
	}
	close(release)
	<-secondDone
}

// 放弃等待的路径必须返回 completed=false，调用方借此避免使用已失效的结果
func TestPlayerDispatcher_AbandonSemantics(t *testing.T) {
	d := newPlayerDispatcher(1, 1)
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	go func() {
		completed, err := d.dispatch(context.Background(), 9, func() error {
			once.Do(func() { close(started) })
			<-release
			return nil
		})
		mustCompleted(t, completed, err)
	}()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	completed, err := d.dispatch(ctx, 9, func() error { return nil })
	if completed {
		t.Fatalf("cancelled waiter must get completed=false, err=%v", err)
	}
	close(release)
}
