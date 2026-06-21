// Package middleware internal/server/http/middleware/requestid.go
package middleware

import (
	"context"

	"trade-gateway/internal/consts"
	ilog "trade-gateway/internal/infra/log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(consts.HeaderRequestID)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Header(consts.HeaderRequestID, rid)
		ctx := context.WithValue(c.Request.Context(), ilog.CtxKeyRequestID, rid)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
