package model

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// GameRoundInfo 对应 game_round_info 表
// 说明：时间为毫秒时间戳在 Repo 层转换；结果采用“数值码+冗余字符串”双写
// game_status: 1=初始 2=下注中 3=封盘 4=已发牌 5=已开奖 6=已结算 7=已结束
// game_result: 0=未设置 1=dragon 2=tiger 3=tie
// is_settled: 0=未结算 1=已结算（防止重复结算）
type GameRoundInfo struct {
	ID            int64  `db:"id"`
	GameRoundID   string `db:"game_round_id"`
	GameID        string `db:"game_id"`
	RoomID        string `db:"room_id"`
	BetStartTime  int64  `db:"bet_start_time"`
	BetStopTime   int64  `db:"bet_stop_time"`
	GameDrawTime  int64  `db:"game_draw_time"`
	CardList      string `db:"card_list"`
	GameResult    int8   `db:"game_result"`
	GameResultStr string `db:"game_result_str"`
	GameStatus    int8   `db:"game_status"`
	IsSettled     int8   `db:"is_settled"` // 是否已结算: 0=未结算 1=已结算
	TraceID       string `db:"trace_id"`
	CreatedAt     int64  `db:"created_at"`
	UpdatedAt     int64  `db:"updated_at"`
}

// EnsureOnStart
func EnsureOnStart(ctx context.Context, exec sqlx.ExtContext, roundID, gameID, roomID, traceID string) error {
	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题

	// 1. 检查回合是否已存在
	var cnt int
	sqlCheck := "SELECT COUNT(1) FROM game_round_info WHERE game_round_id = ?"
	if err := sqlx.GetContext(ctx, exec, &cnt, sqlCheck, roundID); err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}

	// 2. 插入新回合
	now := time.Now().UnixMilli()
	sqlIns := "INSERT INTO game_round_info (game_round_id, game_id, room_id, game_status, trace_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)"
	_, err := exec.ExecContext(ctx, sqlIns, roundID, gameID, roomID, 1, traceID, now, now)
	return err
}

// UpdateDraw
func UpdateDraw(ctx context.Context, exec sqlx.ExtContext, roundID, cardListJSON, resultStr string, status int8) error {
	resCode := toPlayTypeCode(resultStr) // 1=dragon 2=tiger 3=tie
	now := time.Now().UnixMilli()

	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "UPDATE game_round_info SET card_list = ?, game_result = ?, game_result_str = ?, game_status = ?, game_draw_time = ?, updated_at = ? WHERE game_round_id = ?"
	args := []interface{}{cardListJSON, resCode, resultStr, status, now, now, roundID}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// GetStatusForUpdate 在事务中按回合ID加锁并返回当前状态码
func GetStatusForUpdate(ctx context.Context, exec sqlx.ExtContext, roundID string) (int8, error) {
	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "SELECT game_status FROM game_round_info WHERE game_round_id = ? FOR UPDATE"
	var status int8
	if err := sqlx.GetContext(ctx, exec, &status, sqlStr, roundID); err != nil {
		return 0, err
	}
	return status, nil
}

// GetSettlementStatusForUpdate 在事务中按回合ID加锁并返回结算状态
// 返回值: (game_status, is_settled, error)
func GetSettlementStatusForUpdate(ctx context.Context, exec sqlx.ExtContext, roundID string) (int8, int8, error) {
	sqlStr := "SELECT game_status, is_settled FROM game_round_info WHERE game_round_id = ? FOR UPDATE"

	type result struct {
		GameStatus int8 `db:"game_status"`
		IsSettled  int8 `db:"is_settled"`
	}

	var r result
	if err := sqlx.GetContext(ctx, exec, &r, sqlStr, roundID); err != nil {
		return 0, 0, err
	}
	return r.GameStatus, r.IsSettled, nil
}

// MarkAsSettled 标记回合为已结算
func MarkAsSettled(ctx context.Context, exec sqlx.ExtContext, roundID string) error {
	now := time.Now().UnixMilli()
	sqlStr := "UPDATE game_round_info SET is_settled = 1, game_status = 6, updated_at = ? WHERE game_round_id = ?"
	_, err := exec.ExecContext(ctx, sqlStr, now, roundID)
	return err
}

// GetRoundInfo 获取回合信息（不加锁）
func GetRoundInfo(ctx context.Context, exec sqlx.ExtContext, roundID string) (*GameRoundInfo, error) {
	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := `SELECT id, game_round_id, game_id, room_id, bet_start_time, bet_stop_time,
		game_draw_time, card_list, game_result, game_result_str, game_status,
		trace_id, created_at, updated_at
		FROM game_round_info WHERE game_round_id = ?`
	var round GameRoundInfo
	if err := sqlx.GetContext(ctx, exec, &round, sqlStr, roundID); err != nil {
		return nil, err
	}
	return &round, nil
}

// GetRoundForUpdate 获取回合信息并加锁（用于投注时校验时间窗口）
func GetRoundForUpdate(ctx context.Context, exec sqlx.ExtContext, roundID string) (*GameRoundInfo, error) {
	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := `SELECT id, game_round_id, game_id, room_id, bet_start_time, bet_stop_time,
		game_draw_time, card_list, game_result, game_result_str, game_status, is_settled,
		trace_id, created_at, updated_at
		FROM game_round_info WHERE game_round_id = ? FOR UPDATE`
	var round GameRoundInfo
	if err := sqlx.GetContext(ctx, exec, &round, sqlStr, roundID); err != nil {
		return nil, err
	}
	return &round, nil
}

// UpdateState 更新回合状态
func UpdateState(ctx context.Context, exec sqlx.ExtContext, roundID string, newStatus int8) error {
	now := time.Now().UnixMilli()

	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "UPDATE game_round_info SET game_status = ?, updated_at = ? WHERE game_round_id = ?"
	args := []interface{}{newStatus, now, roundID}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// SetBetStartNow 将 bet_start_time 设置为当前时间（毫秒）
func SetBetStartNow(ctx context.Context, exec sqlx.ExtContext, roundID string) error {
	now := time.Now().UnixMilli()

	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "UPDATE game_round_info SET bet_start_time = ?, updated_at = ? WHERE game_round_id = ?"
	args := []interface{}{now, now, roundID}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// SetBetTimes 同时设置 bet_start_time 和 bet_stop_time（用于 game_start 事件）
func SetBetTimes(ctx context.Context, exec sqlx.ExtContext, roundID string, betStartMs, betStopMs int64) error {
	now := time.Now().UnixMilli()

	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "UPDATE game_round_info SET bet_start_time = ?, bet_stop_time = ?, updated_at = ? WHERE game_round_id = ?"
	args := []interface{}{betStartMs, betStopMs, now, roundID}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// SetBetStopNow 将 bet_stop_time 设置为当前时间（毫秒）
func SetBetStopNow(ctx context.Context, exec sqlx.ExtContext, roundID string) error {
	now := time.Now().UnixMilli()

	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "UPDATE game_round_info SET bet_stop_time = ?, updated_at = ? WHERE game_round_id = ?"
	args := []interface{}{now, now, roundID}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// RoundSnapshot 提供 GET 接口所需的最小字段集合
type RoundSnapshot struct {
	GameRoundID   string `db:"game_round_id"`
	GameID        string `db:"game_id"`
	RoomID        string `db:"room_id"`
	BetStartTime  int64  `db:"bet_start_time"`
	BetStopTime   int64  `db:"bet_stop_time"`
	GameDrawTime  int64  `db:"game_draw_time"`
	CardList      string `db:"card_list"`
	GameResultStr string `db:"game_result_str"`
	GameStatus    int8   `db:"game_status"`
}

// GetRoundSnapshot 按回合ID查询所需字段（无锁读取）
func GetRoundSnapshot(ctx context.Context, exec sqlx.ExtContext, roundID string) (*RoundSnapshot, error) {
	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := `SELECT game_round_id, game_id, room_id, bet_start_time, bet_stop_time,
		game_draw_time, card_list, game_result_str, game_status
		FROM game_round_info WHERE game_round_id = ?`
	var rs RoundSnapshot
	if err := sqlx.GetContext(ctx, exec, &rs, sqlStr, roundID); err != nil {
		return nil, err
	}
	return &rs, nil
}
