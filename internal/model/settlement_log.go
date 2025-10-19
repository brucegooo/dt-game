package model

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// SettlementLog 结算日志表（防止重复结算）
type SettlementLog struct {
	ID           int64   `db:"id"`             // 自增ID
	GameRoundID  string  `db:"game_round_id"`  // 游戏回合ID
	CardList     string  `db:"card_list"`      // 牌面信息
	Result       string  `db:"result"`         // 游戏结果: dragon|tiger|tie
	TotalOrders  int     `db:"total_orders"`   // 结算订单总数
	TotalPayout  float64 `db:"total_payout"`   // 总派彩金额
	Operator     string  `db:"operator"`       // 操作人
	TraceID      string  `db:"trace_id"`       // 链路追踪ID
	CreatedAt    int64   `db:"created_at"`     // 创建时间（13位毫秒时间戳）
}

// CreateSettlementLog 创建结算日志（利用唯一索引防止重复结算）
// 如果返回唯一键冲突错误，说明该回合已经结算过
func CreateSettlementLog(ctx context.Context, exec sqlx.ExtContext, log *SettlementLog) error {
	now := time.Now().UnixMilli()
	log.CreatedAt = now

	sqlStr := `INSERT INTO settlement_log (game_round_id, card_list, result, total_orders, total_payout, operator, trace_id, created_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := exec.ExecContext(ctx, sqlStr,
		log.GameRoundID, log.CardList, log.Result, log.TotalOrders, log.TotalPayout, log.Operator, log.TraceID, log.CreatedAt)

	if err != nil {
		return err
	}

	id, _ := result.LastInsertId()
	log.ID = id

	return nil
}

// GetSettlementLog 查询结算日志
func GetSettlementLog(ctx context.Context, db *sqlx.DB, gameRoundID string) (*SettlementLog, error) {
	sqlStr := `SELECT id, game_round_id, card_list, result, total_orders, total_payout, operator, trace_id, created_at
	           FROM settlement_log WHERE game_round_id = ? LIMIT 1`

	var log SettlementLog
	if err := db.GetContext(ctx, &log, sqlStr, gameRoundID); err != nil {
		return nil, err
	}

	return &log, nil
}

