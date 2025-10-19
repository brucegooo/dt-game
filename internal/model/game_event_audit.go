package model

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// GameEventAudit 对应 game_event_audit 表（状态机审计）
// event_type 采用数值枚举（1=game_start 2=game_stop 3=new_card 4=game_draw 5=game_end）
// prev_state/next_state 使用字符串快照，便于直观查询
type GameEventAudit struct {
	ID int64 `db:"id"`
	// 游戏ID
	GameID string `db:"game_id"`
	// 房间ID
	RoomID string `db:"room_id"`
	// 局ID
	GameRoundID string `db:"game_round_id"`
	// 事件类型（数值：1=game_start 2=game_stop 3=new_card 4=game_draw 5=game_end）
	EventType int8   `db:"event_type"`
	PrevState string `db:"prev_state"`
	NextState string `db:"next_state"`
	Operator  string `db:"operator"`
	Source    string `db:"source"`
	Payload   string `db:"payload"`
	TraceID   string `db:"trace_id"`
	CreatedAt int64  `db:"created_at"`
}

// Insert
func (e *GameEventAudit) Insert(ctx context.Context, exec sqlx.ExtContext) error {
	now := time.Now().UnixMilli()

	sqlStr := "INSERT INTO game_event_audit (game_id, room_id, game_round_id, event_type, prev_state, next_state, operator, source, payload, trace_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	args := []interface{}{e.GameID, e.RoomID, e.GameRoundID, e.EventType, e.PrevState, e.NextState, e.Operator, e.Source, e.Payload, e.TraceID, now}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}
