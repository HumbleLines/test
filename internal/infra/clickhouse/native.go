// Package clickhouse internal/infra/clickhouse/native.go
// 作用：封装 ClickHouse “原生协议”连接（tcp:9000），
package clickhouse

import (
	"context"
	"time"
	"trade-gateway/internal/consts"

	ch "github.com/ClickHouse/clickhouse-go/v2" // 官方 v2 驱动
)

// NativeOption —— 原生协议实例配置
type NativeOption struct {
	DSN             string        // 连接串：如 tcp://127.0.0.1:9000?database=app&username=app&password=xxx
	TraceEnabled    bool          // 是否开启链路追踪
	MaxOpen         int           // （可选）最大连接数
	MaxIdle         int           // （可选）最大空闲连接数
	ConnMaxLifetime time.Duration // （可选）连接最大生命周期
	Instance        string        // 例如 "master"
	TracerName      string        // 例如 "infra.clickhouse"
}

// Native —— 原生协议连接封装
type Native struct {
	Conn         ch.Conn // 底层连接
	TraceEnabled bool    // 是否开启追踪
	DBName       string  // 从 DSN 解析，如没配则 default
	Instance     string
	TracerName   string
}

// NewNative —— 根据 DSN 建立原生协议连接
func NewNative(ctx context.Context, opt NativeOption) (*Native, error) {
	var dbName string
	cfg, err := ch.ParseDSN(opt.DSN)
	if err != nil {
		return nil, err
	}
	conn, err := ch.Open(cfg)
	if err != nil {
		return nil, err
	}
	dbName = cfg.Auth.Database
	if opt.TracerName == "" {
		opt.TracerName = consts.CKTracerName
	}

	return &Native{
		Conn:         conn,
		TraceEnabled: opt.TraceEnabled,
		DBName:       dbName,
		Instance:     opt.Instance,
		TracerName:   opt.TracerName,
	}, nil
}

// Exec —— 执行 SQL 语句
func (n *Native) Exec(ctx context.Context, query string, args ...any) error {
	return n.Execer().Exec(ctx, query, args...)
}

func (n *Native) Execer() Execer {
	return ExecAdapter{
		Conn:         n.Conn,
		TraceEnabled: n.TraceEnabled,
		DBName:       n.DBName,
		Instance:     n.Instance,
		TracerName:   n.TracerName,
	}
}
