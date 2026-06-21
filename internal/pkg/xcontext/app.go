package xcontext

import (
	"trade-gateway/internal/app"
	"trade-gateway/internal/consts"

	"github.com/gin-gonic/gin"
)

// MustApp 从 gin.Context 里取 AppDepend；取不到直接 panic
func MustApp(c *gin.Context) app.AppDepend {
	v, ok := c.Get(consts.CtxKeyAppDeps)
	if !ok {
		panic("AppDepend not found in gin.Context (did you use InjectAppDeps?)")
	}
	return v.(app.AppDepend)
}
