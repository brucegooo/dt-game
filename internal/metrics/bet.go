package metrics

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	betTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bet_requests_total",
			Help: "Total bet requests by result and play_type",
		},
		[]string{"result", "play_type"},
	)

	betDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "bet_request_duration_ms",
			Help:    "Bet request duration in milliseconds",
			Buckets: prometheus.ExponentialBuckets(5, 2, 10),
		},
		[]string{"result", "play_type"},
	)
)

// RecordBet records business metrics for a bet call.
// result should be "success" or "fail"; playType is normalized to lower-case.
func RecordBet(result, playType string, started time.Time) {
	res := result
	if res != "success" { res = "fail" }
	pt := strings.ToLower(playType)
	betTotal.WithLabelValues(res, pt).Inc()
	durMs := float64(time.Since(started).Milliseconds())
	betDuration.WithLabelValues(res, pt).Observe(durMs)
}

