package clickhouse

import (
	"context"
	"trade-gateway/internal/consts"

	ck "github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ExecAdapter 将原生 ck.Conn 适配为 Execer，并在开启时统一打点。
type ExecAdapter struct {
	Conn         ck.Conn
	TraceEnabled bool
	// 可选：实例名/库名，用于 span 属性
	DBName     string // 例如 "app"
	Instance   string // 例如 "master"
	TracerName string // 例如 "infra.clickhouse"
}

func (a ExecAdapter) Exec(ctx context.Context, query string, args ...any) error {
	if !a.TraceEnabled {
		return a.Conn.Exec(ctx, query, args...)
	}
	tracer := a.TracerName
	if tracer == "" {
		tracer = consts.CKTracerName
	}
	ctx, span := otel.Tracer(tracer).Start(ctx, consts.CKAttributeOperationValue,
		trace.WithAttributes(
			attribute.String(consts.CKAttributeKey, consts.CKAttributeValue),
			attribute.String(consts.CKAttributeOperationKey, consts.CKAttributeOperationValue),
			attribute.String(consts.CKAttributeStatementKey, query),
			attribute.String(consts.AttrDBInstance, a.Instance),
			attribute.String(consts.AttributeDBNameKey, a.DBName),
		),
	)
	defer span.End()
	return a.Conn.Exec(ctx, query, args...)
}

// 本地最小接口 与驱动返回的 batch 接口能力对齐
type nativeBatch interface {
	Append(args ...any) error
	Send() error
	Abort() error
}

// PrepareBatch ExecAdapter 也实现 Batcher（见 exec.go 中的接口）
func (a ExecAdapter) PrepareBatch(ctx context.Context, query string) (Batch, error) {
	raw, err := a.Conn.PrepareBatch(ctx, query) // 返回的是驱动内部的 batch 类型
	if err != nil {
		return nil, err
	}
	nb, ok := any(raw).(nativeBatch) // 用最小接口 鸭子类型 接住，无需引用 ck.Batch
	if !ok {
		// 理论上不会发生；若发生说明驱动 API 变更
		return nil, err
	}
	tracer := a.TracerName
	if tracer == "" {
		tracer = consts.CKTracerName
	}
	return &batchAdapter{
		ctx:        ctx,
		b:          nb,
		query:      query,
		trace:      a.TraceEnabled,
		dbName:     a.DBName,
		instance:   a.Instance,
		tracerName: tracer,
	}, nil
}

// Batch 实现Append 计数、Send 时只打一个 span
type batchAdapter struct {
	ctx        context.Context
	b          nativeBatch
	query      string
	rows       int
	trace      bool
	dbName     string
	instance   string
	tracerName string
}

func (a *batchAdapter) Append(args ...any) error {
	a.rows++
	return a.b.Append(args...)
}

func (a *batchAdapter) Send() error {
	if !a.trace {
		return a.b.Send()
	}
	_, span := otel.Tracer(a.tracerName).Start(
		a.ctx,
		consts.CkBatchSpanName,
		trace.WithAttributes(
			attribute.String(consts.CKAttributeKey, consts.CKAttributeValue),
			attribute.String(consts.CKAttributeOperationKey, consts.CKAttributeBatchOperationValue),
			attribute.String(consts.CKAttributeStatementKey, a.query),
			attribute.String(consts.AttrDBInstance, a.instance),
			attribute.String(consts.AttributeDBNameKey, a.dbName),
			attribute.Int(consts.CKAttributeBatchRowsOperationKey, a.rows),
		),
	)
	defer span.End()
	return a.b.Send()
}

func (a *batchAdapter) Abort() error { return a.b.Abort() }
