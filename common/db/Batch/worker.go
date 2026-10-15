package Batch

import (
	"context"
	"sync/atomic"
	"time"

	"gameServer/pkg/logger/log2"

	"go.uber.org/zap"
)

// defaultOpTimeout 单轮落库超时，小于分布式锁 TTL，保证正常结束前锁不提前过期。
const defaultOpTimeout = 100 * time.Second

type Worker struct {
	registry  *Registry
	batchSize int64
	lockTTL   time.Duration
	opTimeout time.Duration
	running   atomic.Bool
}

func NewWorker(r *Registry) *Worker {
	return &Worker{
		registry:  r,
		batchSize: DefaultBatchSize,
		lockTTL:   FlushLockTTL,
		opTimeout: defaultOpTimeout,
	}
}

func (w *Worker) batchLimit() int64 {
	if w.batchSize > 0 {
		return w.batchSize
	}
	return DefaultBatchSize
}

func (w *Worker) lockTTLValue() time.Duration {
	if w.lockTTL > 0 {
		return w.lockTTL
	}
	return FlushLockTTL
}

func (w *Worker) opTimeoutValue() time.Duration {
	if w.opTimeout > 0 {
		return w.opTimeout
	}
	return defaultOpTimeout
}

// todo 锁过期，关服钩子，其他地方直接调用Flush，导致多实例并发?
func (w *Worker) Flush(ctx context.Context) {
	for _, typ := range w.registry.Types() {
		w.FlushType(ctx, typ)
	}
}

func (w *Worker) FlushType(ctx context.Context, t DirtyType) {
	// 分布式锁：单 Redis 多实例下同一时刻只有拿到锁的实例刷盘
	token, ok, err := TryAcquireFlushLock(ctx, t, w.lockTTLValue())
	if err != nil {
		logError("batch flush acquire lock failed", zap.String("type", string(t)), zap.Error(err))
		return
	}
	if !ok {
		logInfo("batch flush skipped: locked by another instance", zap.String("type", string(t)))
		return
	}
	// 用独立 ctx 释放锁，避免外层超时导致锁泄漏到 TTL 过期
	defer func() {
		if err := ReleaseFlushLock(context.Background(), t, token); err != nil {
			logError("batch flush release lock failed", zap.String("type", string(t)), zap.Error(err))
		}
	}()

	uids, err := Get(ctx, t, w.batchLimit())
	if err != nil {
		logError("batch flush get dirty uids failed", zap.String("type", string(t)), zap.Error(err))
		return
	}

	var done, skipped int
	for _, uid := range uids {
		// ctx 取消（如超时）直接停，避免拿着锁空转
		if ctx.Err() != nil {
			logError("batch flush canceled", zap.String("type", string(t)), zap.Error(ctx.Err()))
			break
		}
		storage, err := w.registry.Create(t, uid)
		if err != nil {
			skipped++
			continue
		}

		// Redis读取
		if err := storage.Load(ctx); err != nil {
			logError("batch flush load failed", zap.String("type", string(t)), zap.Uint64("uid", uid), zap.Error(err))
			skipped++
			continue
		}
		version := storage.Version()

		// Mongo
		if err := storage.Flush(ctx); err != nil {
			logError("batch flush to mongo failed", zap.String("type", string(t)), zap.Uint64("uid", uid), zap.Error(err))
			skipped++
			continue
		}

		// Dirty确认删除：Lua 原子比对，版本变化/标记已消失都视为正常（返回 nil）
		if err := RemoveIfSame(ctx, t, uid, version); err != nil {
			logError("batch flush remove dirty failed", zap.String("type", string(t)), zap.Uint64("uid", uid), zap.Error(err))
			skipped++
			continue
		}
		done++
	}

	logInfo("batch flush finished", zap.String("type", string(t)), zap.Int("done", done), zap.Int("skipped", skipped))
}

// StartPersist 定时落库入口：进程内防重入 + 单轮超时，多实例串行由分布式锁保证。
func StartPersist(worker *Worker) {
	if worker == nil {
		return
	}
	if !worker.running.CompareAndSwap(false, true) {
		logInfo("batch persist skipped: previous round still running")
		return
	}
	defer worker.running.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), worker.opTimeoutValue())
	defer cancel()
	worker.Flush(ctx)
}

func logInfo(msg string, fields ...zap.Field) {
	if l := log2.Get(); l != nil {
		l.Info(msg, fields...)
	}
}

func logError(msg string, fields ...zap.Field) {
	if l := log2.Get(); l != nil {
		l.Error(msg, fields...)
	}
}
