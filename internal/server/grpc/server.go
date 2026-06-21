// Package grpcserver internal/server/grpc/server.go
package grpcserver

import (
	"net"
	ilog "trade-gateway/internal/infra/log"
	"trade-gateway/internal/pkg/format"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"trade-gateway/internal/app"
	"trade-gateway/internal/config"
)

// Build 返回：*grpc.Server 和 net.Listener；由 main 负责 Serve/优雅关停
func Build(cfg *config.Cfg, logger *ilog.Logger, deps app.AppDepend) (*grpc.Server, net.Listener, error) {
	// 监听端口
	lis, err := net.Listen("tcp", cfg.GRPC.Addr)
	if err != nil {
		return nil, nil, err
	}

	var sopts []grpc.ServerOption

	ka := cfg.GRPC.Keepalive
	// 全局tracer provider
	sopts = append(sopts,
		grpc.StatsHandler(
			otelgrpc.NewServerHandler(
				// 显式指定也可以，不写就用全局
				otelgrpc.WithTracerProvider(otel.GetTracerProvider()),
				otelgrpc.WithPropagators(otel.GetTextMapPropagator()),
			),
		),
	)
	// keepalive: ServerParameters
	sopts = append(sopts,
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     format.ParseDur(ka.MaxConnectionIdle),
			MaxConnectionAge:      format.ParseDur(ka.MaxConnectionAge),
			MaxConnectionAgeGrace: format.ParseDur(ka.MaxConnectionAgeGrace),
			Time:                  format.ParseDur(ka.Time),
			Timeout:               format.ParseDur(ka.Timeout),
		}),
	)

	// keepalive: EnforcementPolicy（可选）
	if ka.MinTime != "" || ka.PermitWithoutStream {
		sopts = append(sopts,
			grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
				MinTime:             format.ParseDur(ka.MinTime),
				PermitWithoutStream: ka.PermitWithoutStream,
			}),
		)
	}

	//  创建 server
	srv := grpc.NewServer(sopts...)

	//  注册健康检查 & 反射
	hs := health.NewServer()
	healthpb.RegisterHealthServer(srv, hs)
	reflection.Register(srv)

	// 注册业务服务（Echo）
	//pb.RegisterEchoServiceServer(srv, handlers.NewEchoHandler(deps, logger))

	return srv, lis, nil
}
