package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// HTTPMetrics contains Prometheus collectors for HTTP request instrumentation.
type HTTPMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

// NewHTTPMetrics creates HTTP metrics collectors and registers them.
func NewHTTPMetrics(registerer prometheus.Registerer) *HTTPMetrics {
	metrics := &HTTPMetrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lodestone",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total HTTP requests processed.",
			},
			[]string{"method", "route", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lodestone",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "route", "status"},
		),
	}

	registerer.MustRegister(metrics.requestsTotal, metrics.requestDuration)
	return metrics
}

func (m *HTTPMetrics) routeLabel(c *gin.Context) string {
	route := c.FullPath()
	if route == "" {
		return "unmatched"
	}
	return route
}

// Middleware instruments HTTP requests with counters and latency histograms.
func (m *HTTPMetrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		route := m.routeLabel(c)
		method := c.Request.Method
		durationSeconds := time.Since(start).Seconds()

		m.requestsTotal.WithLabelValues(method, route, status).Inc()
		m.requestDuration.WithLabelValues(method, route, status).Observe(durationSeconds)
	}
}

var defaultHTTPMetrics = NewHTTPMetrics(prometheus.DefaultRegisterer)

// MetricsMiddleware instruments all incoming API requests for Prometheus scraping.
func MetricsMiddleware() gin.HandlerFunc {
	return defaultHTTPMetrics.Middleware()
}

