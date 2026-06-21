package chmodel

import (
	"context"
	"errors"
	ckinfra "trade-gateway/internal/infra/clickhouse"
)

type TraceModel struct {
	exec ckinfra.Execer
}

func NewTraceModel(exec ckinfra.Execer) *TraceModel { return &TraceModel{exec: exec} }

// Insert 插入一条链路验证记录（ts、id 由表默认值生成）
func (m *TraceModel) Insert(ctx context.Context, action string, value int64, traceID, spanID string) error {
	return m.exec.Exec(ctx,
		"INSERT INTO app.trace_test (action, value, trace_id, span_id) VALUES (?, ?, ?, ?)",
		action, value, traceID, spanID,
	)
}

// TraceRow 批量 一个批次一个 span（由 adapter 的 Send() 打点）
type TraceRow struct {
	Action  string
	Value   int64
	TraceID string
	SpanID  string
}

func (m *TraceModel) InsertBatch(ctx context.Context, rows []TraceRow) error {
	batcher, ok := any(m.exec).(ckinfra.Batcher)
	if !ok {
		return errors.New("clickhouse execer doesn't implement Batcher")
	}
	b, err := batcher.PrepareBatch(ctx,
		"INSERT INTO app.trace_test (action, value, trace_id, span_id) VALUES (?, ?, ?, ?)",
	)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := b.Append(r.Action, r.Value, r.TraceID, r.SpanID); err != nil {
			_ = b.Abort()
			return err
		}
	}
	return b.Send()
}
