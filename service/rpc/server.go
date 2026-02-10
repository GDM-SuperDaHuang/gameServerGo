package rpc

// Server 服务端
type Server interface {
	// Start 启动
	Start() error

	// Stop 停止
	Stop() error

	// Register 注册服务
	//
	// - rcvr: 服务对象，例如 &Forward{}
	Register(rcvr any) error

	// Output 输出当前信息
	Output() string
}

// NewServer 创建服务端
func NewServer(config *server.Config) (Server, error) {
	return server.New(config)
}
