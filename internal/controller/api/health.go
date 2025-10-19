package api

import (
	beego "github.com/beego/beego/v2/server/web"
)

// HealthController 提供健康检查端点：/healthz 与 /readyz
// 后续可拓展就绪检查以探测Redis/MySQL/RocketMQ 连通性
// 并将结果汇总为整体就绪状态

type HealthController struct{ beego.Controller }

// Healthz 存活探针：仅返回进程存活
func (c *HealthController) Healthz() {
	c.Ctx.Output.SetStatus(200)
	_ = c.Ctx.Output.Body([]byte("ok"))
}

// Readyz 就绪探针：当前返回ready；接入依赖检查后反映真实状态
func (c *HealthController) Readyz() {
	c.Ctx.Output.SetStatus(200)
	_ = c.Ctx.Output.Body([]byte("ready"))
}

