package recoverSafe

import (
	"fmt"
	"gameServer/pkg/logger/log2"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"go.uber.org/zap"
)

var (
	DefaultWorkerOpt = WorkerOption{ //默认策略
		Restart:      true,
		RestartDelay: time.Second, //1秒
		MaxRestart:   0,           // 无限重启
		PanicLimit:   5,           // 10秒最多5次 panic
		PanicWindow:  10 * time.Second,
	}
)

type WorkerOption struct {
	Restart      bool          // 是否自动重启
	RestartDelay time.Duration // 重启间隔
	MaxRestart   int           // 最大重启次数（0 = 无限）
	PanicLimit   int           // 短时间 panic 限制
	PanicWindow  time.Duration // 统计窗口
}

// 异常捕获,直接退出
func RecoverSafe(nodeName string) {
	var exitCode = 0
	if r := recover(); r != nil {
		// 记录崩溃信息
		stack := debug.Stack()
		crashLog := map[string]interface{}{
			"time":      time.Now().Format("2006-01-01 15:04:05"),
			"panic":     r,
			"stack":     string(stack),
			"exit_code": 2,
			"node_name": nodeName,
		}

		// 尝试用 zap 记录
		if log2.Get() != nil {
			log2.Get().Error("程序崩溃 PANIC RECOVERED",
				zap.Any("panic", r),
				zap.String("stack", string(stack)),
				zap.Int("exit_code", 2),
			)
			// 强制刷新日志
			log2.Get().Sync()
		}

		// 同时写入专门的崩溃日志文件（防止 zap 也坏了）
		writeCrashLogToFile(crashLog)

		exitCode = 2
		os.Exit(exitCode)
	}
}

// 兜底：直接写文件，不依赖 zap
func writeCrashLogToFile(crashInfo map[string]interface{}) {
	crashDir := "./logs/crash"
	_ = os.MkdirAll(crashDir, 0755)

	filename := filepath.Join(crashDir, time.Now().Format("2006-01-02_15-04-05")+".crash")

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// 最后手段：打印到 stderr
		os.Stderr.WriteString("无法写入崩溃日志: " + err.Error() + "\n")
		return
	}
	defer f.Close()
	// 简单文本格式写入
	f.WriteString("========== CRASH LOG ==========\n")
	panicStr, ok := crashInfo["panic"]
	if ok {
		str := panicStr.(string)
		f.WriteString("panic: " + str + "\n")
	}
	node_name, ok := crashInfo["node_name"]
	if ok {
		str := node_name.(string)
		f.WriteString("Node: " + str + "\n")
	}
	timeStr, ok := crashInfo["time"]
	if ok {
		str := timeStr.(string)
		f.WriteString("Time: " + str + "\n")
	}
	stack, ok := crashInfo["stack"]
	if ok {
		str := stack.(string)
		f.WriteString("Stack:\n")
		f.WriteString(str)
	}
	worker, ok := crashInfo["worker"]
	if ok {
		str := worker.(string)
		f.WriteString("worker: " + str)
	}
	f.WriteString("\n==============================\n")

}

// 异常就退出
func GoSafe(name string, fn func() error) {
	go func() {
		_ = PanicGuard(name, fn)
	}()
}

// 异常会重启
func GoWorker(name string, fn func() error, opt WorkerOption) {
	go func() {
		restartCount := 0

		var panicTimes []time.Time

		for {
			start := time.Now()

			if err := PanicGuard(name, fn); err == nil {
				return
			}

			// 正常退出，不重启
			if !opt.Restart {
				return
			}

			restartCount++

			// 最大重启次数限制
			if opt.MaxRestart > 0 && restartCount > opt.MaxRestart {
				zap.L().Error("worker restart limit reached",
					zap.String("worker", name),
				)
				return
			}

			// ===== panic 频率限制（防雪崩）=====
			now := time.Now()
			panicTimes = append(panicTimes, now)

			// 清理窗口外数据
			valid := panicTimes[:0]
			for _, t := range panicTimes {
				if now.Sub(t) <= opt.PanicWindow {
					valid = append(valid, t)
				}
			}
			panicTimes = valid

			if opt.PanicLimit > 0 && len(panicTimes) > opt.PanicLimit {
				zap.L().Error("worker panic too frequent, stop restarting",
					zap.String("worker", name),
					zap.Int("panicCount", len(panicTimes)),
				)
				return
			}

			// 防止疯狂重启（最关键）
			sleep := opt.RestartDelay
			if sleep == 0 {
				sleep = time.Second
			}

			// 如果执行时间太短，说明可能是秒崩
			if time.Since(start) < time.Second {
				sleep = 2 * time.Second
			}

			zap.L().Warn("worker restarting",
				zap.String("worker", name),
				zap.Int("restartCount", restartCount),
			)

			time.Sleep(sleep)
		}
	}()
}

func PanicGuard(name string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// 记录崩溃信息
			stack := debug.Stack()
			crashLog := map[string]interface{}{
				"time":   time.Now().Format("2006-01-01 15:04:05"),
				"panic":  fmt.Sprintf("%v", r),
				"stack":  string(stack),
				"worker": name,
			}
			// 主日志（必须成功）
			zap.L().Error("panic recovered",
				zap.String("worker", name),
				zap.Any("panic", r),
				zap.ByteString("stack", stack),
			)
			// 日志
			writeCrashLogToFile(crashLog)
			err = fmt.Errorf("%v", r)
		}
	}()
	err = fn()
	return err
}
