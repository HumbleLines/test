package xxl

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
	"trade-gateway/internal/config"

	xxlexec "github.com/xxl-job/xxl-job-executor-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var xxlTracer = otel.Tracer("trade-gateway/infra/xxl")

func SetTracerName(name string) { xxlTracer = otel.Tracer(name) }

////////////////////////////////////////////////////////////////////////////////
// Client：对 xxl-job 执行器的轻封装
////////////////////////////////////////////////////////////////////////////////

// Client 适配 runner.XXLClient（注意：这里是接口，不是 *Executor）
type Client struct {
	exec     xxlexec.Executor // 接口：Init/Run/Stop/RegTask
	cfg      config.WorkerXXLCfg
	initOnce sync.Once
}

// ensureInit 适配 Init 方法
func (c *Client) ensureInit() {
	c.initOnce.Do(func() {
		if c.exec != nil {
			c.exec.Init() // 接口方法
		}
	})
}

// Register 将 handler 注册到执行器（按 runner 约定的签名适配）
func (c *Client) Register(name string, traceEnabled bool,
	fn func(ctx context.Context, param string) (string, error),
) {
	c.ensureInit()
	c.exec.RegTask(name, func(execCtx context.Context, req *xxlexec.RunReq) string {
		var p string
		if req != nil {
			p = req.ExecutorParams
		}

		// ========== 追踪关闭：直接调用 ==========
		if !traceEnabled {
			out, err := fn(execCtx, p) // 透传执行器 ctx（包含取消信号）
			if err != nil {
				panic(err) // XXL 用 panic 表示失败
			}
			return out
		}

		// ========== 追踪开启：每次新的根 Trace ==========
		ctx, span := xxlTracer.Start(
			context.Background(),
			"xxl.job/"+name,
			trace.WithNewRoot(),
		)

		// 兜底防 panic：记录后继续向上抛给 XXL
		defer func() {
			if r := recover(); r != nil {
				span.RecordError(fmt.Errorf("panic: %v", r))
				span.SetStatus(codes.Error, "panic")
				span.End()
				panic(r)
			}
		}()

		// 常用属性（cfg 可能为空时注意判空）
		span.SetAttributes(
			attribute.String("xxl.job.name", name),
			attribute.String("xxl.component", c.cfg.Executor.AppName),
			attribute.Int("xxl.param.length", len(p)),
		)

		out, err := fn(ctx, p)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			panic(err)
		}

		span.SetStatus(codes.Ok, "ok")
		span.End()
		return out
	})
}

// RunAsync 启动执行器；额外做一次“信号桥”把 OS 信号转给主流程
func (c *Client) RunAsync(ctx context.Context) error {
	c.ensureInit()

	// 信号桥：即使库内部也监听信号，我们也转发一份给上层
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-sigCh
		c.exec.Stop()       // 先摘除执行器
		ForwardXXLSignal(s) // 再把信号转给主流程
	}()

	errCh := make(chan error, 1)
	go func() { errCh <- c.exec.Run() }()

	select {
	case <-ctx.Done():
		c.exec.Stop()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (c *Client) Shutdown(context.Context) error {
	c.exec.Stop()
	return nil
}

// Setup
func Setup(cfg config.WorkerXXLCfg) (*Client, error) {
	e := xxlexec.NewExecutor(
		xxlexec.ServerAddr(cfg.Addr),
		xxlexec.AccessToken(cfg.Token),
		xxlexec.ExecutorIp(cfg.Executor.IP),
		xxlexec.ExecutorPort(strconv.Itoa(cfg.Executor.Port)),
		xxlexec.RegistryKey(cfg.Executor.AppName),
		xxlexec.SetLogger(NewBeatLogger(30*time.Second)),
	)
	return &Client{exec: e, cfg: cfg}, nil
}

////////////////////////////////////////////////////////////////////////////////
// Runner：实现 process.Runner 接口，供 Manager 统一托管
////////////////////////////////////////////////////////////////////////////////

// Runner 将上面的 Client 适配为“可被 Manager 管理的任务”
type Runner struct {
	name string
	c    *Client

	// 退出协调
	startOnce sync.Once
	stopOnce  sync.Once
	done      chan struct{}
}

// NewRunner 创建一个可托管的 XXL 执行器 Runner
func NewRunner(c *Client, _ config.WorkerXXLCfg) *Runner {
	return &Runner{
		name: "xxl-job",
		c:    c,
		done: make(chan struct{}),
	}
}

// Name 返回任务名称（Manager 打日志会用到）
func (r *Runner) Name() string { return r.name }

// Start 非阻塞启动执行器（内部自起 goroutine）
func (r *Runner) Start(ctx context.Context) error {
	r.startOnce.Do(func() {
		go func() {
			defer close(r.done)
			// RunAsync 会阻塞直到 ctx 取消或执行器返回
			_ = r.c.RunAsync(ctx)
		}()
	})
	return nil
}

// Stop 优雅关停，尊重 ctx 超时
func (r *Runner) Stop(ctx context.Context) error {
	r.stopOnce.Do(func() {
		_ = r.c.Shutdown(ctx) // 触发执行器 Stop
	})
	select {
	case <-r.done: // 等待后台 goroutine 退出
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// NewWithConfig 一步到位（从配置创建 Client，并返回 Runner）
func NewWithConfig(cfg config.WorkerXXLCfg) (*Runner, error) {
	c, err := Setup(cfg)
	if err != nil {
		return nil, err
	}
	return NewRunner(c, cfg), nil
}

func (r *Runner) Client() *Client { return r.c }

var (
	sigBridgeMu sync.RWMutex
	sigBridgeCh chan<- os.Signal
)

// BridgeSignalTo 注册一个用于接收 XXL 执行器转发信号的通道（可选）。
// 如果不调用，ForwardXXLSignal 将是安全的 no-op。
func BridgeSignalTo(ch chan<- os.Signal) {
	sigBridgeMu.Lock()
	sigBridgeCh = ch
	sigBridgeMu.Unlock()
}

// ForwardXXLSignal 将收到的信号非阻塞地转发到主流程注册的通道。
// 若未注册或通道阻塞，则静默丢弃（避免卡住执行器退出流程）。
func ForwardXXLSignal(s os.Signal) {
	sigBridgeMu.RLock()
	ch := sigBridgeCh
	sigBridgeMu.RUnlock()
	if ch == nil {
		return // 未注册则 no-op
	}
	select {
	case ch <- s:
	default:
		// 主流程未及时消费，避免阻塞，这里静默丢弃
	}
}
