package test

import (
	"gameServer/pkg/redis"
	"testing"
)

func TestRun(t *testing.T) {
	redis.NewRedisClient("127.0.0.1:16379")
}
