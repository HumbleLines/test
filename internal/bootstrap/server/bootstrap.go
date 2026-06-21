// internal/bootstrap/server/bootstrap.go
package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"trade-gateway/internal/app"
	wiring "trade-gateway/internal/bootstrap/wiring" // 业务服务集合
	"trade-gateway/internal/config"
	ilog "trade-gateway/internal/infra/log"
	itrace "trade-gateway/internal/infra/trace"
	grpcserver "trade-gateway/internal/server/grpc"
	httpbootstrap "trade-gateway/internal/server/http"
)

type App struct {
	Cfg           *config.Cfg
	Logger        *ilog.Logger
	Deps          app.AppDepend
	Services      *wiring.Set
	HTTPSrv       *http.Server
	GRPCSrv       *grpc.Server // 标准库的 *grpc.Server
	GRPCLis       net.Listener
	shutdownTrace func(ctx context.Context) error
}

// Bootstrap 日志 → Trace → AppDepend → 业务服务集合 → HTTP/GRPC
// 传入已加载好的 cfg（保持与你现有 main.go 一致的调用方式）
func Bootstrap(ctx context.Context, cfg *config.Cfg) (*App, error) {
	// 日志
	logger := ilog.NewLogger(cfg.App.Env, cfg.Logging.Level, cfg.Logging.Console, cfg.Logging.FilePath)

	// Trace（OTLP → Tempo）
	shutdownTrace, err := itrace.Init(ctx, itrace.Config{
		Enabled:      cfg.Tracing.Enabled,
		Exporter:     cfg.Tracing.Exporter,
		Endpoint:     cfg.Tracing.Endpoint,
		Insecure:     cfg.Tracing.Insecure,
		SampleRatio:  cfg.Tracing.SampleRatio,
		ServiceName:  cfg.App.Name,
		Env:          cfg.App.Env,
		ResourceAttr: cfg.Tracing.ResourceAttr,
	})
	if err != nil {
		_ = logger.Sync()
		return nil, err
	}

	//  依赖容器
	deps, err := app.BuildContainer(ctx, cfg, logger)
	if err != nil {
		_ = shutdownTrace(context.Background())
		_ = logger.Sync()
		return nil, err
	}

	// 业务服务集合（单例）
	set := wiring.NewSet(deps)

	// HTTP
	router := httpbootstrap.BuildHTTPServer(httpbootstrap.Deps{
		Config:  cfg,
		Logger:  logger,
		AppDeps: deps,
	})
	httpSrv := &http.Server{
		Addr:    cfg.HTTP.Addr,
		Handler: router,
	}

	// gRPC
	grpcSrv, grpcLis, err := grpcserver.Build(cfg, logger, deps) // (*grpc.Server, net.Listener, error)
	if err != nil {
		_ = shutdownTrace(context.Background())
		_ = logger.Sync()
		return nil, err
	}

	return &App{
		Cfg:           cfg,
		Logger:        logger,
		Deps:          deps,
		Services:      set,
		HTTPSrv:       httpSrv,
		GRPCSrv:       grpcSrv,
		GRPCLis:       grpcLis,
		shutdownTrace: shutdownTrace,
	}, nil
}

// Start 并发启动 HTTP 与 gRPC
func (a *App) Start() {
	if a.Cfg.HTTP.Addr != "" {
		// HTTP
		go func() {
			a.Logger.Info("http server listen", zap.String("addr", a.Cfg.HTTP.Addr))
			if err := a.HTTPSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				a.Logger.Error("http server error", zap.Error(err))
			}
		}()
	} else {
		a.Logger.Info("http server listen not enabled", zap.String("addr", a.Cfg.HTTP.Addr))
	}

	if a.Cfg.GRPC.Addr != "" {
		// gRPC
		go func() {
			a.Logger.Info("grpc server listen", zap.String("addr", a.Cfg.GRPC.Addr))
			if err := a.GRPCSrv.Serve(a.GRPCLis); err != nil {
				a.Logger.Error("grpc serve error", zap.Error(err))
			}
		}()
	} else {
		a.Logger.Info("grpc server listen not enabled", zap.String("addr", a.Cfg.GRPC.Addr))
	}
}

// Stop 优雅关闭 HTTP 与 gRPC，并刷新 Trace/日志
func (a *App) Stop(ctx context.Context) {
	_ = a.HTTPSrv.Shutdown(ctx)
	a.GRPCSrv.GracefulStop()
	a.Deps.CloseKafkaWriters()
	c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = a.shutdownTrace(c)
	_ = a.Logger.Sync()
}
