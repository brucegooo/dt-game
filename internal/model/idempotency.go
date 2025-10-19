package model

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
)

// IdempotencyKey 对应 idempotency_keys 表
// 仅用于幂等插入（唯一键: idempotency_key）
type IdempotencyKey struct {
	ID             int64  `db:"id"`
	IdempotencyKey string `db:"idempotency_key"`
	Purpose        string `db:"purpose"`
	Ref            string `db:"ref"`
	CreatedAt      int64  `db:"created_at"`
}

// Insert 插入一条幂等键记录
func (k *IdempotencyKey) Insert(ctx context.Context, exec sqlx.ExtContext) error {
	now := time.Now().UnixMilli()

	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "INSERT INTO idempotency_keys (idempotency_key, purpose, ref, created_at) VALUES (?, ?, ?, ?)"
	args := []interface{}{k.IdempotencyKey, k.Purpose, k.Ref, now}

	_, err := exec.ExecContext(ctx, sqlStr, args...)
	return err
}

// CreateOutboxFromIdem 是一个便捷函数：插入幂等键并创建对应的 outbox
// 注意：调用方应在事务中调用以保证原子性
func CreateOutboxFromIdem(ctx context.Context, exec sqlx.ExtContext, idemKey, purpose, ref, topic string, payload any) error {
	// 先插入幂等键，若重复将触发唯一键冲突
	if err := (&IdempotencyKey{IdempotencyKey: idemKey, Purpose: purpose, Ref: ref}).Insert(ctx, exec); err != nil {
		return err
	}
	// 再写 outbox
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return (&Outbox{Topic: topic, BizKey: ref, Payload: string(b)}).Insert(ctx, exec)
}

// SelectRefByIdemKey 按幂等键查询 ref（例如 bill_no）
func SelectRefByIdemKey(ctx context.Context, db *sqlx.DB, key string) (string, error) {
	// 使用原生 SQL 以避免 goqu 在某些 MySQL 版本上的兼容性问题
	sqlStr := "SELECT ref FROM idempotency_keys WHERE idempotency_key = ? LIMIT 1"
	var ref string
	if err := sqlx.GetContext(ctx, db, &ref, sqlStr, key); err != nil {
		return "", err
	}
	return ref, nil
}
