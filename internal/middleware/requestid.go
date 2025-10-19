package middleware

import (
	"github.com/beego/beego/v2/server/web/context"
	"github.com/google/uuid"
)

// RequestIDFilter 为每个请求注入并返回一个 X-Request-Id，用于链路追踪的最小闭环
func RequestIDFilter(ctx *context.Context) {
	id := ctx.Input.Header("X-Request-Id")
	if id == "" {
		id = uuid.NewString()
	}
	ctx.Input.SetData("trace_id", id)
	ctx.Output.Header("X-Request-Id", id)
}
