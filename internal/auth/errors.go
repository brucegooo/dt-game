package auth

import "errors"

// 认证相关错误定义
var (
	// 平台认证错误
	ErrMissingAuthHeaders   = errors.New("missing authentication headers")
	ErrInvalidAppKey        = errors.New("invalid app_key")
	ErrInvalidPlatform      = errors.New("invalid platform")
	ErrPlatformDisabled     = errors.New("platform is disabled")
	ErrTimestampExpired     = errors.New("timestamp expired")
	ErrNonceReused          = errors.New("nonce already used")
	ErrInvalidSignature     = errors.New("invalid signature")
	ErrIPNotAllowed         = errors.New("ip address not allowed")
	ErrMissingPlatformUser  = errors.New("missing platform user id")
	ErrInvalidPlatformUser  = errors.New("invalid platform user id format")

	// JWT Token 错误
	ErrMissingToken          = errors.New("missing authorization token")
	ErrInvalidTokenFormat    = errors.New("invalid token format")
	ErrInvalidToken          = errors.New("invalid token")
	ErrTokenExpired          = errors.New("token expired")
	ErrTokenRevoked          = errors.New("token revoked")
	ErrInvalidSigningMethod  = errors.New("invalid signing method")
	ErrPlatformMismatch      = errors.New("platform mismatch")

	// 管理员认证错误
	ErrInvalidAdminToken = errors.New("invalid admin token")
	ErrAdminAuthDisabled = errors.New("admin authentication is disabled")
)

