package service

import "context"

// RoundService 负责单局状态推进、开奖与结算(幂等)
type RoundService interface {
	// SettleRound 结算单局：幂等(按game_round_id)。
	// 1) 校验状态: 仅game_draw~game_end间可执行
	// 2) 原子更新注单&账本(事务)
	// 3) Outbox投递MQ
	// 4) 标记结算完成
	SettleRound(ctx context.Context, roundID string) error
}

