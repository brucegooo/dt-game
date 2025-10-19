package response

import (
	"time"

	beego "github.com/beego/beego/v2/server/web"
)

// APIResponse 统一 API 响应结构
// 所有 API 都应该返回这个结构，无论成功还是失败
type APIResponse struct {
	Code      int         `json:"code"`                // 业务错误码：0=成功，非0=失败
	Message   string      `json:"message"`             // 错误消息
	Data      interface{} `json:"data,omitempty"`      // 业务数据（失败时为 null）
	TraceID   string      `json:"trace_id,omitempty"`  // 请求追踪ID
	Timestamp int64       `json:"timestamp,omitempty"` // 响应时间戳（Unix 毫秒）
}

// 错误码定义
const (
	CodeSuccess             = 0    // 成功
	CodeBadRequest          = 1000 // 参数错误
	CodeBusinessError       = 2000 // 业务错误（通用）
	CodeDuplicateInFlight   = 2001 // 重复请求进行中
	CodeDuplicateKey        = 2002 // 幂等键冲突
	CodeInvalidState        = 2003 // 状态不允许
	CodeBetWindowNotStart   = 2004 // 投注窗口未开始
	CodeBetWindowClosed     = 2005 // 投注窗口已关闭
	CodeConflictingBet      = 2006 // 冲突投注
	CodeInsufficientBalance = 2007 // 余额不足
	CodeInvalidStateDraw    = 2008 // 开奖状态不允许
	CodeInvalidStateGameEnd = 2009 // 游戏结束状态不允许
	CodeUnauthorized        = 3000 // 未授权
	CodeInvalidToken        = 3001 // Token 无效
	CodeTokenExpired        = 3002 // Token 过期
	CodeTokenRevoked        = 3003 // Token 已撤销
	CodeInvalidSignature    = 3004 // 签名无效
	CodeTimestampExpired    = 3005 // 时间戳过期
	CodeNonceReused         = 3006 // Nonce 重复使用
	CodeInvalidPlatform     = 3007 // 平台无效
	CodePlatformDisabled    = 3008 // 平台已禁用
	CodeForbidden           = 3009 // 禁止访问
	CodeIPNotAllowed        = 3010 // IP 不在白名单
	CodeNotFound            = 4004 // 资源不存在
	CodeRateLimitExceeded   = 4000 // 请求频率超限
	CodeSystemError         = 5000 // 系统错误
)

// ErrorMessages 错误消息映射
var ErrorMessages = map[int]string{
	CodeSuccess:             "success",
	CodeBadRequest:          "参数错误",
	CodeBusinessError:       "业务处理失败",
	CodeDuplicateInFlight:   "重复请求进行中，请稍后重试",
	CodeDuplicateKey:        "重复的请求",
	CodeInvalidState:        "当前状态不允许此操作",
	CodeBetWindowNotStart:   "投注窗口未开始",
	CodeBetWindowClosed:     "投注窗口已关闭",
	CodeConflictingBet:      "不能同时投注龙和虎",
	CodeInsufficientBalance: "余额不足",
	CodeInvalidStateDraw:    "当前状态不允许开奖",
	CodeInvalidStateGameEnd: "游戏尚未开奖，不能结束",
	CodeNotFound:            "资源不存在",
	CodeSystemError:         "系统繁忙，请稍后重试",
}

// Success 成功响应
// 参数：
//   - c: Beego Controller
//   - data: 业务数据（可以是 map、struct、slice 等）
//   - traceID: 请求追踪ID
//
// 示例：
//
//	response.Success(c, map[string]interface{}{
//	    "bill_no": "DT20251019123456789012345",
//	    "remain_amount": "900.00",
//	}, traceID)
func Success(c *beego.Controller, data interface{}, traceID string) {
	c.Data["json"] = APIResponse{
		Code:      CodeSuccess,
		Message:   ErrorMessages[CodeSuccess],
		Data:      data,
		TraceID:   traceID,
		Timestamp: time.Now().UnixMilli(),
	}
	c.ServeJSON()
}

// Error 错误响应（使用预定义的错误消息）
// 参数：
//   - c: Beego Controller
//   - httpStatus: HTTP 状态码（如 400、409、500）
//   - code: 业务错误码（如 CodeBetWindowClosed）
//   - traceID: 请求追踪ID
//
// 示例：
//
//	response.Error(c, 409, response.CodeBetWindowClosed, traceID)
func Error(c *beego.Controller, httpStatus int, code int, traceID string) {
	c.Ctx.Output.SetStatus(httpStatus)
	c.Data["json"] = APIResponse{
		Code:      code,
		Message:   getErrorMessage(code),
		Data:      nil,
		TraceID:   traceID,
		Timestamp: time.Now().UnixMilli(),
	}
	c.ServeJSON()
}

// ErrorWithMessage 错误响应（使用自定义错误消息）
// 参数：
//   - c: Beego Controller
//   - httpStatus: HTTP 状态码（如 400、409、500）
//   - code: 业务错误码（如 CodeBusinessError）
//   - message: 自定义错误消息
//   - traceID: 请求追踪ID
//
// 示例：
//
//	response.ErrorWithMessage(c, 500, response.CodeSystemError, "数据库连接失败", traceID)
func ErrorWithMessage(c *beego.Controller, httpStatus int, code int, message string, traceID string) {
	c.Ctx.Output.SetStatus(httpStatus)
	c.Data["json"] = APIResponse{
		Code:      code,
		Message:   message,
		Data:      nil,
		TraceID:   traceID,
		Timestamp: time.Now().UnixMilli(),
	}
	c.ServeJSON()
}

// BadRequest 参数错误响应（HTTP 400）
// 参数：
//   - c: Beego Controller
//   - message: 错误消息（如 "user_id is required"）
//   - traceID: 请求追踪ID
//
// 示例：
//
//	response.BadRequest(c, "user_id is required", traceID)
func BadRequest(c *beego.Controller, message string, traceID string) {
	ErrorWithMessage(c, 400, CodeBadRequest, message, traceID)
}

// Conflict 资源冲突响应（HTTP 409）
// 参数：
//   - c: Beego Controller
//   - code: 业务错误码（如 CodeDuplicateKey、CodeInvalidState）
//   - traceID: 请求追踪ID
//
// 示例：
//
//	response.Conflict(c, response.CodeDuplicateKey, traceID)
func Conflict(c *beego.Controller, code int, traceID string) {
	Error(c, 409, code, traceID)
}

// NotFound 资源不存在响应（HTTP 404）
// 参数：
//   - c: Beego Controller
//   - message: 错误消息（如 "游戏回合不存在"）
//   - traceID: 请求追踪ID
//
// 示例：
//
//	response.NotFound(c, "游戏回合不存在", traceID)
func NotFound(c *beego.Controller, message string, traceID string) {
	ErrorWithMessage(c, 404, CodeNotFound, message, traceID)
}

// InternalError 系统错误响应（HTTP 500）
// 参数：
//   - c: Beego Controller
//   - traceID: 请求追踪ID
//
// 示例：
//
//	response.InternalError(c, traceID)
func InternalError(c *beego.Controller, traceID string) {
	Error(c, 500, CodeSystemError, traceID)
}

// InternalErrorWithMessage 系统错误响应（HTTP 500，自定义消息）
// 注意：生产环境不应该暴露详细的错误信息，应该记录到日志
// 参数：
//   - c: Beego Controller
//   - message: 错误消息（仅用于开发/调试，生产环境应该使用通用消息）
//   - traceID: 请求追踪ID
//
// 示例：
//
//	response.InternalErrorWithMessage(c, "database connection failed", traceID)
func InternalErrorWithMessage(c *beego.Controller, message string, traceID string) {
	ErrorWithMessage(c, 500, CodeSystemError, message, traceID)
}

// Accepted 请求已接受但尚未处理完成（HTTP 202）
// 用于异步处理场景，如重复请求进行中
// 参数：
//   - c: Beego Controller
//   - message: 提示消息
//   - traceID: 请求追踪ID
//
// 示例：
//
//	response.Accepted(c, "重复请求进行中，请稍后重试", traceID)
func Accepted(c *beego.Controller, message string, traceID string) {
	c.Ctx.Output.SetStatus(202)
	c.Ctx.Output.Header("Retry-After", "1") // 建议客户端 1 秒后重试
	c.Data["json"] = APIResponse{
		Code:      CodeDuplicateInFlight,
		Message:   message,
		Data:      nil,
		TraceID:   traceID,
		Timestamp: time.Now().UnixMilli(),
	}
	c.ServeJSON()
}

// getErrorMessage 获取错误消息，如果未定义则返回通用消息
func getErrorMessage(code int) string {
	if msg, ok := ErrorMessages[code]; ok {
		return msg
	}
	return "未知错误"
}
