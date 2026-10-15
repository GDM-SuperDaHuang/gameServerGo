package services

// Service 对外服务
type ServiceInterface interface {
	// ID 服务唯一 id
	ID() uint32
	// Name 服务名称
	Name() string
	// Version 服务版本号
	Version() uint32

	// Start 启动服务，可以接入新数据
	Start() error
	// Close 关闭服务
	Close() error
}
