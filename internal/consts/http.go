package consts

// 与 HTTP 相关的稳定标识符
const (
	APIV1Prefix = "/api/v1"          // 统一的 API 前缀
	MIMEJSON    = "application/json" // 常见 MIME
	MIMETEXT    = "text/plain; charset=utf-8"

	HeaderRequestID = "X-Request-Id" // 请求 ID 头
	HeaderTraceID   = "X-Trace-Id"   // Trace ID 头（可选）
)
