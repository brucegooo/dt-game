package metrics

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	gameEventTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "game_event_total",
			Help: "Total game events handled by result and event_type",
		},
		[]string{"result", "event_type"},
	)

	gameEventDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "game_event_duration_ms",
			Help:    "Game event handling duration in milliseconds",
			Buckets: prometheus.ExponentialBuckets(5, 2, 10),
		},
		[]string{"result", "event_type"},
	)
)

// RecordGameEvent 记录 GameEvent 的业务指标
// result: "success" | "fail"
// eventType: 事件类型（小写）
func RecordGameEvent(result, eventType string, started time.Time) {
	res := result
	if res != "success" { res = "fail" }
	et := strings.ToLower(strings.TrimSpace(eventType))
	if et == "" { et = "unknown" }
	gameEventTotal.WithLabelValues(res, et).Inc()
	durMs := float64(time.Since(started).Milliseconds())
	gameEventDuration.WithLabelValues(res, et).Observe(durMs)
}

