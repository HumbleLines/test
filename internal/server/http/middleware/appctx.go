package middleware

import (
	"trade-gateway/internal/consts"

	"github.com/gin-gonic/gin"

	"trade-gateway/internal/app"
)

// InjectAppDeps 将 AppDepend 放进 gin.Context，后续 handler 里可随取随用
func InjectAppDeps(deps app.AppDepend) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(consts.CtxKeyAppDeps, deps)
		c.Next()
	}
}
