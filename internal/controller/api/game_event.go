package api

import (
	"errors"

	helper "dt-server/internal/common/helper"
	"dt-server/internal/common/response"
	infmysql "dt-server/internal/infra/mysql"
	"dt-server/internal/model"
	"dt-server/internal/service"

	beego "github.com/beego/beego/v2/server/web"
)

var newGameEventService = service.NewGameEventService

// GameEventController 处理游戏事件接口：/api/game_event
// 业务含义：驱动局状态机在合法路径上流转（不负责开奖，开奖由 /api/drawresult 执行）
type GameEventController struct{ beego.Controller }

// GameEventRequestParam 游戏事件入参
// - event_type 取值：
type GameEventRequestParam struct {
	GameId      string `json:"game_id"`
	RoomId      string `json:"room_id"`
	GameRoundId string `json:"game_round_id"`
	EventType   int    `json:"event_type"` // 1=game_start 2=game_stop 3=new_card 4=game_draw 5=game_end
}

// Post 接收并处理事件
// 步骤：
// 1) 解析入参与基本校验
// 2) 调用 Service 层执行业务与状态检查
// 3) 按错误类型映射 HTTP 状态码：400 参数错误；409 非法状态跳转
func (c *GameEventController) GameEvent() {
	gp, ok, msg := helper.ParseAndValidateGameEvent(c.Ctx)
	if !ok {
		response.BadRequest(&c.Controller, msg, helper.GetTraceID(c.Ctx))
		return
	}
	svc := newGameEventService()
	traceID := helper.GetTraceID(c.Ctx)
	if err := svc.Handle(c.Ctx.Request.Context(), service.GameEventInput{
		GameID:      gp.GameId,
		RoomID:      gp.RoomId,
		GameRoundID: gp.GameRoundId,
		EventType:   int8(gp.EventType),
		TraceID:     traceID,
	}); err != nil {
		if errors.Is(err, service.ErrBadRequest) {
			response.BadRequest(&c.Controller, "invalid request", traceID)
			return
		}
		response.Conflict(&c.Controller, response.CodeInvalidState, traceID)
		return
	}
	// 对于 game_start（1），返回数据库中写入的下注开始时间，保持与入库时间强一致
	if gp.EventType == 1 {
		// 从数据库读取刚写入的 bet_start_time 与 bet_stop_time（仅透传 DB 值）
		rs, err := model.GetRoundSnapshot(c.Ctx.Request.Context(), infmysql.SQLX(), gp.GameRoundId)
		if err == nil && rs != nil {
			betStart := rs.BetStartTime
			betStop := rs.BetStopTime
			response.Success(&c.Controller, map[string]interface{}{
				"bet_start_time":   betStart,
				"bet_stop_time":    betStop,
				"countdown_second": 45, // 倒计时秒数（方便测试）
				"game_round_id":    gp.GameRoundId,
				"game_id":          gp.GameId,
				"room_id":          gp.RoomId,
			}, traceID)
			return
		}
		// 兜底：若读取失败，返回通用成功
		response.Success(&c.Controller, nil, traceID)
		return
	}
	// 其他事件类型，返回通用成功
	response.Success(&c.Controller, nil, traceID)
}
