package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	infmysql "dt-server/internal/infra/mysql"
	infrds "dt-server/internal/infra/redis"
	"dt-server/internal/metrics"
	"dt-server/internal/model"
	"dt-server/internal/state"
)

type GameEventInput struct {
	GameID      string
	RoomID      string
	GameRoundID string //局ID
	EventType   int8   // 1=game_start 2=game_stop 3=new_card 4=game_draw 5=game_end
	TraceID     string
}

type GameEventService interface {
	Handle(ctx context.Context, in GameEventInput) error
}

type gameEventService struct{}

func NewGameEventService() GameEventService { return &gameEventService{} }

const (
	betWindowSeconds = 45               // 下注窗口 45s
	roundInfoTTL     = 60 * time.Second // 回合信息缓存 60s（应大于下注窗口）
)

func (s *gameEventService) Handle(ctx context.Context, in GameEventInput) error {

	// 基本校验：必须有回合ID，且事件类型为 1..5
	if in.GameRoundID == "" || in.EventType < 1 || in.EventType > 5 {
		fmt.Printf("[GameEvent]  参数校验失败: round_id=%s, event_type=%d, trace_id=%s\n",
			in.GameRoundID, in.EventType, in.TraceID)
		return ErrBadRequest
	}

	// 指标：仅在输入校验通过后开始计时
	start := time.Now()
	resultLabel := "fail"
	// 将事件代码转换为状态机事件名（用于内部状态机与指标标签）
	evtStr := eventCodeToString(in.EventType)
	defer func() { metrics.RecordGameEvent(resultLabel, evtStr, start) }()

	fmt.Printf("[GameEvent] 收到事件: event=%s(%d), round_id=%s, game_id=%s, room_id=%s, trace_id=%s\n",
		evtStr, in.EventType, in.GameRoundID, in.GameID, in.RoomID, in.TraceID)

	// ========== 生产审计：事务管理 ==========
	//  高优先级：未设置事务超时
	// 问题：长时间运行的事务可能导致死锁并阻塞其他请求
	// 建议：添加上下文超时
	// txCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	// defer cancel()
	// tx, err := infmysql.SQLX().BeginTxx(txCtx, nil)
	// 原因：在生产环境中，网络问题或查询速度慢可能导致事务
	// 无限期挂起，阻塞其他用户并引发级联故障
	// ===============================================================
	tx, err := infmysql.SQLX().BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// game_start 时确保创建回合
	if evtStr == state.EvtGameStart {
		fmt.Printf("[GameEvent] game_start: 确保回合存在, round_id=%s, trace_id=%s\n",
			in.GameRoundID, in.TraceID)
		if err := model.EnsureOnStart(ctx, tx, in.GameRoundID, in.GameID, in.RoomID, in.TraceID); err != nil {
			return err
		}
	}

	prevStatus, err := model.GetStatusForUpdate(ctx, tx, in.GameRoundID)
	if err != nil {
		fmt.Printf("[GameEvent] 获取回合状态失败: round_id=%s, error=%v, trace_id=%s\n",
			in.GameRoundID, err, in.TraceID)
		return err
	}
	prev := codeToState(prevStatus)

	fmt.Printf("[GameEvent] 当前状态: state=%s(%d), round_id=%s, trace_id=%s\n",
		prev, prevStatus, in.GameRoundID, in.TraceID)

	// 计算目标状态（不处理 draw）
	nextStr, err := state.NextState(prev, evtStr)
	if err != nil {
		fmt.Printf("[GameEvent] 状态转换失败: %s --%s--> ?, round_id=%s, trace_id=%s\n",
			prev, evtStr, in.GameRoundID, in.TraceID)
		return err
	}
	nextCode := stateToCode(nextStr)

	var (
		betStartMs int64
		betStopMs  int64
	)
	// 根据事件设置相应时间戳
	switch evtStr {
	case state.EvtGameStart:
		betStartMs = time.Now().UnixMilli()
		betStopMs = betStartMs + int64(betWindowSeconds*1000)
		fmt.Printf("[GameEvent] game_start: 设置投注时间窗口, bet_start=%d, bet_stop=%d, window=%ds, round_id=%s, trace_id=%s\n",
			betStartMs, betStopMs, betWindowSeconds, in.GameRoundID, in.TraceID)
		// 同时设置 bet_start_time 和 bet_stop_time
		if err := model.SetBetTimes(ctx, tx, in.GameRoundID, betStartMs, betStopMs); err != nil {
			return err
		}
	case state.EvtGameStop:
		fmt.Printf("[GameEvent] game_stop: 封盘, round_id=%s, trace_id=%s\n",
			in.GameRoundID, in.TraceID)
		if err := model.SetBetStopNow(ctx, tx, in.GameRoundID); err != nil {
			fmt.Printf("[GameEvent] 设置封盘时间失败: round_id=%s, error=%v, trace_id=%s\n",
				in.GameRoundID, err, in.TraceID)
			return err
		}
	case state.EvtNewCard:
		fmt.Printf("[GameEvent] new_card: 发牌, round_id=%s, trace_id=%s\n",
			in.GameRoundID, in.TraceID)
	case state.EvtGameDraw:
		fmt.Printf("[GameEvent] game_draw: 准备开奖, round_id=%s, trace_id=%s\n",
			in.GameRoundID, in.TraceID)
		fmt.Printf("[GameEvent] 提示: game_draw 事件已触发，状态将变为 drawn(5)，请调用 /api/drawresult 接口输入开奖结果进行结算\n")
	case state.EvtGameEnd:
		fmt.Printf("[GameEvent] game_end: 游戏结束, round_id=%s, trace_id=%s\n",
			in.GameRoundID, in.TraceID)

		// 校验：必须确保已经调用过开奖接口（即已经有开奖结果）
		round, err := model.GetRoundForUpdate(ctx, tx, in.GameRoundID)
		if err != nil {
			fmt.Printf("[GameEvent] 获取回合信息失败: round_id=%s, error=%v, trace_id=%s\n",
				in.GameRoundID, err, in.TraceID)
			return err
		}

		// 检查是否已经有开奖结果
		if round.GameResult == 0 || round.CardList == "" {
			fmt.Printf("[GameEvent] 游戏结束失败: 当前牌局尚未开奖，不能触发游戏结束事件, round_id=%s, game_result=%d, card_list=%s, trace_id=%s\n",
				in.GameRoundID, round.GameResult, round.CardList, in.TraceID)
			return ErrGameEndWithoutDrawResult
		}

		// 检查是否已经结算
		if round.IsSettled == 0 {
			fmt.Printf("[GameEvent] 警告: 当前牌局尚未结算，建议先调用 /api/drawresult 进行结算, round_id=%s, trace_id=%s\n",
				in.GameRoundID, in.TraceID)
		}

	}

	if err := model.UpdateState(ctx, tx, in.GameRoundID, nextCode); err != nil {
		return err
	}

	// Outbox：game_start、game_draw 与 game_end 保证事务内写入
	if evtStr == state.EvtGameStart {
		payload := map[string]any{
			"event":          "game_started",
			"game_id":        in.GameID,
			"room_id":        in.RoomID,
			"game_round_id":  in.GameRoundID,
			"bet_start_time": betStartMs,
			"bet_stop_time":  betStopMs,
			"trace_id":       in.TraceID,
		}
		if err := model.CreateOutbox(ctx, tx, "game_started", in.GameRoundID, payload); err != nil {
			return err
		}
	}
	if evtStr == state.EvtGameDraw {
		payload := map[string]any{
			"event":         "game_draw_ready",
			"game_id":       in.GameID,
			"room_id":       in.RoomID,
			"game_round_id": in.GameRoundID,
			"trace_id":      in.TraceID,
		}

		if err := model.CreateOutbox(ctx, tx, "game_draw_ready", in.GameRoundID, payload); err != nil {
			return err
		}
	}
	if evtStr == state.EvtGameEnd {
		payload := map[string]any{
			"event":         "game_ended",
			"game_id":       in.GameID,
			"room_id":       in.RoomID,
			"game_round_id": in.GameRoundID,
			"trace_id":      in.TraceID,
		}
		fmt.Printf("[GameEvent] 写入 Outbox: topic=game_ended, round_id=%s, trace_id=%s\n",
			in.GameRoundID, in.TraceID)
		if err := model.CreateOutbox(ctx, tx, "game_ended", in.GameRoundID, payload); err != nil {
			fmt.Printf("[GameEvent] 写入 Outbox 失败: topic=game_ended, round_id=%s, error=%v, trace_id=%s\n",
				in.GameRoundID, err, in.TraceID)
			return err
		}
	}

	// 审计（补充 game_id/room_id 入库，满足非空约束）
	aud := &model.GameEventAudit{
		GameID:      in.GameID,
		RoomID:      in.RoomID,
		GameRoundID: in.GameRoundID,
		EventType:   in.EventType,
		PrevState:   prev,
		NextState:   nextStr,
		Operator:    "system",
		Source:      "api",
		Payload:     "{}",
		TraceID:     in.TraceID,
	}
	if err := aud.Insert(ctx, tx); err != nil {
		fmt.Printf("[GameEvent]  写入审计日志失败: round_id=%s, error=%v, trace_id=%s\n",
			in.GameRoundID, err, in.TraceID)
		return err
	}

	if err := tx.Commit(); err != nil {
		fmt.Printf("[GameEvent]  提交事务失败: round_id=%s, error=%v, trace_id=%s\n",
			in.GameRoundID, err, in.TraceID)
		return err
	}

	// 事务提交后：写/删 Redis（避免未提交数据被读取）
	if r := infrds.Client(); r != nil {
		switch evtStr {
		case state.EvtGameStart:
			val := map[string]any{
				"game_id":        in.GameID,
				"room_id":        in.RoomID,
				"game_round_id":  in.GameRoundID,
				"bet_start_time": betStartMs,
				"bet_stop_time":  betStopMs,
				"game_status":    2, // betting
			}
			if b, e := json.Marshal(val); e == nil {
				fmt.Printf("[GameEvent] 写入 Redis 缓存: key=%s, ttl=%v, round_id=%s, trace_id=%s\n",
					infrds.RoundInfoKey(in.GameRoundID), roundInfoTTL, in.GameRoundID, in.TraceID)
				_ = r.Set(ctx, infrds.RoundInfoKey(in.GameRoundID), b, roundInfoTTL).Err()
			}
		case state.EvtGameEnd:
			fmt.Printf("[GameEvent] 删除 Redis 缓存: key=%s, round_id=%s, trace_id=%s\n",
				infrds.RoundInfoKey(in.GameRoundID), in.GameRoundID, in.TraceID)
			_ = r.Del(ctx, infrds.RoundInfoKey(in.GameRoundID)).Err()
		}
	}

	resultLabel = "success"
	fmt.Printf("[GameEvent] 事件处理完成: event=%s(%d), round_id=%s, prev=%s, next=%s, trace_id=%s\n",
		evtStr, in.EventType, in.GameRoundID, prev, nextStr, in.TraceID)
	return nil
}

// 约定的状态码映射：1=init, 2=betting, 3=sealed, 4=dealt, 5=drawn, 6=settled, 7=finished
func codeToState(c int8) string {
	switch c {
	case 1:
		return state.StateInit
	case 2:
		return state.StateBetting
	case 3:
		return state.StateSealed
	case 4:
		return state.StateDealt
	case 5:
		return state.StateDrawn
	case 6:
		return state.StateSettled
	case 7:
		return state.StateFinished
	default:
		return state.StateInit
	}
}

func stateToCode(s string) int8 {
	switch s {
	case state.StateInit:
		return 1
	case state.StateBetting:
		return 2
	case state.StateSealed:
		return 3
	case state.StateDealt:
		return 4
	case state.StateDrawn:
		return 5
	case state.StateSettled:
		return 6
	case state.StateFinished:
		return 7
	default:
		return 1
	}

}

// eventCodeToString: 将数值事件代码映射为状态机事件名
func eventCodeToString(c int8) string {
	switch c {
	case 1:
		return state.EvtGameStart
	case 2:
		return state.EvtGameStop
	case 3:
		return state.EvtNewCard
	case 4:
		return state.EvtGameDraw
	case 5:
		return state.EvtGameEnd
	default:
		return ""
	}
}

var (
	ErrGameEndWithoutDrawResult = errors.New("game end not allowed: draw result not found")
)
