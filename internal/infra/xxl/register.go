package xxl

import (
	"context"
	"trade-gateway/internal/bootstrap/worker"
	"trade-gateway/internal/process/runners/xxl"

	"trade-gateway/internal/config"
)

// BuildRegisteredRunner 在 infra 内部完成 “构建 + 注册任务”，但不启动
func BuildRegisteredRunner(_ context.Context, app *worker.App, cfg config.WorkerXXLCfg) (*Runner, error) {
	r, err := NewWithConfig(cfg)
	if err != nil {
		return nil, err
	}

	// 在 infra 内部集中注册 XXL 任务
	r.Client().Register("task.test", true, xxl.DemoHello(app))

	// 需要更多任务时，都在这里继续 r.Client().Register(...)
	return r, nil
}
