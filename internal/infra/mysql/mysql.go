// Package db internal/infra/db/mysql.go
package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"time"
	"trade-gateway/internal/consts"

	// otelsql：用于对 database/sql 层做 OpenTelemetry 链路追踪的包装
	otelsql "github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"

	// 语义化规范常量（如 db.system = "mysql"）
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	// gorm MySQL 驱动（把已建立的 *sql.DB 交给 gorm 管理）
	gmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	// MySQL 官方驱动（必须导入以注册 database/sql 中的 "mysql" driver）
	// 即使使用 gorm 的 mysql dialector，也建议显式导入，避免在某些版本组合下出现“unknown driver 'mysql'”问题
	_ "github.com/go-sql-driver/mysql"
)

// MySQLOption 用于控制单个 MySQL 实例的初始化参数
// 注意：TraceEnabled 控制“是否为该实例注入链路追踪”；不开启时零侵入、零额外开销
type MySQLOption struct {
	DSN             string        // 连接串，建议包含 parseTime=true&loc=Local，避免时间扫描问题
	TraceEnabled    bool          // 是否对本实例开启链路追踪（按实例开关）
	MaxOpenConns    int           // 最大打开连接数（连接池）
	MaxIdleConns    int           // 最大空闲连接数（连接池）
	ConnMaxLifetime time.Duration // 单连接最大生命周期（避免长连接导致负载不均或中间设备清理）
	InstanceName    string        //用于在 span 上打 db.instance
}

// NewMySQL 根据配置创建一个带（或不带）OTel 链路追踪的 *gorm.DB
// - 当 TraceEnabled=true 时：使用 otelsql.Register 包装原生 driver，所有 SQL 调用都会产生日志/Trace span
// - 当 TraceEnabled=false 时：直接使用原生 "mysql" driver，不产生日志/Trace span
// - 返回的 *gorm.DB 由调用方持有；需要关闭时可通过 gdb.DB() 拿到底层 *sql.DB 执行 Close()
func NewMySQL(ctx context.Context, opt MySQLOption) (*gorm.DB, error) {
	// driverName 默认使用原生 mysql 驱动名
	driverName := "mysql"

	// 如果该实例开启了链路追踪，就注册一个“带 OTel 包装”的 driverName
	// otelsql.Register 会返回一个“新”的 driver name（例如 "otel-mysql-xxxx"），后续 sql.Open 使用它
	if opt.TraceEnabled {
		attrs := []attribute.KeyValue{
			semconv.DBSystemMySQL, // 等价 attribute.String("db.system","mysql")
		}
		if opt.InstanceName != "" {
			attrs = append(attrs, attribute.String(consts.AttrDBInstance, opt.InstanceName))
		}
		// WithAttributes：为该 driver 打的每个 span 增加固定属性（如 db.system=mysql）
		// WithSQLCommenter：将 trace 上下文信息注入 SQL 注释中（方便 DBA 侧排查，生产可按需关闭/采样）
		wrappedName, err := otelsql.Register(
			driverName,
			// 统一附加到所有 span 的属性
			otelsql.WithAttributes(attrs...),
			// 把 trace 上下文注入 SQL 注释（DBA 排查方便；不需要可关）
			otelsql.WithSQLCommenter(true),
			// 控制生成哪些子 span（去噪）
			otelsql.WithSpanOptions(otelsql.SpanOptions{
				// 常见噪声：连接从池里取出时的会话重置
				OmitConnResetSession: true,
				// 省略 conn.prepare（很多场景只关心真正执行）
				OmitConnPrepare: true,
				// 省略 rows 生命周期（查询时的一些零时长 span）
				OmitRows: true,
				// driver.ErrSkip 不算错误（不然日志里会出现“错误”标红）
				DisableErrSkip: true,
				// 只保留关心的方法
				// 说明：
				// - MySQL 写多为 StmtExec（或 ConnExec）
				// - 查询想看的话保留 ConnQuery
				// - 关闭了 GORM 默认事务，可去掉 Tx* 两项
				SpanFilter: func(ctx context.Context, m otelsql.Method, _ string, _ []driver.NamedValue) bool {
					switch m {
					case otelsql.MethodStmtExec, // 预编译语句执行（常见）
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
		if err != nil {
			return nil, err
		}
		driverName = wrappedName
	}

	// 用 database/sql 打开底层连接（注意：这里的 driverName 可能是 otelsql.Register 返回的“包装名”）
	raw, err := sql.Open(driverName, opt.DSN)
	if err != nil {
		return nil, err
	}

	// 连接池参数建议根据你的业务量与数据库承载能力调优
	// - MaxOpenConns：过小会打满；过大可能压垮 DB
	// - MaxIdleConns：适度保留空闲连接，避免突刺延迟
	// - ConnMaxLifetime：建议设置，避免连接无限长导致问题（如负载不均、网关空闲超时）
	if opt.MaxOpenConns > 0 {
		raw.SetMaxOpenConns(opt.MaxOpenConns)
	}
	if opt.MaxIdleConns > 0 {
		raw.SetMaxIdleConns(opt.MaxIdleConns)
	}
	if opt.ConnMaxLifetime > 0 {
		raw.SetConnMaxLifetime(opt.ConnMaxLifetime)
	}

	// 可选的连通性检测（PingContext）：确保初始化阶段就暴露配置/网络问题
	// 注：若服务启动路径中已包含健康检查，这里也可以省略以减少启动时延
	if err := raw.PingContext(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}

	// 将已建立好的 *sql.DB 交给 gorm 使用（避免 gorm 再次根据 DSN 打开一次）
	// 这里不再传 DSN，而是直接传 Conn（即我们上面创建并可能已注入 OTel 的连接）
	gdb, err := gorm.Open(gmysql.New(gmysql.Config{
		Conn: raw, // 使用我们已打开且可观测（或不可观测）的底层连接
	}), &gorm.Config{
		SkipDefaultTransaction: true, // 关闭 GORM 默认事务（避免多余开销）
		// 可按需在这里配置 gorm 的 Logger、命名策略等
	})
	if err != nil {
		_ = raw.Close()
		return nil, err
	}

	return gdb, nil
}
