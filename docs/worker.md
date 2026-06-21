# go-boilerplate（Worker 开发与运维指南）

> 本文**基于代码**整理，覆盖三类 Worker：
>
> 1. **Daemon 自定义守护进程**（示例：`redis_sub`）
> 2. **Kafka 消费进程**（多消费者、多 Topic、带 Trace 透传）
> 3. **XXL-Job 定时任务**（集中注册、信号桥接、优雅停机）
> 架构
> 还包含：启动方式、优雅关停、如何新增 Runner/任务/消费者、常见问题排查。

---

## 目录与核心文件定位

```
cmd/worker/main.go                      # Worker 入口：加载配置、Bootstrap、注册 Runner、启动/停机
internal/process/runner.go              # Runner 接口（Name/Start/Stop）
internal/process/manager.go             # 统一管理多个 Runner 的启动/停止

# XXL 定时任务
internal/infra/xxl/register.go         # 构建 XXL Runner + 统一注册任务（r.Client().Register）
internal/process/runners/xxl/hello.go   # 任务函数 demo_hello：DB/Redis/Kafka 调用

# Kafka 消费
internal/infra/mq/kafka/register.go     # 注册 Kafka Runner（多消费者/多 Topic，支持 handler 路由与 tracing 包装）
internal/process/runners/mq/kafka/test.go # 示例 handler：Test / Test1

# Daemon 守护进程 示例进程
internal/process/runners/daemon/redis_sub/sub.go  # 自定义守护 Runner：循环任务 + 优雅退出
```

---

## 一、Worker 启动与生命周期

### 启动命令

```bash
# 二选一或同时传：
# -config       本地 worker 配置（YAML）
# -bootstrap    Nacos 连接启动配置（worker-bootstrap.yaml）

go run ./cmd/worker -config ./configs/worker-local.yaml -bootstrap ./configs/worker-bootstrap.yaml
```

### main.go 流程概览

```go
appCfg, from, err := config.ResolveWorker(ctx, cfgFlag, bootstrapFlag)
appEnv, cleanup, err := wboot.Bootstrap(rootCtx, appCfg)    // 初始化依赖与服务集合
lg := appEnv.Deps.Logger()

quit := make(chan os.Signal, 1)
signal.Notify(quit, SIGKILL,SIGQUIT,SIGINT,SIGTERM)
infraxxl.BridgeSignalTo(quit)  // 将 XXL 内部信号也桥接到 quit

mgr := process.NewManager(lg)
xxlRunner, _ := infraxxl.BuildRegisteredRunner(rootCtx, appEnv, appCfg.Worker.XXL)
mgr.Register(xxlRunner)                    // ① XXL Runner

daemon.RegisterAllRunners(appEnv, mgr)    // ② Daemon 组（示例：redis_sub）

infraka.RegisterKafka(appEnv, mgr)        // ③ Kafka 组（示例：test/test1）

_ = mgr.StartAll(rootCtx)                  // 启动所有 Runner（非阻塞）
<-quit                                     // 等待退出信号
_ = mgr.StopAll(shutdownCtx)               // 统一优雅停机（含超时）
```

### 统一 Runner 接口与 Manager 机制

```go
type Runner interface {
    Name() string
    Start(ctx context.Context) error // 注意：内部自起 goroutine，Start 需尽快返回
    Stop(ctx context.Context) error  // 关闭通道/退出循环，尊重 ctx 超时
}
```

* `Manager.StartAll`：为每个 Runner 启独立 goroutine 执行 `Start`。
* `Manager.StopAll`：并发调用每个 Runner 的 `Stop`，`WaitGroup` 等待直至结束或 `ctx` 超时。

> **最佳实践**：每个 Runner 自己持有 `stop`（关闭即停止）与 `done`（退出信号）通道，并在 `Stop` 中等待 `done`，或在 `ctx.Done()` 超时时返回。

---

## 二、XXL-Job 定时任务

### 1) 任务注册位置（集中）

**文件**：`internal/infra/xxl/register.go`

```go
func BuildRegisteredRunner(_ context.Context, app *worker.App, cfg config.WorkerXXLCfg) (*Runner, error) {
    r, err := NewWithConfig(cfg)
    if err != nil { return nil, err }

    // 统一集中在这里注册 XXL 任务：
    r.Client().Register("demo_hello", true, xxl.DemoHello(app))
    // 继续在这里追加：r.Client().Register("<name>", true, xxl.你的任务(app))
    return r, nil
}
```

* **只在一个地方集中注册**，便于排查与审计。
* 第二个参数 `true` 表示是否该定时器需要链路追踪。

### 2) 任务实现（示例）

**文件**：`internal/process/runners/xxl/hello.go`

```go
func DemoHello(app *worker.App) func(ctx context.Context, p string) (string, error) {
    return func(ctx context.Context, p string) (string, error) {
        app.Logger.SugarWithContext(ctx).Infow("xxl demo_hello", "param", p)
        app.Services.Health.DBOK(ctx)                          // 打点 MySQL/PG 健康
        app.Services.RedisTrace.Incr(ctx, "xxl_demo_hello", 222*time.Second)

        if w, ok := app.Deps.KafkaProducer("default"); ok && w != nil {
            err := kafka.Publish(ctx, w, "", `{"hello":"hello"}`, map[string]string{
                "param": p, "at": time.Now().Format(time.RFC3339), "from": "xxl.demo_hello",
            })
            if err != nil { app.Logger.SugarWithContext(ctx).Errorw("kafka publish failed", "err", err) }
        } else {
            app.Logger.SugarWithContext(ctx).Warnw("no kafka writer", "name", "default")
        }
        return "pong", nil
    }
}
```

### 3) XXL-Admin 配置与联调（操作指引）

1. **在 XXL-Admin 新建执行器**：填写你的 worker 地址与心跳（按 `WorkerXXLCfg`）。
2. **新建任务**：

    * 执行器：选择上一步创建的执行器
    * JobHandler：填 `demo_hello`
    * 调度类型：Cron/固定速率/手动触发均可
    * 参数：示例 `param="from_admin"`
3. **联调**：在 Admin 点击**运行**，Worker 日志应出现 `xxl demo_hello`；若配置了 Kafka，则还会打出发布成功/失败日志。
4. **优雅停机**：Worker 收到 `SIGINT/SIGTERM` 时将通过 `BridgeSignalTo` 合流，统一 `StopAll`。

> **排查**：任务注册名必须与 Admin 的 `JobHandler` 一致，否则会 404/找不到处理函数。

---

## 三、Kafka 消费进程

### 1) 注册入口与路由规则

**文件**：`internal/infra/mq/kafka/register.go`

```go
// RegisterKafkaWithHandlers：多消费者/多 Topic 路由
// 1. 先按消费者名匹配：handlers[name]
// 2. 再按 Topic 匹配：handlers["topic:"+topic]
// 3. 最后兜底：handlers["_default"]
func RegisterKafkaWithHandlers(app *worker.App, mgr *process.Manager, handlers map[string]FuncHandler)

// 兼容旧用法（示例为 test/test1 两个消费者名）：
func RegisterKafka(app *worker.App, mgr *process.Manager) {
    RegisterKafkaWithHandlers(app, mgr, map[string]FuncHandler{
        "test":  prockafka.Test(app),
        "test1": prockafka.Test1(app),
    })
}
```

* **多 Topic**：配置里支持逗号分隔，`splitCSVTopics()` 会解析为多个 topic。
* **Tracing**：内置 `WrapTracing(fn, enabled, tracerName, spanName, spanConsumer)` 可为 handler 打上 `SpanKindConsumer`，并通过 `carrierFromCtx` 自动**提取上下游链路信息**（W3C traceparent）。

### 2) Handler 示例

**文件**：`internal/process/runners/mq/kafka/test.go`

```go
func Test(app *worker.App) func(ctx context.Context, key, val string) error {
    return func(ctx context.Context, key, val string) error {
        app.Logger.SugarWithContext(ctx).Infof("test success value:%s", val)
        // TODO: 你的消费逻辑
        return nil
    }
}
```

### 3) 如何自定义你的 Kafka 消费逻辑

1. **写业务处理函数**（推荐放在 `internal/process/runners/mq/kafka/xxx.go`）：

   ```go
   func HandleOrder(app *worker.App) kafka.FuncHandler {
       return func(ctx context.Context, key, val string) error {
           // 解析 val -> 处理订单 app可以拿到其他的中间件信息，比如db或者http
           return nil
       }
   }
   ```
2. **注册到 Runner**（选择一种方式）：

    * 方式 A：在 `RegisterKafka` 里追加：

      ```go
      RegisterKafkaWithHandlers(app, mgr, map[string]kafka.FuncHandler{
          "orders_consumer": prockafka.HandleOrder(app),
      })
      ```
    * 方式 B：直接调用 `RegisterKafkaWithHandlers(app, mgr, handlers)`，并在 `handlers` 里按**消费者名**或**topic**路由。
3. **配置消费者**（示例 YAML 片段）：

   ```yaml
   mq:
     kafkaConsumers:
       orders_consumer:
         brokers: ["127.0.0.1:9092"]
         topic: "orders,orders_dlq"   # 逗号分隔多个 topic
         groupID: "orders_group"
   ```

> **排查**：若未找到匹配 handler，会触发 `panic("kafka handler not found")` —— 请检查消费者名/Topic 与注册表是否一致。

---

## 四、Daemon 自定义守护 Runner

### 1) 示例：`redis_sub` 守护进程

**文件**：`internal/process/runners/daemon/redis_sub/sub.go`

```go
type Runner struct {
    app  *worker.App
    stop chan struct{}
    done chan struct{}
}

func New(app *worker.App) *Runner { return &Runner{app: app, stop: make(chan struct{}), done: make(chan struct{})} }
func (r *Runner) Name() string { return "daemon:redis_sub" }

func (r *Runner) Start(ctx context.Context) error {
    pool, _ := r.app.Deps.GoPool("worker")
    pool.Go(func() {
        defer close(r.done)
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done(): return
            case <-r.stop:     return
            case <-ticker.C:
                r.app.Services.Health.DBOK(ctx)
                fmt.Println("Runner is running...")
            }
        }
    })
    return nil
}

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
```

* **关键点**：

    * `Start` 内部起 goroutine，快速返回；
    * 用 `ticker` 节流定时任务，或替换为阻塞式订阅/拉取；
    * `Stop` 仅需关闭 `stop`，并等待 `done` 即可；
    * 使用 `GoPool("worker")` 托管 goroutine，避免无界增长。

### 2) 如何新增一个 Daemon Runner（模板）

```bash
# 建议目录
internal/process/runners/daemon/<your_task>/your_task.go
```

```go
func New(app *worker.App) *Runner { /* 与示例一致 */ }
func (r *Runner) Name() string { return "daemon:<your_task>" }
func (r *Runner) Start(ctx context.Context) error { /* select ctx.Done()/r.stop + ticker/阻塞循环 */ }
func (r *Runner) Stop(ctx context.Context) error  { /* close(r.stop); wait r.done or ctx timeout */ }
```

在 **集中注册处**（如 `internal/infra/daemon/register.go`）里：

```go
func RegisterAllRunners(app *worker.App, mgr *process.Manager) {
    mgr.Register(redis_sub.New(app))
    // mgr.Register(<your_task>.New(app))
}
```

---

## 五、如何新增能力（Cheatsheet）

### A. 新增一个 XXL 任务

1. `internal/process/runners/xxl/` 下新增任务函数：`func YourJob(app *worker.App) func(ctx context.Context, p string) (string, error)`。
2. 在 `internal/infra/xxl/register.go` 里追加：`r.Client().Register("your_job", true, xxl.YourJob(app))`。
3. 在 **XXL-Admin** 里创建同名 `JobHandler`（`your_job`），配置调度与参数。

### B. 新增一个 Kafka 消费者

1. 写 handler：`func HandleX(app *worker.App) kafka.FuncHandler`。
2. 在 `RegisterKafka` 或 `RegisterKafkaWithHandlers` 中注册与消费者名/Topic 的映射。
3. 在配置里新增对应消费者（brokers/topic/groupID 等）。

### C. 新增一个 Daemon Runner

1. 创建 `internal/process/runners/daemon/<name>/` 目录并实现 `Runner` 接口。
2. 在 `RegisterAllRunners` 集中注册。

---

## 六、观测性与健壮性建议

* **上下文贯穿**：所有外部调用（DB/Redis/HTTP/Kafka）务必 `WithContext(ctx)`，以串联 Trace。
* **Kafka Trace**：使用 `WrapTracing` 包装 handler，可记录 Consumer Span；确保上下游通过 header 传递 W3C traceparent。
* **优雅关停**：所有循环必须 `select { case <-ctx.Done(): ... }`；避免 `time.Sleep` 死等。
* **错误处理**：在 handler/任务中记录结构化日志（`SugarWithContext`），必要时上报告警（飞书/企微）。
* **panic 防护**：核心 goroutine 外层加 `defer recover`（可在 GoPool 统一处理）。

---

## 七、常见问题排查（FAQ）

| 现象                                  | 可能原因                              | 处理建议                                                            |
| ----------------------------------- | --------------------------------- | --------------------------------------------------------------- |
| Worker 启动后无任务执行                     | 未注册 Runner / XXL 任务未注册            | 确认 `BuildRegisteredRunner` 已 `Register`，`mgr.Register(...)` 已调用 |
| XXL 触发报 404/找不到 Handler             | JobHandler 名与注册名不一致               | 管理台 `JobHandler` 必须与 `Register("name", ...)` 中的 `name` 一致       |
| Kafka 消费报 `kafka handler not found` | 消费者名/Topic 与 handlers 表不匹配        | 检查 `RegisterKafka(WithHandlers)` 的映射是否包含该消费者/Topic              |
| 停机卡住                                | Runner 未关闭内部循环 / 未监听 `ctx.Done()` | 确保 `Stop` 里关闭 `stop` 且 `Start` 中 select 了 `ctx.Done()`          |
| Trace 看不到 Consumer Span             | 未启用 `WrapTracing` 或 headers 未透传   | 使用 `WrapTracing`；检查上游是否注入了 trace headers                        |

---

## 八、最小可运行验证

1. **启动 Worker**：`go run ./cmd/worker -config ./configs/worker.yaml`
2. **XXL 任务**：在 Admin 里创建 `demo_hello`，点击运行，查看 Worker 日志包含 `xxl demo_hello` 与 Kafka 发布日志。
3. **Kafka 消费**：向配置的 Topic 写入一条消息（例如 `echo '{"k":"v"}' | kafkacat -b 127.0.0.1:9092 -t test`），观察日志打印 `test success value:...`。
4. **Daemon**：等待 5s tick，日志应定期输出 `Runner is running...`，且 `Health.DBOK` 打点成功。

> 至此，三类 Worker（Daemon / MQ / XXL）均可按本文档操作使用、扩展与排错。
