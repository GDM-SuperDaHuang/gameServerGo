package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ctx         = context.Background()
	redisClient *redis.Client
)

func NewRedisClient(addr string) {
	op := &redis.Options{
		Addr:         addr,
		Password:     "123456",
		DB:           0,
		DialTimeout:  5 * time.Second, // 连接超时
		ReadTimeout:  3 * time.Second, // 读超时
		WriteTimeout: 3 * time.Second, // 写超时
		PoolSize:     10,              // 连接池大小
	}
	redisClient = redis.NewClient(op)
	ping := redisClient.Ping(ctx)
	if ping.Err() != nil {
		panic(ping.Err())
	}
}

func GetRedisClient() *redis.Client {
	return redisClient
}

func GetCtx() context.Context {
	return ctx
}

// 发布消息
func PublishMessage(channel, message string) error {
	return redisClient.Publish(ctx, channel, message).Err()
}

// 订阅消息
func SubscribeMessage(channel string, handler func(string)) {
	pubsub := redisClient.Subscribe(ctx, channel)
	ch := pubsub.Channel()
	go func() {
		for msg := range ch {
			key := msg.Payload
			handler(key)
		}
	}()
}
