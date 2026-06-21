// Package worker 封装 Worker 进程的启动期装配（配置/日志/Trace/依赖容器/业务服务集合）
package worker

import (
	"context"
	"time"

	"trade-gateway/internal/app"
	wiring "trade-gateway/internal/bootstrap/wiring"
	"trade-gateway/internal/config"
	ilog "trade-gateway/internal/infra/log"
	itrace "trade-gateway/internal/infra/trace"
)

// App 代表 Worker 运行所需的环境对象集合
type App struct {
	Cfg           *config.Cfg                 // 已加载配置
	Logger        *ilog.Logger                // 统一 Logger
	Deps          app.AppDepend               // 依赖容器（MySQL/PG/Redis/ClickHouse/HTTP/GRPC…）
	Services      *wiring.Set                 // 业务服务集合（单例，供 worker 使用）
	shutdownTrace func(context.Context) error // Trace 关闭函数
}

// Bootstrap 根据 cfg 完成：日志 → Trace → 依赖容器 → 业务服务集合 的装配
// 说明：Worker 不在这里启动任何协程；仅返回环境对象与清理函数，便于 cmd/worker 里自行控制生命周期。
func Bootstrap(ctx context.Context, cfg *config.Cfg) (*App, func(), error) {
	// 日志初始化
	logger := ilog.NewLogger(cfg.App.Env, cfg.Logging.Level, cfg.Logging.Console, cfg.Logging.FilePath)

	// Trace（OTLP → Tempo）
	shutdownTrace, err := itrace.Init(ctx, itrace.Config{
		Enabled:      cfg.Tracing.Enabled,
		Exporter:     cfg.Tracing.Exporter,
		Endpoint:     cfg.Tracing.Endpoint,
		Insecure:     cfg.Tracing.Insecure,
		SampleRatio:  cfg.Tracing.SampleRatio,
		ServiceName:  cfg.App.Name + "-worker", // 与 server 区分，便于检索
		Env:          cfg.App.Env,
		ResourceAttr: cfg.Tracing.ResourceAttr,
	})
	if err != nil {
		_ = logger.Sync()
		return nil, nil, err
	}

	// 构建依赖容器（多实例 + 实例级 Trace 开关）
	deps, err := app.BuildContainer(ctx, cfg, logger)
	if err != nil {
		_ = shutdownTrace(context.Background())
		_ = logger.Sync()
		return nil, nil, err
	}

	// 装配业务服务集合（单例）
	set := wiring.NewSet(deps)

	//清理函数（给 cmd/worker 在退出时调用）
	cleanup := func() {

		// 先 flush Trace，再同步日志
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTrace(c)
		_ = logger.Sync()
		deps.CloseKafkaWriters()
		// 如需：在此追加关闭 gRPC 客户端连接、ClickHouse、Redis 等资源
		// 由于这些通常由连接池/驱动自己管理，按需补充即可。
	}

	return &App{
		Cfg:           cfg,
		Logger:        logger,
		Deps:          deps,
		Services:      set,
		shutdownTrace: shutdownTrace,
	}, cleanup, nil
}
