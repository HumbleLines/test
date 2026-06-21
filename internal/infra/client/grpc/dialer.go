package grpcclient

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/resolver"
	manual "google.golang.org/grpc/resolver/manual"
)

// DialOption
/* ======================= 外部可用的 DialOption ======================= */
type DialOption struct {
	// 目标（建议显式 scheme）：
	//   - 直连：passthrough:///127.0.0.1:9090
	//   - DNS：  dns:///svc.ns.svc.cluster.local:9090
	Target  string
	Targets []string // 静态多地址列表（用于本地验证 LB）

	// 连接安全（与 TLS 二选一）
	Insecure bool
	TLS      credentials.TransportCredentials

	// 打开分布式追踪（使用全局 Provider/Propagator）
	TraceEnabled bool

	// 你自己的拦截器（按需，可追加任意多个）
	UnaryInts  []grpc.UnaryClientInterceptor
	StreamInts []grpc.StreamClientInterceptor

	// 负载均衡/服务配置（二选一；若 ServiceConfig 非空则覆盖 Balancer）
	//  - Balancer 取值示例："" | "round_robin" | "pick_first"
	Balancer      string
	ServiceConfig string // 完整 JSON，会覆盖 Balancer

	// keepalive
	Keepalive keepalive.ClientParameters // Time/Timeout/PermitWithoutStream

	// 其他拨号/调用默认项
	UserAgent        string
	MaxRecvMsgSize   int  // >0 生效
	MaxSendMsgSize   int  // >0 生效
	DefaultWaitReady bool // 为所有调用默认 WaitForReady(true)

	// 阻塞到 Ready（等价旧时代 WithBlock）
	Block   bool
	Timeout time.Duration

	// 透传底层原始 DialOptions（可选）
	Extra []grpc.DialOption
}

/* ======================= 内部工具：Targets 静态解析 ======================= */

// 为每次 Dial 生成独立 scheme，避免 builder 状态相互覆盖
var staticSeq int64

func newStaticTarget(addrs []string) string {
	scheme := fmt.Sprintf("static-%d", atomic.AddInt64(&staticSeq, 1))
	b := manual.NewBuilderWithScheme(scheme)

	rs := resolver.State{Addresses: make([]resolver.Address, 0, len(addrs))}
	for _, a := range addrs {
		rs.Addresses = append(rs.Addresses, resolver.Address{Addr: a})
	}
	b.InitialState(rs)

	// 全局注册本次的 builder；由于 scheme 唯一，不会与其他连接冲突
	resolver.Register(b)

	// 目标字符串随便写个 path；manual/static 解析器不关心 path 内容
	return scheme + ":///static"
}

// 当配置 target 没有 scheme 时，回退到 passthrough
func normalizeTarget(t string) string {
	if t == "" {
		return t
	}
	if strings.Contains(t, ":///") || strings.Contains(t, "://") {
		return t
	}
	return "passthrough:///" + t
}

/* ======================= 核心拨号逻辑 ======================= */

// Dial —— 统一拨号入口（grpc.NewClient + 可选阻塞到 Ready）
func Dial(ctx context.Context, opt DialOption) (*grpc.ClientConn, error) {
	var dops []grpc.DialOption

	// 若提供了静态 Targets，则构造一个临时 scheme 作为目标
	target := strings.TrimSpace(opt.Target)
	if len(opt.Targets) > 0 {
		target = newStaticTarget(opt.Targets)
	} else {
		target = normalizeTarget(target)
	}

	// 证书/明文
	switch {
	case opt.TLS != nil:
		dops = append(dops, grpc.WithTransportCredentials(opt.TLS))
	case opt.Insecure:
		dops = append(dops, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// OTel（新版推荐 StatsHandler）
	if opt.TraceEnabled {
		dops = append(dops, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	}

	// 你的自定义拦截器（与 OTel 并存）
	if len(opt.UnaryInts) > 0 {
		dops = append(dops, grpc.WithChainUnaryInterceptor(opt.UnaryInts...))
	}
	if len(opt.StreamInts) > 0 {
		dops = append(dops, grpc.WithChainStreamInterceptor(opt.StreamInts...))
	}

	// keepalive
	if opt.Keepalive.Time != 0 || opt.Keepalive.Timeout != 0 || opt.Keepalive.PermitWithoutStream {
		dops = append(dops, grpc.WithKeepaliveParams(opt.Keepalive))
	}

	// 负载均衡/服务配置
	if opt.ServiceConfig != "" {
		// 指定完整 ServiceConfig（会覆盖 Balancer）
		dops = append(dops, grpc.WithDefaultServiceConfig(opt.ServiceConfig))
	} else if opt.Balancer != "" {
		switch opt.Balancer {
		case "round_robin":
			dops = append(dops, grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`))
		case "pick_first":
			dops = append(dops, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"pick_first"}`))
		}
	}

	// 其他默认项
	if opt.UserAgent != "" {
		dops = append(dops, grpc.WithUserAgent(opt.UserAgent))
	}
	if opt.MaxRecvMsgSize > 0 {
		dops = append(dops, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(opt.MaxRecvMsgSize)))
	}
	if opt.MaxSendMsgSize > 0 {
		dops = append(dops, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(opt.MaxSendMsgSize)))
	}
	if opt.DefaultWaitReady {
		dops = append(dops, grpc.WithDefaultCallOptions(grpc.WaitForReady(true)))
	}

	// 透传
	dops = append(dops, opt.Extra...)

	// 创建连接（非阻塞）
	conn, err := grpc.NewClient(target, dops...)
	if err != nil {
		return nil, err
	}

	// 按需阻塞到 Ready（等价 Dial+WithBlock）
	if opt.Block {
		if opt.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opt.Timeout)
			defer cancel()
		}
		conn.Connect() // 触发连接（注意：无返回值）
		for {
			s := conn.GetState()
			if s == connectivity.Ready {
				break
			}
			// 上下文结束（超时/取消）时返回
			if !conn.WaitForStateChange(ctx, s) {
				_ = conn.Close()
				return nil, ctx.Err()
			}
		}
	}

	return conn, nil
}
