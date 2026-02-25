package cache

import (
	"context"
	"gameServer/pkg/bytes"
	"gameServer/pkg/config"

	"github.com/redis/go-redis/v9"
)

// Sep 缓存分隔符
func Sep() byte {
	return ':'
}

// Config Redis配置
type Config struct {
	Addr           string // Redis地址
	Password       string // Redis密码
	DB             int    // 数据库编号
	MinIdleConns   int
	MaxActiveConns int
}

var globalClient *redis.Client

// Get 获取全局缓存
func Get() *redis.Client {
	return globalClient
}

// InitTest 初始化测试用例使用的配置
func InitTest() error {
	return InitFromConfig()
}

// InitFromConfig 从配置表获取数据进行初始化
func InitFromConfig() error {
	c := config.Get()
	if c == nil {
		panic("please call config.InitTest first")
	}
	config := &Config{
		Addr:           c.CacheAddress(),
		Password:       c.CachePassword(),
		DB:             c.CacheDB(),
		MinIdleConns:   c.CacheMinIdle(),
		MaxActiveConns: c.CacheMaxActive(),
	}
	return Init(config)
}

// Init 初始化全局缓存
func Init(cfg *Config) error {
	c, err := New(cfg)
	if err != nil {
		return err
	}
	globalClient = c
	return nil
}

// New 创建Redis客户端
func New(cfg *Config) (*redis.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:           cfg.Addr,
		Password:       cfg.Password,
		DB:             cfg.DB,
		MaxActiveConns: cfg.MaxActiveConns,
		MinIdleConns:   cfg.MinIdleConns,
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

// Key 缓存 key 添加前缀
// 如果当前在具体的服务中，会加上服务名称
// 例如: 当前在 game-1 中，会自动加上 game-1:
func Key(key string) string {
	c := config.Get()
	if c.IsPublic() {
		return PublicKey(key)
	}

	buffer := bytes.Get().Buffer(128)
	buffer.WriteString(c.CachePrefix())
	buffer.WriteByte(Sep())
	buffer.WriteString(c.NodeName())
	buffer.WriteByte(Sep())
	buffer.WriteString(key)

	v := buffer.String()

	bytes.Get().Release(buffer)
	return v
}

// PublicKey 只会在 key 之前加上项目前缀，不会加上服务内容
// 适用于所有服的公共缓存，例如缓存所有区服玩家 id
func PublicKey(key string) string {
	c := config.Get()

	buffer := bytes.Get().Buffer(128)
	buffer.WriteString(c.CachePrefix())
	buffer.WriteByte(Sep())
	buffer.WriteString(key)

	v := buffer.String()

	bytes.Get().Release(buffer)
	return v
}
