package model

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// WalletLedger 对应 wallet_ledger 表（追加式账本）
// 说明：金额为非负；方向由 before_amount/after_amount 与 biz_type 推导
// biz_type: 1=bet 下注 2=settle 结算 3=refund 退款 4=adjust 后台调整
// 同时冗余 biz_type_str 便于查询
type WalletLedger struct {
	ID           int64   `db:"id"`
	UserID       int64   `db:"user_id"`
	BizType      int     `db:"biz_type"`
	BizTypeStr   string  `db:"biz_type_str"`
	Amount       float64 `db:"amount"`
	BeforeAmount float64 `db:"before_amount"`
	AfterAmount  float64 `db:"after_amount"`
	Currency     string  `db:"currency"`
	BillNo       string  `db:"bill_no"`
	GameRoundID  string  `db:"game_round_id"`
	GameID       string  `db:"game_id"`
	RoomID       string  `db:"room_id"`
	Remark       string  `db:"remark"`
	TraceID      string  `db:"trace_id"`
	CreatedAt    int64   `db:"created_at"`
}

// Insert 新增一条账本记录（biz_type 数值码与字符串双写）
func (l *WalletLedger) Insert(ctx context.Context, exec sqlx.ExtContext) error {
	now := time.Now().UnixMilli()
	code := l.BizType
	str := l.BizTypeStr
	if code == 0 && str != "" {
		switch strings.ToLower(str) {
		case "bet":
			code = 1
		case "settle":
			code = 2
		case "refund":
			code = 3
		case "adjust":
			code = 4
		}
	}
	if str == "" && code != 0 {
		switch code {
		case 1:
			str = "bet"
		case 2:
			str = "settle"
		case 3:
			str = "refund"
		case 4:
			str = "adjust"
		}
	}
	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "INSERT INTO wallet_ledger (user_id, biz_type, biz_type_str, amount, before_amount, after_amount, currency, bill_no, game_round_id, game_id, room_id, remark, trace_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	args := []interface{}{l.UserID, code, str, l.Amount, l.BeforeAmount, l.AfterAmount, l.Currency, l.BillNo, l.GameRoundID, l.GameID, l.RoomID, l.Remark, l.TraceID, now}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}
