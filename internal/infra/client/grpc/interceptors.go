// Package grpcclient internal/infra/client/grpc/interceptors.go
package grpcclient

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// SpanAttrsUnary 给当前调用的 span 打一些固定属性（可按需在 Dial() 里 append）
func SpanAttrsUnary(attrs ...attribute.KeyValue) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if sp := trace.SpanFromContext(ctx); sp != nil {
			sp.SetAttributes(attrs...)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
