package metrics

import (
	"time"

	"github.com/beego/beego/v2/server/web/context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpReqTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method", "status"},
	)

	httpReqDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_ms",
			Help:    "HTTP request duration in ms",
			Buckets: prometheus.ExponentialBuckets(5, 2, 10),
		},
		[]string{"path", "method"},
	)
)

// HTTPMetricsFilter 记录 HTTP 请求指标
func HTTPMetricsFilter(ctx *context.Context) {
	start := time.Now()
	// 让后续处理继续
	ctx.Input.SetData("_metrics_start", start)
}

// HTTPMetricsAfter 用于在响应完成后记录耗时与状态码
func HTTPMetricsAfter(ctx *context.Context) {
	v := ctx.Input.GetData("_metrics_start")
	start, _ := v.(time.Time)
	if !start.IsZero() {
		dur := time.Since(start).Milliseconds()
		path := ctx.Input.URL()
		method := ctx.Input.Method()
		status := ctx.ResponseWriter.Status
		httpReqDuration.WithLabelValues(path, method).Observe(float64(dur))
		httpReqTotal.WithLabelValues(path, method, itoa(status)).Inc()
	}
}

func itoa(i int) string { return fmtInt(i) }

// 轻量整数转字符串，避免额外依赖
func fmtInt(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
