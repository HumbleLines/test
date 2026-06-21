package notifier

import "context"

// Notifier 对外只暴露 Send(ctx, msg)
type Notifier interface {
	Send(ctx context.Context, msg string) error
}

// Noop 一个空实现（关闭通知时使用）
type Noop struct{}

func (Noop) Send(ctx context.Context, msg string) error { return nil }
