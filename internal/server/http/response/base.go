// Package response 统一 HTTP 返回结构与便捷方法
// 提供固定的响应包裹格式，避免各处手写 c.JSON(...)。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope 统一的返回包裹
type Envelope struct {
	Code int    `json:"code"`           // 业务码：0=成功；非 0 表示错误
	Msg  string `json:"msg"`            // 返回说明
	Data any    `json:"data,omitempty"` // 业务数据（可空）
}

// OK 成功返回（HTTP 200，业务码 0）
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{
		Code: 0,
		Msg:  "OK",
		Data: data,
	})
}

// Error 通用错误返回（自定义 HTTP 状态码与业务码）
func Error(c *gin.Context, httpStatus int, bizCode int, msg string, data any) {
	c.JSON(httpStatus, Envelope{
		Code: bizCode,
		Msg:  msg,
		Data: data,
	})
}

// BadRequest 400 的便捷方法（参数错误等）
func BadRequest(c *gin.Context, msg string, data any) {
	Error(c, http.StatusBadRequest, 400100, msg, data)
}

// Internal 500 的便捷方法（服务端异常等）
func Internal(c *gin.Context, msg string, data any) {
	Error(c, http.StatusInternalServerError, 500100, msg, data)
}
