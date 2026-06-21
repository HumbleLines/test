package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "route", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets, // 0.005 .. 10s
		},
		[]string{"method", "route", "status"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

// PrometheusHTTPMetrics 记录每个 HTTP 请求的数量与时延
func PrometheusHTTPMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath() // 用路由模板，避免高基数: /users/:id
		if route == "" {
			route = c.Request.URL.Path
		} // 兜底（如 404）

		labels := prometheus.Labels{
			"method": c.Request.Method,
			"route":  route,
			"status": strconv.Itoa(c.Writer.Status()),
		}

		httpRequestsTotal.With(labels).Inc()
		httpRequestDuration.With(labels).Observe(time.Since(start).Seconds())
	}
}
