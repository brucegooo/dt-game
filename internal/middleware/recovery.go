package middleware

import (
	"runtime/debug"

	"dt-server/common/logger"
	"dt-server/internal/common/helper"
	"dt-server/internal/common/response"

	beegocontext "github.com/beego/beego/v2/server/web/context"
	"go.uber.org/zap"
)

// RecoveryFilter Panic Recovery 中间件
// 捕获所有未处理的 panic，防止进程崩溃
func RecoveryFilter(ctx *beegocontext.Context) {
	defer func() {
		if err := recover(); err != nil {
			traceID := helper.GetTraceID(ctx)

			// 记录 panic 信息和堆栈
			logger.Error("panic recovered",
				zap.String("trace_id", traceID),
				zap.String("method", ctx.Request.Method),
				zap.String("path", ctx.Request.URL.Path),
				zap.Any("error", err),
				zap.String("stack", string(debug.Stack())))

			// 返回 500 错误
			ctx.Output.SetStatus(500)
			ctx.Output.JSON(response.APIResponse{
				Code:      response.CodeSystemError,
				Message:   "系统繁忙，请稍后重试",
				Data:      nil,
				TraceID:   traceID,
				Timestamp: 0,
			}, false, false)

			// 阻止继续执行
			ctx.Abort(500, "panic recovered")
		}
	}()
}
