package middleware

import (
	"time"

	"dt-server/common/logger"
	"dt-server/internal/auth"
	"dt-server/internal/common/helper"
	"dt-server/internal/common/response"
	"dt-server/internal/config"

	beegocontext "github.com/beego/beego/v2/server/web/context"
	"go.uber.org/zap"
)

// PlatformAuthFilter 平台认证过滤器
// 验证平台签名，提取平台用户信息
func PlatformAuthFilter(ctx *beegocontext.Context) {
	cfg := config.Get()
	traceID := helper.GetTraceID(ctx)

	// 辅助函数：返回错误
	returnError := func(httpCode int, bizCode int, message string) {
		ctx.Output.SetStatus(httpCode)
		ctx.Output.JSON(response.APIResponse{
			Code:      bizCode,
			Message:   message,
			Data:      nil,
			TraceID:   traceID,
			Timestamp: time.Now().UnixMilli(),
		}, false, false)
	}

	// 演示模式：简化认证
	if cfg != nil && cfg.Auth.DemoMode {
		// 演示模式：从请求参数或请求头中提取用户信息
		platformUserID := ctx.Input.Header("X-Platform-User-Id")
		if platformUserID == "" {
			platformUserID = ctx.Input.Query("user_id")
		}
		if platformUserID == "" {
			platformUserID = "demo_user_001" // 默认演示用户
		}

		platformUserName := ctx.Input.Header("X-Platform-User-Name")
		if platformUserName == "" {
			platformUserName = "Demo User"
		}

		// 注入演示平台信息
		ctx.Input.SetData("platform_id", cfg.Auth.DemoPlatform.PlatformID)
		ctx.Input.SetData("platform_user_id", platformUserID)
		ctx.Input.SetData("platform_user_name", platformUserName)
		ctx.Input.SetData("demo_mode", true)

		logger.Debug("demo mode authentication",
			zap.String("trace_id", traceID),
			zap.String("platform_user_id", platformUserID))
		return
	}

	// 生产模式：完整的平台签名验证
	// 1. 验证平台签名
	platform, err := auth.VerifyPlatformSignature(ctx)
	if err != nil {
		logger.Warn("platform authentication failed",
			zap.String("trace_id", traceID),
			zap.Error(err))

		// 根据错误类型返回不同的错误码
		switch err {
		case auth.ErrMissingAuthHeaders:
			returnError(401, response.CodeUnauthorized, "缺少认证信息")
		case auth.ErrTimestampExpired:
			returnError(401, response.CodeTimestampExpired, "时间戳已过期")
		case auth.ErrNonceReused:
			returnError(401, response.CodeNonceReused, "Nonce已被使用")
		case auth.ErrInvalidSignature:
			returnError(401, response.CodeInvalidSignature, "签名验证失败")
		case auth.ErrInvalidPlatform:
			returnError(401, response.CodeInvalidPlatform, "无效的平台")
		case auth.ErrPlatformDisabled:
			returnError(403, response.CodePlatformDisabled, "平台已禁用")
		case auth.ErrIPNotAllowed:
			returnError(403, response.CodeIPNotAllowed, "IP不在白名单")
		default:
			returnError(401, response.CodeUnauthorized, "认证失败")
		}
		return
	}

	// 2. 提取平台用户ID（必填）
	platformUserID := ctx.Input.Header("X-Platform-User-Id")
	if platformUserID == "" {
		logger.Warn("missing platform user id",
			zap.String("trace_id", traceID),
			zap.String("platform", platform.AppKey))
		returnError(400, response.CodeBadRequest, "X-Platform-User-Id is required")
		return
	}

	// 3. 验证用户ID格式
	if !auth.IsValidPlatformUserID(platformUserID) {
		logger.Warn("invalid platform user id format",
			zap.String("trace_id", traceID),
			zap.String("platform_user_id", platformUserID))
		returnError(400, response.CodeBadRequest, "invalid platform_user_id format")
		return
	}

	// 4. 提取平台用户名（可选）
	platformUserName := ctx.Input.Header("X-Platform-User-Name")

	// 5. 将信息存入 context
	ctx.Input.SetData("platform", platform)
	ctx.Input.SetData("platform_id", platform.PlatformID)
	ctx.Input.SetData("platform_user_id", platformUserID)
	ctx.Input.SetData("platform_user_name", platformUserName)

	logger.Debug("platform authentication successful",
		zap.String("trace_id", traceID),
		zap.String("platform", platform.AppKey),
		zap.Int8("platform_id", platform.PlatformID),
		zap.String("platform_user_id", platformUserID))
}

// DemoAuthFilter 演示模式认证过滤器（简化版）
// 用于演示和测试，跳过复杂的签名验证
func DemoAuthFilter(ctx *beegocontext.Context) {
	cfg := config.Get()
	if cfg == nil || !cfg.Auth.DemoMode {
		return
	}

	traceID := helper.GetTraceID(ctx)

	// 检查是否已经通过正式认证
	if ctx.Input.GetData("platform_id") != nil {
		return // 已认证，跳过
	}

	// 演示模式：从请求参数或请求头中提取用户信息
	platformUserID := ctx.Input.Header("X-Platform-User-Id")
	if platformUserID == "" {
		platformUserID = ctx.Input.Query("user_id")
	}
	if platformUserID == "" {
		platformUserID = "demo_user_001" // 默认演示用户
	}

	platformUserName := ctx.Input.Header("X-Platform-User-Name")
	if platformUserName == "" {
		platformUserName = "Demo User"
	}

	// 注入演示平台信息
	ctx.Input.SetData("platform_id", cfg.Auth.DemoPlatform.PlatformID)
	ctx.Input.SetData("platform_user_id", platformUserID)
	ctx.Input.SetData("platform_user_name", platformUserName)
	ctx.Input.SetData("demo_mode", true)

	logger.Debug("demo mode authentication",
		zap.String("trace_id", traceID),
		zap.String("platform_user_id", platformUserID))
}
