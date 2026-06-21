// 文件：internal/infra/trace/init.go
package trace

import (
	"context"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"trade-gateway/internal/consts"
)

type Config struct {
	Enabled      bool
	Exporter     string // consts.TraceExporterOTLPHTTP / TraceExporterOTLPGRPC / TraceExporterOTLP(=grpc)
	Endpoint     string // 例："127.0.0.1:4318" 或 "http://127.0.0.1:4318"；grpc 用 "127.0.0.1:4317"
	Insecure     bool
	SampleRatio  float64
	ServiceName  string
	Env          string
	ResourceAttr map[string]string
}

func Init(ctx context.Context, c Config) (func(context.Context) error, error) {
	if !c.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	exp, err := newExporter(ctx, c)
	if err != nil {
		return nil, err
	}

	// 资源属性
	resAttrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(c.ServiceName),
		semconv.DeploymentEnvironmentKey.String(c.Env),
	}
	for k, v := range c.ResourceAttr {
		resAttrs = append(resAttrs, attribute.String(k, v))
	}
	res, _ := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, resAttrs...))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp, sdktrace.WithBatchTimeout(consts.SdkTraceWithBatchTimeout)),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(c.SampleRatio))),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	// 重要：全局传播器（A系统 注入，B 提取，这样才能跨服务传播tracer）
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, // traceparent/tracestate
			propagation.Baggage{},      // 可选
		),
	)
	return tp.Shutdown, nil
}

/* ==================== 内部：创建导出器，带端点纠偏 ==================== */

func newExporter(ctx context.Context, c Config) (*otlptrace.Exporter, error) {
	mode := strings.ToLower(strings.TrimSpace(c.Exporter))
	switch mode {
	case consts.TraceExporterOTLPHTTP, "http", "otlp-http":
		ep := normalizeHTTP(c.Endpoint)

		opts := []otlptracehttp.Option{}
		if hasScheme(ep) {
			// 传完整 URL（http/https）
			opts = append(opts, otlptracehttp.WithEndpointURL(ep))
		} else {
			// 传 host:port
			opts = append(opts, otlptracehttp.WithEndpoint(ep))
		}
		if c.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)

	// "otlp" 作为 gRPC 的别名，保持兼容
	case consts.TraceExporterOTLPGRPC, consts.TraceExporterOTLP, "grpc", "otlp-grpc":
		ep := normalizeGRPC(c.Endpoint)

		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(ep), // host:port
		}
		if c.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(ctx, opts...)

	default:
		// 未知值：回退到 HTTP（最通用）
		ep := normalizeHTTP(c.Endpoint)

		opts := []otlptracehttp.Option{}
		if hasScheme(ep) {
			opts = append(opts, otlptracehttp.WithEndpointURL(ep))
		} else {
			opts = append(opts, otlptracehttp.WithEndpoint(ep))
		}
		if c.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)
	}
}

func hasScheme(ep string) bool {
	return strings.HasPrefix(ep, consts.HttpPrefix) || strings.HasPrefix(ep, consts.HttpsPrefix)
}

// HTTP 端点规范化：去掉 /v1/traces；localhost → 127.0.0.1；URL 情况清空 path/query
func normalizeHTTP(ep string) string {
	ep = strings.TrimSpace(ep)
	ep = strings.ReplaceAll(ep, consts.Localhost, consts.LocalhostIP)
	ep = strings.TrimSuffix(ep, consts.TraceSuffix)
	ep = strings.TrimSuffix(ep, consts.TraceSuffixSlash)
	if hasScheme(ep) {
		if u, err := url.Parse(ep); err == nil {
			u.Path = ""
			u.RawQuery = ""
			return u.String()
		}
	}
	return ep // host:port
}

// gRPC 端点规范化：必须 host:port；去掉 http(s)://；localhost → 127.0.0.1
func normalizeGRPC(ep string) string {
	ep = strings.TrimSpace(ep)
	ep = strings.TrimPrefix(ep, consts.HttpPrefix)
	ep = strings.TrimPrefix(ep, consts.HttpsPrefix)
	ep = strings.ReplaceAll(ep, consts.Localhost, consts.LocalhostIP)
	return ep
}
