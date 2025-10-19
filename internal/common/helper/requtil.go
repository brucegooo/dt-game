package helper

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	beegocontext "github.com/beego/beego/v2/server/web/context"
)

// IsJSONContentType 判断是否为 JSON 请求
func IsJSONContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	return strings.Contains(ct, "json")
}

// 金额格式校验：非负，最多两位小数（预编译正则）
var moneyRe = regexp.MustCompile(`^(?:0|[1-9]\d*)(?:\.\d{1,2})?$`)

// IsMoneyFormat 判断金额格式
func IsMoneyFormat(s string) bool {
	return moneyRe.MatchString(strings.TrimSpace(s))
}

// 默认输入保护参数
const (
	defaultJSONMaxBytes int64         = 1 << 20 // 1MB
	defaultParseTimeout time.Duration = 1 * time.Second
)

type deadlineReader struct {
	r        io.Reader
	deadline time.Time
}

func (dr *deadlineReader) Read(p []byte) (int, error) {
	if time.Now().After(dr.deadline) {
		return 0, fmt.Errorf("read timeout")
	}
	return dr.r.Read(p)
}

// jsonBodyReader 在 JSON 分支下为请求体增加大小限制与解析超时保护
func jsonBodyReader(ctx *beegocontext.Context) io.Reader {
	lr := io.LimitReader(ctx.Request.Body, defaultJSONMaxBytes)
	return &deadlineReader{r: lr, deadline: time.Now().Add(defaultParseTimeout)}
}

// GetTraceID 统一提取 trace_id：优先从中间件注入的数据取，其次从常见请求头降级
func GetTraceID(ctx *beegocontext.Context) string {
	if v := ctx.Input.GetData("trace_id"); v != nil {
		return fmt.Sprint(v)
	}
	if h := strings.TrimSpace(ctx.Input.Header("X-Trace-ID")); h != "" {
		return h
	}
	if h := strings.TrimSpace(ctx.Input.Header("Trace-Id")); h != "" {
		return h
	}
	return ""
}

// parseByContentType 按 Content-Type 选择解析函数，减少重复 if/else 分支
func parseByContentType[T any](ctx *beegocontext.Context,
	jsonParser func(io.Reader) (T, bool, string),
	formParser func(*beegocontext.Context) (T, bool, string),
) (T, bool, string) {
	ct := ctx.Input.Header("Content-Type")
	if IsJSONContentType(ct) {
		return jsonParser(jsonBodyReader(ctx))
	}
	return formParser(ctx)
}

// BetParsed 为解析后的投注入参（与控制器/服务层解耦）
type BetParsed struct {
	GameId         string `json:"game_id"`
	RoomId         string `json:"room_id"`
	GameRoundId    string `json:"game_round_id"`
	UserId         int64  `json:"user_id"`
	BetAmount      string `json:"bet_amount"`
	PlayType       int    `json:"play_type"`
	Platform       int    `json:"platform"`
	IdempotencyKey string `json:"idempotency_key"`
}

// ParseBetFromJSON 解析 JSON 到 BetParsed。失败返回 false 与错误消息。
func ParseBetFromJSON(r io.Reader) (BetParsed, bool, string) {
	var out BetParsed
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return BetParsed{}, false, "invalid json body"
	}
	return out, true, ""
}

// ParseBetFromForm 从表单读取字段并做强校验，返回 BetParsed。失败返回 false 与可读错误信息。
func ParseBetFromForm(ctx *beegocontext.Context) (BetParsed, bool, string) {
	var out BetParsed
	out.GameId = strings.TrimSpace(ctx.Input.Query("game_id"))
	out.RoomId = strings.TrimSpace(ctx.Input.Query("room_id"))
	out.GameRoundId = strings.TrimSpace(ctx.Input.Query("game_round_id"))
	if out.GameId == "" || out.RoomId == "" || out.GameRoundId == "" {
		return BetParsed{}, false, "missing required fields: game_id/room_id/game_round_id"
	}

	uidStr := strings.TrimSpace(ctx.Input.Query("user_id"))
	if uidStr == "" {
		return BetParsed{}, false, "user_id required"
	}
	u64, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return BetParsed{}, false, "user_id must be integer"
	}
	out.UserId = u64

	out.BetAmount = strings.TrimSpace(ctx.Input.Query("bet_amount"))
	if out.BetAmount == "" || !IsMoneyFormat(out.BetAmount) {
		return BetParsed{}, false, "bet_amount must be numeric with up to 2 decimals"
	}

	ptStr := strings.TrimSpace(ctx.Input.Query("play_type"))
	if ptStr == "" {
		return BetParsed{}, false, "play_type required"
	}
	pi, err := strconv.Atoi(ptStr)
	if err != nil || (pi != 1 && pi != 2 && pi != 3) {
		return BetParsed{}, false, "play_type must be 1|2|3"
	}
	out.PlayType = pi

	// platform: 可选，默认 1；如传入，需为 1..127 的整数
	pStr := strings.TrimSpace(ctx.Input.Query("platform"))
	if pStr == "" {
		out.Platform = 1
	} else {
		pn, err := strconv.Atoi(pStr)
		if err != nil || pn <= 0 || pn >= 128 {
			out.Platform = 1
		} else {
			out.Platform = pn
		}
	}

	out.IdempotencyKey = strings.TrimSpace(ctx.Input.Query("idempotency_key"))
	if out.IdempotencyKey == "" {
		return BetParsed{}, false, "idempotency_key required"
	}

	return out, true, ""
}

// ValidateBet 对通用字段做二次校验（适用于 JSON 与 FORM）。失败返回 false 与错误消息。
func ValidateBet(in *BetParsed) (bool, string) {
	// 注意：UserId 不再是必填字段，因为多平台认证系统使用 platform_id + platform_user_id
	if in.GameId == "" || in.RoomId == "" || in.GameRoundId == "" || strings.TrimSpace(in.BetAmount) == "" || in.PlayType == 0 || in.IdempotencyKey == "" {
		return false, "missing or invalid fields"
	}
	// 额外长度保护，避免异常超长输入
	if len(in.GameId) > 64 || len(in.RoomId) > 64 || len(in.GameRoundId) > 64 || len(in.IdempotencyKey) > 64 || len(in.BetAmount) > 32 {
		return false, "invalid request"
	}
	// 玩法校验
	if in.PlayType != 1 && in.PlayType != 2 && in.PlayType != 3 {
		return false, "play_type must be 1|2|3"
	}
	// 金额校验
	if !IsMoneyFormat(in.BetAmount) {
		return false, "bet_amount must be numeric with up to 2 decimals"
	}
	return true, ""
}

// ParseAndValidateBet 按 Content-Type 自动解析并做统一校验
func ParseAndValidateBet(ctx *beegocontext.Context) (BetParsed, bool, string) {
	out, ok, msg := parseByContentType(ctx, ParseBetFromJSON, ParseBetFromForm)
	if !ok {
		return BetParsed{}, false, msg
	}
	if ok, msg := ValidateBet(&out); !ok {
		return BetParsed{}, false, msg
	}
	return out, true, ""
}

// -------- DrawResult helpers --------

// IsValidCardList 校验 card_list 参数格式：
// - 简化演示：允许 "dragon"|"tiger"|"tie"
// - 兼容牌面格式：包含 D/T/R 字符（不做更细颗粒解析）
func IsValidCardList(s string) bool {
	ls := strings.ToLower(strings.TrimSpace(s))
	if ls == "dragon" || ls == "tiger" || ls == "tie" {
		return true
	}
	return strings.Contains(ls, "d") && strings.Contains(ls, "t") && strings.Contains(ls, "r")
}

type DrawResultParsed struct {
	GameId      string `json:"game_id"`
	RoomId      string `json:"room_id"`
	GameRoundId string `json:"game_round_id"`
	CardList    string `json:"card_list"`
	DrawTime    int64  `json:"draw_time"`
}

func ParseDrawResultFromJSON(r io.Reader) (DrawResultParsed, bool, string) {
	var out DrawResultParsed
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return DrawResultParsed{}, false, "invalid request"
	}
	return out, true, ""
}

func ParseDrawResultFromForm(ctx *beegocontext.Context) (DrawResultParsed, bool, string) {
	var out DrawResultParsed
	out.GameId = ctx.Input.Query("game_id")
	out.RoomId = ctx.Input.Query("room_id")
	out.GameRoundId = ctx.Input.Query("game_round_id")
	out.CardList = ctx.Input.Query("card_list")
	if ts := strings.TrimSpace(ctx.Input.Query("draw_time")); ts != "" {
		if v, err := strconv.ParseInt(ts, 10, 64); err == nil {
			out.DrawTime = v
		}
	}
	return out, true, ""
}

func ValidateDrawResult(in *DrawResultParsed) (bool, string) {
	if in.GameRoundId == "" || len(in.CardList) == 0 {
		return false, "invalid request"
	}
	if len(in.GameRoundId) > 64 {
		return false, "invalid request"
	}
	if len(in.CardList) > 256 {
		return false, "invalid card_list"
	}
	if !IsValidCardList(in.CardList) {
		return false, "invalid card_list"
	}
	return true, ""
}

// -------- GameEvent helpers --------

type GameEventParsed struct {
	GameId      string `json:"game_id"`
	RoomId      string `json:"room_id"`
	GameRoundId string `json:"game_round_id"`
	EventType   int    `json:"event_type"` // 仅支持数值：1=game_start 2=game_stop 3=new_card 4=game_draw 5=game_end
}

// ParseGameEventFromJSON 仅接受数值型 event_type（1..5）
func ParseGameEventFromJSON(r io.Reader) (GameEventParsed, bool, string) {
	var raw map[string]any
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return GameEventParsed{}, false, "invalid request"
	}
	var out GameEventParsed
	if v, ok := raw["game_id"].(string); ok {
		out.GameId = v
	}
	if v, ok := raw["room_id"].(string); ok {
		out.RoomId = v
	}
	if v, ok := raw["game_round_id"].(string); ok {
		out.GameRoundId = v
	}
	// 仅当 event_type 为 JSON 数字时赋值
	if v, ok := raw["event_type"].(float64); ok {
		out.EventType = int(v)
	}
	return out, true, ""
}

// ParseAndValidateDrawResult 按 Content-Type 自动解析并校验
func ParseAndValidateDrawResult(ctx *beegocontext.Context) (DrawResultParsed, bool, string) {
	out, ok, msg := parseByContentType(ctx, ParseDrawResultFromJSON, ParseDrawResultFromForm)
	if !ok {
		return DrawResultParsed{}, false, msg
	}
	if ok, msg := ValidateDrawResult(&out); !ok {
		return DrawResultParsed{}, false, msg
	}
	return out, true, ""
}

func ParseAndValidateGameEvent(ctx *beegocontext.Context) (GameEventParsed, bool, string) {
	out, ok, msg := parseByContentType(ctx, ParseGameEventFromJSON, ParseGameEventFromForm)
	if !ok {
		return GameEventParsed{}, false, msg
	}
	if ok, msg := ValidateGameEvent(&out); !ok {
		return GameEventParsed{}, false, msg
	}
	return out, true, ""
}

func ParseGameEventFromForm(ctx *beegocontext.Context) (GameEventParsed, bool, string) {
	var out GameEventParsed
	out.GameId = ctx.Input.Query("game_id")
	out.RoomId = ctx.Input.Query("room_id")
	out.GameRoundId = ctx.Input.Query("game_round_id")
	et := strings.TrimSpace(ctx.Input.Query("event_type"))
	if et != "" {
		if n, err := strconv.Atoi(et); err == nil {
			out.EventType = n
		}
	}
	return out, true, ""
}

func ValidateGameEvent(in *GameEventParsed) (bool, string) {
	if strings.TrimSpace(in.GameRoundId) == "" || in.EventType == 0 {
		return false, "invalid request"
	}
	if len(in.GameRoundId) > 64 {
		return false, "invalid request"
	}
	if in.EventType < 1 || in.EventType > 5 {
		return false, "event_type must be one of: 1|2|3|4|5"
	}
	return true, ""
}
