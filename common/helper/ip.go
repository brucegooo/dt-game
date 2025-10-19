package helper

import (
	"encoding/binary"
	"net"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/valyala/fasthttp"
)

// Should use canonical format of the header key s
// https://golang.org/pkg/net/http/#CanonicalHeaderKey

// Header may return multiple IP addresses in the format: "client IP, proxy 1 IP, proxy 2 IP", so we take the the first one.
var xForwardedForHeader = http.CanonicalHeaderKey("X-Original-Forwarded-For")
var xForwardedHeader = http.CanonicalHeaderKey("X-Forwarded")
var forwardedForHeader = http.CanonicalHeaderKey("Forwarded-For")
var forwardedHeader = http.CanonicalHeaderKey("Forwarded")

// Standard headers used by Amazon EC2, Heroku, and others
var xClientIPHeader = http.CanonicalHeaderKey("X-Client-IP")

// Nginx proxy/FastCGI
var xRealIPHeader = http.CanonicalHeaderKey("X-Real-IP")

// Cloudflare.
// @see https://support.cloudflare.com/hc/en-us/articles/200170986-How-does-Cloudflare-handle-HTTP-Request-headers-
// CF-Connecting-IP - applied to every request to the origin.
var cfConnectingIPHeader = http.CanonicalHeaderKey("X-Original-Forwarded-For")

// Fastly CDN and Firebase hosting header when forwared to a cloud function
var fastlyClientIPHeader = http.CanonicalHeaderKey("Fastly-Client-Ip")

// Akamai and Cloudflare
var trueClientIPHeader = http.CanonicalHeaderKey("True-Client-Ip")

var cidrs []*net.IPNet

func Ip2long(ipAddr string) (uint32, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return 0, errors.New("wrong ipAddr format")
	}
	ip = ip.To4()
	return binary.BigEndian.Uint32(ip), nil
}

func init() {
	maxCidrBlocks := []string{
		"127.0.0.1/8",    // localhost
		"10.0.0.0/8",     // 24-bit block
		"172.16.0.0/12",  // 20-bit block
		"192.168.0.0/16", // 16-bit block
		"169.254.0.0/16", // link local address
		"::1/128",        // localhost IPv6
		"fc00::/7",       // unique local address IPv6
		"fe80::/10",      // link local address IPv6
	}

	cidrs = make([]*net.IPNet, len(maxCidrBlocks))
	for i, maxCidrBlock := range maxCidrBlocks {
		_, cidr, _ := net.ParseCIDR(maxCidrBlock)
		cidrs[i] = cidr
	}
}

// isLocalAddress works by checking if the address is under private CIDR blocks.
// List of private CIDR blocks can be seen on :
//
// https://en.wikipedia.org/wiki/Private_network
//
// https://en.wikipedia.org/wiki/Link-local_address
func isPrivateAddress(address string) (bool, error) {
	ipAddress := net.ParseIP(address)
	if ipAddress == nil {
		return false, errors.New("address is not valid")
	}

	for i := range cidrs {
		if cidrs[i].Contains(ipAddress) {
			return true, nil
		}
	}

	return false, nil
}

// FromRequest returns client's real public IP address from http request headers.
func FromRequest(ctx *fasthttp.RequestCtx) string {
	// 尝试从各种头获取IP，并进行安全验证
	if ip := getValidIPFromHeader(ctx, xClientIPHeader); ip != "" {
		return ip
	}

	if xForwardedFor := ctx.Request.Header.Peek(xForwardedForHeader); xForwardedFor != nil {
		if requestIP, err := retrieveForwardedIP(string(xForwardedFor)); err == nil {
			if validIP := validateAndCleanIP(requestIP); validIP != "" {
				return validIP
			}
		}
	}

	if ip, err := fromSpecialHeaders(ctx); err == nil {
		if validIP := validateAndCleanIP(ip); validIP != "" {
			return validIP
		}
	}

	if ip, err := fromForwardedHeaders(ctx); err == nil {
		if validIP := validateAndCleanIP(ip); validIP != "" {
			return validIP
		}
	}

	// 最后从RemoteAddr获取，并进行严格验证
	remoteAddr := ctx.RemoteAddr()
	if remoteAddr != nil {
		var remoteIP string
		remoteAddrStr := remoteAddr.String()

		if strings.ContainsRune(remoteAddrStr, ':') {
			remoteIP, _, _ = net.SplitHostPort(remoteAddrStr)
		} else {
			remoteIP = remoteAddrStr
		}

		if validIP := validateAndCleanIP(remoteIP); validIP != "" {
			return validIP
		}
	}

	// 如果所有方法都失败，返回unknown而不是无效IP
	return "unknown"
}

func fromSpecialHeaders(ctx *fasthttp.RequestCtx) (string, error) {
	ipHeaders := [...]string{cfConnectingIPHeader, fastlyClientIPHeader, trueClientIPHeader, xRealIPHeader}
	for _, iplHeader := range ipHeaders {
		if clientIP := ctx.Request.Header.Peek(iplHeader); clientIP != nil {
			return string(clientIP), nil
		}
	}
	return "", errors.New("can't get ip from special headers")
}

func fromForwardedHeaders(ctx *fasthttp.RequestCtx) (string, error) {
	forwardedHeaders := [...]string{xForwardedHeader, forwardedForHeader, forwardedHeader}
	for _, forwardedHeader := range forwardedHeaders {
		if forwarded := ctx.Request.Header.Peek(forwardedHeader); forwarded != nil {
			if clientIP, err := retrieveForwardedIP(string(forwarded)); err == nil {
				return clientIP, nil
			}
		}
	}
	return "", errors.New("can't get ip from forwarded headers")
}

func retrieveForwardedIP(forwardedHeader string) (string, error) {
	for _, address := range strings.Split(forwardedHeader, ",") {
		if len(address) > 0 {
			address = strings.TrimSpace(address)
			isPrivate, err := isPrivateAddress(address)
			switch {
			case !isPrivate && err == nil:
				return address, nil
			case isPrivate && err == nil:
				return "", errors.New("forwarded ip is private")
			default:
				return "", errors.WithStack(err)
			}
		}
	}
	return "", errors.New("empty or invalid forwarded header")
}

func IpInList(ip string, ipList []string) bool {
	for _, v := range ipList {
		if v == ip {
			return true
		}
	}
	return false
}

// validateAndCleanIP 验证并清理IP地址
func validateAndCleanIP(ip string) string {
	if ip == "" {
		return ""
	}

	// 清理空白字符
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}

	// 验证IP格式
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ""
	}

	// 过滤无效IP (0.0.0.0, ::, 等)
	if parsedIP.IsUnspecified() {
		return ""
	}

	// 过滤回环地址 (127.0.0.1, ::1)
	if parsedIP.IsLoopback() {
		return ""
	}

	return ip
}

// getValidIPFromHeader 从指定头获取有效IP
func getValidIPFromHeader(ctx *fasthttp.RequestCtx, headerName string) string {
	headerValue := ctx.Request.Header.Peek(headerName)
	if headerValue == nil {
		return ""
	}

	ip := string(headerValue)
	return validateAndCleanIP(ip)
}

// GetBestClientIP 融合版本：结合原有功能和安全验证的最佳IP获取方法
func GetBestClientIP(ctx *fasthttp.RequestCtx) string {
	// 1. 优先级最高：X-Real-IP (通常最可靠)
	if ip := getValidIPFromHeader(ctx, xRealIPHeader); ip != "" {
		// 检查是否为私有IP，如果是公网IP则直接返回
		if isPublic, _ := isPrivateAddress(ip); !isPublic {
			return ip
		}
	}

	// 2. X-Client-IP (Amazon EC2, Heroku等)
	if ip := getValidIPFromHeader(ctx, xClientIPHeader); ip != "" {
		if isPublic, _ := isPrivateAddress(ip); !isPublic {
			return ip
		}
	}

	// 3. CF-Connecting-IP (Cloudflare)
	if ip := getValidIPFromHeader(ctx, cfConnectingIPHeader); ip != "" {
		if isPublic, _ := isPrivateAddress(ip); !isPublic {
			return ip
		}
	}

	// 4. True-Client-IP (Akamai, Cloudflare)
	if ip := getValidIPFromHeader(ctx, trueClientIPHeader); ip != "" {
		if isPublic, _ := isPrivateAddress(ip); !isPublic {
			return ip
		}
	}

	// 5. Fastly-Client-IP (Fastly CDN)
	if ip := getValidIPFromHeader(ctx, fastlyClientIPHeader); ip != "" {
		if isPublic, _ := isPrivateAddress(ip); !isPublic {
			return ip
		}
	}

	// 6. X-Forwarded-For (可能包含多个IP)
	if xForwardedFor := ctx.Request.Header.Peek(xForwardedForHeader); xForwardedFor != nil {
		ips := strings.Split(string(xForwardedFor), ",")
		for _, ip := range ips {
			if validIP := validateAndCleanIP(ip); validIP != "" {
				if isPublic, _ := isPrivateAddress(validIP); !isPublic {
					return validIP
				}
			}
		}
	}

	// 7. 其他转发头
	if ip, err := fromForwardedHeaders(ctx); err == nil {
		if validIP := validateAndCleanIP(ip); validIP != "" {
			if isPublic, _ := isPrivateAddress(validIP); !isPublic {
				return validIP
			}
		}
	}

	// 8. 最后尝试RemoteAddr
	remoteAddr := ctx.RemoteAddr()
	if remoteAddr != nil {
		var remoteIP string
		remoteAddrStr := remoteAddr.String()

		if strings.ContainsRune(remoteAddrStr, ':') {
			remoteIP, _, _ = net.SplitHostPort(remoteAddrStr)
		} else {
			remoteIP = remoteAddrStr
		}

		if validIP := validateAndCleanIP(remoteIP); validIP != "" {
			return validIP
		}
	}

	// 9. 如果所有方法都失败，返回unknown
	return "unknown"
}
