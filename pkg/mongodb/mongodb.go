package mongodb

import (
	"context"
	"errors"
	"fmt"
	"gameServer/pkg/config"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

/**
[common.mongodb]
uri = "mongodb://user:pass@127.0.0.1:27017/?replicaSet=rs0"
database = "egame"
authSource = "admin"
connectTimeoutSec = 5
pingTimeoutSec = 3
maxPoolSize = 100
minPoolSize = 5
/
*/

// Config MongoDB 配置
type Config struct {
	URI            string
	Database       string
	AuthSource     string
	ConnectTimeout time.Duration
	PingTimeout    time.Duration
	MaxPoolSize    uint64
	MinPoolSize    uint64
}

var (
	globalClient *mongo.Client
	globalDB     *mongo.Database
)

// GetClient 获取全局 MongoDB 客户端
func GetClient() *mongo.Client {
	return globalClient
}

// GetDB 获取全局 MongoDB 数据库实例
func GetDB() *mongo.Database {
	return globalDB
}

// InitFromConfig 从配置文件初始化 MongoDB
func InitFromConfig() error {
	c := config.Get()
	if c == nil {
		panic("please call config.InitTest first")
	}

	cfg := &Config{
		URI:            c.MongoURI(),
		Database:       c.MongoDatabase(),
		AuthSource:     c.MongoAuthSource(),
		ConnectTimeout: time.Duration(c.MongoConnectTimeoutSec()) * time.Second,
		PingTimeout:    time.Duration(c.MongoPingTimeoutSec()) * time.Second,
		MaxPoolSize:    c.MongoMaxPoolSize(),
		MinPoolSize:    c.MongoMinPoolSize(),
	}
	return Init(cfg)
}

// Init 初始化全局 MongoDB 客户端
func Init(cfg *Config) error {
	client, db, err := New(cfg)
	if err != nil {
		return err
	}

	globalClient = client
	globalDB = db
	return nil
}

// New 创建 MongoDB 客户端及数据库实例
func New(cfg *Config) (*mongo.Client, *mongo.Database, error) {
	if cfg == nil {
		return nil, nil, errors.New("mongodb config is nil")
	}
	if cfg.URI == "" {
		return nil, nil, errors.New("mongodb uri is empty")
	}
	if cfg.Database == "" {
		return nil, nil, errors.New("mongodb database is empty")
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	if cfg.PingTimeout <= 0 {
		cfg.PingTimeout = 3 * time.Second
	}

	opts := options.Client().ApplyURI(cfg.URI)
	opts.SetConnectTimeout(cfg.ConnectTimeout)
	if cfg.AuthSource != "" {
		opts.SetAuth(options.Credential{AuthSource: cfg.AuthSource})
	}
	if cfg.MaxPoolSize > 0 {
		opts.SetMaxPoolSize(cfg.MaxPoolSize)
	}
	if cfg.MinPoolSize > 0 {
		opts.SetMinPoolSize(cfg.MinPoolSize)
	}

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("mongodb connect failed: %w", err)
	}

	pingCtx, pingCancel := context.WithTimeout(context.Background(), cfg.PingTimeout)
	defer pingCancel()
	if err = client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("mongodb ping failed: %w", err)
	}

	return client, client.Database(cfg.Database), nil
}

// Close 关闭全局 MongoDB 客户端
func Close() error {
	if globalClient == nil {
		return nil
	}

	err := globalClient.Disconnect(context.Background())
	globalClient = nil
	globalDB = nil
	return err
}
