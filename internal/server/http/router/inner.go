package router

import (
	"trade-gateway/internal/consts"
	"trade-gateway/internal/server/http/controllers/v1/siristarts"

	"github.com/gin-gonic/gin"

	serviceset "trade-gateway/internal/bootstrap/wiring"
	"trade-gateway/internal/config"
)

// RegisterInnerRoutes 内部/管理接口（举例放 /internal）
// 建议加 IP 白名单/签名/BasicAuth 等中间件
func RegisterInnerRoutes(r *gin.Engine, set *serviceset.Set, cfg *config.Cfg) {
	h := siristarts.NewHandlers(set)
	_ = cfg // 需要时使用
	api := r.Group(consts.APIV1Prefix)

	inner := api.Group("/trade-gateway")
	{
		inner.POST("/balance/account/transfer", h.BalanceAccountTransfer)
		inner.POST("/balance/transfer", h.BalanceTransfer)
		// 示例：内部自检、调试开关等
		inner.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })
	}
}
