package middleware

import (
	"fmt"
	"strings"

	"dt-server/internal/config"

	beegocontext "github.com/beego/beego/v2/server/web/context"
)

// CORSFilter CORS 跨域中间件
func CORSFilter(ctx *beegocontext.Context) {
	cfg := config.Get()
	if cfg == nil || !cfg.CORS.Enabled {
		return
	}

	origin := ctx.Request.Header.Get("Origin")
	if origin == "" {
		return
	}

	// 检查 Origin 是否在允许列表中
	allowed := false
	for _, allowedOrigin := range cfg.CORS.AllowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			allowed = true
			break
		}
	}

	if !allowed {
		return
	}

	// 设置 CORS 响应头
	ctx.Output.Header("Access-Control-Allow-Origin", origin)
	ctx.Output.Header("Access-Control-Allow-Methods", strings.Join(cfg.CORS.AllowedMethods, ", "))
	ctx.Output.Header("Access-Control-Allow-Headers", strings.Join(cfg.CORS.AllowedHeaders, ", "))
	ctx.Output.Header("Access-Control-Expose-Headers", strings.Join(cfg.CORS.ExposedHeaders, ", "))
	ctx.Output.Header("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.CORS.MaxAge))

	if cfg.CORS.AllowCredentials {
		ctx.Output.Header("Access-Control-Allow-Credentials", "true")
	}

	// 处理 OPTIONS 预检请求
	if ctx.Request.Method == "OPTIONS" {
		ctx.Output.SetStatus(204)
		ctx.ResponseWriter.WriteHeader(204)
		return
	}
}
