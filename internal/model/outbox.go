package model

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
)

// Outbox 对应 outbox 表（事务消息表）
// status: 1=待发送 2=已发送 3=失败
// 说明：业务通常按行读取轻量投递所需字段，可使用 OutboxRow 投影类型
type Outbox struct {
	ID         int64  `db:"id"`          // 自增ID
	Topic      string `db:"topic"`       // 主题
	BizKey     string `db:"biz_key"`     // 业务键（去重/幂等用）
	Payload    string `db:"payload"`     // 消息体(JSON字符串)
	Status     int8   `db:"status"`      // 状态
	RetryCount int    `db:"retry_count"` // 重试次数
	LastError  string `db:"last_error"`  // 最后一次错误
	CreatedAt  int64  `db:"created_at"`  // 创建时间
	UpdatedAt  int64  `db:"updated_at"`  // 更新时间
}

// Insert 插入一条 Outbox 记录（状态默认 1）
func (o *Outbox) Insert(ctx context.Context, exec sqlx.ExtContext) error {
	now := time.Now().UnixMilli()

	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "INSERT INTO outbox (topic, biz_key, payload, status, retry_count, last_error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	args := []interface{}{o.Topic, o.BizKey, o.Payload, 1, 0, "", now, now}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// OutboxRow 是调度器扫描用的轻量投影
type OutboxRow struct {
	ID      int64  `db:"id"`      // 自增ID
	Topic   string `db:"topic"`   // 主题
	BizKey  string `db:"biz_key"` // 业务键
	Payload string `db:"payload"` // 消息体
}

// ListOutboxPending 查询待发送的 outbox 轻量投影
// 只查询 status=1 且 retry_count < 10 的记录（避免无限重试）
func ListOutboxPending(ctx context.Context, exec sqlx.ExtContext, limit int) ([]OutboxRow, error) {
	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "SELECT id, topic, biz_key, payload FROM outbox WHERE status = ? AND retry_count < ? ORDER BY id ASC LIMIT ?"
	args := []interface{}{1, 10, limit}

	var list []OutboxRow
	if err := sqlx.SelectContext(ctx, exec, &list, sqlStr, args...); err != nil {
		return nil, err
	}
	return list, nil
}

// MarkOutboxSent 标记一条 Outbox 为已发送
func MarkOutboxSent(ctx context.Context, exec sqlx.ExtContext, id int64) error {
	now := time.Now().UnixMilli()

	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "UPDATE outbox SET status = ?, updated_at = ? WHERE id = ?"
	args := []interface{}{2, now, id}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// MarkOutboxFailed 标记一条 Outbox 为失败并记录最后错误
// 如果 retry_count >= 9（即将达到 10 次），则标记为永久失败（status=3）
// 否则保持 status=1 以便继续重试
func MarkOutboxFailed(ctx context.Context, exec sqlx.ExtContext, id int64, lastError string) error {
	now := time.Now().UnixMilli()

	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	// 使用 CASE WHEN 根据 retry_count 决定 status
	sqlStr := "UPDATE outbox SET status = CASE WHEN retry_count >= 9 THEN 3 ELSE 1 END, last_error = ?, retry_count = retry_count + 1, updated_at = ? WHERE id = ?"
	args := []interface{}{lastError, now, id}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// CreateOutbox creates and inserts an outbox record from topic, bizKey and payload(any)
func CreateOutbox(ctx context.Context, exec sqlx.ExtContext, topic, bizKey string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	o := &Outbox{Topic: topic, BizKey: bizKey, Payload: string(b)}
	return o.Insert(ctx, exec)
}
