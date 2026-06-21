package clickhouse

import "context"

// Execer CK单条执行
type Execer interface {
	Exec(ctx context.Context, query string, args ...any) error
}

// Batcher CK 批量相关接口
type Batcher interface {
	PrepareBatch(ctx context.Context, query string) (Batch, error)
}

type Batch interface {
	Append(args ...any) error
	Send() error
	Abort() error
}
