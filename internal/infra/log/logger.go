package log

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct{ *zap.Logger }

func levelFromString(s string) zapcore.Level {
	switch strings.ToLower(s) {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn", "warning":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

// NewLogger ：
// - 必备：stdout(JSON) → 供采集器（K8s/容器）用
// - 可选：stderr(Console) → 开发可读/可点击
// - 可选：file(JSON) → 本机 Promtail 采集
func NewLogger(env, level string, console bool, filePath string) *Logger {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "@timestamp"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.MessageKey = "message"
	encCfg.CallerKey = "caller"
	encCfg.EncodeCaller = zapcore.FullCallerEncoder

	lv := levelFromString(level)
	if level == "" {
		if env == "dev" {
			lv = zap.DebugLevel
		} else {
			lv = zap.InfoLevel
		}
	}

	// core1: JSON → stdout
	jsonEnc := zapcore.NewJSONEncoder(encCfg)
	jsonCore := zapcore.NewCore(jsonEnc, zapcore.AddSync(os.Stdout), lv)

	var cores []zapcore.Core
	cores = append(cores, jsonCore)

	// core2: Console → stderr（仅在 console=true 时开启）
	if console {
		consoleEnc := zapcore.NewConsoleEncoder(encCfg)
		consoleCore := zapcore.NewCore(consoleEnc, zapcore.AddSync(os.Stderr), lv)
		cores = append(cores, consoleCore)
	}

	// core3: 写文件（仅当 filePath != ""）
	if filePath != "" {
		_ = os.MkdirAll(filepath.Dir(filePath), 0o755)
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			fileCore := zapcore.NewCore(jsonEnc, zapcore.AddSync(f), lv)
			cores = append(cores, fileCore)
		}
	}

	core := zapcore.NewTee(cores...)
	z := zap.New(core, zap.AddCaller())
	return &Logger{Logger: z}
}

// WithContext 返回一个自动带 trace_id/span_id/request_id 的 *zap.Logger
func (l *Logger) WithContext(ctx context.Context, fields ...zap.Field) *zap.Logger {
	// 1) OTEL trace/span
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		fields = append(fields,
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
		)
	}
	// 2) request_id（如果server有放到 ctx 里）
	if rid := RequestIDFromContext(ctx); rid != "" {
		fields = append(fields, zap.String("request_id", rid))
	}
	return l.Logger.With(fields...)
}

func (l *Logger) Sugar() *zap.SugaredLogger { return l.Logger.Sugar() }

// SugarWithContext 同上，但返回 *zap.SugaredLogger
func (l *Logger) SugarWithContext(ctx context.Context) *zap.SugaredLogger {
	return l.WithContext(ctx).Sugar()
}

// ------ request_id 读取（配合中间件/工具） ------
type ctxKey struct{}                  // 未导出类型，避免冲突
var CtxKeyRequestID ctxKey = ctxKey{} // 导出变量，其他包可用作 key

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(CtxKeyRequestID).(string); ok {
		return v
	}
	return ""
}
