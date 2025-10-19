package model

import (
"context"
"time"

"github.com/jmoiron/sqlx"
)

// Inbox 对应 inbox 表（消费幂等落库表）
// 说明：message_id+topic 可用于天然去重
type Inbox struct {
ID        int64  `db:"id"`         // 自增ID
MessageID string `db:"message_id"` // MQ 消息ID
Topic     string `db:"topic"`      // 主题
Payload   string `db:"payload"`    // 消息体(JSON字符串)
CreatedAt int64  `db:"created_at"` // 创建时间
}

// InboxRow 是消费者读取用的轻量投影
type InboxRow struct {
ID        int64  `db:"id"`         // 自增ID
MessageID string `db:"message_id"` // 消息ID
Topic     string `db:"topic"`      // 主题
Payload   string `db:"payload"`    // 消息体
}

// UpsertInbox 将消息按 message_id+topic 去重入库（存在则不变更 processed_at）
func UpsertInbox(ctx context.Context, exec sqlx.ExtContext, messageID, topic, payload string, processedAtMs int64) error {
now := time.Now().UnixMilli()

// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
sqlStr := "INSERT INTO inbox (message_id, topic, payload, processed_at, created_at) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE processed_at=processed_at"
args := []interface{}{messageID, topic, payload, processedAtMs, now}

_, err := exec.ExecContext(ctx, sqlStr, args...)
return err
}
