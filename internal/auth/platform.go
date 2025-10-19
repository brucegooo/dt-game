package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"dt-server/common/logger"
	"dt-server/internal/config"
	infrds "dt-server/internal/infra/redis"

	beegocontext "github.com/beego/beego/v2/server/web/context"
	"go.uber.org/zap"
)

// Platform 平台信息
type Platform struct {
	PlatformID int8     `json:"platform_id"`
	AppKey     string   `json:"app_key"`
	AppSecret  string   `json:"app_secret"`
	Name       string   `json:"name"`
	Status     int8     `json:"status"`
	RateLimit  int      `json:"rate_limit"`
	AllowedIPs []string `json:"allowed_ips"`
}

// VerifyPlatformSignature 验证平台签名
// 从请求头中提取认证信息，验证签名的有效性
func VerifyPlatformSignature(ctx *beegocontext.Context) (*Platform, error) {
	// 1. 提取请求头
	appKey := strings.TrimSpace(ctx.Input.Header("X-Platform-Key"))
	timestamp := strings.TrimSpace(ctx.Input.Header("X-Timestamp"))
	nonce := strings.TrimSpace(ctx.Input.Header("X-Nonce"))
	signature := strings.TrimSpace(ctx.Input.Header("X-Signature"))

	// 2. 基本校验
	if appKey == "" || timestamp == "" || nonce == "" || signature == "" {
		logger.Warn("missing authentication headers",
			zap.String("app_key", appKey),
			zap.Bool("has_timestamp", timestamp != ""),
			zap.Bool("has_nonce", nonce != ""),
			zap.Bool("has_signature", signature != ""))
		return nil, ErrMissingAuthHeaders
	}

	// 3. 时间戳校验（防重放攻击）
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		logger.Warn("invalid timestamp format", zap.String("timestamp", timestamp))
		return nil, ErrTimestampExpired
	}

	now := time.Now().Unix()
	diff := math.Abs(float64(now - ts))
	if diff > 300 { // 5分钟有效期
		logger.Warn("timestamp expired",
			zap.Int64("timestamp", ts),
			zap.Int64("now", now),
			zap.Float64("diff_seconds", diff))
		return nil, ErrTimestampExpired
	}

	// 4. Nonce 校验（防重放攻击）
	if err := checkAndSetNonce(ctx.Request.Context(), appKey, nonce); err != nil {
		logger.Warn("nonce check failed",
			zap.String("app_key", appKey),
			zap.String("nonce", nonce),
			zap.Error(err))
		return nil, err
	}

	// 5. 查询平台信息
	platform, err := GetPlatformByAppKey(appKey)
	if err != nil {
		logger.Warn("platform not found", zap.String("app_key", appKey))
		return nil, ErrInvalidPlatform
	}

	// 6. 检查平台状态
	if platform.Status != 1 {
		logger.Warn("platform is disabled",
			zap.String("app_key", appKey),
			zap.Int8("status", platform.Status))
		return nil, ErrPlatformDisabled
	}

	// 7. IP 白名单校验（如果配置了）
	if len(platform.AllowedIPs) > 0 {
		clientIP := getClientIP(ctx)
		if !isIPAllowed(clientIP, platform.AllowedIPs) {
			logger.Warn("ip not allowed",
				zap.String("app_key", appKey),
				zap.String("client_ip", clientIP),
				zap.Strings("allowed_ips", platform.AllowedIPs))
			return nil, ErrIPNotAllowed
		}
	}

	// 8. 读取请求体（用于签名验证）
	body := readRequestBody(ctx)

	// 9. 重新计算签名
	expectedSig := generateSignature(appKey, timestamp, nonce, body, platform.AppSecret)

	// 10. 比较签名（使用恒定时间比较，防止时序攻击）
	if !secureCompare(signature, expectedSig) {
		logger.Warn("signature verification failed",
			zap.String("app_key", appKey),
			zap.String("expected", expectedSig[:16]+"..."), // 只记录前16位
			zap.String("received", signature[:16]+"..."))
		return nil, ErrInvalidSignature
	}

	logger.Debug("platform authentication successful",
		zap.String("app_key", appKey),
		zap.Int8("platform_id", platform.PlatformID))

	return platform, nil
}

// GetPlatformByAppKey 根据 AppKey 获取平台信息
func GetPlatformByAppKey(appKey string) (*Platform, error) {
	cfg := config.Get()
	if cfg == nil || cfg.Auth.Platforms == nil {
		return nil, ErrInvalidPlatform
	}

	// 从配置中查找平台
	for _, p := range cfg.Auth.Platforms {
		if p.AppKey == appKey {
			return &Platform{
				PlatformID: p.PlatformID,
				AppKey:     p.AppKey,
				AppSecret:  p.AppSecret,
				Name:       p.Name,
				Status:     p.Status,
				RateLimit:  p.RateLimit,
				AllowedIPs: p.AllowedIPs,
			}, nil
		}
	}

	return nil, ErrInvalidPlatform
}

// checkAndSetNonce 检查并设置 Nonce（防重放）
func checkAndSetNonce(ctx context.Context, appKey, nonce string) error {
	rdb := infrds.Client()
	if rdb == nil {
		// Redis 不可用时，跳过 Nonce 检查（降级）
		logger.Warn("redis not available, skip nonce check")
		return nil
	}

	nonceKey := fmt.Sprintf("nonce:%s:%s", appKey, nonce)

	// 检查是否已存在
	exists, err := rdb.Exists(ctx, nonceKey).Result()
	if err != nil {
		logger.Warn("redis exists check failed", zap.Error(err))
		return nil // 降级：Redis 错误时不阻断请求
	}

	if exists > 0 {
		return ErrNonceReused
	}

	// 设置 Nonce（TTL 10分钟）
	err = rdb.SetEx(ctx, nonceKey, "1", 10*time.Minute).Err()
	if err != nil {
		logger.Warn("redis setex failed", zap.Error(err))
		return nil // 降级：Redis 错误时不阻断请求
	}

	return nil
}

// generateSignature 生成签名
// 签名算法：HMAC-SHA256(app_key + timestamp + nonce + body, app_secret)
func generateSignature(appKey, timestamp, nonce, body, secret string) string {
	signString := appKey + timestamp + nonce + body
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signString))
	return hex.EncodeToString(h.Sum(nil))
}

// secureCompare 恒定时间字符串比较（防止时序攻击）
func secureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return hmac.Equal([]byte(a), []byte(b))
}

// readRequestBody 读取请求体
func readRequestBody(ctx *beegocontext.Context) string {
	// 如果是 GET 请求，返回空字符串
	if ctx.Request.Method == "GET" || ctx.Request.Method == "DELETE" {
		return ""
	}

	// 从 context 中获取已读取的 body（避免重复读取）
	if body := ctx.Input.GetData("request_body"); body != nil {
		if bodyStr, ok := body.(string); ok {
			return bodyStr
		}
	}

	// 读取 body（注意：这会消耗 body，需要在中间件中提前读取并缓存）
	bodyBytes := ctx.Input.RequestBody
	bodyStr := string(bodyBytes)

	// 缓存到 context
	ctx.Input.SetData("request_body", bodyStr)

	return bodyStr
}

// getClientIP 获取客户端真实IP
func getClientIP(ctx *beegocontext.Context) string {
	// 优先从 X-Real-IP 获取
	if ip := ctx.Input.Header("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}

	// 其次从 X-Forwarded-For 获取（取第一个）
	if xff := ctx.Input.Header("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 最后使用 RemoteAddr
	return ctx.Request.RemoteAddr
}

// isIPAllowed 检查IP是否在白名单中
func isIPAllowed(clientIP string, allowedIPs []string) bool {
	if len(allowedIPs) == 0 {
		return true // 未配置白名单，允许所有IP
	}

	for _, ip := range allowedIPs {
		if strings.TrimSpace(ip) == clientIP {
			return true
		}
	}

	return false
}

// IsValidPlatformUserID 验证平台用户ID格式
func IsValidPlatformUserID(id string) bool {
	// 1. 长度检查
	if len(id) == 0 || len(id) > 64 {
		return false
	}

	// 2. 字符检查：只允许字母、数字、下划线、连字符
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '-') {
			return false
		}
	}

	return true
}

// MarshalPlatform 序列化平台信息（用于日志，隐藏敏感信息）
func MarshalPlatform(p *Platform) string {
	if p == nil {
		return "{}"
	}

	safe := map[string]interface{}{
		"platform_id": p.PlatformID,
		"app_key":     p.AppKey,
		"name":        p.Name,
		"status":      p.Status,
	}

	b, _ := json.Marshal(safe)
	return string(b)
}
