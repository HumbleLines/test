package consumer

import (
	"context"
	"trade-gateway/internal/bootstrap/worker"
	"trade-gateway/internal/consts"
	"trade-gateway/internal/process"

	prockafka "trade-gateway/internal/process/runners/mq/kafka"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

//
// 本文件包含：
// 1) ctx ↔ headers 桥接 + TextMapCarrier（供 OTEL Extract 使用）
// 2) 对业务处理函数的 tracing 包装（可配置开关）
// 3) 注册入口：支持多 topic、多个消费端（多 handler）
//

// ---------- headers ⟷ ctx 桥接 & carrier ----------

type headersCtxKey struct{}

func withHeaders(ctx context.Context, h map[string]string) context.Context {
	return context.WithValue(ctx, headersCtxKey{}, h)
}

func headersFromCtx(ctx context.Context) map[string]string {
	if v := ctx.Value(headersCtxKey{}); v != nil {
		if m, ok := v.(map[string]string); ok {
			return m
		}
	}
	return nil
}

type headerCarrier struct{ h map[string]string }

func (c headerCarrier) Get(key string) string {
	if c.h == nil {
		return ""
	}
	return c.h[key]
}
func (c headerCarrier) Set(_, _ string) {} // 只读
func (c headerCarrier) Keys() []string {
	if c.h == nil {
		return nil
	}
	out := make([]string, 0, len(c.h))
	for k := range c.h {
		out = append(out, k)
	}
	return out
}

func carrierFromCtx(ctx context.Context) propagation.TextMapCarrier {
	return headerCarrier{h: headersFromCtx(ctx)}
}

// ----------  业务消费函数签名 & tracing 包装 ----------

type FuncHandler func(ctx context.Context, key, val string) error

func defaultTracingHandler(fn FuncHandler, tracerName, spanName string, spanConsumer bool) FuncHandler {
	return func(ctx context.Context, key, val string) error {
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrierFromCtx(ctx))
		tr := otel.Tracer(tracerName)
		var opts []trace.SpanStartOption
		if spanConsumer {
			opts = append(opts, trace.WithSpanKind(trace.SpanKindConsumer))
		}
		ctx2, span := tr.Start(ctx, spanName, opts...)
		span.SetAttributes(
			attribute.String(consts.KAAttributeSystemKey, consts.KAAttributeSystemValue),
			attribute.String(consts.KAAttributeOperationKey, consts.KAAttributeOperationValue),
			attribute.String(consts.KAAttributeMessageKey, key),
			attribute.Int(consts.KAAttributeSizeKey, len(val)),
		)
		defer span.End()

		if err := fn(ctx2, key, val); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
		span.SetStatus(codes.Ok, "ok")
		return nil
	}
}

func WrapTracing(fn FuncHandler, enabled bool, tracerName, spanName string, spanConsumer bool) FuncHandler {
	if !enabled {
		return fn
	}
	return defaultTracingHandler(fn, tracerName, spanName, spanConsumer)
}

// ---------- 注册入口（支持多 topic / 多 handler） ----------

// RegisterKafkaWithHandlers Handler 路由规则：
//  1. 优先使用按“消费者名称”注册的 handler：handlers[name]
//  2. 其次按“topic 维度”注册：handlers["topic:<topic>"]
//  3. 兜底：handlers["_default"]（若无则用示例 prockafka.Test(app)）
//
// 使用方式：
//
//	handlers := map[string]kafka.FuncHandler{
//	    "user_consumer":  prockafka.HandleUser(app),              // 按消费者名
//	    "topic:orders":   prockafka.HandleOrder(app),             // 按 topic
//	    "_default":       prockafka.Test(app),                    // 兜底
//	}
//	kafka.RegisterKafkaWithHandlers(app, mgr, handlers)
//
// 仍保留原来的 RegisterKafka（单一默认 handler）：兼容旧用法。
func RegisterKafkaWithHandlers(app *worker.App, mgr *process.Manager, handlers map[string]FuncHandler) {
	for name, c := range app.Cfg.MQ.KafkaConsumers {
		// 解析多个 topic（逗号分隔）
		topicList := splitCSVTopics(c.Topic)

		// 选择 handler
		var h FuncHandler
		if handlers != nil {
			// 1) 按消费者名
			if v, ok := handlers[name]; ok && v != nil {
				h = v
			}
			// 2) 按 topic
			if h == nil {
				for _, t := range topicList {
					if v, ok := handlers["topic:"+t]; ok && v != nil {
						h = v
						break
					}
				}
			}

		}
		if h == nil {
			panic("kafka handler not found")
			return
		}

		// 组装 Runner 并托管
		r := NewKafkaRawRunner(name, c, h)
		mgr.Register(r)
	}
}

// RegisterKafka 兼容旧用法：所有消费者统一用一个默认 handler。
func RegisterKafka(app *worker.App, mgr *process.Manager) {
	RegisterKafkaWithHandlers(app, mgr, map[string]FuncHandler{
		"topic:orders.cmd.test": prockafka.Test(app),
		//"test1": prockafka.Test1(app),
	})
}
