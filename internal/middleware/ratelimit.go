package middleware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"dt-server/common/logger"
	"dt-server/internal/common/helper"
	"dt-server/internal/common/response"
	"dt-server/internal/config"
	infrds "dt-server/internal/infra/redis"

	beegocontext "github.com/beego/beego/v2/server/web/context"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimitFilter 限流中间件
// 支持多维度限流：全局、按IP、按用户、按平台
func RateLimitFilter(ctx *beegocontext.Context) {
	cfg := config.Get()
	if cfg == nil || !cfg.RateLimit.Enabled {
		return
	}

	traceID := helper.GetTraceID(ctx)
	rdb := infrds.Client()
	if rdb == nil {
		// Redis 不可用时，跳过限流（降级）
		logger.Warn("redis not available, skip rate limit", zap.String("trace_id", traceID))
		return
	}

	reqCtx := ctx.Request.Context()

	// 辅助函数：返回限流错误
	returnRateLimitError := func() {
		ctx.Output.SetStatus(429)
		ctx.Output.JSON(response.APIResponse{
			Code:      response.CodeRateLimitExceeded,
			Message:   "请求频率超限，请稍后重试",
			Data:      nil,
			TraceID:   traceID,
			Timestamp: time.Now().UnixMilli(),
		}, false, false)
	}

	// 1. 全局限流
	if cfg.RateLimit.Global.RequestsPerSecond > 0 {
		if !checkRateLimit(reqCtx, rdb, "global", "all", cfg.RateLimit.Global.RequestsPerSecond, 1) {
			logger.Warn("global rate limit exceeded", zap.String("trace_id", traceID))
			returnRateLimitError()
			return
		}
	}

	// 2. 按IP限流
	if cfg.RateLimit.ByIP.RequestsPerSecond > 0 {
		clientIP := getClientIP(ctx)
		if !checkRateLimit(reqCtx, rdb, "ip", clientIP, cfg.RateLimit.ByIP.RequestsPerSecond, cfg.RateLimit.ByIP.WindowSeconds) {
			logger.Warn("ip rate limit exceeded",
				zap.String("trace_id", traceID),
				zap.String("client_ip", clientIP))
			returnRateLimitError()
			return
		}
	}

	// 3. 按平台限流
	if cfg.RateLimit.ByPlatform.RequestsPerSecond > 0 {
		if platformID := ctx.Input.GetData("platform_id"); platformID != nil {
			platformKey := fmt.Sprintf("platform_%d", platformID.(int8))
			if !checkRateLimit(reqCtx, rdb, "platform", platformKey, cfg.RateLimit.ByPlatform.RequestsPerSecond, cfg.RateLimit.ByPlatform.WindowSeconds) {
				logger.Warn("platform rate limit exceeded",
					zap.String("trace_id", traceID),
					zap.Int8("platform_id", platformID.(int8)))
				returnRateLimitError()
				return
			}
		}
	}

	// 4. 按用户限流
	if cfg.RateLimit.ByUser.RequestsPerSecond > 0 {
		if platformUserID := ctx.Input.GetData("platform_user_id"); platformUserID != nil {
			userKey := fmt.Sprintf("user_%s", platformUserID.(string))
			if !checkRateLimit(reqCtx, rdb, "user", userKey, cfg.RateLimit.ByUser.RequestsPerSecond, cfg.RateLimit.ByUser.WindowSeconds) {
				logger.Warn("user rate limit exceeded",
					zap.String("trace_id", traceID),
					zap.String("platform_user_id", platformUserID.(string)))
				returnRateLimitError()
				return
			}
		}
	}
}

// checkRateLimit 检查限流（使用滑动窗口算法）
// dimension: 维度（global/ip/platform/user）
// key: 具体的key
// limit: 限制数量
// windowSeconds: 时间窗口（秒）
func checkRateLimit(ctx context.Context, rdb *redis.Client, dimension, key string, limit int, windowSeconds int) bool {
	if rdb == nil {
		return true // 降级：Redis 不可用时不限流
	}

	redisKey := fmt.Sprintf("ratelimit:%s:%s", dimension, key)
	now := time.Now().Unix()
	windowStart := now - int64(windowSeconds)

	// 使用 Redis Sorted Set 实现滑动窗口
	pipe := rdb.Pipeline()

	// 1. 移除窗口外的记录
	pipe.ZRemRangeByScore(ctx, redisKey, "0", strconv.FormatInt(windowStart, 10))

	// 2. 统计当前窗口内的请求数
	countCmd := pipe.ZCount(ctx, redisKey, strconv.FormatInt(windowStart, 10), "+inf")

	// 3. 添加当前请求
	pipe.ZAdd(ctx, redisKey, redis.Z{
		Score:  float64(now),
		Member: fmt.Sprintf("%d_%d", now, time.Now().UnixNano()),
	})

	// 4. 设置过期时间
	pipe.Expire(ctx, redisKey, time.Duration(windowSeconds+10)*time.Second)

	// 执行管道
	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Warn("rate limit check failed", zap.Error(err))
		return true // 降级：Redis 错误时不限流
	}

	count, err := countCmd.Result()
	if err != nil {
		logger.Warn("rate limit count failed", zap.Error(err))
		return true // 降级：Redis 错误时不限流
	}

	// 检查是否超过限制
	return count < int64(limit)
}

// getClientIP 获取客户端真实IP
func getClientIP(ctx *beegocontext.Context) string {
	// 优先从 X-Real-IP 获取
	if ip := ctx.Input.Header("X-Real-IP"); ip != "" {
		return ip
	}

	// 其次从 X-Forwarded-For 获取（取第一个）
	if xff := ctx.Input.Header("X-Forwarded-For"); xff != "" {
		for idx := 0; idx < len(xff); idx++ {
			if xff[idx] == ',' {
				return xff[:idx]
			}
		}
		return xff
	}

	// 最后使用 RemoteAddr
	return ctx.Request.RemoteAddr
}
