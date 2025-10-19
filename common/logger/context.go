package logger

import (
	"context"
)

type ctxKey string

const traceIDKey ctxKey = "trace_id"

// 从 context 中获取 traceId
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// 将 traceId 注入到 context 中
func WithTraceID(ctx context.Context, traceId string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceId)
}
