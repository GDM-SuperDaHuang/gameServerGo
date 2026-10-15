package rpc

import (
	"context"
	"errors"
	"fmt"
	"gameServer/common/errorCode"
	"gameServer/service/common"
)

// 分片参数：分片数固定，无需按 uid 建 map，没有 goroutine 泄漏和清理问题。
// 同一 uid 恒定映射到同一分片，分片内单 goroutine 串行执行 → 玩家个人串行；
// 分片之间各自独立执行 → 玩家之间并发。
// 注意：不同 uid 落到同一分片时会互相排队（哈希碰撞），慢 handler 会拖住
// 同分片的其他玩家；网关侧应设置调用超时，业务 handler 应避免长时间阻塞。
const (
	playerShardCount = 1024
	playerQueueLen   = 1024
)

// ErrDispatchBusy 串行队列满时的快速失败错误。调用方应将其转为可重试/
// “操作频繁”错误码，不要无限阻塞 rpcx 工作线程。
var ErrDispatchBusy = errors.New("dispatch queue full")

// Dispatch 网关发来的消息派遣：玩家个人串行，玩家之间并发。
// 仍是同步 RPC 语义：当前调用会阻塞到轮到自己执行完、resp 填好后才返回，
// 网关侧无需任何改动。
//
// 并发安全说明：worker 只写 per-request 的 shadow Resp，等待者仍在
// （task.done 先到达）时才把结果拷贝回 rpcx 的 resp；等待者已因 ctx
// 超时离开时不再触碰 resp，避免“返回后还被 worker 改写”的 data race。
func (f *Forward) Dispatch(ctx context.Context, req *common.RpcMessage, resp *common.Resp) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if req == nil {
		if resp != nil {
			resp.Code = errorCode.ErrorCode_GateReqNull
		}
		return errors.New("req is nil")
	}

	// 原代码这里 `resp = &common.Resp{}` 只改了局部指针，调用方拿不到，
	// resp 为 nil 时直接报错即可（rpcx 正常调用不会走到这里）。
	if resp == nil {
		return errors.New("resp is nil")
	}

	if req.Player == nil || req.Player.Uid == 0 {
		resp.Code = errorCode.ErrorCode_GateReqNull
		return errors.New("player is nil")
	}

	if req.Data == nil || req.Data.Head == nil {
		resp.Code = errorCode.ErrorCode_GateReqNull
		return errors.New("message is nil")
	}

	protocolMethod, found := f.protocoles[req.Data.Head.Protocol]
	if !found {
		resp.Code = errorCode.ErrorCode_ProtocolNotFound
		return fmt.Errorf("protocol not found: %d", req.Data.Head.Protocol)
	}

	// 兼容零值 Forward（非 NewForward 构造）：直接执行，保持原行为。
	if f.dispatcher == nil {
		return protocolMethod.Call(ctx, req.Player, req, resp)
	}

	uid := req.Player.Uid
	// shadow 只会被 worker goroutine 写入（入队→出队有 channel happens-before），
	// 等待者仅在 completed==true（收到 task.done，同样有 happens-before）后读取，
	// 放弃等待的路径绝不触碰 shadow，因此与 worker 无竞争。
	shadow := &common.Resp{}
	exec := func() (err error) {
		defer func() {
			// handler panic 也不能把串行队列卡死，且让网关能拿到错误码
			if r := recover(); r != nil {
				shadow.Code = errorCode.ErrorCode_Panic
				err = fmt.Errorf("dispatch panic: %v", r)
			}
		}()
		return protocolMethod.Call(ctx, req.Player, req, shadow)
	}

	completed, err := f.dispatcher.dispatch(ctx, uid, exec)
	if !completed {
		// 排队失败（ctx 取消/超时）或队列满：shadow 仍归 worker 所有，
		// 这里绝不能把 shadow 拷贝回 resp，也不能再读写 shadow。
		if errors.Is(err, ErrDispatchBusy) {
			resp.Code = errorCode.ErrorCode_ServerBusy
		}
		return err
	}
	// 等待者仍在，worker 已结束，此时独占 shadow，可安全拷贝。
	resp.Code = shadow.Code
	resp.Flag = shadow.Flag
	resp.Body = shadow.Body
	return err
}

// ---------------- 按玩家串行的分片执行器 ----------------

type dispatchTask struct {
	ctx  context.Context
	exec func() error
	done chan error // 带 1 个缓冲：等待者已因 ctx 超时离开时，worker 也不会被卡住
}

type playerShard struct {
	queue chan dispatchTask
}

type playerDispatcher struct {
	shards []*playerShard
}

func newPlayerDispatcher(shardCount, queueLen int) *playerDispatcher {
	if shardCount <= 0 {
		shardCount = playerShardCount
	}
	if queueLen <= 0 {
		queueLen = playerQueueLen
	}
	d := &playerDispatcher{
		shards: make([]*playerShard, shardCount),
	}
	for i := range d.shards {
		s := &playerShard{queue: make(chan dispatchTask, queueLen)}
		d.shards[i] = s
		go s.loop()
	}
	return d
}

func (d *playerDispatcher) shardOf(uid int64) *playerShard {
	// uid > 0 已在 Dispatch 保证；uint64 转换对异常值同样确定性映射
	return d.shards[uint64(uid)%uint64(len(d.shards))]
}

// dispatch 把任务排进 uid 所在分片并等待执行完成；ctx 可中断排队和等待。
// 同一分片内任务按入队顺序一个个执行，因此同一 uid 必串行；不同分片互不干扰。
//
// 返回值：completed==true 表示任务已执行完（无论 err 是否为 nil），
// 调用方可以安全使用 exec 产生的副作用结果；completed==false 表示任务
// 未被等待（ctx 已取消/超时，或队列满快速失败），调用方必须丢弃本次结果。
// 注意 completed==false 时任务仍可能正在/稍后执行（已开始则无法抢占，
// 未开始则会被跳过），调用方重试必须保证业务幂等。
func (d *playerDispatcher) dispatch(ctx context.Context, uid int64, exec func() error) (completed bool, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// 先快检：ctx 已死无需入队。
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	shard := d.shardOf(uid)
	task := dispatchTask{ctx: ctx, exec: exec, done: make(chan error, 1)}

	// 队列满时快速失败，避免网关 ctx 无 deadline 时无限堆积、
	// 拖垮 rpcx 工作线程；网关拿到 ErrDispatchBusy 可转“操作频繁”重试。
	select {
	case shard.queue <- task:
	case <-ctx.Done():
		return false, ctx.Err()
	default:
		return false, ErrDispatchBusy
	}

	select {
	case err := <-task.done:
		return true, err
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (s *playerShard) loop() {
	for task := range s.queue {
		task.run()
	}
}

func (t dispatchTask) run() {
	defer func() {
		// 兜底：exec 内部已有 recover，这里防止 worker 挂掉导致整个分片卡死。
		// 注意只发 error，不触碰任何 resp（resp 归属见 Dispatch 说明）。
		if r := recover(); r != nil {
			select {
			case t.done <- fmt.Errorf("dispatch panic: %v", r):
			default:
				// 等待者已离开且 done 已被占用（理论上不会发生，done 带缓冲且只发一次），
				// 绝不能阻塞 worker，否则整个分片卡死。
			}
		}
	}()
	// 排队期间 ctx 已超时/取消：跳过执行，避免超时请求仍产生副作用。
	// 已开始执行的无法抢占，仍会跑完（见 dispatch 注释的幂等要求）。
	if t.ctx != nil && t.ctx.Err() != nil {
		t.done <- t.ctx.Err()
		return
	}
	t.done <- t.exec()
}
