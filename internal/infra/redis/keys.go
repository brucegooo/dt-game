package redis

// Redis Key 定义与构造器
// 统一管理业务使用的 Redis Key，避免散落的魔法字符串，便于统一维护与变更。

const (
	// PrefixBetIdemResult：投注幂等“结果缓存”Key 的前缀。
	// 作用：缓存某个 idempotency key 对应的第一次成功结果（BetOutput JSON），用于后续重复请求直接返回。
	PrefixBetIdemResult = "bet:idem:result:"
	// PrefixBetIdemLock：投注幂等“进行中锁”Key 的前缀。
	// 作用：使用 SETNX + TTL 标记 idempotency key 正在处理，吸收瞬时重复请求，减轻数据库压力。
	PrefixBetIdemLock = "bet:idem:lock:"

	// PrefixRoundInfo：开局信息缓存（例如下注窗口），用于前端倒计时等快速查询
	PrefixRoundInfo = "game:round:"
	// PrefixRoundResult：开奖结果缓存
	PrefixRoundResult = "game:result:"
)

// IdemResultKey：构造幂等“结果缓存”的完整 Key。
// 形如：bet:idem:result:{idempotency_key}
func IdemResultKey(k string) string { return PrefixBetIdemResult + k }

// IdemLockKey：构造幂等“进行中锁”的完整 Key。
// 形如：bet:idem:lock:{idempotency_key}
func IdemLockKey(k string) string { return PrefixBetIdemLock + k }

// RoundInfoKey：构造游戏局信息缓存 Key。形如：game:round:{round_id}
func RoundInfoKey(roundID string) string { return PrefixRoundInfo + roundID }

// RoundResultKey：构造开奖结果缓存 Key。形如：game:result:{round_id}
func RoundResultKey(roundID string) string { return PrefixRoundResult + roundID }
