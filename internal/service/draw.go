package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	infmysql "dt-server/internal/infra/mysql"
	infrds "dt-server/internal/infra/redis"
	"dt-server/internal/metrics"
	"dt-server/internal/model"
	"dt-server/internal/state"

	decimal "github.com/shopspring/decimal"
)

type DrawInput struct {
	GameID      string
	RoomID      string
	GameRoundID string
	CardList    string
	TraceID     string
}

type DrawService interface {
	SubmitDrawResult(ctx context.Context, in DrawInput) error
}

type drawService struct{}

func NewDrawService() DrawService { return &drawService{} }

// SubmitDrawResult: 计算牌型结果，更新回合，结算所有订单（账本与订单），记录审计
func (s *drawService) SubmitDrawResult(ctx context.Context, in DrawInput) error {
	if in.GameRoundID == "" || len(in.CardList) == 0 {
		fmt.Printf("[DrawResult]  参数校验失败: round_id=%s, card_list=%s, trace_id=%s\n",
			in.GameRoundID, in.CardList, in.TraceID)
		return ErrBadRequest
	}

	fmt.Printf("[DrawResult] 收到开奖请求: round_id=%s, card_list=%s, game_id=%s, room_id=%s, trace_id=%s\n",
		in.GameRoundID, in.CardList, in.GameID, in.RoomID, in.TraceID)

	// 指标：在输入校验通过后开始计时
	start := time.Now()
	resultLabel := "fail"
	outcomeLabel := "unknown"
	defer func() { metrics.RecordDraw(resultLabel, outcomeLabel, start) }()

	// 计算游戏结果并验证
	// 1. 计算游戏结果
	// 2. 验证结果不为空
	// 3. 验证结果为有效值（dragon/tiger/tie）
	res := decideResult(in.CardList)
	outcomeLabel = res

	// 验证结果不为空
	if res == "" {
		return errors.New("invalid card list format: unable to determine result")
	}

	// 验证结果为有效值
	if res != "dragon" && res != "tiger" && res != "tie" {
		return fmt.Errorf("invalid game result: %s", res)
	}

	tx, err := infmysql.SQLX().BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// ========== 幂等性保护 #1: 检查结算状态 ==========
	statusCode, isSettled, err := model.GetSettlementStatusForUpdate(ctx, tx, in.GameRoundID)
	if err != nil {
		// 如果是 "no rows" 错误，说明回合不存在
		if strings.Contains(err.Error(), "no rows") {
			return ErrGameRoundNotFound
		}
		return err
	}

	currentState := codeToState(statusCode)
	fmt.Printf("[DrawResult]  当前状态: state=%s(%d), is_settled=%d, round_id=%s, trace_id=%s\n",
		currentState, statusCode, isSettled, in.GameRoundID, in.TraceID)

	// 如果已经结算过，直接返回成功（幂等）
	if isSettled == 1 {
		fmt.Printf("[DrawResult] 该回合已结算，跳过重复结算: round_id=%s, trace_id=%s\n",
			in.GameRoundID, in.TraceID)
		resultLabel = "success_idempotent"
		return nil
	}

	// 校验当前回合状态：仅允许在 drawn(已触发game_draw事件) 状态执行开奖结算
	if currentState != state.StateDrawn {
		return ErrInvalidStateDraw
	}

	// 更新回合信息的开奖结果（状态保持为 drawn(5)）。card_list 按原始字符串入库。
	if err := model.UpdateDraw(ctx, tx, in.GameRoundID, in.CardList, res, 5); err != nil {
		return err
	}

	// 发送玩法开奖事件到 Outbox（事务内写入，确保与数据库状态一致）
	fmt.Printf("[DrawResult] 写入 Outbox: topic=game_drawn, round_id=%s, trace_id=%s\n",
		in.GameRoundID, in.TraceID)
	if err := model.CreateOutbox(ctx, tx, "game_drawn", in.GameRoundID, map[string]any{
		"event":         "game_drawn",
		"game_id":       in.GameID,
		"room_id":       in.RoomID,
		"game_round_id": in.GameRoundID,
		"card_list":     in.CardList,
		"result":        res,
		"trace_id":      in.TraceID,
	}); err != nil {
		fmt.Printf("[DrawResult]  写入 Outbox 失败: round_id=%s, error=%v, trace_id=%s\n",
			in.GameRoundID, err, in.TraceID)
		return err
	}

	// ========== 幂等性保护 #2: 创建结算日志 ==========
	// 利用唯一索引防止重复结算（双重保护）
	totalPayout := 0.0
	settlementLog := &model.SettlementLog{
		GameRoundID: in.GameRoundID,
		CardList:    in.CardList,
		Result:      res,
		TotalOrders: 0, // 稍后更新
		TotalPayout: 0, // 稍后更新
		Operator:    "admin",
		TraceID:     in.TraceID,
	}

	if err := model.CreateSettlementLog(ctx, tx, settlementLog); err != nil {
		// 如果是唯一键冲突，说明已经结算过（双重保护）
		if isMySQLDuplicateKeyError(err) {
			fmt.Printf("[DrawResult] 结算日志已存在，跳过重复结算: round_id=%s, trace_id=%s\n",
				in.GameRoundID, in.TraceID)
			resultLabel = "success_idempotent"
			return nil
		}
		fmt.Printf("[DrawResult] 创建结算日志失败: round_id=%s, error=%v, trace_id=%s\n",
			in.GameRoundID, err, in.TraceID)
		return err
	}

	// 查询需要结算的订单
	orders, err := model.ListByRoundForUpdate(ctx, tx, in.GameRoundID)
	if err != nil {
		return err
	}

	fmt.Printf("[DrawResult]  找到 %d 个待结算订单: round_id=%s, trace_id=%s\n",
		len(orders), in.GameRoundID, in.TraceID)

	// 将游戏结果字符串转换为数值枚举
	gameResultCode := resultToCode(res)

	// 第一步：更新所有订单的结算状态和游戏结果，并计算总派彩
	totalPayout = 0.0
	for i := range orders {
		o := orders[i]
		payout := settlePayout(o, res)
		totalPayout += payout
		billStatus := int8(2) // 2=已结算
		if err := model.UpdateSettlement(ctx, tx, o.BillNo, payout, billStatus, gameResultCode); err != nil {
			return err
		}
	}

	// 第二步：按用户分组，批量处理余额更新（避免同一用户多次锁定）
	type userSettlement struct {
		userID        int64
		totalPayout   float64
		orders        []model.Order
		payoutAmounts []float64
	}

	userMap := make(map[int64]*userSettlement)
	for i := range orders {
		o := orders[i]
		payout := settlePayout(o, res)

		if payout > 0 {
			if _, exists := userMap[o.UserID]; !exists {
				userMap[o.UserID] = &userSettlement{
					userID:        o.UserID,
					totalPayout:   0,
					orders:        []model.Order{},
					payoutAmounts: []float64{},
				}
			}
			userMap[o.UserID].totalPayout += payout
			userMap[o.UserID].orders = append(userMap[o.UserID].orders, o)
			userMap[o.UserID].payoutAmounts = append(userMap[o.UserID].payoutAmounts, payout)
		}
	}

	// 第三步：每个用户只锁定一次，批量更新余额和账本
	for _, us := range userMap {
		// 锁定用户
		user, err := model.GetUserByIDForUpdate(ctx, tx, us.userID)
		if err != nil {
			return err
		}

		// 使用 decimal 进行精确计算
		beforeDec := decimal.NewFromFloat(user.Balance)
		totalPayoutDec := decimal.NewFromFloat(us.totalPayout)
		afterDec := beforeDec.Add(totalPayoutDec).Round(2)

		// 更新余额
		if err := model.UpdateUserBalance(ctx, tx, us.userID, afterDec.InexactFloat64()); err != nil {
			return err
		}

		// 为每笔订单创建账本记录
		// 使用 decimal 累计计算，确保精度
		currentBalanceDec := beforeDec
		for idx, o := range us.orders {
			payoutDec := decimal.NewFromFloat(us.payoutAmounts[idx])
			currentBalanceDec = currentBalanceDec.Add(payoutDec).Round(2)

			ledger := &model.WalletLedger{
				UserID:       o.UserID,
				BizType:      2,
				BizTypeStr:   "settle",
				Amount:       payoutDec.InexactFloat64(),
				BeforeAmount: currentBalanceDec.Sub(payoutDec).Round(2).InexactFloat64(),
				AfterAmount:  currentBalanceDec.InexactFloat64(),
				Currency:     o.Currency,
				BillNo:       o.BillNo,
				GameRoundID:  in.GameRoundID,
				GameID:       in.GameID,
				RoomID:       in.RoomID,
				Remark:       "bet payout",
				TraceID:      in.TraceID,
			}
			if err := ledger.Insert(ctx, tx); err != nil {
				return err
			}
		}
	}

	// 第四步：为所有订单创建 Outbox 消息
	for i := range orders {
		o := orders[i]
		payout := settlePayout(o, res)

		if err := model.CreateOutbox(ctx, tx, "order_settled", o.BillNo, map[string]any{
			"event":         "order_settled",
			"bill_no":       o.BillNo,
			"user_id":       o.UserID,
			"game_id":       in.GameID,
			"room_id":       in.RoomID,
			"game_round_id": in.GameRoundID,
			"play_type":     o.PlayType,
			"payout":        payout,
			"result":        res,
			"trace_id":      in.TraceID,
		}); err != nil {
			return err
		}
	}

	// 明确输出到控制台
	fmt.Printf("drawresult: round=%s result=%s cards=%v\n", in.GameRoundID, res, in.CardList)

	// ========== 幂等性保护 #3: 标记为已结算 ==========
	if err := model.MarkAsSettled(ctx, tx, in.GameRoundID); err != nil {
		return err
	}

	// 更新结算日志的统计信息（写入数据库）
	if err := model.UpdateSettlementStats(ctx, tx, in.GameRoundID, len(orders), totalPayout); err != nil {
		fmt.Printf("[DrawResult] 更新结算日志统计失败: round_id=%s, error=%v, trace_id=%s\n",
			in.GameRoundID, err, in.TraceID)
		return err
	}

	// 审计事件 - draw_result（开奖结算）
	auditPayload := map[string]any{
		"card_list":    in.CardList,
		"result":       res,
		"total_orders": len(orders),
		"total_payout": totalPayout,
	}
	// 事件类型 4 = game_draw（这里记录的是开奖结算操作）
	aud := &model.GameEventAudit{
		GameID:      in.GameID,
		RoomID:      in.RoomID,
		GameRoundID: in.GameRoundID,
		EventType:   4,
		PrevState:   "drawn",
		NextState:   "settled",
		Operator:    "system",
		Source:      "api",
		Payload:     toJSON(auditPayload),
		TraceID:     in.TraceID,
	}
	if err := aud.Insert(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		fmt.Printf("[DrawResult] 提交事务失败: round_id=%s, error=%v, trace_id=%s\n",
			in.GameRoundID, err, in.TraceID)
		return err
	}

	// 将开奖结果写入 Redis，便于后续查询/回放
	if r := infrds.Client(); r != nil {
		val := map[string]any{
			"game_id":       in.GameID,
			"room_id":       in.RoomID,
			"game_round_id": in.GameRoundID,
			"card_list":     in.CardList,
			"result":        res,
			"game_status":   6, // settled
			"is_settled":    1,
			"total_orders":  len(orders),
			"total_payout":  totalPayout,
		}
		if b, e := json.Marshal(val); e == nil {
			fmt.Printf("[DrawResult]  写入 Redis 缓存: key=%s, ttl=2m, round_id=%s, trace_id=%s\n",
				infrds.RoundResultKey(in.GameRoundID), in.GameRoundID, in.TraceID)
			_ = r.Set(ctx, infrds.RoundResultKey(in.GameRoundID), b, 2*time.Minute).Err()
		}
	}

	resultLabel = "success"
	fmt.Printf("[DrawResult] 开奖处理完成: round_id=%s, result=%s, current_state=settled(6), total_orders=%d, total_payout=%.2f, trace_id=%s\n",
		in.GameRoundID, res, len(orders), totalPayout, in.TraceID)
	fmt.Printf("[DrawResult] 提示: 请手动调用 /api/game_event (event_type=5) 来结束游戏\n")
	return nil
}

// decideResult 解析开奖结果
// 标准格式：D<数字>,T<数字>,R<结果>
// 示例：
//   - "D9,T8,Rd"     -> Dragon 9点，Tiger 8点，Dragon 赢
//   - "D1,T2,Rt"     -> Dragon 1点，Tiger 2点，Tiger 赢
//   - "D9,T9,Rtie"   -> Dragon 9点，Tiger 9点，和局
//   - "D13,T10,Rd"   -> Dragon K，Tiger 10，Dragon 赢
func decideResult(cardListRaw string) string {
	input := strings.TrimSpace(cardListRaw)
	if input == "" {
		return ""
	}

	// 解析标准格式：D<数字>,T<数字>,R<结果>
	tokens := strings.Split(input, ",")
	if len(tokens) != 3 {
		return ""
	}

	var dragonCard, tigerCard int
	var result string

	for _, token := range tokens {
		tok := strings.ToLower(strings.TrimSpace(token))
		if tok == "" {
			continue
		}

		switch tok[0] {
		case 'd':
			// 解析 Dragon 牌点：D<数字>
			if val, err := strconv.Atoi(tok[1:]); err == nil && val >= 1 && val <= 13 {
				dragonCard = val
			}
		case 't':
			// 解析 Tiger 牌点：T<数字>
			if val, err := strconv.Atoi(tok[1:]); err == nil && val >= 1 && val <= 13 {
				tigerCard = val
			}
		case 'r':
			// 解析结果标记：R<结果>
			result = parseResultToken(tok[1:])
		}
	}

	// 验证必须有 Dragon 和 Tiger 牌点
	if dragonCard == 0 || tigerCard == 0 {
		return "" // 格式错误，返回默认值
	}

	// 优先使用显式指定的结果
	if result != "" {
		return result
	}

	// 根据牌点自动计算结果
	return calculateResult(dragonCard, tigerCard)
}

// parseResultToken 解析结果标记
// 支持：d, dragon, -dragon, t, tiger, -tiger, tie
func parseResultToken(token string) string {
	// 转换为小写进行匹配
	tok := strings.ToLower(token)
	switch tok {
	case "d", "dragon", "-dragon":
		return "dragon"
	case "t", "tiger", "-tiger":
		return "tiger"
	case "tie":
		return "tie"
	default:
		return ""
	}
}

// calculateResult 根据牌点计算结果
func calculateResult(dragonCard, tigerCard int) string {
	if dragonCard > tigerCard {
		return "dragon"
	} else if dragonCard < tigerCard {
		return "tiger"
	}
	return "tie"
}

// 计算派彩金额
// 重要：下注时已经扣款，派彩金额是用户最终收到的金额
// 规则：
// 1. 投注龙/虎，结果匹配 -> 派彩 = 本金 + 本金 × 赔率 = 本金 × (1 + 赔率)
// 2. 投注龙/虎，结果是和 -> 派彩 = 本金（退还本金）
// 3. 投注龙/虎，结果不匹配 -> 派彩 = 0（输掉本金）
// 4. 投注和，结果是和 -> 派彩 = 本金 + 本金 × 赔率 = 本金 × (1 + 赔率)
// 5. 投注和，结果不是和 -> 派彩 = 0（输掉本金）
//
// 示例：
// - 投注龙 100，赔率 0.95，龙赢 -> 派彩 = 100 + 100×0.95 = 195（用户余额变化：-100+195=+95）
// - 投注和 50，赔率 8.00，和局 -> 派彩 = 50 + 50×8 = 450（用户余额变化：-50+450=+400）
// - 投注龙 100，和局 -> 派彩 = 100（用户余额变化：-100+100=0，不输不赢）
func settlePayout(o model.Order, result string) float64 {
	pt := strings.ToLower(o.PlayType)
	betAmt := decimal.NewFromFloat(o.BetAmount)

	// 情况1: 投注类型与结果匹配（赢）-> 返还本金 + 赢的钱
	if pt == result {
		// 使用 decimal 进行精确计算，避免浮点数精度问题
		odds := decimal.NewFromFloat(o.BetOdds)
		// 派彩 = 本金 + 本金 × 赔率 = 本金 × (1 + 赔率)
		one := decimal.NewFromInt(1)
		payout := betAmt.Mul(one.Add(odds)).Round(2)
		return payout.InexactFloat64()
	}

	// 情况2: 投注龙或虎，结果是和局 -> 退还本金
	if (pt == "dragon" || pt == "tiger") && result == "tie" {
		return betAmt.InexactFloat64()
	}

	// 情况3: 其他情况（输）
	return 0
}

// resultToCode 将游戏结果字符串转换为数值枚举
// dragon -> 1, tiger -> 2, tie -> 3, 其他 -> 0
func resultToCode(result string) int8 {
	switch strings.ToLower(result) {
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

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// isMySQLDuplicateKeyError 判断是否为 MySQL 唯一键冲突错误
func isMySQLDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// MySQL 错误码 1062: Duplicate entry
	return strings.Contains(errMsg, "Error 1062") ||
		strings.Contains(errMsg, "Duplicate entry") ||
		strings.Contains(errMsg, "duplicate key")
}

var (
	ErrBadRequest        = errors.New("bad request")
	ErrInvalidStateDraw  = errors.New("draw not allowed in current state")
	ErrGameRoundNotFound = errors.New("game round not found")
)
