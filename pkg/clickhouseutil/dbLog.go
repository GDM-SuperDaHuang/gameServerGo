package clickhouseutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Client 是对 ClickHouse 连接的封装
type Client struct {
	conn driver.Conn // 原生连接，性能更高，支持 ClickHouse 所有特性
	// db   *sql.DB   // 可选：如果更习惯 database/sql 标准接口，可以同时封装
	config Config
}

// Config 用于配置 ClickHouse 连接
type Config struct {
	Host     string // 主机地址，例如 "127.0.0.1"
	Port     int    // 端口，例如 9000(TCP) 或 8123(HTTP)
	Database string // 数据库名
	Username string // 用户名
	Password string // 密码

	// 可选配置
	Debug           bool          // 是否开启调试
	MaxOpenConns    int           // 最大打开连接数，建议设为CPU核心数的2-4倍[reference:0]
	MaxIdleConns    int           // 最大空闲连接数，建议设为 MaxOpenConns 的 1/2[reference:1]
	ConnMaxLifetime time.Duration // 连接最大生命周期，建议设为 1 小时[reference:2]
	DialTimeout     time.Duration // 连接超时
	ReadTimeout     time.Duration // 读超时
	//WriteTimeout    time.Duration // 写超时

	// 协议与压缩
	Protocol  clickhouse.Protocol // 可选：clickhouse.Native 或 clickhouse.HTTP
	TLSConfig *tls.Config         // 可选：TLS配置
}

// NewClient 创建一个新的 ClickHouse 客户端
func NewClient(cfg Config) (*Client, error) {
	// 设置默认值
	if cfg.Port == 0 {
		cfg.Port = 9000
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 10
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 5
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = time.Hour
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.Protocol == 0 {
		cfg.Protocol = clickhouse.Native
	}

	// 构建连接选项
	opts := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Debug:           cfg.Debug,
		DialTimeout:     cfg.DialTimeout,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ReadTimeout:     cfg.ReadTimeout,

		Protocol: cfg.Protocol,
		TLS:      cfg.TLSConfig,
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse connection: %w", err)
	}

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}

	return &Client{
		conn:   conn,
		config: cfg,
	}, nil
}

// Close 关闭连接
func (c *Client) Close() error {
	return c.conn.Close()
}

// Conn 返回原生连接，供高级操作使用
func (c *Client) Conn() driver.Conn {
	return c.conn
}
