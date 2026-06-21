# 项目目录与职责说明（结合 API 与 Worker 实现）

> 本文基于已有**最新目录结构**与现有代码实现编写，旨在让其他同事一眼看懂：每个目录/文件夹**负责什么**、**谁依赖谁**、以及在 **API 服务** 与 **Worker 服务** 中如何共用中间件与基础设施（HTTP 客户端、MySQL、PostgreSQL、Redis、Kafka、ClickHouse、Nacos、XXL-Job、日志、Tracing 等）。

---

## 总体结构（简版回顾）

```
D:.
├─cmd/{migrate,server,worker}
├─configs
├─deploy/{docker/{clickhouse/init,configs,grafana-data/{csv,pdf,plugins,png}},k8s}
├─docs
├─internal
│  ├─app
│  ├─bootstrap/{server,wiring,worker}
│  ├─config
│  ├─consts
│  ├─infra
│  │  ├─clickhouse
│  │  ├─client/{grpc,http/codec/okex}
│  │  ├─daemon
│  │  ├─gopool
│  │  ├─log
│  │  ├─metrics
│  │  ├─mq/kafka
│  │  ├─mysql
│  │  ├─nacos
│  │  ├─notifier/{lark,wecom}
│  │  ├─postgres
│  │  ├─redis
│  │  ├─trace
│  │  └─xxl
│  ├─models/{clickhouse,mysql,postgres,redis}
│  ├─pkg/{backoff,errs,format,xcontext,xsql}
│  ├─process/runners/{daemon/redis_sub,mq/kafka,xxl}
│  ├─server/{grpc/handlers,http/{controllers/v1,middleware,request,response,router,validator}}
│  └─services
├─migrations/{clickhouse,mysql,postgres}
└─runtime/{nacoscache/config/encrypted-data-key,nacoslog}
```

---

## 顶层目录说明

### `cmd/`

* **server/**：HTTP 服务入口（API）。初始化配置与依赖，启动 Gin 引擎，挂载路由，监听停止信号，优雅关停。
* **worker/**：Worker 服务入口。加载配置 → `bootstrap/worker` 初始化 → 构建 **Manager** → 注册并启动三类 Runner（**XXL**、**Daemon**、**Kafka**）→ 等信号优雅停机。
* **migrate/**：数据库迁移或初始化入口（如需）。

### `configs/`

* 配置文件目录（本地 YAML、Nacos bootstrap 等）。
* **注意**：敏感信息建议通过环境变量 / Nacos 加密数据键存储。

### `deploy/`

* **docker/**：本地一键起环境（ClickHouse 初始化脚本、Grafana 数据等）。
* **k8s/**：容器化与 K8S 配置（这个需要运维配合）。

### `docs/`

* 文档与架构图（本指南建议放入此处，或链接到 Wiki或者Lark文档）。

### `migrations/`

* 各存储的 DDL/迁移脚本（`clickhouse/`、`mysql/`、`postgres/`）。

### `runtime/`

* **nacoscache/**：Nacos 客户端本地缓存（可清理）。
* **nacoslog/**：Nacos 客户端日志（排障用，可按需轮转/清理）。

---

## `internal/` 目录详解

> 规则：**对外不可见**的私有实现全部放 `internal/`。API 与 Worker 共享同一套基础设施与装配模式。

### 1) `internal/app/`

* **职责**：聚合运行期环境（`Deps` + `Services`），作为**依赖注入容器**面向业务暴露：

    * `Deps`：基础设施（各类实例按**实例名**获取，比如 `MySQL("master")/Postgres("slave")/Redis("default")`、HTTP 客户端、Kafka Producer、Logger、Go 协程池、Notifier、Tracer 等）。
    * `Services`：业务服务集合（面向控制器/API 或 Runner/任务）。
* **被谁使用**：

    * API：在 `bootstrap/server` 装配后放入 Gin Context（见 `xcontext.MustApp(c)`）。
    * Worker：`bootstrap/worker` 返回 `*worker.App`，直接用于注册 Runner 与任务。

### 2) `internal/bootstrap/`

* **server/**：构建 HTTP Server 所需依赖（引擎、路由注册、全局中间件、观测性等）。
* **worker/**：构建 Worker 运行环境（Deps/Services 装配），返回可供 Runner 使用的 `*worker.App`。
* **wiring/**：**服务集合**装配（`wiring.Set`），把 `app.AppDepend` 注入到每个 **Service** 中，集中管理业务服务依赖。

### 3) `internal/config/`

* 配置解析与合并（本地 YAML / Nacos），提供 `ResolveServer/ResolveWorker` 等方法。

### 4) `internal/consts/`

* 系统常量（如 `/api/v1` 前缀、OTEL 属性键等）。

### 5) `internal/infra/`

> **基础设施层**：为上层屏蔽第三方 SDK 细节、实例化与观测性。

* **clickhouse/**：官方驱动封装、Exec/Batch 适配、指标与 Trace。
* **client/**：

    * **http/**：统一 HTTP 客户端（可配置超时/重试/日志脱敏/curl 输出）；`codec/okex` 为第三方 API 适配示例。
    * **grpc/**：gRPC 客户端封装（含拦截器、trace）。
* **daemon/**：集中注册**自定义守护 Runner**（如 `redis_sub`）。
* **gopool/**：协程池封装，避免 goroutine 无界增长；支持分命名池（`server`/`worker`）。
* **log/**：日志接口适配（`Logger`），提供 `WithContext` 注入 trace_id/span_id/request_id。
* **metrics/**：Prometheus 指标采集。
* **mq/kafka/**：Kafka 生产/消费封装与注册、**trace 透传**（W3C headers Extract）、多 Topic/多消费者路由。
* **mysql/**：MySQL 实例工厂 + `otelsql` 打点。
* **nacos/**：配置中心/服务发现接入，含本地缓存/日志目录。
* **notifier/{lark,wecom}**：告警/通知推送。
* **postgres/**：PostgreSQL（`pgx stdlib` + GORM 驱动）+ `otelsql` 打点。
* **redis/**：`go-redis/v9` + `redisotel` 打点。
* **trace/**：全局 OpenTelemetry 初始化（OTLP/HTTP -> Tempo），统一 Propagator/采样配置。
* **xxl/**：XXL-Job 客户端封装、**任务集中注册**、信号桥接到全局退出通道。

### 6) `internal/models/`

* 各存储的 **ORM/DAO 模型**：

    * **postgres/**：如 `User`（注意保留字需 `TableName() string { return "user" }`）。
    * **mysql/**、**redis/**、**clickhouse/**：对应模型/查询封装。

### 7) `internal/pkg/`

* 纯工具包（无业务状态）：

    * **backoff/**：重试退避策略。
    * **errs/**：错误定义与包装。
    * **format/**：格式化/时间处理等。
    * **xcontext/**：在 Gin/Worker 中获取 `App` 依赖（`xcontext.MustApp(c)`）。
    * **xsql/**：SQL 工具与扫描器。

### 8) `internal/process/`

* **runner.go**：定义标准 Runner 接口。
* **manager.go**：统一托管 Runner 的启动/停止，支持并发关停 + 超时。
* **runners/**：实际 Runner 实现：

    * **daemon/redis_sub/**：自定义守护任务（例：定时 DB/Redis 操作、订阅、扫描等）。
    * **mq/kafka/**：Kafka 消费处理器（例：`Test/Test1`）。
    * **xxl/**：XXL 任务函数（例：`hello.go`）。

### 9) `internal/server/`

* **http/**：

    * **controllers/v1/**：API 控制器（例：`health.go`）。
    * **middleware/**：统一中间件（日志/追踪/限流/CORS/请求ID 等）。
    * **request/**：请求 DTO（带 `binding` 标签）。
    * **response/**：统一响应 Envelope（`OK/BadRequest/Internal`）。
    * **router/**：路由注册（例：`outer.go` 的 `/api/v1` 分组）。
    * **validator/**：参数绑定与校验封装（`Bind/BindQuery`）。
* **grpc/**：gRPC 端点（如有）。

### 10) `internal/services/`

* 业务服务层，**只依赖 `app.AppDepend` 获取资源**，内部决定多实例/主从策略（例如 `HealthService.DBOK/PGOK`）。
* API 控制器与 Worker Runner 均通过 **wiring.Set** 引用这些 Service，实现 **代码复用**。

---

## API 与 Worker 如何共用中间件/依赖

* **统一来源**：`bootstrap/server` 与 `bootstrap/worker` 最终都构建同一套 `App`（Deps + Services）。
* **实例命名**：通过 `deps.<Store>("instanceName")` 选择具体实例，例如：

    * `deps.MySQL("master"|"slave")`、`deps.Postgres("master"|"slave")`
    * `deps.Redis("default")`、`deps.KafkaProducer("default")`
    * `deps.HTTP("default")`、`deps.GRPClient("xxx")`
* **观测性一致**：统一 OpenTelemetry 配置（HTTP/GRPC/DB/Redis/Kafka/ClickHouse），确保 API 与 Worker 的 Trace 能贯通。
* **协程池**：`gopool` 支持按场景划分（API: `server`，Worker: `worker`）。
* **统一错误与日志**：`log.Logger.WithContext(ctx)` 注入 traceId/spanId/requestId；API 使用 `response.*` 封装，Worker 直接结构化日志 + 可选 `notifier`。

---

## 典型调用链（示意）

### A) API：HTTP 请求链

```
Client → Gin Router (/api/v1) → Controller → validator.Bind* → Service
      → (Deps: MySQL/Postgres/Redis/HTTP/GRPC/CK/Kafka)
      → response.OK / Error → Return
```

### B) Worker：XXL 任务链

```
XXL-Admin → XXL Client → demo_hello（集中注册） → Service
         → Redis.Incr / DB.Ping / Kafka.Publish
         → 日志 + Trace → 返回执行结果
```

### C) Worker：Kafka 消费链

```
Kafka Broker → Kafka Runner (consumer) → handler 路由（name/topic/_default）
            → WrapTracing Extract → 业务处理（Service / Deps）
            → 日志 + 指标 + 错误处理
```

### D) Worker：Daemon 守护链

```
Manager.StartAll → redis_sub.Runner.Start → gopool.Go(ticker loop)
                → 每 N 秒执行任务（DB/Redis/HTTP/...）
                → Stop: close(stop) → 等待 done 或 ctx 超时
```

---

## 上手路径（强烈建议）

1. **读 API 示例**：`controllers/v1/health.go` → 理解 `validator/response` 用法与 `h.S.*` 取 Service 的方式。
2. **读 Service 示例**：`services/health.go` → 学会用 `deps` 选择实例（`master/slave/default`）。
3. **跑 Worker**：

    * XXL：在 `infra/xxl/register.go` 查看如何 `Register("demo_hello", ...)`。
    * Kafka：在 `infra/mq/kafka/register.go` 看多消费者/多 Topic 的注册方法。
    * Daemon：在 `process/runners/daemon/redis_sub/sub.go` 看优雅停机模型。
4. **新增功能**：

    * 新 API：按《api.md》步骤；
    * 新 Runner：按《worker.md》模板；
    * 共用依赖：一律从 `App.Depend` 获取，不在控制器/Runner 里直接 new SDK。

---

## 清理与维护

* **Nacos 本地缓存**：`runtime/nacoscache/` 可安全清空（重启后自动恢复）。
* **Nacos 日志**：`runtime/nacoslog/` 建议按大小/时间轮转；问题排查时保留最近窗口即可。
* **配置变更**：优先使用 Nacos 管理；本地 YAML 仅作回退（`ResolveWorker/ResolveServer` 会标注来源）。
* **依赖实例**：通过配置打开/关闭 trace 与 metric；保证实例名稳定，避免与代码中的查找键不一致。

---

> 本文档建议与《api.md》《worker.md》一并放入 `docs/`，供团队统一参考。
