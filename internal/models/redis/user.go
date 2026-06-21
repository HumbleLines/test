package redismodel

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// CounterModel
// 面向某个具体 key 的计数模型，封装底层命令。
// 只负责最小的读写，不做业务逻辑（那是 service 的职责）。
type CounterModel struct {
	rdb redis.UniversalClient
}

func NewCounterModel(rdb redis.UniversalClient) *CounterModel {
	return &CounterModel{rdb: rdb}
}

// Incr 自增 1
func (m *CounterModel) Incr(ctx context.Context, key string) (int64, error) {
	return m.rdb.Incr(ctx, key).Result()
}

// Expire 设置过期（如果需要的话）
func (m *CounterModel) Expire(ctx context.Context, key string, ttl time.Duration) error {
	// 保持与 Redis 语义一致：ttl <= 0 时通常不设置，这里由上层控制
	return m.rdb.Expire(ctx, key, ttl).Err()
}
