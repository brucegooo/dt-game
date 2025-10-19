package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	infmysql "dt-server/internal/infra/mysql"
	infrds "dt-server/internal/infra/redis"
	"dt-server/internal/model"

	beego "github.com/beego/beego/v2/server/web"
	goredis "github.com/redis/go-redis/v9"
)

// RoundController 提供查询回合信息与开奖结果的接口（便于调试/回放）
// GET /api/round/:round_id
// 返回 { ok, round_info, draw_result }
// - round_info 与 draw_result 均从 Redis 读取，如均不存在则 404
// - 若仅存在其一，则返回存在的字段
// 注意：此接口为读缓存接口，不访问数据库

type RoundController struct {
	beego.Controller
}

func (c *RoundController) GetRound() {
	roundID := c.Ctx.Input.Param(":round_id")
	if roundID == "" {
		c.CustomAbort(400, "round_id is required")
		return
	}

	r := infrds.Client()
	if r == nil {
		c.CustomAbort(503, "redis unavailable")
		return
	}

	ctx := context.Background()

	var roundInfo map[string]any
	var drawResult map[string]any

	// 读取 round info
	if bs, err := r.Get(ctx, infrds.RoundInfoKey(roundID)).Bytes(); err == nil {
		_ = json.Unmarshal(bs, &roundInfo)
	} else if err != goredis.Nil {
		// 非不存在错误，视为服务不可用
		c.CustomAbort(503, "redis error")
		return
	}

	// 读取 draw result
	if bs, err := r.Get(ctx, infrds.RoundResultKey(roundID)).Bytes(); err == nil {
		_ = json.Unmarshal(bs, &drawResult)
	} else if err != goredis.Nil {
		c.CustomAbort(503, "redis error")
		return
	}

	if roundInfo == nil && drawResult == nil {
		// DB fallback：从数据库读取，并回填 Redis
		rs, err := model.GetRoundSnapshot(ctx, infmysql.SQLX(), roundID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.CustomAbort(404, "round not found")
				return
			}
			c.CustomAbort(503, "db error")
			return
		}
		// 组装 round_info
		roundInfo = map[string]any{
			"game_id":        rs.GameID,
			"room_id":        rs.RoomID,
			"game_round_id":  rs.GameRoundID,
			"bet_start_time": rs.BetStartTime,
			"bet_stop_time":  rs.BetStopTime,
			"game_status":    rs.GameStatus,
		}
		// 组装 draw_result（如有）
		if rs.GameResultStr != "" || rs.CardList != "" {
			var cards []string
			_ = json.Unmarshal([]byte(rs.CardList), &cards)
			drawResult = map[string]any{
				"game_round_id": rs.GameRoundID,
				"card_list":     cards,
				"result":        rs.GameResultStr,
			}
		}
		// 回填 Redis
		if b, e := json.Marshal(roundInfo); e == nil {
			_ = r.Set(ctx, infrds.RoundInfoKey(roundID), b, 20*time.Second).Err()
		}
		if drawResult != nil {
			if b, e := json.Marshal(drawResult); e == nil {
				_ = r.Set(ctx, infrds.RoundResultKey(roundID), b, 2*time.Minute).Err()
			}
		}
	}

	c.Data["json"] = map[string]any{
		"ok":          true,
		"round_info":  roundInfo,
		"draw_result": drawResult,
	}
	_ = c.ServeJSON()
}
