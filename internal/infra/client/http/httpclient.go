package http

import (
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Option 配置项
type Option struct {
	Timeout         time.Duration
	TraceEnabled    bool
	MaxIdlePerHost  int
	IdleConnTimeout time.Duration
}

// New 创建带可选链路追踪的 http.Client
func New(opt Option) *http.Client {
	// 默认 Transport
	baseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: opt.MaxIdlePerHost,
		IdleConnTimeout:     opt.IdleConnTimeout,
	}

	var tr http.RoundTripper = baseTransport

	// 如果启用 trace，用 otelhttp 包装 Transport
	if opt.TraceEnabled {
		tr = otelhttp.NewTransport(baseTransport)
	}

	return &http.Client{
		Transport: tr,
		Timeout:   opt.Timeout,
	}
}
