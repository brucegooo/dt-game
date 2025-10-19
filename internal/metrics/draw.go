package metrics

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	drawTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "draw_requests_total",
			Help: "Total draw result submissions by result and outcome",
		},
		[]string{"result", "outcome"},
	)

	drawDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "draw_request_duration_ms",
			Help:    "Draw result processing duration in milliseconds",
			Buckets: prometheus.ExponentialBuckets(5, 2, 10),
		},
		[]string{"result", "outcome"},
	)
)

// RecordDraw 记录 DrawResult 的业务指标
// result: "success" | "fail"
// outcome: "dragon" | "tiger" | "tie" | "unknown"
func RecordDraw(result, outcome string, started time.Time) {
	res := result
	if res != "success" { res = "fail" }
	oc := strings.ToLower(strings.TrimSpace(outcome))
	if oc == "" { oc = "unknown" }
	drawTotal.WithLabelValues(res, oc).Inc()
	durMs := float64(time.Since(started).Milliseconds())
	drawDuration.WithLabelValues(res, oc).Observe(durMs)
}

