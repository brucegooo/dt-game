package routers

import (
	"dt-server/internal/config"
	"dt-server/internal/controller/api"
	"dt-server/internal/metrics"
	"dt-server/internal/middleware"

	beego "github.com/beego/beego/v2/server/web"
)

// init 注册HTTP路由与全局过滤器
func init() {
	cfg := config.Get()

	// 全局过滤器（按执行顺序）
	// 1. Panic Recovery（最先执行，捕获所有 panic）
	beego.InsertFilter("/*", beego.BeforeRouter, middleware.RecoveryFilter)

	// 2. 请求ID注入
	beego.InsertFilter("/*", beego.BeforeRouter, middleware.RequestIDFilter)

	// 3. CORS 处理（如果启用）
	if cfg != nil && cfg.CORS.Enabled {
		beego.InsertFilter("/*", beego.BeforeExec, middleware.CORSFilter)
	}

	// 4. HTTP 指标收集
	beego.InsertFilter("/*", beego.BeforeExec, metrics.HTTPMetricsFilter)
	beego.InsertFilter("/*", beego.FinishRouter, metrics.HTTPMetricsAfter)

	// 静态文件服务
	beego.SetStaticPath("/static", "static")
	beego.SetStaticPath("/debug", "static/debug.html")

	// 健康检查（无需认证）
	// beego.Router("/healthz", &api.HealthController{}, "get:Healthz")
	// beego.Router("/readyz", &api.HealthController{}, "get:Readyz")

	// ========== 业务 API（需要认证） ==========

	// 投注接口：平台认证 + 限流
	if cfg != nil && cfg.Auth.DemoMode {
		// 演示模式：简化认证
		beego.InsertFilter("/api/bet", beego.BeforeExec, middleware.DemoAuthFilter)
	} else {
		// 生产模式：平台签名认证
		beego.InsertFilter("/api/bet", beego.BeforeExec, middleware.PlatformAuthFilter)
	}
	if cfg != nil && cfg.RateLimit.Enabled {
		beego.InsertFilter("/api/bet", beego.BeforeExec, middleware.RateLimitFilter)
	}
	beego.Router("/api/bet", &api.BetController{}, "post:Bet")

	// 用户查询接口：平台认证（用户只能查询自己的数据）
	if cfg != nil && cfg.Auth.DemoMode {
		beego.InsertFilter("/api/user/*", beego.BeforeExec, middleware.DemoAuthFilter)
	} else {
		beego.InsertFilter("/api/user/*", beego.BeforeExec, middleware.PlatformAuthFilter)
	}
	beego.Router("/api/user/balance", &api.UserController{}, "get:Balance")
	beego.Router("/api/user/bets", &api.UserController{}, "get:Bets")

	// ========== 管理 API（需要管理员认证） ==========

	// 游戏事件接口：管理员认证
	if cfg != nil && cfg.Auth.Admin.Enabled {
		beego.InsertFilter("/api/game_event", beego.BeforeExec, middleware.AdminAuthFilter)
	}
	beego.Router("/api/game_event", &api.GameEventController{}, "post:GameEvent")

	// 开奖结果接口：管理员认证
	if cfg != nil && cfg.Auth.Admin.Enabled {
		beego.InsertFilter("/api/drawresult", beego.BeforeExec, middleware.AdminAuthFilter)
	}
	beego.Router("/api/drawresult", &api.DrawResultController{}, "post:Drawresult")

	// 局游戏调试接口：从 Redis 读取局缓存与结果缓存
	// beego.Router("/api/round/:round_id", &api.RoundController{}, "get:GetRound")

}
