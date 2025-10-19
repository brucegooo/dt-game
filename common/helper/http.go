package helper

import (
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
)

// 第三方支付超时配置常量
const (
	ThirdPayTimeout = 8 * time.Second // 第三方支付统一超时时间
	FastTimeout     = 3 * time.Second // 快速接口超时时间
)

// 并发统计指标
var (
	activeConnections int64 // 当前活跃连接数
	totalRequests     int64 // 总请求数
)

// 全局优化的HTTP客户端，支持连接复用
var (
	globalClient = &fasthttp.Client{
		ReadTimeout:                   5 * time.Second,
		WriteTimeout:                  5 * time.Second,
		MaxIdleConnDuration:           90 * time.Second, // 连接空闲时间
		MaxConnsPerHost:               50,               // 每个主机最大连接数
		MaxConnWaitTimeout:            3 * time.Second,  // 等待连接超时
		DisableHeaderNamesNormalizing: true,             // 禁用header名称标准化以提升性能
	}

	// 专用于第三方支付的客户端 - 高并发优化
	thirdPayClient = &fasthttp.Client{
		ReadTimeout:                   ThirdPayTimeout,
		WriteTimeout:                  ThirdPayTimeout,
		MaxIdleConnDuration:           60 * time.Second,
		MaxConnsPerHost:               100,             // 增加到100个并发连接
		MaxConnWaitTimeout:            1 * time.Second, // 减少等待时间
		DisableHeaderNamesNormalizing: true,
	}
)

func HttpDoTimeout(requestBody []byte, method string, requestURI string, headers map[string]string, timeout time.Duration) ([]byte, int, error) {

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	defer func() {
		fasthttp.ReleaseResponse(resp)
		fasthttp.ReleaseRequest(req)
	}()

	req.SetRequestURI(requestURI)
	req.Header.SetMethod(method)

	switch method {
	case "POST":
		req.SetBody(requestBody)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 使用全局客户端以复用连接
	err := globalClient.DoTimeout(req, resp, timeout)

	var respBytes []byte
	statusCode := 0
	if err == nil {
		respBytes = append(respBytes, resp.Body()...)
		statusCode = resp.StatusCode()
	}

	return respBytes, statusCode, errors.WithStack(err)
}

// HttpDoTimeoutForThirdPay 专门用于第三方支付的HTTP请求，使用优化的客户端配置
func HttpDoTimeoutForThirdPay(requestBody []byte, method string, requestURI string, headers map[string]string, timeout time.Duration) ([]byte, int, error) {
	// 并发统计
	atomic.AddInt64(&activeConnections, 1)
	atomic.AddInt64(&totalRequests, 1)
	defer atomic.AddInt64(&activeConnections, -1)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(requestURI)
	req.Header.SetMethod(method)

	if method == "POST" {
		req.SetBody(requestBody)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 使用专门的第三方支付客户端
	err := thirdPayClient.DoTimeout(req, resp, timeout)

	var respBytes []byte
	statusCode := 0
	if err == nil {
		respBytes = append(respBytes, resp.Body()...)
		statusCode = resp.StatusCode()
	}

	return respBytes, statusCode, errors.WithStack(err)
}

// GetConcurrencyStats 获取并发统计信息
func GetConcurrencyStats() (activeConns int64, totalReqs int64) {
	return atomic.LoadInt64(&activeConnections), atomic.LoadInt64(&totalRequests)
}
