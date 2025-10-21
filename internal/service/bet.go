package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	chelper "dt-server/common/helper"
	infmysql "dt-server/internal/infra/mysql"
	infrds "dt-server/internal/infra/redis"
	"dt-server/internal/metrics"
	"dt-server/internal/model"
	"dt-server/internal/state"

	mysqlerr "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	decimal "github.com/shopspring/decimal"
)

// 处理投注业务逻辑
const (
	BIZ_TYPE_BET = 1
)

// BetInput 输入参数
// 所有字段均为必填
type BetInput struct {
	GameID           string
	RoomID           string
	GameRoundID      string
	PlatformID       int8   // 平台ID
	PlatformUserID   string // 平台用户ID
	PlatformUserName string // 平台用户名（可选）
	BetAmount        string
	PlayType         int // 1dragon|2tiger|3tie（API层为int）
	IdempotencyKey   string
	TraceID          string
}

type BetOutput struct {
	BillNo       string
	RemainAmount string // 剩余金额
}

type BetService interface {
	PlaceBet(ctx context.Context, in BetInput) (*BetOutput, error)
}

type betService struct{}

func NewBetService() BetService { return &betService{} }

const (
	// Redis 进行中锁 TTL：建议小于最短投注窗口，避免长时间阻塞重复请求
	// idemLockTTL = 15 * time.Second
	idemLockTTL = 45 * time.Second
	// 结果缓存 TTL：用于重复请求直接返回第一次成功结果；应覆盖到大多数“短时重试”窗口
	idemResultTTL = 1 * time.Minute
)

// 默认事务超时时间，防止长事务占用资源影响并发（若上游已有 deadline，则沿用上游）
const defaultTxTimeout = 3 * time.Second

// Redis key 构造见 internal/infra/redis/keys.go
var (
	ErrDuplicateInFlight    = errors.New("duplicate request in flight")
	ErrInvalidStateBet      = errors.New("bet not allowed in current state")
	ErrBetWindowNotStart    = errors.New("bet window not started")
	ErrBetWindowClosed      = errors.New("bet window closed")
	ErrConflictingPlayTypes = errors.New("cannot bet on both dragon and tiger in the same round")
)

// PlaceBet 处理下注主流程：
// 下注逻辑
func (s *betService) PlaceBet(ctx context.Context, in BetInput) (*BetOutput, error) {

	start := time.Now()
	result := "fail"

	// ========== 投注金额解析和验证==========
	// 1. 解析金额字符串
	// 2. 验证金额为正数
	// 3. 验证最小投注限制
	// 4. 验证最大投注限制
	// ================================================

	// 解析投注金额
	amtDec, err := decimal.NewFromString(strings.TrimSpace(in.BetAmount))
	if err != nil {
		fmt.Printf("[Bet]  无效的投注金额格式: bet_amount=%s, error=%v, trace_id=%s\n",
			in.BetAmount, err, in.TraceID)
		return nil, errors.New("invalid bet amount format")
	}

	// 验证金额必须大于0
	if amtDec.LessThanOrEqual(decimal.Zero) {
		fmt.Printf("[Bet]  投注金额必须大于0: bet_amount=%s, trace_id=%s\n",
			in.BetAmount, in.TraceID)
		return nil, errors.New("bet amount must be positive")
	}

	// 验证最小投注限制（0.01）
	minBet := decimal.NewFromFloat(0.01)
	if amtDec.LessThan(minBet) {
		fmt.Printf("[Bet]  投注金额低于最小限制: bet_amount=%s, min=%s, trace_id=%s\n",
			in.BetAmount, minBet.String(), in.TraceID)
		return nil, fmt.Errorf("bet amount below minimum limit: %s", minBet.String())
	}

	// 验证最大投注限制（1,000,000）
	maxBet := decimal.NewFromInt(1000000)
	if amtDec.GreaterThan(maxBet) {
		fmt.Printf("[Bet]  投注金额超过最大限制: bet_amount=%s, max=%s, trace_id=%s\n",
			in.BetAmount, maxBet.String(), in.TraceID)
		return nil, fmt.Errorf("bet amount exceeds maximum limit: %s", maxBet.String())
	}

	ptMap := map[int]string{1: "dragon", 2: "tiger", 3: "tie"}
	ptStr := ptMap[in.PlayType]
	defer func() { metrics.RecordBet(result, ptStr, start) }()

	// 打印接收到的投注请求
	fmt.Printf("[Bet]  收到投注请求: round_id=%s, platform_id=%d, platform_user_id=%s, amount=%s, play_type=%d(%s), idem_key=%s, trace_id=%s\n",
		in.GameRoundID, in.PlatformID, in.PlatformUserID, in.BetAmount, in.PlayType, ptStr, in.IdempotencyKey, in.TraceID)

	// Redis 快路径：若已有结果缓存，直接返回
	if r := infrds.Client(); r != nil {
		if bs, _ := r.Get(ctx, infrds.IdemResultKey(in.IdempotencyKey)).Bytes(); len(bs) > 0 {
			var out BetOutput
			if json.Unmarshal(bs, &out) == nil {
				fmt.Printf("[Bet]  Redis 缓存命中: idem_key=%s, bill_no=%s, trace_id=%s\n",
					in.IdempotencyKey, out.BillNo, in.TraceID)
				return &out, nil
			}
		}
		// ========== 分布式锁实现==========
		// 1. 生成唯一锁值（UUID）防止误删其他请求的锁
		// 2. 使用 SetNX 获取锁
		// 3. 使用 Lua 脚本原子释放（仅当锁值匹配时删除）
		// 4. 记录锁释放失败的情况用于监控
		// ================================================

		// 生成唯一锁值，防止误删其他请求的锁
		lockValue := uuid.New().String()
		lockKey := infrds.IdemLockKey(in.IdempotencyKey)

		// 进行中锁，吸收瞬时重复
		ok, _ := r.SetNX(ctx, lockKey, lockValue, idemLockTTL).Result()
		if !ok {
			// 检查是否有缓存的结果
			if bs, _ := r.Get(ctx, infrds.IdemResultKey(in.IdempotencyKey)).Bytes(); len(bs) > 0 {
				var out BetOutput
				if json.Unmarshal(bs, &out) == nil {
					fmt.Printf("[Bet] Redis 缓存命中（重复请求）: idem_key=%s, bill_no=%s, trace_id=%s\n",
						in.IdempotencyKey, out.BillNo, in.TraceID)
					return &out, nil
				}
			}
			fmt.Printf("[Bet]  重复请求进行中: idem_key=%s, trace_id=%s\n",
				in.IdempotencyKey, in.TraceID)
			return nil, ErrDuplicateInFlight
		}

		// 使用 Lua 脚本原子释放锁（仅当锁值匹配时删除）
		defer func() {
			// Lua 脚本：只有当锁的值等于我们设置的值时才删除
			script := `
				if redis.call("get", KEYS[1]) == ARGV[1] then
					return redis.call("del", KEYS[1])
				else
					return 0
				end
			`
			result, err := r.Eval(ctx, script, []string{lockKey}, lockValue).Result()
			if err != nil {
				fmt.Printf("[Bet] 释放分布式锁失败: idem_key=%s, error=%v, trace_id=%s\n",
					in.IdempotencyKey, err, in.TraceID)
				// TODO: 记录指标用于监控 metrics.RecordLockReleaseFailure("bet", "redis_error")
			} else if result == int64(0) {
				fmt.Printf("[Bet] 分布式锁已被其他请求释放或过期: idem_key=%s, trace_id=%s\n",
					in.IdempotencyKey, in.TraceID)
				// TODO: 记录指标用于监控 metrics.RecordLockReleaseFailure("bet", "lock_mismatch")
			}
		}()
	}

	// ========== 生产环境审计：交易超时 ==========
	// 建议：将 defaultTxTimeout 移至配置文件
	// - 开发环境：10 秒（用于调试）
	// - 生产环境：3 秒（用于性能）
	// - 负载测试：1 秒（用于查找瓶颈）
	// =======================================================================

	// 开启 MySQL 事务（带默认超时，防止长事务影响并发）。
	// 若上游 ctx 已设置 deadline，则沿用；否则使用默认 defaultTxTimeout。
	txCtx := ctx
	if _, has := ctx.Deadline(); !has {
		c, cancel := context.WithTimeout(ctx, defaultTxTimeout)
		txCtx = c
		defer cancel()
	}
	tx, err := infmysql.SQLX().BeginTxx(txCtx, nil)
	if err != nil {
		fmt.Printf("[Bet] 开启事务失败: error=%v, round_id=%s, trace_id=%s\n",
			err, in.GameRoundID, in.TraceID)
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 获取或创建用户（自动注册）
	user, err := getOrCreateUserInTx(txCtx, tx, in.PlatformID, in.PlatformUserID, in.PlatformUserName)
	if err != nil {
		fmt.Printf("[Bet] 获取或创建用户失败: error=%v, platform_id=%d, platform_user_id=%s, trace_id=%s\n",
			err, in.PlatformID, in.PlatformUserID, in.TraceID)
		return nil, fmt.Errorf("failed to get or create user: %w", err)
	}

	// 获取当前赔率
	odds := calcOdds(ptStr)
	// 生成订单号（使用可读格式，使用内部用户ID）
	billNo := generateBillNo(user.ID)

	// 获取回合信息并锁定
	round, err := model.GetRoundForUpdate(txCtx, tx, in.GameRoundID)
	if err != nil {
		fmt.Printf("[Bet]  查询游戏回合失败: error=%v, round_id=%s, trace_id=%s\n",
			err, in.GameRoundID, in.TraceID)
		return nil, fmt.Errorf("failed to get round info: %w", err)
	}

	// 校验回合状态：仅在 betting 状态允许下注
	currentState := codeToState(round.GameStatus)
	if currentState != state.StateBetting {
		fmt.Printf("[Bet]  游戏状态不允许投注: current_state=%s(%d), expected=betting(2), round_id=%s, trace_id=%s\n",
			currentState, round.GameStatus, in.GameRoundID, in.TraceID)
		return nil, ErrInvalidStateBet
	}

	// 验证时间窗口
	now := time.Now().UnixMilli()
	if now < round.BetStartTime {
		fmt.Printf("[Bet] 投注窗口未开始: now=%d, bet_start=%d, round_id=%s, trace_id=%s\n",
			now, round.BetStartTime, in.GameRoundID, in.TraceID)
		return nil, ErrBetWindowNotStart
	}
	if now > round.BetStopTime {
		fmt.Printf("[Bet] 投注窗口已关闭: now=%d, bet_stop=%d, round_id=%s, trace_id=%s\n",
			now, round.BetStopTime, in.GameRoundID, in.TraceID)
		return nil, ErrBetWindowClosed
	}

	// 检查是否存在冲突的投注（同一局不能同时投注龙和虎）
	hasConflict, err := checkConflictingBets(txCtx, tx, in.GameRoundID, in.PlatformID, in.PlatformUserID, in.PlayType)
	if err != nil {
		return nil, fmt.Errorf("failed to check conflicting bets: %w", err)
	}
	if hasConflict {
		fmt.Printf("[Bet] 存在冲突投注: round_id=%s, platform_user_id=%s, play_type=%d, trace_id=%s\n",
			in.GameRoundID, in.PlatformUserID, in.PlayType, in.TraceID)
		return nil, ErrConflictingPlayTypes
	}

	// 幂等：先占幂等键，ref 记录 bill_no
	if err := (&model.IdempotencyKey{IdempotencyKey: in.IdempotencyKey, Purpose: "bet", Ref: billNo}).Insert(ctx, tx); err != nil {
		// 若幂等冲突：尝试返回上次结果
		if me, ok := err.(*mysqlerr.MySQLError); ok && me.Number == 1062 {
			fmt.Printf("[Bet]  幂等键冲突，尝试返回上次结果: idem_key=%s, trace_id=%s\n",
				in.IdempotencyKey, in.TraceID)
			_ = tx.Rollback()
			// Redis 先查
			if r := infrds.Client(); r != nil {
				if bs, _ := r.Get(ctx, infrds.IdemResultKey(in.IdempotencyKey)).Bytes(); len(bs) > 0 {
					var out BetOutput
					if json.Unmarshal(bs, &out) == nil {
						fmt.Printf("[Bet]  从 Redis 返回上次结果: bill_no=%s, trace_id=%s\n",
							out.BillNo, in.TraceID)
						return &out, nil
					}
				}
			}
			// DB 回源：根据幂等键查 bill_no，再查用户余额
			ref, e1 := model.SelectRefByIdemKey(txCtx, infmysql.SQLX(), in.IdempotencyKey)
			if e1 == nil && ref != "" {
				// 查询用户余额
				u, e2 := model.GetUserByPlatformUser(txCtx, infmysql.SQLX(), in.PlatformID, in.PlatformUserID)
				if e2 == nil {
					fmt.Printf("[Bet]  从数据库返回上次结果: bill_no=%s, trace_id=%s\n",
						ref, in.TraceID)
					return &BetOutput{BillNo: ref, RemainAmount: chelper.TrimDecimal(decimal.NewFromFloat(u.Balance))}, nil
				}
			}
		}
		fmt.Printf("[Bet]  插入幂等键失败: error=%v, idem_key=%s, trace_id=%s\n",
			err, in.IdempotencyKey, in.TraceID)
		return nil, fmt.Errorf("idempotency conflict or insert failed: %w", err)
	}

	// 校验用户状态（user 已经在事务中加锁）
	if user.Status != 1 {
		fmt.Printf("[Bet]  用户状态异常: user_id=%d, status=%d, trace_id=%s\n",
			user.ID, user.Status, in.TraceID)
		return nil, errors.New("user disabled")
	}
	// 校验余额（decimal 比较）
	if decimal.NewFromFloat(user.Balance).Cmp(amtDec) < 0 {
		return nil, errors.New("insufficient balance")
	}

	beforeDec := decimal.NewFromFloat(user.Balance)
	afterDec := beforeDec.Sub(amtDec)

	// 更新余额（两位小数）
	if err := model.UpdateUserBalance(txCtx, tx, user.ID, afterDec.Round(2).InexactFloat64()); err != nil {
		return nil, err
	}

	// 写账本，此处为扣款
	ledger := &model.WalletLedger{
		UserID:       user.ID,
		BizType:      BIZ_TYPE_BET, //1
		BizTypeStr:   "bet",        // 冗余
		Amount:       amtDec.Round(2).InexactFloat64(),
		BeforeAmount: beforeDec.Round(2).InexactFloat64(),
		AfterAmount:  afterDec.Round(2).InexactFloat64(),
		Currency:     "CNY",
		BillNo:       billNo,
		GameRoundID:  in.GameRoundID,
		GameID:       in.GameID,
		RoomID:       in.RoomID,
		Remark:       "bet deduct",
		TraceID:      in.TraceID,
	}
	if err := ledger.Insert(txCtx, tx); err != nil {
		fmt.Printf("[Bet]  写入账本失败: error=%v, bill_no=%s, trace_id=%s\n",
			err, billNo, in.TraceID)
		return nil, err
	}

	// 落注单（bet_status:2成功, bill_status:1待结算）
	ord := &model.Order{
		BillNo:         billNo,
		RoomID:         in.RoomID,
		GameRoundID:    in.GameRoundID,
		GameID:         in.GameID,
		UserID:         user.ID,
		PlatformID:     in.PlatformID,
		PlatformUserID: in.PlatformUserID,
		UserName:       user.Username,
		BetAmount:      amtDec.Round(2).InexactFloat64(),
		PlayType:       ptStr,
		BetStatus:      2,
		BillStatus:     1,
		WinAmount:      0,
		BetOdds:        odds,
		Currency:       "CNY",
		IdempotencyKey: in.IdempotencyKey,
		TraceID:        in.TraceID,
	}
	if err := ord.Insert(txCtx, tx); err != nil {
		fmt.Printf("[Bet]  创建订单失败: error=%v, bill_no=%s, trace_id=%s\n",
			err, billNo, in.TraceID)
		return nil, err
	}

	// Outbox 消息（异步）
	payload := map[string]any{
		"event":            "bet_placed",
		"bill_no":          billNo,
		"user_id":          user.ID,
		"platform_id":      in.PlatformID,
		"platform_user_id": in.PlatformUserID,
	}
	if err := model.CreateOutbox(txCtx, tx, "bet_placed", billNo, payload); err != nil {
		fmt.Printf("[Bet]  写入 Outbox 失败: error=%v, bill_no=%s, trace_id=%s\n",
			err, billNo, in.TraceID)
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		fmt.Printf("[Bet]  提交事务失败: error=%v, bill_no=%s, trace_id=%s\n",
			err, billNo, in.TraceID)
		return nil, err
	}

	result = "success"
	out := &BetOutput{BillNo: billNo, RemainAmount: chelper.TrimDecimal(afterDec)}

	// 写入 Redis 结果缓存（降级容错）
	if r := infrds.Client(); r != nil {
		if b, e := json.Marshal(out); e == nil {
			_ = r.Set(ctx, infrds.IdemResultKey(in.IdempotencyKey), b, idemResultTTL).Err()
		}
	}

	return out, nil
}

// 赔率
// 这里只是方便演示，暂时硬编码
func calcOdds(pt string) float64 {

	switch strings.ToLower(pt) {
	case "dragon":
		return 0.97
	case "tiger":
		return 0.97
	case "tie":
		return 8.0
	}
	return 0.0
}

// generateBillNo 生成可读的订单号
// 格式：DT{YYYYMMDD}{HHmmss}{UserID后4位}{随机3位十六进制}
// 示例：DT20251017143025100156A
// 优点：
//   - 可读：包含日期、时间、用户信息
//   - 有序：按时间排序
//   - 唯一：时间 + 用户 + 随机数保证唯一性
//   - 可追踪：可以从订单号看出下单时间和用户
func generateBillNo(userID int64) string {
	now := time.Now()
	// 日期时间部分：YYYYMMDD HHmmss
	dateTime := now.Format("20060102150405")
	// 用户ID后4位
	userSuffix := fmt.Sprintf("%04d", userID%10000)
	// 随机3位十六进制（0-FFF，4096种可能）
	randomBytes := make([]byte, 2)
	rand.Read(randomBytes)
	randomHex := strings.ToUpper(hex.EncodeToString(randomBytes)[:3])

	return fmt.Sprintf("DT%s%s%s", dateTime, userSuffix, randomHex)
}

// checkConflictingBets 检查用户在当前游戏轮次是否已投注冲突的玩法
// 规则：同一局游戏，一个用户不可以同时投注龙和虎
// 返回：true 表示有冲突，false 表示无冲突
func checkConflictingBets(ctx context.Context, tx *sqlx.Tx, gameRoundID string, platformID int8, platformUserID string, playType int) (bool, error) {
	// 只检查 Dragon(1) 和 Tiger(2) 的冲突
	// Tie(3) 可以与 Dragon/Tiger 共存
	if playType != 1 && playType != 2 {
		return false, nil // Tie 不检查冲突
	}

	// 查询该用户在当前游戏轮次的所有投注
	query := `
		SELECT play_type
		FROM orders
		WHERE game_round_id = ? AND platform_id = ? AND platform_user_id = ? AND bill_status IN (1, 2)
	`

	var existingPlayTypes []int
	err := tx.SelectContext(ctx, &existingPlayTypes, query, gameRoundID, platformID, platformUserID)
	if err != nil {
		return false, fmt.Errorf("failed to check existing bets: %w", err)
	}

	// 检查是否有冲突
	for _, existingType := range existingPlayTypes {
		// 如果当前要投注 Dragon(1)，但已经投注了 Tiger(2)
		if playType == 1 && existingType == 2 {
			return true, nil
		}
		// 如果当前要投注 Tiger(2)，但已经投注了 Dragon(1)
		if playType == 2 && existingType == 1 {
			return true, nil
		}
	}

	return false, nil
}

// getOrCreateUserInTx 在事务中获取或创建用户
// 如果用户不存在，自动创建；如果存在，返回现有用户并加锁
func getOrCreateUserInTx(ctx context.Context, tx *sqlx.Tx, platformID int8, platformUserID, username string) (*model.Customers, error) {
	// 1. 先尝试加锁查询
	user, err := model.GetUserByPlatformUserForUpdate(ctx, tx, platformID, platformUserID)
	if err == nil {
		return user, nil // 用户已存在
	}

	// 2. 如果用户不存在，创建用户
	if err.Error() == "sql: no rows in result set" {
		now := time.Now().UnixMilli() // 13位毫秒时间戳
		newUser := &model.Customers{
			PlatformID:     platformID,
			PlatformUserID: platformUserID,
			Username:       username,
			Balance:        0.00, // 初始余额
			Status:         1,    // 正常状态
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		// 在事务中插入
		query := `INSERT INTO customers (platform_id, platform_user_id, username, balance, status, created_at, updated_at)
		          VALUES (?, ?, ?, ?, ?, ?, ?)`
		result, err := tx.ExecContext(ctx, query,
			newUser.PlatformID, newUser.PlatformUserID, newUser.Username, newUser.Balance, newUser.Status, newUser.CreatedAt, newUser.UpdatedAt)
		if err != nil {
			// 处理并发创建的情况（唯一索引冲突）
			if me, ok := err.(*mysqlerr.MySQLError); ok && me.Number == 1062 {
				// 重新查询并加锁
				return model.GetUserByPlatformUserForUpdate(ctx, tx, platformID, platformUserID)
			}
			return nil, err
		}

		id, _ := result.LastInsertId()
		newUser.ID = id

		return newUser, nil
	}

	return nil, err
}
