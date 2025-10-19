package middleware

import (
	"time"

	"dt-server/common/logger"
	"dt-server/internal/auth"
	"dt-server/internal/common/helper"
	"dt-server/internal/common/response"

	beegocontext "github.com/beego/beego/v2/server/web/context"
	"go.uber.org/zap"
)

// UserAuthFilter 用户认证过滤器（JWT Token）
// 验证用户的 JWT Token，提取用户信息
func UserAuthFilter(ctx *beegocontext.Context) {
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

	// 1. 验证 JWT Token
	claims, err := auth.VerifyJWTToken(ctx)
	if err != nil {
		logger.Warn("user authentication failed",
			zap.String("trace_id", traceID),
			zap.Error(err))

		// 根据错误类型返回不同的错误码
		switch err {
		case auth.ErrMissingToken:
			returnError(401, response.CodeUnauthorized, "缺少认证Token")
		case auth.ErrInvalidTokenFormat:
			returnError(401, response.CodeInvalidToken, "Token格式无效")
		case auth.ErrInvalidToken:
			returnError(401, response.CodeInvalidToken, "Token无效")
		case auth.ErrTokenExpired:
			returnError(401, response.CodeTokenExpired, "Token已过期")
		case auth.ErrTokenRevoked:
			returnError(401, response.CodeTokenRevoked, "Token已撤销")
		default:
			returnError(401, response.CodeUnauthorized, "认证失败")
		}
		return
	}

	// 2. 验证平台一致性（如果已经过平台认证）
	if platformID := ctx.Input.GetData("platform_id"); platformID != nil {
		if claims.PlatformID != platformID.(int8) {
			logger.Warn("platform mismatch",
				zap.String("trace_id", traceID),
				zap.Int8("token_platform_id", claims.PlatformID),
				zap.Int8("request_platform_id", platformID.(int8)))
			returnError(403, response.CodeForbidden, "平台不匹配")
			return
		}
	}

	// 3. 将用户信息存入 context
	ctx.Input.SetData("user_id", claims.UserID)
	ctx.Input.SetData("username", claims.Username)
	ctx.Input.SetData("jwt_claims", claims)

	// 如果没有平台信息，从 Token 中提取
	if ctx.Input.GetData("platform_id") == nil {
		ctx.Input.SetData("platform_id", claims.PlatformID)
		ctx.Input.SetData("app_key", claims.AppKey)
	}

	logger.Debug("user authentication successful",
		zap.String("trace_id", traceID),
		zap.Int64("user_id", claims.UserID),
		zap.String("username", claims.Username),
		zap.Int8("platform_id", claims.PlatformID))
}
