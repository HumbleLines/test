// Package postgres internal/infra/postgres/postgres.go
// 说明：使用 pgx 的 stdlib 驱动把 PostgreSQL 暴露为 database/sql，
//
//	再用 otelsql 做链路追踪；最后用 gorm 的 postgres 驱动包装成 *gorm.DB。
package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"time"
	"trade-gateway/internal/consts"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"

	// pgx 的 database/sql 兼容层（驱动名为 "pgx"）
	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// PGOption PostgreSQL 实例配置
type PGOption struct {
	DSN             string        // 连接串：postgres://user:pass@host:5432/db?sslmode=disable
	TraceEnabled    bool          // 是否开启链路追踪（开启后每条 SQL 都会打 span）
	MaxOpenConns    int           // 连接池：最大连接数
	MaxIdleConns    int           // 连接池：最大空闲连接数
	ConnMaxLifetime time.Duration // 连接生命周期
	// 可选：用于打点区分实例（建议传 name）
	InstanceName string
}

// New 创建 *gorm.DB；若开启 Trace，则用 otelsql 打点
func New(ctx context.Context, opt PGOption) (*gorm.DB, error) {
	var (
		sqldb *sql.DB
		err   error
	)

	if opt.TraceEnabled {
		sqldb, err = otelsql.Open(
			consts.PGDriverName,
			opt.DSN,
			otelsql.WithAttributes(
				attribute.String(consts.PGAttributeSystemKey, consts.PGAttributeSystemValue),
				// 没有就删掉这行
				attribute.String(consts.AttrDBInstance, opt.InstanceName),
			),
			// 把 trace 信息注入 SQL 注释，便于在数据库侧排查
			otelsql.WithSQLCommenter(true),
			// 关键：控制生成哪些子 span
			otelsql.WithSpanOptions(otelsql.SpanOptions{
				// 常见噪声：连接从池里取出时的会话重置
				OmitConnResetSession: true,
				// 省略 conn.prepare（很多场景只关心真正执行）
				OmitConnPrepare: true,
				// 省略 rows 生命周期（查询时的一些零时长 span）
				OmitRows: true,
				// driver.ErrSkip 不算错误（不然日志里会出现“错误”标红）
				DisableErrSkip: true,

				// 进一步精简：只保留关心的方法
				// 说明：
				// - MySQL 写多为 StmtExec（或 ConnExec）
				// - 查询想看的话保留 ConnQuery
				// - 如果关闭了 GORM 默认事务，可去掉 Tx* 两项
				SpanFilter: func(ctx context.Context, m otelsql.Method, _ string, _ []driver.NamedValue) bool {
					switch m {
					case otelsql.MethodStmtExec, // 预编译语句执行（常见）
						otelsql.MethodConnExec,  // 直连执行（偶见）
						otelsql.MethodConnQuery, // 需要观测查询时打开
						otelsql.MethodTxCommit,  // 需要观测事务提交时打开
						otelsql.MethodTxRollback:
						return true
					default:
						return false
					}
				},
			}),
		)
	} else {
		sqldb, err = sql.Open(consts.PGDriverName, opt.DSN)
	}
	if err != nil {
		return nil, err
	}

	// 连接池参数
	if opt.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(opt.MaxOpenConns)
	}
	if opt.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(opt.MaxIdleConns)
	}
	if opt.ConnMaxLifetime > 0 {
		sqldb.SetConnMaxLifetime(opt.ConnMaxLifetime)
	}

	// 预热：确认可连接
	if err := sqldb.PingContext(ctx); err != nil {
		return nil, err
	}

	// 用已打开的 *sql.DB 创建 gorm
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqldb}), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		return nil, err
	}
	return gdb, nil
}
