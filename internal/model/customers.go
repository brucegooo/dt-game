package model

import (
	"context"
	"database/sql"
	"time"

	"dt-server/common/logger"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// Customers 用户表
// 用户唯一标识 = platform_id + platform_user_id
type Customers struct {
	ID             int64   `db:"user_id"`          // 自增ID（内部使用）
	PlatformID     int8    `db:"platform_id"`      // 平台ID
	PlatformUserID string  `db:"platform_user_id"` // 平台用户ID
	Username       string  `db:"username"`         // 用户名（可选）
	Balance        float64 `db:"balance"`          // 余额
	Status         int8    `db:"status"`           // 状态: 1=正常 0=禁用
	CreatedAt      int64   `db:"created_at"`       // 创建时间（13位毫秒时间戳）
	UpdatedAt      int64   `db:"updated_at"`       // 更新时间（13位毫秒时间戳）
}

// GetUserByPlatformUser 根据平台ID和平台用户ID查询用户
func GetUserByPlatformUser(ctx context.Context, db *sqlx.DB, platformID int8, platformUserID string) (*Customers, error) {
	query := `SELECT user_id, platform_id, platform_user_id, username, balance, status, created_at, updated_at
	          FROM customers
	          WHERE platform_id = ? AND platform_user_id = ?
	          LIMIT 1`

	var user Customers
	err := db.GetContext(ctx, &user, query, platformID, platformUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		logger.Error("get user by platform user failed",
			zap.Int8("platform_id", platformID),
			zap.String("platform_user_id", platformUserID),
			zap.Error(err))
		return nil, err
	}

	return &user, nil
}

// GetUserByPlatformUserForUpdate 根据平台ID和平台用户ID查询用户（加锁）
// 必须在事务中调用
func GetUserByPlatformUserForUpdate(ctx context.Context, exec sqlx.ExtContext, platformID int8, platformUserID string) (*Customers, error) {
	query := `SELECT user_id, platform_id, platform_user_id, username, balance, status, created_at, updated_at
	          FROM customers
	          WHERE platform_id = ? AND platform_user_id = ?
	          FOR UPDATE`

	var user Customers
	err := sqlx.GetContext(ctx, exec, &user, query, platformID, platformUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		logger.Error("get user by platform user for update failed",
			zap.Int8("platform_id", platformID),
			zap.String("platform_user_id", platformUserID),
			zap.Error(err))
		return nil, err
	}

	return &user, nil
}

// GetUserByID 根据内部ID查询用户
func GetUserByID(ctx context.Context, db *sqlx.DB, userID int64) (*Customers, error) {
	query := `SELECT user_id, platform_id, platform_user_id, username, balance, status, created_at, updated_at
	          FROM customers
	          WHERE user_id = ?
	          LIMIT 1`

	var user Customers
	err := db.GetContext(ctx, &user, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		logger.Error("get user by id failed",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return nil, err
	}

	return &user, nil
}

// GetUserByIDForUpdate 根据内部ID查询用户（加锁）
// 必须在事务中调用
func GetUserByIDForUpdate(ctx context.Context, exec sqlx.ExtContext, userID int64) (*Customers, error) {
	query := `SELECT user_id, platform_id, platform_user_id, username, balance, status, created_at, updated_at
	          FROM customers
	          WHERE user_id = ?
	          FOR UPDATE`

	var user Customers
	err := sqlx.GetContext(ctx, exec, &user, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		logger.Error("get user by id for update failed",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return nil, err
	}

	return &user, nil
}

// Insert 插入用户
func (u *Customers) Insert(ctx context.Context, db *sqlx.DB) error {
	now := getCurrentMillis() // 13位毫秒时间戳
	u.CreatedAt = now
	u.UpdatedAt = now

	query := `INSERT INTO customers (platform_id, platform_user_id, username, balance, status, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`

	result, err := db.ExecContext(ctx, query,
		u.PlatformID, u.PlatformUserID, u.Username, u.Balance, u.Status, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		logger.Error("insert user failed",
			zap.Int8("platform_id", u.PlatformID),
			zap.String("platform_user_id", u.PlatformUserID),
			zap.Error(err))
		return err
	}

	id, _ := result.LastInsertId()
	u.ID = id

	logger.Info("user created",
		zap.Int64("id", u.ID),
		zap.Int8("platform_id", u.PlatformID),
		zap.String("platform_user_id", u.PlatformUserID),
		zap.String("username", u.Username))

	return nil
}

// UpdateBalance 更新用户余额
func UpdateUserBalance(ctx context.Context, exec sqlx.ExtContext, userID int64, newBalance float64) error {
	now := getCurrentMillis() // 13位毫秒时间戳
	query := `UPDATE customers SET balance = ?, updated_at = ? WHERE user_id = ?`

	_, err := exec.ExecContext(ctx, query, newBalance, now, userID)
	if err != nil {
		logger.Error("update user balance failed",
			zap.Int64("user_id", userID),
			zap.Float64("new_balance", newBalance),
			zap.Error(err))
		return err
	}

	return nil
}

// GetOrCreateUser 获取或创建用户（自动注册）
// 如果用户不存在，自动创建；如果存在，返回现有用户
func GetOrCreateUser(ctx context.Context, db *sqlx.DB, platformID int8, platformUserID, username string) (*Customers, error) {
	// 1. 先查询用户是否存在
	user, err := GetUserByPlatformUser(ctx, db, platformID, platformUserID)
	if err == nil {
		return user, nil // 用户已存在
	}

	// 2. 用户不存在，自动创建
	if err == sql.ErrNoRows {
		newUser := &Customers{
			PlatformID:     platformID,
			PlatformUserID: platformUserID,
			Username:       username,
			Balance:        0.00, // 初始余额
			Status:         1,    // 正常状态
		}

		if err := newUser.Insert(ctx, db); err != nil {
			// 处理并发创建的情况（唯一索引冲突）
			// 如果是唯一索引冲突，重新查询
			if isMySQLDuplicateKeyError(err) {
				logger.Info("concurrent user creation detected, retry query",
					zap.Int8("platform_id", platformID),
					zap.String("platform_user_id", platformUserID))
				return GetUserByPlatformUser(ctx, db, platformID, platformUserID)
			}
			return nil, err
		}

		return newUser, nil
	}

	return nil, err
}

// isMySQLDuplicateKeyError 判断是否为 MySQL 唯一键冲突错误
func isMySQLDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// MySQL 错误码 1062: Duplicate entry
	return err.Error() != "" && (err.Error() == "Error 1062" ||
		err.Error() == "Error 1062: Duplicate entry" ||
		err.Error() == "Duplicate entry")
}

// GetUserBalance 获取用户余额（非锁查询）
func GetUserBalance(ctx context.Context, db *sqlx.DB, platformID int8, platformUserID string) (float64, error) {
	query := `SELECT balance FROM customers WHERE platform_id = ? AND platform_user_id = ? LIMIT 1`

	var balance float64
	err := db.GetContext(ctx, &balance, query, platformID, platformUserID)
	if err != nil {
		logger.Error("get user balance failed",
			zap.Int8("platform_id", platformID),
			zap.String("platform_user_id", platformUserID),
			zap.Error(err))
		return 0, err
	}

	return balance, nil
}

// getCurrentMillis 获取当前13位毫秒时间戳
func getCurrentMillis() int64 {
	return time.Now().UnixMilli()
}
