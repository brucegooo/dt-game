package api

import (
	"errors"
	"strings"

	helper "dt-server/internal/common/helper"
	"dt-server/internal/common/response"
	"dt-server/internal/service"

	beego "github.com/beego/beego/v2/server/web"
)

var newDrawService = service.NewDrawService

type DrawResultController struct{ beego.Controller }

// 开奖结果请求参数
type DrawResultRequestParam struct {
	GameId      string `json:"game_id"`
	RoomId      string `json:"room_id"`
	GameRoundId string `json:"game_round_id"`
	// D5,T3,RDragon (Dragon赢) | D1,T2,RTiger (Tiger赢) | D9,T9,RTie (和局)
	CardList string `json:"card_list"`
	// 扩展一个游戏结果生成的时间戳
	DrawTime int64 `json:"draw_time"`
}

// Drawresult 人工开奖接口：POST /api/drawresult
func (c *DrawResultController) Drawresult() {
	dp, ok, msg := helper.ParseAndValidateDrawResult(c.Ctx)
	if !ok {
		response.BadRequest(&c.Controller, msg, helper.GetTraceID(c.Ctx))
		return
	}

	svc := newDrawService()
	traceID := helper.GetTraceID(c.Ctx)
	if err := svc.SubmitDrawResult(c.Ctx.Request.Context(), service.DrawInput{
		GameID:      dp.GameId,
		RoomID:      dp.RoomId,
		GameRoundID: dp.GameRoundId,
		CardList:    dp.CardList,
		TraceID:     traceID,
	}); err != nil {
		if errors.Is(err, service.ErrInvalidStateDraw) {
			response.Conflict(&c.Controller, response.CodeInvalidStateDraw, traceID)
			return
		}
		if errors.Is(err, service.ErrGameRoundNotFound) {
			response.NotFound(&c.Controller, "游戏回合不存在", traceID)
			return
		}
		if errors.Is(err, service.ErrBadRequest) {
			response.BadRequest(&c.Controller, "invalid request", traceID)
			return
		}
		// 处理卡牌格式和结果验证错误（新增）
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid card list format") ||
			strings.Contains(errMsg, "invalid game result") {
			response.BadRequest(&c.Controller, errMsg, traceID)
			return
		}
		response.InternalError(&c.Controller, traceID)
		return
	}
	response.Success(&c.Controller, nil, traceID)
}
