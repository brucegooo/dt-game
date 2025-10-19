package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// 全局 Redis 客户端（可选初始化）
var rdb *goredis.Client

// Init 根据配置初始化 Redis 客户端；addr 为空则跳过。
func Init(addr, password string, db int) {
	if addr == "" {
		return
	}
	rdb = goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

// Client 返回 Redis 客户端实例（可能为 nil）。
func Client() *goredis.Client { return rdb }

// Ping 在给定超时时间内探测 Redis 连接是否可用。
func Ping(ctx context.Context, timeout time.Duration) error {
	if rdb == nil {
		return nil
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return rdb.Ping(c).Err()
}

