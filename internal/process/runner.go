package process

import "context"

// Runner 每个任务只要实现这俩方法就能被 Manager 管起来。
type Runner interface {
	Name() string
	Start(ctx context.Context) error // 自己起 goroutine，非阻塞返回
	Stop(ctx context.Context) error  // 优雅关停，尊重 ctx 超时
}
