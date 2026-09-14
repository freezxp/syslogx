package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type HTTP struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewHTTP(reg prometheus.Registerer) *HTTP {
	m := &HTTP{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "syslogx_api_requests_total", Help: "HTTP API requests by normalized route and status."}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "syslogx_api_request_duration_seconds", Help: "HTTP API request duration by normalized route.", Buckets: prometheus.DefBuckets}, []string{"method", "route"}),
	}
	reg.MustRegister(m.requests, m.duration)
	return m
}

func (m *HTTP) Observe(method, route string, status int, duration time.Duration) {
	if route == "" {
		route = "unmatched"
	}
	m.requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.duration.WithLabelValues(method, route).Observe(duration.Seconds())
}
