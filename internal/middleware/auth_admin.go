package middleware

import (
	"strings"
	"time"

	"dt-server/common/logger"
	"dt-server/internal/common/helper"
	"dt-server/internal/common/response"
	"dt-server/internal/config"

	beegocontext "github.com/beego/beego/v2/server/web/context"
	"go.uber.org/zap"
)

// AdminAuthFilter 管理员认证过滤器（简单Token）
// 用于保护管理接口（如开奖、游戏事件等）
func AdminAuthFilter(ctx *beegocontext.Context) {
	cfg := config.Get()
	traceID := helper.GetTraceID(ctx)

	// 如果未启用管理员认证，跳过
	if cfg == nil || !cfg.Auth.Admin.Enabled {
		logger.Debug("admin auth disabled, skip", zap.String("trace_id", traceID))
		return
	}

	// 辅助函数：返回认证错误
	returnAuthError := func(code int, message string) {
		ctx.Output.SetStatus(code)
		ctx.Output.JSON(response.APIResponse{
			Code:      response.CodeUnauthorized,
			Message:   message,
			Data:      nil,
			TraceID:   traceID,
			Timestamp: time.Now().UnixMilli(),
		}, false, false)
	}

	// 提取 Authorization 头
	authHeader := strings.TrimSpace(ctx.Input.Header("Authorization"))
	if authHeader == "" {
		logger.Warn("missing admin token", zap.String("trace_id", traceID))
		returnAuthError(401, "缺少管理员认证信息")
		return
	}

	// 解析 Bearer Token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		logger.Warn("invalid admin token format", zap.String("trace_id", traceID))
		returnAuthError(401, "无效的认证格式")
		return
	}

	token := parts[1]

	// 验证 Token
	if token != cfg.Auth.Admin.Token {
		logger.Warn("invalid admin token",
			zap.String("trace_id", traceID),
			zap.String("token_prefix", token[:min(len(token), 8)]+"..."))
		returnAuthError(401, "无效的管理员Token")
		return
	}

	// 标记为管理员请求
	ctx.Input.SetData("is_admin", true)

	logger.Debug("admin authentication successful", zap.String("trace_id", traceID))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
