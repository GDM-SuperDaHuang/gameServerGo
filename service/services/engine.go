package services

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap/zapcore"
)

// Engine 应用管理, 管理当前进程
//type Engine interface {
//	Add(list ...services.Service) error
//	Run() error
//}

//type Engine struct {
//	// services 本进程内的所有服务
//	services  []services.Service
//	closeOnce sync.Once
//}

// Options 创建参数
type Options struct {
	NodeName    string
	ServiceName string
	ConfigPath  string
	ConfigName  string
	ConfigType  string
	LogPath     string
	LogName     string
	LogLevel    string
}

func New(option *Options) (*Engine, error) {
	// 1. 配置 0 时区
	time.Local = time.UTC

	// 2.  配置
	//if err := config.Init(option.NodeName, option.ServiceName, option.ConfigPath, option.ConfigName, option.ConfigType); err != nil {
	//	return nil, err
	//}
	// 3. 日志
	var logLevel zapcore.Level
	if err := logLevel.Set(option.LogLevel); err != nil {
		return nil, err
	}
	//logger.Init(logLevel, option.LogName, option.LogPath, !config.Get().IsDevelop())

	engine := &Engine{ //engine
		services: make([]Service, 0),
	}

	//logger.Get().Info("[engine] create success")
	return engine, nil
}

func (e *Engine) Add(list ...Service) error {
	serviceMap := make(map[uint32]struct{})
	for _, s := range e.services {
		serviceMap[s.ID()] = struct{}{}
	}

	for _, v := range list {
		if _, exists := serviceMap[v.ID()]; exists {
			return fmt.Errorf("service repeated, id: %d, name: %s", v.ID(), v.Name())
		}
		e.services = append(e.services, v)
	}

	return nil
}

// Run 运行
func (e *Engine) Run() error {
	//for _, s := range e.services {
	//	if err := s.Init(); err != nil {
	//		return err
	//	}
	//}

	for _, s := range e.services {
		if err := s.Start(); err != nil {
			return err
		}
	}

	e.listenSignal()

	return nil
}

// listenSignal 监听信号
func (e *Engine) listenSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	sig := <-ch
	signal.Stop(ch)

	//logger.Get().Sugar().Infof("received signal: %+v", sig)

	switch sig {
	case syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT:
		//logger.Get().Info("close signal .. shufhown server ..")
		e.close()
	default:
		//logger.Get().Sugar().Errorf("unsupport signal: %s", sig.String())
	}
}

// close 关闭服务
func (e *Engine) close() {
	e.closeOnce.Do(func() {
		for _, s := range e.services {
			if err := s.Close(); err != nil {
				//logger.Get().Sugar().Errorf("service: %s(%d) close failed: %s", s.Name(), s.ID(), err.Error())
			} else {
				//logger.Get().Sugar().Infof("service: %s(%d) closed successfully", s.Name(), s.ID())
			}
		}
		//logger.Get().Info("all services closed")
	})
}
