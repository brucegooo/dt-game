package api

import (
	"errors"
	"strconv"
	"strings"

	helper "dt-server/internal/common/helper"
	"dt-server/internal/common/response"
	"dt-server/internal/service"

	beego "github.com/beego/beego/v2/server/web"

	mysqlerr "github.com/go-sql-driver/mysql"
)

var newBetService = service.NewBetService

type BetController struct{ beego.Controller }

// 投注请求参数
type BetRequestParam struct {
	GameId      string `json:"game_id"`       // 游戏ID
	RoomId      string `json:"room_id"`       // 房间ID
	GameRoundId string `json:"game_round_id"` // 局ID
	UserId      int64  `json:"user_id"`       // 用户ID
	BetAmount   string `json:"bet_amount"`    // 投注金额
	PlayType    int    `json:"play_type"`     // 下注玩法 1=dragon2=tiger3=tie
	// Platform    int    `json:"platform"`      // 平台 1:测试演示 2.
	/*
		幂等键：客户端生成并随请求传入，用于在网络重试/超时重发/服务端重试时保证“同一业务请求只生效一次”。
		使用约定：
		- 对于“同一次下注”的所有重试，请传相同的 idempotency_key；
		- 业务语义不同（如金额/玩法/回合/用户不同）的请求必须使用不同的 key；
		- 建议生成方式：UUID（推荐）或对 user_id+game_round_id+play_type+bet_amount 做哈希；
		- 建议在客户端将 key 与该次操作绑定并在超时/失败后复用。
		服务端幂等保证（多层防护）：
		1) Redis 进行中锁（约45秒）：并发重复请求直接返回 202，并携带 Retry-After: 1；
		2) MySQL 唯一键：在事务内先插入 idempotency_keys(idempotency_key)，若已存在则视为重复请求，返回首次请求的结果；
		3) 结果缓存：首次成功结果会写入 Redis（短期缓存），后续重复可直接读缓存快速返回。
		错误语义：
		- 并发重复（正在处理）：HTTP 202 + { ok:false, message:"duplicate request in flight" }
		- 历史重复（已处理完）：返回首次的 bill_no 与余额，不算错误。
	*/
	IdempotencyKey string `json:"idempotency_key"`
}

// BetController 处理投注接口：POST /api/bet
func (c *BetController) Bet() {
	// 1) 解析入参与基本校验
	// 这里必须要对业务参数严格校验，后续service不再重复校验
	bp, ok, msg := helper.ParseAndValidateBet(c.Ctx)
	if !ok {
		response.BadRequest(&c.Controller, msg, helper.GetTraceID(c.Ctx))
		return
	}

	svc := newBetService()
	traceID := helper.GetTraceID(c.Ctx)

	// 从 context 提取平台信息（由认证中间件注入）
	platformID := int8(0)
	platformUserID := ""
	platformUserName := ""

	if v := c.Ctx.Input.GetData("platform_id"); v != nil {
		if pid, ok := v.(int8); ok {
			platformID = pid
		}
	}
	if v := c.Ctx.Input.GetData("platform_user_id"); v != nil {
		if puid, ok := v.(string); ok {
			platformUserID = puid
		}
	}
	if v := c.Ctx.Input.GetData("platform_user_name"); v != nil {
		if pname, ok := v.(string); ok {
			platformUserName = pname
		}
	}

	// 如果中间件未注入平台信息，使用请求中的 user_id
	if platformUserID == "" && bp.UserId != 0 {
		platformID = 0 // 系统默认平台
		platformUserID = strconv.FormatInt(bp.UserId, 10)
	}

	// 进行投注业务逻辑处理
	out, err := svc.PlaceBet(c.Ctx.Request.Context(), service.BetInput{
		GameID:           bp.GameId,
		RoomID:           bp.RoomId,
		GameRoundID:      bp.GameRoundId,
		PlatformID:       platformID,
		PlatformUserID:   platformUserID,
		PlatformUserName: platformUserName,
		BetAmount:        bp.BetAmount,
		PlayType:         bp.PlayType,
		IdempotencyKey:   bp.IdempotencyKey,
		TraceID:          traceID,
	})
	if err != nil {
		// MySQL 唯一键冲突
		if me, ok := err.(*mysqlerr.MySQLError); ok && me.Number == 1062 {
			response.Conflict(&c.Controller, response.CodeDuplicateKey, traceID)
			return
		}
		// 重复请求进行中
		if errors.Is(err, service.ErrDuplicateInFlight) {
			response.Accepted(&c.Controller, "重复请求进行中，请稍后重试", traceID)
			return
		}
		// 状态不允许投注
		if errors.Is(err, service.ErrInvalidStateBet) {
			response.Conflict(&c.Controller, response.CodeInvalidState, traceID)
			return
		}
		// 投注窗口未开始
		if errors.Is(err, service.ErrBetWindowNotStart) {
			response.Conflict(&c.Controller, response.CodeBetWindowNotStart, traceID)
			return
		}
		// 投注窗口已关闭
		if errors.Is(err, service.ErrBetWindowClosed) {
			response.Conflict(&c.Controller, response.CodeBetWindowClosed, traceID)
			return
		}
		// 冲突投注（同时投注龙和虎）
		if errors.Is(err, service.ErrConflictingPlayTypes) {
			response.Conflict(&c.Controller, response.CodeConflictingBet, traceID)
			return
		}
		// 投注金额验证失败
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid bet amount") ||
			strings.Contains(errMsg, "bet amount must be positive") ||
			strings.Contains(errMsg, "below minimum limit") ||
			strings.Contains(errMsg, "exceeds maximum limit") {
			response.BadRequest(&c.Controller, errMsg, traceID)
			return
		}
		// 余额不足
		if strings.Contains(errMsg, "insufficient balance") {
			response.BadRequest(&c.Controller, "余额不足", traceID)
			return
		}
		// 用户状态异常
		if strings.Contains(errMsg, "user disabled") {
			response.BadRequest(&c.Controller, "用户状态异常", traceID)
			return
		}
		// 系统错误
		response.InternalError(&c.Controller, traceID)
		return
	}

	// 成功响应
	response.Success(&c.Controller, map[string]interface{}{
		"bill_no":       out.BillNo,
		"remain_amount": out.RemainAmount,
	}, traceID)
}
