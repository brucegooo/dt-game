package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dt-server/common/logger"
	"dt-server/internal/config"
	infrds "dt-server/internal/infra/redis"

	beegocontext "github.com/beego/beego/v2/server/web/context"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// JWTClaims JWT Token 的 Claims 结构
type JWTClaims struct {
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	PlatformID int8   `json:"platform_id"`
	AppKey     string `json:"app_key"`
	TokenType  string `json:"token_type"` // access / refresh
	jwt.RegisteredClaims
}

// GenerateAccessToken 生成访问令牌
func GenerateAccessToken(userID int64, username string, platformID int8, appKey string) (string, error) {
	cfg := config.Get()
	if cfg == nil {
		return "", fmt.Errorf("config not loaded")
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(cfg.Auth.JWT.AccessTokenTTL) * time.Second)

	claims := JWTClaims{
		UserID:     userID,
		Username:   username,
		PlatformID: platformID,
		AppKey:     appKey,
		TokenType:  "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    cfg.Auth.JWT.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Auth.JWT.Secret))
}

// GenerateRefreshToken 生成刷新令牌
func GenerateRefreshToken(userID int64, username string, platformID int8, appKey string) (string, error) {
	cfg := config.Get()
	if cfg == nil {
		return "", fmt.Errorf("config not loaded")
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(cfg.Auth.JWT.RefreshTokenTTL) * time.Second)

	claims := JWTClaims{
		UserID:     userID,
		Username:   username,
		PlatformID: platformID,
		AppKey:     appKey,
		TokenType:  "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    cfg.Auth.JWT.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Auth.JWT.Secret))
}

// VerifyJWTToken 验证 JWT Token
func VerifyJWTToken(ctx *beegocontext.Context) (*JWTClaims, error) {
	// 1. 提取 Authorization 头
	authHeader := strings.TrimSpace(ctx.Input.Header("Authorization"))
	if authHeader == "" {
		return nil, ErrMissingToken
	}

	// 2. 解析 Bearer Token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, ErrInvalidTokenFormat
	}
	tokenString := parts[1]

	// 3. 解析和验证 JWT
	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSigningMethod
		}
		return []byte(cfg.Auth.JWT.Secret), nil
	})

	if err != nil {
		logger.Warn("jwt parse failed", zap.Error(err))
		return nil, ErrInvalidToken
	}

	// 4. 提取 Claims
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// 5. 检查 Token 是否在黑名单中
	if IsTokenBlacklisted(ctx.Request.Context(), tokenString) {
		logger.Warn("token is blacklisted",
			zap.Int64("user_id", claims.UserID),
			zap.String("token_type", claims.TokenType))
		return nil, ErrTokenRevoked
	}

	logger.Debug("jwt verification successful",
		zap.Int64("user_id", claims.UserID),
		zap.String("username", claims.Username),
		zap.Int8("platform_id", claims.PlatformID))

	return claims, nil
}

// RevokeToken 撤销 Token（加入黑名单）
func RevokeToken(ctx context.Context, tokenString string, expiresAt time.Time) error {
	rdb := infrds.Client()
	if rdb == nil {
		logger.Warn("redis not available, cannot revoke token")
		return nil // 降级：Redis 不可用时不阻断
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil // Token 已过期，无需加入黑名单
	}

	key := fmt.Sprintf("token:blacklist:%s", tokenString)
	err := rdb.SetEx(ctx, key, "1", ttl).Err()
	if err != nil {
		logger.Warn("failed to add token to blacklist", zap.Error(err))
		return err
	}

	logger.Info("token revoked", zap.String("key", key), zap.Duration("ttl", ttl))
	return nil
}

// IsTokenBlacklisted 检查 Token 是否在黑名单中
func IsTokenBlacklisted(ctx context.Context, tokenString string) bool {
	rdb := infrds.Client()
	if rdb == nil {
		return false // 降级：Redis 不可用时不阻断
	}

	key := fmt.Sprintf("token:blacklist:%s", tokenString)
	exists, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		logger.Warn("failed to check token blacklist", zap.Error(err))
		return false // 降级：Redis 错误时不阻断
	}

	return exists > 0
}
