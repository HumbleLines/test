package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	ilog "trade-gateway/internal/infra/log"
)

// GinZap logs HTTP requests with trace_id/span_id if available.
func GinZap(l *ilog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		span := trace.SpanFromContext(c.Request.Context())
		var traceID, spanID string
		if span != nil {
			if sc := span.SpanContext(); sc.IsValid() {
				traceID = sc.TraceID().String()
				spanID = sc.SpanID().String()
			}
		}

		l.Info("http_request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.FullPath()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency_ms", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.String("trace_id", traceID),
			zap.String("span_id", spanID),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}
