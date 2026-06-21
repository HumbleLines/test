package redis_sub

import (
	"context"
	"fmt"
	"time"
	"trade-gateway/internal/bootstrap/worker"
)

type Runner struct {
	app  *worker.App
	stop chan struct{}
	done chan struct{}
}

func New(app *worker.App) *Runner {
	return &Runner{
		app:  app,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (r *Runner) Name() string { return "daemon:redis_sub" }

// Start 要怎么循环、怎么轮询、多久一次，完全自由发挥 只要遵循实现start和stop接口即可。
func (r *Runner) Start(ctx context.Context) error {
	pool, _ := r.app.Deps.GoPool("worker")
	lark, _ := r.app.Deps.Notifier("lark")
	lark.Send(ctx, "链接断开")
	pool.Go(func() {
		defer close(r.done)
		//notifier, _ := r.app.Deps.Notifier("lark")
		//_ = notifier.Send(ctx, "redis_sub started")
		//panic("test")
		// 模拟panic
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.stop:
				return
			case <-ticker.C:
				// TODO: 业务逻辑：到点处理
				r.app.Services.Health.DBOK(ctx)
				fmt.Println("Runner is running...")
			}
		}
	})
	return nil
}

// Stop 只要关闭 stop，并等 goroutine 退出即可。
func (r *Runner) Stop(ctx context.Context) error {
	close(r.stop)
	select {
	case <-r.done:
		r.app.Logger.Sugar().Infof("%s stopped", r.Name())
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
