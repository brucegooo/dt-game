package model

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// Order 对应 orders 表
// 说明：金额为非负；下注/结算状态采用数值枚举（从1开始）
// bet_status: 1=创建 2=成功 3=失败
// bill_status: 1=待结算 2=已结算 3=已取消
// game_result: 0=未开奖 1=dragon 2=tiger 3=tie
type Order struct {
	BillNo         string  `db:"bill_no"`          // 注单号(主键)
	RoomID         string  `db:"room_id"`          // 房间ID
	GameRoundID    string  `db:"game_round_id"`    // 局号ID
	GameID         string  `db:"game_id"`          // 游戏ID
	UserID         int64   `db:"user_id"`          // 用户ID（内部ID）
	PlatformID     int8    `db:"platform_id"`      // 平台ID
	PlatformUserID string  `db:"platform_user_id"` // 平台用户ID
	UserName       string  `db:"user_name"`        // 用户名
	BetAmount      float64 `db:"bet_amount"`       // 下注金额(非负)
	PlayType       string  `db:"play_type"`        // 玩法: dragon|tiger|tie（入库为数值枚举，模型用字符串）
	BetStatus      int8    `db:"bet_status"`       // 下注状态
	BetTime        int64   `db:"bet_time"`         // 下注时间（毫秒戳由调用方维护）
	BillStatus     int8    `db:"bill_status"`      // 结算状态
	GameResult     int8    `db:"game_result"`      // 游戏结果: 0=未开奖 1=dragon 2=tiger 3=tie
	WinAmount      float64 `db:"win_amount"`       // 派彩金额
	BetOdds        float64 `db:"bet_odds"`         // 赔率
	Currency       string  `db:"currency"`         // 币种
	IdempotencyKey string  `db:"idempotency_key"`  // 幂等键
	TraceID        string  `db:"trace_id"`         // 链路追踪ID
	CreatedAt      int64   `db:"created_at"`       // 创建时间
	UpdatedAt      int64   `db:"updated_at"`       // 更新时间
}

// Insert 插入一条 Order 记录
func (o *Order) Insert(ctx context.Context, exec sqlx.ExtContext) error {
	now := time.Now().UnixMilli()
	bt := o.BetTime
	if bt == 0 {
		bt = now
	}
	playCode := toPlayTypeCode(o.PlayType)

	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := `INSERT INTO orders (bill_no, room_id, game_round_id, game_id, user_id, platform_id, platform_user_id, user_name,
		bet_amount, play_type, bet_status, bet_time, bill_status, win_amount, bet_odds, currency,
		idempotency_key, trace_id, created_at, updated_at, game_result)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := exec.ExecContext(ctx, sqlStr, o.BillNo, o.RoomID, o.GameRoundID, o.GameID, o.UserID, o.PlatformID, o.PlatformUserID, o.UserName,
		o.BetAmount, playCode, o.BetStatus, bt, o.BillStatus, o.WinAmount, o.BetOdds, o.Currency,
		o.IdempotencyKey, o.TraceID, now, now, o.GameResult)
	return err
}

// ListByRoundForUpdate 按局号查询需结算的订单（FOR UPDATE），需要在事务中调用
func ListByRoundForUpdate(ctx context.Context, exec sqlx.ExtContext, roundID string) ([]Order, error) {
	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := `SELECT bill_no, user_id, user_name, bet_amount, play_type, bet_odds, currency
		FROM orders WHERE game_round_id = ? AND bill_status = 1 AND bet_status = 2 FOR UPDATE`

	// 使用中间投影结构接收数值型 play_type，然后映射回字符串
	type row struct {
		BillNo    string  `db:"bill_no"`
		UserID    int64   `db:"user_id"`
		UserName  string  `db:"user_name"`
		BetAmount float64 `db:"bet_amount"`
		PlayCode  int8    `db:"play_type"`
		BetOdds   float64 `db:"bet_odds"`
		Currency  string  `db:"currency"`
	}
	var rs []row
	if err := sqlx.SelectContext(ctx, exec, &rs, sqlStr, roundID); err != nil {
		return nil, err
	}
	out := make([]Order, 0, len(rs))
	for _, r := range rs {
		out = append(out, Order{
			BillNo:    r.BillNo,
			UserID:    r.UserID,
			UserName:  r.UserName,
			BetAmount: r.BetAmount,
			PlayType:  fromPlayTypeCode(r.PlayCode),
			BetOdds:   r.BetOdds,
			Currency:  r.Currency,
		})
	}
	return out, nil
}

// 简易的玩法映射（与仓储层保持一致）
func toPlayTypeCode(s string) int8 {
	switch s {
	case "dragon":
		return 1
	case "tiger":
		return 2
	case "tie":
		return 3
	default:
		return 0
	}
}

func fromPlayTypeCode(c int8) string {
	switch c {
	case 1:
		return "dragon"
	case 2:
		return "tiger"
	case 3:
		return "tie"
	default:
		return ""
	}
}

// UpdateSettlement 更新订单的派彩、结算状态和游戏结果
func UpdateSettlement(ctx context.Context, exec sqlx.ExtContext, billNo string, winAmount float64, billStatus int8, gameResult int8) error {
	now := time.Now().UnixMilli()

	sqlStr := "UPDATE orders SET win_amount = ?, bill_status = ?, game_result = ?, updated_at = ? WHERE bill_no = ?"
	args := []interface{}{winAmount, billStatus, gameResult, now, billNo}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// BetRecord 投注记录（用于查询接口）
type BetRecord struct {
	BillNo      string  `db:"bill_no" json:"bill_no"`             // 订单号
	GameRoundID string  `db:"game_round_id" json:"game_round_id"` // 游戏回合ID
	PlayType    int8    `db:"play_type" json:"play_type"`         // 投注类型：1=Dragon, 2=Tiger, 3=Tie
	BetAmount   float64 `db:"bet_amount" json:"bet_amount"`       // 投注金额
	BetStatus   int8    `db:"bet_status" json:"bet_status"`       // 下注状态：1=创建, 2=成功, 3=失败
	BillStatus  int8    `db:"bill_status" json:"bill_status"`     // 订单状态：1=待结算, 2=已结算, 3=已取消
	GameResult  int8    `db:"game_result" json:"game_result"`     // 游戏结果：0=未开奖, 1=Dragon, 2=Tiger, 3=Tie
	WinAmount   float64 `db:"win_amount" json:"win_amount"`       // 盈亏金额
	BetOdds     float64 `db:"bet_odds" json:"bet_odds"`           // 赔率
	BetTime     int64   `db:"bet_time" json:"bet_time"`           // 投注时间（毫秒时间戳）
	CreatedAt   int64   `db:"created_at" json:"created_at"`       // 创建时间（毫秒时间戳）
	UpdatedAt   int64   `db:"updated_at" json:"updated_at"`       // 更新时间（毫秒时间戳）
}

// ListUserBets 查询用户的投注记录（按平台用户ID查询）
// 参数：
//   - platformID: 平台ID
//   - platformUserID: 平台用户ID
//   - gameRoundID: 游戏回合ID（可选，为空则查询所有）
//   - limit: 返回记录数量（默认 10）
func ListUserBets(ctx context.Context, db *sqlx.DB, platformID int8, platformUserID string, gameRoundID string, limit int) ([]BetRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100 // 最多返回 100 条
	}

	var sqlStr string
	var args []interface{}

	if gameRoundID != "" {
		// 查询指定回合的投注记录
		sqlStr = `SELECT bill_no, game_round_id, play_type, bet_amount, bet_status, bill_status,
			game_result, win_amount, bet_odds, bet_time, created_at, updated_at
			FROM orders
			WHERE platform_id = ? AND platform_user_id = ? AND game_round_id = ?
			ORDER BY bet_time DESC
			LIMIT ?`
		args = []interface{}{platformID, platformUserID, gameRoundID, limit}
	} else {
		// 查询所有投注记录
		sqlStr = `SELECT bill_no, game_round_id, play_type, bet_amount, bet_status, bill_status,
			game_result, win_amount, bet_odds, bet_time, created_at, updated_at
			FROM orders
			WHERE platform_id = ? AND platform_user_id = ?
			ORDER BY bet_time DESC
			LIMIT ?`
		args = []interface{}{platformID, platformUserID, limit}
	}

	var records []BetRecord
	if err := db.SelectContext(ctx, &records, sqlStr, args...); err != nil {
		return nil, err
	}

	return records, nil
}
