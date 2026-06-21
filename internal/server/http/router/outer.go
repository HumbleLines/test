package router

import (
	"trade-gateway/internal/server/http/controllers/v1/siristarts"

	"github.com/gin-gonic/gin"

	serviceset "trade-gateway/internal/bootstrap/wiring"
	"trade-gateway/internal/config"
	"trade-gateway/internal/consts"
)

// RegisterOuterRoutes 对外 API 分组（/api/v1）
func RegisterOuterRoutes(r *gin.Engine, set *serviceset.Set, _ *config.Cfg) {
	h := siristarts.NewHandlers(set)
	_ = h
	api := r.Group(consts.APIV1Prefix)
	{
		_ = api
	}
}
