package model

import (
	"context"
	"time"

	log "dt-server/common/logger"

	"github.com/jmoiron/sqlx"
)

// Customer 对应 customers 表
// 说明：所有时间为毫秒级时间戳在 Repo 层转换为 time.Time
// 金额使用 DECIMAL(18,2) 存储，Go 层以 float64 表示；数据库已做 UNSIGNED 约束
// status: 1=启用 2=禁用
type Customer struct {
	UserID    int64     `db:"user_id"`    // 用户ID(主键)
	Username  string    `db:"username"`   // 用户名
	Balance   float64   `db:"balance"`    // 余额（非负）
	Status    int8      `db:"status"`     // 用户状态 1=启用 2=禁用
	CreatedAt time.Time `db:"created_at"` // 创建时间
	UpdatedAt time.Time `db:"updated_at"` // 更新时间
}

// GetForUpdate 按 user_id 加锁查询（FOR UPDATE），请在事务中调用
func GetForUpdate(ctx context.Context, exec sqlx.ExtContext, userID int64) (*Customer, error) {
	sqlStr := "SELECT user_id, username, balance, status, created_at, updated_at FROM customers WHERE user_id = ? FOR UPDATE"

	type row struct {
		UserID    int64   `db:"user_id"`
		Username  string  `db:"username"`
		Balance   float64 `db:"balance"`
		Status    int8    `db:"status"`
		CreatedAt int64   `db:"created_at"`
		UpdatedAt int64   `db:"updated_at"`
	}
	var r row
	if err := sqlx.GetContext(ctx, exec, &r, sqlStr, userID); err != nil {
		return nil, err
	}
	return &Customer{
		UserID:    r.UserID,
		Username:  r.Username,
		Balance:   r.Balance,
		Status:    r.Status,
		CreatedAt: time.UnixMilli(r.CreatedAt),
		UpdatedAt: time.UnixMilli(r.UpdatedAt),
	}, nil
}

// UpdateAmount 更新客户余额
func UpdateAmount(ctx context.Context, exec sqlx.ExtContext, userID int64, newAmount float64) error {
	now := time.Now().UnixMilli()

	sqlStr := "UPDATE customers SET balance = ?, updated_at = ? WHERE user_id = ?"
	args := []interface{}{newAmount, now, userID}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// GetAmount 非锁查询余额（用于幂等冲突后的回补读取）
func GetAmount(ctx context.Context, db *sqlx.DB, userID int64) (float64, error) {
	sqlStr := "SELECT balance FROM customers WHERE user_id = ? LIMIT 1"
	var balance float64
	if err := db.GetContext(ctx, &balance, sqlStr, userID); err != nil {
		return 0, err
	}
	return balance, nil
}

// GetCustomer 非锁查询用户信息（用于查询接口）
func GetCustomer(ctx context.Context, db *sqlx.DB, userID int64) (*Customer, error) {
	sqlStr := "SELECT user_id, username, balance, status, created_at, updated_at FROM customers WHERE user_id = ? LIMIT 1"

	type row struct {
		UserID    int64   `db:"user_id"`
		Username  string  `db:"username"`
		Balance   float64 `db:"balance"`
		Status    int8    `db:"status"`
		CreatedAt int64   `db:"created_at"`
		UpdatedAt int64   `db:"updated_at"`
	}
	var r row
	if err := db.GetContext(ctx, &r, sqlStr, userID); err != nil {
		log.Error("[GetCustomer] query failed: " + err.Error())
		return nil, err
	}
	return &Customer{
		UserID:    r.UserID,
		Username:  r.Username,
		Balance:   r.Balance,
		Status:    r.Status,
		CreatedAt: time.UnixMilli(r.CreatedAt),
		UpdatedAt: time.UnixMilli(r.UpdatedAt),
	}, nil
}
