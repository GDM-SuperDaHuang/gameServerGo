package server

import (
	"Server/service/rpc"
	"Server/service/utils"
	"fmt"
	"reflect"
	"strings"

	"github.com/rpcxio/rpcx-etcd/serverplugin"
	rpcxServer "github.com/smallnest/rpcx/server"
	"time"
)

// NewServer 创建服务端
func NewServer(config *RpcConfig) (rpc.ServerInterface, error) {
	s := &Server{
		config: config,
		server: rpcxServer.NewServer(),
	}
	return s, nil
}

// Output 输出当前信息
func (s *Server) Output() string {
	var address string
	addr := s.server.Address()
	if addr != nil {
		address = fmt.Sprintf("%s@%s", addr.Network(), addr.String())
	}

	return fmt.Sprintf("%s(id: %d)[version: %d][listen: %s]", s.config.Name, s.config.ID, s.config.Version, address)
}

// Start 启动服务, 非阻塞
func (s *Server) Start() error {
	if s.closed.Load() {
		return fmt.Errorf("server: %s already closed", s.Output())
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.serve()
	}()

	// 启动rpcx服务端
	select {
	case err := <-errCh:
		return fmt.Errorf("server: %s start failed: %v", s.Output(), err)
	case <-time.After(5 * time.Second):
		// 继续执行
	}

	config := s.config
	addr := s.server.Address()
	if addr == nil {
		return fmt.Errorf("server: %s get address failed", s.Output())
	}

	// 向etcd注册
	address := fmt.Sprintf("%s@%s", addr.Network(), addr.String())
	r := &serverplugin.EtcdV3RegisterPlugin{
		BasePath:       s.publicServicePath(), // eg：xxx/gate
		ServiceAddress: address,
		EtcdServers:    config.EtcdEndpoints,  //往etcd注册地址
		UpdateInterval: config.UpdateInterval, //信息更新间隔
	}

	if err := r.Start(); err != nil {
		return fmt.Errorf("server: %s etcd registry failed: %v", s.Output(), err)
	}

	s.server.Plugins.Add(r)
	s.registry = r

	return nil
}

// Stop 停止服务
func (s *Server) Stop() error {
	if s == nil {
		return nil
	}

	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	var errs []error

	// 注销服务
	if s.registry != nil {
		if err := s.registry.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("etcd registry stop failed: %v", err))
		}
	}

	// 关闭服务器
	if err := s.server.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close server failed: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("stop server with errors: %v", errs)
	}
	return nil
}

// Register 注册服务
//
// - rcvr 函数相关，例如 new(Forward)
func (s *Server) Register(rcvr any) error {
	metadata := s.buildMetadata()

	// 服务对象名称，例如 Forward
	servicePath := reflect.Indirect(reflect.ValueOf(rcvr)).Type().Name()

	return s.server.RegisterName(servicePath, rcvr, metadata)
}

// serve 启动 rpc 服务，对外监听
//
// 使用系统分配的端口
func (s *Server) serve() error {
	ip := utils.LocalIP()
	address := fmt.Sprintf("%s:%d", ip, 0)
	if err := s.server.Serve("tcp", address); err != nil {
		return fmt.Errorf("server: %s, serve failed: %v", s.Output(), err)
	}
	return nil
}

// buildMetadata 编写服务器元数据
func (s *Server) buildMetadata() string {
	metadata := strings.Join([]string{
		fmt.Sprintf("id=%d", s.config.ID),
		fmt.Sprintf("version=%d", s.config.Version),
	}, "&")

	return metadata
}

func (s *Server) publicServicePath() string {
	return fmt.Sprintf("%s/%s", s.config.BasePath, s.config.ServiceName)
}
