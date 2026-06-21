# go-boilerplate（API 开发手册）

> 本文完全基于现有代码结构编写，涵盖：**路由 → 控制器 → Service → wiring 映射 → DTO/Validator/Response → 启动与调试**。
> 所有路径、命名、函数名均与项目一致，可直接照搬。

---

## 📂 目录结构定位

* 路由：`internal/server/http/router/outer.go`
* 控制器：

    * 基类：`internal/server/http/controllers/v1/base.go`
    * Health 控制器：`internal/server/http/controllers/v1/health.go`
* Service：`internal/services/health.go`
* Wiring：`internal/bootstrap/wiring/set.go`
* 统一层：

    * DTO：`internal/server/http/request`
    * 校验：`internal/server/http/validator`
    * 响应：`internal/server/http/response`
    * 上下文依赖：`internal/pkg/xcontext`

---

## 一、路由注册（outer.go）

```go
package router

import (
    "github.com/gin-gonic/gin"

    serviceset "trade-gateway/internal/bootstrap/wiring"
    "trade-gateway/internal/config"
    "trade-gateway/internal/consts"
    v1 "trade-gateway/internal/server/http/controllers/v1"
)

// RegisterOuterRoutes 对外 API 分组（/api/v1）
func RegisterOuterRoutes(r *gin.Engine, set *serviceset.Set, _ *config.Cfg) {
    h := v1.NewHandlers(set)

    api := r.Group(consts.APIV1Prefix)
    {
        api.GET("/health/ping", h.HealthPing)
        // 以下按需开放
        // api.GET("/health/db", h.HealthDB)
        // api.GET("/health/pg", h.HealthPG)
        api.POST("/health/test", h.HealthTest)
    }
}
```

> ✅ 建议：所有新增模块均以 `/api/v1` 分组挂载。

---

## 二、控制器

### 1️⃣ base.go

```go
package v1

import serviceset "trade-gateway/internal/bootstrap/wiring"

type Handlers struct {
    S *serviceset.Set
}

func NewHandlers(s *serviceset.Set) *Handlers { return &Handlers{S: s} }
```

---

### 2️⃣ health.go

```go
package v1

import (
    "fmt"
    "trade-gateway/internal/pkg/xcontext"
    "trade-gateway/internal/server/http/request"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"

    "trade-gateway/internal/server/http/response"
    "trade-gateway/internal/server/http/validator"
)

// GET /api/v1/health/ping
func (h *Handlers) HealthPing(c *gin.Context) {
    c.String(http.StatusOK, "pong")
}

// GET /api/v1/health/db
func (h *Handlers) HealthDB(c *gin.Context) {
    if err := h.S.Health.DBOK(c.Request.Context()); err != nil {
        response.Internal(c, "db down", gin.H{"error": err.Error()})
        return
    }
    response.OK(c, gin.H{"db": "ok"})
}

// GET /api/v1/health/pg?mode=fast|full
func (h *Handlers) HealthPG(c *gin.Context) {
    type query struct {
        Mode string `form:"mode" binding:"required,oneof=fast full"`
    }
    var q query
    if ok := validator.BindQuery(c, &q); !ok {
        return
    }
    if err := h.S.Health.PGOK(c.Request.Context()); err != nil {
        response.Internal(c, "pg down", gin.H{"error": err.Error(), "mode": q.Mode})
        return
    }
    response.OK(c, gin.H{"pg": "ok", "mode": q.Mode})
}

// POST /api/v1/health/test
func (h *Handlers) HealthTest(c *gin.Context) {
    var req request.CreateUserReq
    if ok := validator.Bind(c, &req); !ok {
        return
    }

    var q struct {
        Action string `form:"action" binding:"required"`
        Value  int64  `form:"value"`
    }
    if ok := validator.BindQuery(c, &q); !ok {
        return
    }

    deps := xcontext.MustApp(c)
    pool, _ := deps.GoPool("server")
    pool.Go(func() {
        fmt.Println("hello world")
    })

    mysqlRes, mysqlErr := h.S.MYSQLTrace.Insert(c.Request.Context(), req)
    if mysqlErr != nil {
        response.Internal(c, "mysql create user failed", gin.H{"error": mysqlErr.Error()})
        return
    }

    pgsqlRes, pgsqlErr := h.S.PGSQLTrace.Insert(c.Request.Context(), req)
    if pgsqlErr != nil {
        response.Internal(c, "pgsql create user failed", gin.H{"error": pgsqlErr.Error()})
        return
    }

    val, err := h.S.RedisTrace.Incr(c.Request.Context(), q.Action, time.Duration(q.Value)*time.Second)
    if err != nil {
        response.BadRequest(c, "Incr parameters", err.Error())
        return
    }

    if ckErr := h.S.CKTrace.Insert(c.Request.Context(), q.Action, q.Value); ckErr != nil {
        response.Internal(c, "clickhouse insert failed", gin.H{"error": ckErr.Error()})
        return
    }

    if resp, err := h.S.HTTPTrace.CallJSON(c.Request.Context()); err != nil {
        response.Internal(c, "http call failed", gin.H{"error": err.Error()})
        return
    } else {
        grpcRes, _ := h.S.NewGRPCDemo.CallB(c.Request.Context(), "hello")
        response.OK(c, gin.H{
            "action":    q.Action,
            "value":     val,
            "mysql_id":  mysqlRes.ID,
            "pgsql_id":  pgsqlRes.ID,
            "grpc_res":  grpcRes,
            "http_res":  resp,
        })
    }
}
```

---

## 三、Service 层（health.go）

```go
package services

import (
    "context"
    "fmt"
    "trade-gateway/internal/app"
    mysqlmodel "trade-gateway/internal/models/mysql"
    pgmodel "trade-gateway/internal/models/postgres"
)

type HealthService struct {
    deps app.AppDepend
}

func NewHealthService(deps app.AppDepend) *HealthService {
    return &HealthService{deps: deps}
}

func (s *HealthService) DBOK(ctx context.Context) error {
    s.deps.Logger().SugarWithContext(ctx).Infof("checking mysql health")
    if db, ok := s.deps.MySQL("master"); ok {
        return mysqlmodel.NewHealthModel(db).Ping(ctx)
    }
    if db, ok := s.deps.MySQL("slave"); ok {
        return mysqlmodel.NewHealthModel(db).Ping(ctx)
    }
    return fmt.Errorf("no mysql instance available")
}

func (s *HealthService) PGOK(ctx context.Context) error {
    s.deps.Logger().WithContext(ctx).Info("checking postgres health")
    if db, ok := s.deps.Postgres("slave"); ok {
        return pgmodel.NewHealthModel(db).Ping(ctx)
    }
    if db, ok := s.deps.Postgres("master"); ok {
        return pgmodel.NewHealthModel(db).Ping(ctx)
    }
    return fmt.Errorf("no postgres instance available")
}
```

---

## 四、依赖装配（set.go）

```go
package wiring

import "trade-gateway/internal/app"
import "trade-gateway/internal/services"

type Set struct {
    RedisTrace  *services.RedisTraceService
    Health      *services.HealthService
    CKTrace     *services.CKTraceService
    MYSQLTrace  *services.MysqlTraceService
    PGSQLTrace  *services.PgsqlTraceService
    HTTPTrace   *services.HTTPDemoService
    NewGRPCDemo *services.GRPCDemoService
}

func NewSet(deps app.AppDepend) *Set {
    return &Set{
        RedisTrace:  services.NewCounterService(deps),
        Health:      services.NewHealthService(deps),
        CKTrace:     services.NewCHTraceService(deps),
        MYSQLTrace:  services.NewMysqlTraceService(deps),
        PGSQLTrace:  services.NewPgsqlTraceService(deps),
        HTTPTrace:   services.NewHTTPDemoService(deps),
        NewGRPCDemo: services.NewGRPCDemoService(deps),
    }
}
```

---

## 五、启动与测试

### 启动

```bash
#  不指定config的话，默认从nacos获取
go run ./cmd/worker  -config ./configs/api.yaml
```

### 测试接口

```bash
curl -i http://127.0.0.1:8080/api/v1/health/ping
# pong

curl --location 'http://localhost:8080/api/v1/health/test?action=test&value=1' \
--header 'Content-Type: application/json' \
--data-raw '{"name":"Tom","age":28,"email":"tom@example.com","tags":["a","b"]}''
```

**返回示例：**

```json
{
  "code": 0,
  "msg": "OK",
  "data": {
    "action": "demo",
    "value": 1,
    "mysql_id": 101,
    "pgsql_id": 55,
    "grpc_res": "...",
    "http_res": "..."
  }
}
```

---

## 六、新人操作 checklist ✅

1. 在 `internal/services` 下新增业务逻辑 Service。
2. 在 `internal/bootstrap/wiring/set.go` 注册新 service。
3. 在 `internal/server/http/controllers/v1` 新增控制器函数。
4. 在 `router/outer.go` 添加路由。
5. 启动 `cmd/server`，用 `curl` 验证。
6. 所有依赖都从 `AppDepend` 获取，禁止直接全局调用。

---

## 七、常见问题

| 问题           | 原因 / 解决方案                                                                                                               |
|--------------|-------------------------------------------------------------------------------------------------------------------------|
| Bind 报错      | DTO 的 `binding` 标签不对；必须有 `json/form`                                                                                    |
| 服务启动报错       | docker compose -f D:\GoWorkspace\go-boilerplate\deploy\docker\docker-compose.data.yaml   up -d  docker启动所需的组件 路径替换成自己的。 |
| trace 不显示    | DB/Redis 操作未 `WithContext(ctx)`                                                                                         |
| MySQL/PG 未连接 | 配置实例名和 wiring 不一致                                                                                                       |
| GoPool 不工作   | 检查 `xcontext.MustApp(c)` 是否返回有效实例                                                                                       |
---
