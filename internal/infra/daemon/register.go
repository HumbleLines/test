package daemon

import (
	"trade-gateway/internal/bootstrap/worker"
	"trade-gateway/internal/process"
)

// RegisterAllRunners 统一注册所有自定义 Runner
// - app: 包含依赖的 App（DB/Redis/Logger/...）
// - mgr: process.Manager，调用方负责 StartAll/StopAll
func RegisterAllRunners(app *worker.App, mgr *process.Manager) {
	// 注册 redissub
	//mgr.Register(redis_sub.New(app))

	//xxlJob, _ := xxl.BuildRegisteredRunner(context.Background(), app, app.Cfg.Worker.XXL)
	//mgr.Register(xxlJob)

	//mgr.Register(engine.New(app))
	// 以后如果有更多 Runner，也在这里集中注册
}
