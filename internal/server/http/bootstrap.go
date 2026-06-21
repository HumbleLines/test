package httpserver

import (
	"net/http"
	"trade-gateway/internal/server/http/response"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"trade-gateway/internal/app"
	serviceset "trade-gateway/internal/bootstrap/wiring"
	"trade-gateway/internal/config"
	ilog "trade-gateway/internal/infra/log"
	httpmw "trade-gateway/internal/server/http/middleware"
	"trade-gateway/internal/server/http/router"
)

type Deps struct {
	Config  *config.Cfg
	Logger  *ilog.Logger
	AppDeps app.AppDepend
}

// BuildHTTPServer 启动期装配器 HTTP Server（中间件 + 系统路由 + 业务分组）
func BuildHTTPServer(d Deps) *gin.Engine {
	r := gin.New()

	// 链路追踪（全局开关）
	if d.Config.Tracing.Enabled {
		r.Use(otelgin.Middleware(d.Config.App.Name))
	}

	// 日志 & Panic 恢复
	r.Use(httpmw.GinZap(d.Logger), gin.Recovery())

	// Prometheus 指标
	r.Use(httpmw.PrometheusHTTPMetrics())

	// 注入 AppDepend、请求ID
	r.Use(httpmw.InjectAppDeps(d.AppDeps))
	r.Use(httpmw.RequestID())

	// ============ 系统级路由（不进入业务分组） ============
	r.GET("/actuator/health/liveness", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "UP"})

	})
	r.GET("/actuator/health/readiness", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "UP"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	// ============ 业务分组注册 ============
	// 由 serviceset 组装业务服务（单例），再交给 router 注册
	set := serviceset.NewSet(d.AppDeps)

	// 对外 API（/api/v1/...）
	router.RegisterOuterRoutes(r, set, d.Config)
	// 内部接口（/internal/...），可选
	router.RegisterInnerRoutes(r, set, d.Config)
	return r
}
