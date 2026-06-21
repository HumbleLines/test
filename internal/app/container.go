package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"os"
	"strings"
	"time"

	"trade-gateway/internal/config"
	"trade-gateway/internal/consts"
	clickhx "trade-gateway/internal/infra/clickhouse"
	grpcclient "trade-gateway/internal/infra/client/grpc"
	ihttp "trade-gateway/internal/infra/client/http"
	"trade-gateway/internal/infra/gopool"
	ilog "trade-gateway/internal/infra/log"
	imysql "trade-gateway/internal/infra/mysql"
	"trade-gateway/internal/infra/notifier"
	"trade-gateway/internal/infra/notifier/lark"
	ipostgres "trade-gateway/internal/infra/postgres"
	iredis "trade-gateway/internal/infra/redis"
	"trade-gateway/internal/pkg/format"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"gorm.io/gorm"
)

// container 实现 AppDepend：保存 实例名 -> 连接/客户端 的所有依赖
type container struct {
	mysql     map[string]*gorm.DB
	pg        map[string]*gorm.DB
	redis     map[string]redis.UniversalClient
	chN       map[string]*clickhx.Native
	httpC     map[string]*ihttp.Client
	grpcC     map[string]*grpc.ClientConn
	pools     map[string]gopool.Pool
	notifiers map[string]notifier.Notifier
	kafkaW    map[string]*kafka.Writer
	logger    *ilog.Logger
	cfg       *config.Cfg
}

// ---- 对外访问器 / 关闭 ----

func (c *container) KafkaProducer(name string) (*kafka.Writer, bool) {
	w, ok := c.kafkaW[name]
	return w, ok
}

func (c *container) KafkaWriter(name string) (*kafka.Writer, bool) { // 兼容老命名
	return c.KafkaProducer(name)
}

func (c *container) CloseKafkaWriters() {
	for name, w := range c.kafkaW {
		c.logger.Sugar().Infow("[kafka-writer] stopped", "instance", name)
		_ = w.Close()
	}
}

func (c *container) MySQL(name string) (*gorm.DB, bool)    { v, ok := c.mysql[name]; return v, ok }
func (c *container) Postgres(name string) (*gorm.DB, bool) { v, ok := c.pg[name]; return v, ok }
func (c *container) Redis(name string) (redis.UniversalClient, bool) {
	v, ok := c.redis[name]
	return v, ok
}
func (c *container) ClickHouseNative(name string) (*clickhx.Native, bool) {
	v, ok := c.chN[name]
	return v, ok
}
func (c *container) HTTPClient(name string) (*ihttp.Client, bool) {
	v, ok := c.httpC[name]
	return v, ok
}
func (c *container) GRPCClient(name string) (*grpc.ClientConn, bool) {
	v, ok := c.grpcC[name]
	return v, ok
}
func (c *container) GoPool(name string) (gopool.Pool, bool) { v, ok := c.pools[name]; return v, ok }
func (c *container) Notifier(name string) (notifier.Notifier, bool) {
	v, ok := c.notifiers[name]
	return v, ok
}
func (c *container) Logger() *ilog.Logger { return c.logger }
func (c *container) Config() *config.Cfg  { return c.cfg }

// BuildContainer 读取配置并初始化所有基础设施实例（拆分为多个小步骤）
func BuildContainer(ctx context.Context, cfg *config.Cfg, logger *ilog.Logger) (AppDepend, error) {
	res := &container{
		mysql:     map[string]*gorm.DB{},
		pg:        map[string]*gorm.DB{},
		redis:     map[string]redis.UniversalClient{},
		chN:       map[string]*clickhx.Native{},
		httpC:     map[string]*ihttp.Client{},
		grpcC:     map[string]*grpc.ClientConn{},
		pools:     map[string]gopool.Pool{},
		notifiers: map[string]notifier.Notifier{},
		kafkaW:    map[string]*kafka.Writer{},
		logger:    logger,
		cfg:       cfg,
	}

	// 通知器：构建集合 + 选择用于 goroutine 池的默认通知器
	defaultN := buildNotifiers(cfg, res)

	// Pool：命名 goroutine 池
	if err := buildPools(cfg, res, defaultN); err != nil {
		return nil, err
	}

	// MySQL
	if err := buildMySQL(ctx, cfg, res); err != nil {
		return nil, err
	}

	// PostgreSQL
	if err := buildPostgres(ctx, cfg, res); err != nil {
		return nil, err
	}

	// Redis
	if err := buildRedis(cfg, res); err != nil {
		return nil, err
	}

	// ClickHouse (native)
	if err := buildClickHouse(ctx, cfg, res); err != nil {
		return nil, err
	}

	// HTTP Clients
	if err := buildHTTPClients(cfg, res); err != nil {
		return nil, err
	}

	// gRPC Clients
	if err := buildGRPCClients(ctx, cfg, res); err != nil {
		return nil, err
	}

	// Kafka Producers
	if err := buildKafkaProducers(cfg, res); err != nil {
		return nil, err
	}

	return res, nil
}

// ---------- 下方是每一步的小函数，逻辑与原实现保持一致 ----------

// buildNotifiers 构建 notifier 集合，并返回 默认通知器 （用于 gopool）
func buildNotifiers(cfg *config.Cfg, res *container) notifier.Notifier {
	// lark 业务通知器
	if cfg.Notifier.Lark.Enabled && cfg.Notifier.Lark.BusinessWebhook != "" {
		res.notifiers["lark"] = lark.New(cfg.Notifier.Lark.BusinessWebhook, cfg.Notifier.Lark.Timeout)
	}

	// 如未配置任何 -> 提供一个 Noop
	if len(res.notifiers) == 0 {
		res.notifiers["default"] = notifier.Noop{}
	}

	// 选择用于 goroutine 池的“默认通知器”
	var n notifier.Notifier = notifier.Noop{}
	if cfg.Notifier.Lark.Enabled && cfg.Notifier.Lark.ExceptionWebhook != "" {
		n = lark.New(cfg.Notifier.Lark.ExceptionWebhook, cfg.Notifier.Lark.Timeout)
	}
	return n
}

// buildPools 初始化命名 goroutine 池（默认 default=64）
func buildPools(cfg *config.Cfg, res *container, n notifier.Notifier) error {
	if len(cfg.Pools) == 0 {
		res.pools["default"] = gopool.New(gopool.Option{
			Name:          "default",
			MaxGoroutines: 1000,
			Notifier:      n,
		})
		return nil
	}

	for name, spec := range cfg.Pools {
		size := spec.Size
		if size <= 0 {
			size = 1000
		}
		res.pools[name] = gopool.New(gopool.Option{
			Name:          name,
			MaxGoroutines: size,
			Notifier:      n,
		})
	}
	if _, ok := res.pools["default"]; !ok {
		res.pools["default"] = gopool.New(gopool.Option{
			Name:          "default",
			MaxGoroutines: 1000,
			Notifier:      n,
		})
	}
	return nil
}

// buildMySQL 初始化所有 MySQL 连接（GORM + otelsql）
func buildMySQL(ctx context.Context, cfg *config.Cfg, res *container) error {
	for name, m := range cfg.Databases.MySQL {
		gdb, err := imysql.NewMySQL(ctx, imysql.MySQLOption{
			DSN:             m.DSN,
			TraceEnabled:    m.TraceEnabled,
			MaxOpenConns:    m.MaxOpenConns,
			MaxIdleConns:    m.MaxIdleConns,
			ConnMaxLifetime: format.ParseDur(m.ConnMaxLifetime),
			InstanceName:    name,
		})
		if err != nil {
			return err
		}
		res.mysql[name] = gdb
	}
	return nil
}

// buildPostgres 初始化所有 PostgreSQL 连接（pgx/stdlib + GORM + otelsql）
func buildPostgres(ctx context.Context, cfg *config.Cfg, res *container) error {
	for name, p := range cfg.Databases.Postgres {
		gdb, err := ipostgres.New(ctx, ipostgres.PGOption{
			DSN:             p.DSN,
			TraceEnabled:    p.TraceEnabled,
			MaxOpenConns:    p.MaxOpenConns,
			MaxIdleConns:    p.MaxIdleConns,
			ConnMaxLifetime: format.ParseDur(p.ConnMaxLifetime),
			InstanceName:    name,
		})
		if err != nil {
			return err
		}
		res.pg[name] = gdb
	}
	return nil
}

// buildRedis 初始化所有 Redis 客户端（go-redis + redisotel）
func buildRedis(cfg *config.Cfg, res *container) error {
	redisPass := os.Getenv("REDIS_PASS")
	if redisPass == "" {
		log.Println("WARNING: REDIS_PASS environment variable not set")
	}
	for name, r := range cfg.Databases.RedisInstances {
		res.redis[name] = iredis.New(iredis.Option{
			Mode:                  iredis.Mode(r.Mode), // standalone/cluster/sentinel
			Addr:                  r.Addr,
			Addrs:                 r.Addrs,
			Username:              r.Username,
			Password:              redisPass,
			DB:                    r.DB, // Cluster 固定为 0（内部忽略）
			TraceEnabled:          r.TraceEnabled,
			MetricsEnable:         r.MetricsEnabled,
			DialTimeout:           r.DialTimeout,
			ReadTimeout:           r.ReadTimeout,
			WriteTimeout:          r.WriteTimeout,
			PoolSize:              r.PoolSize,
			MinIdleConns:          r.MinIdleConns,
			SentinelMasterName:    r.SentinelMasterName,
			TLSEnabled:            r.TlsEnabled,
			TLSInsecureSkipVerify: r.TLSInsecureSkipVerify,
		})
	}
	return nil
}

// buildClickHouse 初始化 ClickHouse（原生协议）
func buildClickHouse(ctx context.Context, cfg *config.Cfg, res *container) error {
	for name, ch := range cfg.Databases.ClickHouse {
		n, err := clickhx.NewNative(ctx, clickhx.NativeOption{
			DSN:             ch.NativeDSN,
			TraceEnabled:    ch.TraceEnabled,
			MaxOpen:         ch.MaxOpen,
			MaxIdle:         ch.MaxIdle,
			ConnMaxLifetime: format.ParseDur(ch.ConnMaxLifetime),
			Instance:        name,
			TracerName:      consts.CKTracerName,
		})
		if err != nil {
			return err
		}
		res.chN[name] = n
	}
	return nil
}

// buildHTTPClients 初始化全部 HTTP 客户端（多实例）
func buildHTTPClients(cfg *config.Cfg, res *container) error {
	if len(cfg.Clients.HTTP) == 0 {
		// 无配置：给 1 个 default，trace 跟随全局开关
		res.httpC["default"] = ihttp.NewClient(ihttp.Option{
			TraceEnabled:    cfg.Tracing.Enabled,
			Timeout:         3 * time.Second,
			MaxIdlePerHost:  10,
			IdleConnTimeout: 90 * time.Second,
		})
		return nil
	}

	for name, h := range cfg.Clients.HTTP {
		res.httpC[name] = ihttp.NewClient(ihttp.Option{
			TraceEnabled:    h.TraceEnabled,
			Timeout:         format.ParseDur(h.Timeout),
			MaxIdlePerHost:  h.MaxIdlePerHost,
			IdleConnTimeout: format.ParseDur(h.IdleConnTimeout),
		})
	}
	return nil
}

// buildGRPCClients 初始化全部 gRPC 客户端（多实例）
func buildGRPCClients(ctx context.Context, cfg *config.Cfg, res *container) error {
	for name, g := range cfg.Clients.GRPC {
		// 拨号超时
		var timeout time.Duration
		if strings.TrimSpace(g.Timeout) != "" {
			if d, err := time.ParseDuration(g.Timeout); err == nil {
				timeout = d
			}
		}

		// TLS
		var creds credentials.TransportCredentials
		if !g.Insecure {
			var err error
			creds, err = loadTLS(g.TLS)
			if err != nil {
				return err
			}
		}

		// keepalive
		var ka keepalive.ClientParameters
		if s := strings.TrimSpace(g.Keepalive.Time); s != "" {
			if d, err := time.ParseDuration(s); err == nil {
				ka.Time = d
			}
		}
		if s := strings.TrimSpace(g.Keepalive.Timeout); s != "" {
			if d, err := time.ParseDuration(s); err == nil {
				ka.Timeout = d
			}
		}
		ka.PermitWithoutStream = g.Keepalive.PermitWithoutStream

		// 组装拨号选项
		dopt := grpcclient.DialOption{
			Target:           g.Target,
			Targets:          g.Targets, // 非空则使用静态解析
			Insecure:         g.Insecure,
			TLS:              creds,
			TraceEnabled:     g.TraceEnabled,
			Balancer:         g.Balancer,
			ServiceConfig:    strings.TrimSpace(g.ServiceConfig),
			Keepalive:        ka,
			UserAgent:        g.UserAgent,
			MaxRecvMsgSize:   g.MaxRecvMsgSize,
			MaxSendMsgSize:   g.MaxSendMsgSize,
			DefaultWaitReady: g.DefaultWaitReady,
			Block:            g.Block,
			Timeout:          timeout,
		}

		conn, err := grpcclient.Dial(ctx, dopt)
		if err != nil {
			return err
		}
		res.grpcC[name] = conn
	}
	return nil
}

// buildKafkaProducers 初始化全部 Kafka Writer（segmentio/kafka-go）
func buildKafkaProducers(cfg *config.Cfg, res *container) error {
	for name, p := range cfg.MQ.KafkaProducers {
		// 负载均衡策略
		var balancer kafka.Balancer
		switch strings.ToLower(strings.TrimSpace(p.Balancer)) {
		case "hash":
			balancer = &kafka.Hash{}
		case "leastbytes", "least_bytes":
			balancer = &kafka.LeastBytes{}
		default:
			balancer = &kafka.RoundRobin{}
		}

		// acks：none=0, one/local=1, all=-1
		var acks int
		switch strings.ToLower(strings.TrimSpace(p.RequiredAcks)) {
		case "none", "0":
			acks = 0
		case "one", "local", "1":
			acks = 1
		default:
			acks = -1
		}

		// 批量/超时
		bt := format.ParseDur(p.BatchTimeout)
		wt := format.ParseDur(p.WriteTimeout)
		rt := format.ParseDur(p.ReadTimeout)

		// 需要鉴权或 TLS 时才自定义 Dialer，否则留空 -> 用默认
		var dialer *kafka.Dialer
		if p.Username != "" || p.Password != "" || p.EnableTLS {
			d := &kafka.Dialer{
				Timeout:   10 * time.Second,
				DualStack: true,
			}
			if p.Username != "" || p.Password != "" {
				d.SASLMechanism = plain.Mechanism{
					Username: p.Username,
					Password: p.Password,
				}
			}
			if p.EnableTLS {
				d.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
			}
			dialer = d
		}

		// 用 WriterConfig（内部会构造非空 Transport，避免 nil panic）
		wr := kafka.NewWriter(kafka.WriterConfig{
			Brokers:      p.Brokers,
			Topic:        strings.TrimSpace(p.Topic), // 留空也行，发消息时指定 Message.Topic
			Dialer:       dialer,                     // nil -> 默认
			Balancer:     balancer,
			MaxAttempts:  0, // 保持默认 10；这里 0 表示使用库默认
			BatchSize:    p.BatchSize,
			BatchBytes:   p.BatchBytes,
			BatchTimeout: bt,
			ReadTimeout:  rt,
			WriteTimeout: wt,
			RequiredAcks: acks,    // 注意：WriterConfig 需要的是 int
			Async:        p.Async, // 是否异步写
			Logger: kafka.LoggerFunc(func(format string, args ...interface{}) {
				res.logger.Sugar().Infof("[kafka-writer] "+format, args...)
			}),
			ErrorLogger: kafka.LoggerFunc(func(format string, args ...interface{}) {
				res.logger.Sugar().Errorf("[kafka-writer] "+format, args...)
			}),
		})

		res.kafkaW[name] = wr
	}
	return nil
}

// loadTLS 加载 gRPC 客户端 TLS
func loadTLS(spec config.TLSSpec) (credentials.TransportCredentials, error) {
	// CA
	var cp *x509.CertPool
	if spec.CAFile != "" {
		caPEM, err := os.ReadFile(spec.CAFile)
		if err != nil {
			return nil, err
		}
		cp = x509.NewCertPool()
		cp.AppendCertsFromPEM(caPEM)
	}

	// client cert
	var certs []tls.Certificate
	if spec.CertFile != "" && spec.KeyFile != "" {
		crt, err := tls.LoadX509KeyPair(spec.CertFile, spec.KeyFile)
		if err != nil {
			return nil, err
		}
		certs = []tls.Certificate{crt}
	}

	cfg := &tls.Config{
		RootCAs:      cp,
		Certificates: certs,
		ServerName:   spec.ServerName,
		MinVersion:   tls.VersionTLS12,
	}
	return credentials.NewTLS(cfg), nil
}
