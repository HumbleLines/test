package config

import (
	"crypto/tls"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

/* ===================== 顶层 ===================== */

type Cfg struct {
	App       AppCfg              `yaml:"app"`
	HTTP      HTTPServerCfg       `yaml:"http"`
	GRPC      GRPCServerCfg       `yaml:"grpc"`
	Clients   ClientsCfg          `yaml:"clients"`
	Tracing   TracingCfg          `yaml:"tracing"`
	Metrics   MetricsCfg          `yaml:"metrics"`
	Logging   LoggingCfg          `yaml:"logging"`
	Databases DatabasesCfg        `yaml:"databases"`
	MQ        MQCfg               `yaml:"mq"`
	Pools     map[string]PoolSpec `yaml:"pools"`
	Notifier  NotifierSpec        `yaml:"notifier"`
	Worker    WorkerCfg           `yaml:"worker"`
	Exchange  Exchange            `yaml:"exchange"`
}

type Exchange struct {
	Siristars ExchangeCfg `yaml:"siristars"`
}

type ExchangeCfg struct {
	SpotHost    string `yaml:"spot_host"`
	FuturesHost string `yaml:"futures_host"`
}

/* ===================== App / Server ===================== */

type AppCfg struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"`
}

type HTTPServerCfg struct {
	Addr string `yaml:"addr"`
}

type GRPCServerCfg struct {
	Addr      string            `yaml:"addr"`
	Keepalive GRPCKeepaliveSpec `yaml:"keepalive"`
}

type GRPCKeepaliveSpec struct {
	// ServerParameters（字符串时长，运行时自己转为 time.Duration）
	MaxConnectionIdle     string `yaml:"max_connection_idle"`
	MaxConnectionAge      string `yaml:"max_connection_age"`
	MaxConnectionAgeGrace string `yaml:"max_connection_age_grace"`
	Time                  string `yaml:"time"`
	Timeout               string `yaml:"timeout"`
	// EnforcementPolicy
	MinTime             string `yaml:"min_time"`
	PermitWithoutStream bool   `yaml:"permit_without_stream"`
}

/* ===================== Clients ===================== */

type ClientsCfg struct {
	HTTP map[string]HTTPClientCfg `yaml:"http"`
	GRPC map[string]GRPCClientCfg `yaml:"grpc"`
}

type HTTPClientCfg struct {
	Timeout         string `yaml:"timeout"` // e.g. "3s"
	TraceEnabled    bool   `yaml:"trace_enabled"`
	MaxIdlePerHost  int    `yaml:"max_idle_per_host"` // 默认 10
	IdleConnTimeout string `yaml:"idle_conn_timeout"` // e.g. "90s"
}

type GRPCClientCfg struct {
	Target  string   `yaml:"target"`  // e.g. "passthrough:///127.0.0.1:9090"
	Targets []string `yaml:"targets"` // 静态多地址（内置静态 resolver）

	Insecure bool    `yaml:"insecure"` // true=明文；false=TLS
	TLS      TLSSpec `yaml:"tls"`      // Insecure=false 时可选

	TraceEnabled bool `yaml:"trace_enabled"`

	Balancer      string `yaml:"balancer"`       // "", "round_robin", "pick_first"
	ServiceConfig string `yaml:"service_config"` // 覆盖 balancer 的完整 JSON

	Keepalive struct {
		Time                string `yaml:"time"`
		Timeout             string `yaml:"timeout"`
		PermitWithoutStream bool   `yaml:"permit_without_stream"`
	} `yaml:"keepalive"`

	UserAgent        string `yaml:"user_agent"`
	MaxRecvMsgSize   int    `yaml:"max_recv_msg_size"`
	MaxSendMsgSize   int    `yaml:"max_send_msg_size"`
	DefaultWaitReady bool   `yaml:"default_wait_ready"`

	Block   bool   `yaml:"block"`
	Timeout string `yaml:"timeout"`
}

/* ===================== Tracing / Metrics / Logging ===================== */

type TracingCfg struct {
	Enabled      bool              `yaml:"enabled"`
	Exporter     string            `yaml:"exporter"`
	Endpoint     string            `yaml:"endpoint"`
	Insecure     bool              `yaml:"insecure"`
	SampleRatio  float64           `yaml:"sample_ratio"`
	ResourceAttr map[string]string `yaml:"resource_attrs"`
}

type MetricsCfg struct {
	Prometheus PrometheusSpec `yaml:"prometheus"`
}

type PrometheusSpec struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

type LoggingCfg struct {
	Console  bool   `yaml:"console"`
	Level    string `yaml:"level"`
	FilePath string `yaml:"file_path"`
}

/* ===================== Databases ===================== */

type DatabasesCfg struct {
	MySQL          map[string]MySQLCfg      `yaml:"mysql"`
	Postgres       map[string]PostgresCfg   `yaml:"postgres"`
	RedisInstances map[string]RedisInstance `yaml:"redis"`
	ClickHouse     map[string]ClickHouseCfg `yaml:"clickhouse"`
}

type MySQLCfg struct {
	DSN             string `yaml:"dsn"`
	TraceEnabled    bool   `yaml:"trace_enabled"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

type PostgresCfg struct {
	DSN             string `yaml:"dsn"`
	TraceEnabled    bool   `yaml:"trace_enabled"`
	MaxOpenConns    int    `yaml:"max_conns"`         // 保持你原有 tag
	MaxIdleConns    int    `yaml:"min_conns"`         // 保持你原有 tag
	ConnMaxLifetime string `yaml:"max_conn_lifetime"` // 保持你原有 tag
}

// RedisInstance 支持 standalone / cluster / sentinel
type RedisInstance struct {
	Mode string `yaml:"mode"` // standalone / cluster / sentinel

	Addr  string   `yaml:"addr"`  // Standalone
	Addrs []string `yaml:"addrs"` // Cluster / Sentinel

	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"` // Cluster 固定 0

	TraceEnabled bool `yaml:"trace_enabled"`
	TlsEnabled   bool `yaml:"tls_enabled"`
	// 证书链有问题才用（一般不要开）
	TLSInsecureSkipVerify bool `yaml:"tls_insecure_skip_verify"`
	// 代码注入用（不从 YAML 解析）
	TLS            *tls.Config `yaml:"-"`
	MetricsEnabled bool        `yaml:"metrics_enabled"`

	SentinelMasterName string `yaml:"sentinel_master_name"`

	// YAML 可直接写 "3s"；time.Duration 会自动解析
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	PoolSize     int           `yaml:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns"`
}

type ClickHouseCfg struct {
	NativeDSN       string `yaml:"native_dsn"`
	TraceEnabled    bool   `yaml:"trace_enabled"`
	MaxOpen         int    `yaml:"max_open"`
	MaxIdle         int    `yaml:"max_idle"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

/* ===================== MQ (Kafka) ===================== */

type MQCfg struct {
	KafkaConsumers map[string]KafkaConsumerCfg `yaml:"kafka_consumer"`
	KafkaProducers map[string]KafkaProducerCfg `yaml:"kafka_producer"`
}

type KafkaProducerCfg struct {
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"` // 默认写入的 topic（也可在 Message 里覆盖）

	Balancer     string `yaml:"balancer"`      // round_robin|hash|least_bytes（默认 round_robin）
	BatchSize    int    `yaml:"batch_size"`    // 每批条数上限
	BatchBytes   int    `yaml:"batch_bytes"`   // 每批字节上限
	BatchTimeout string `yaml:"batch_timeout"` // e.g. "200ms"
	WriteTimeout string `yaml:"write_timeout"` // e.g. "3s"
	ReadTimeout  string `yaml:"read_timeout"`  // e.g. "3s"
	RequiredAcks string `yaml:"required_acks"` // none|one|local|all -> 0/1/-1

	// 可选：SASL/PLAIN；如需 TLS，自行在构造 writer.Transport 时注入
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	EnableTLS bool   `yaml:"enable_tls"`

	Async        bool `yaml:"async"`         // 异步写：需设置 Completion 才能感知结果
	TraceEnabled bool `yaml:"trace_enabled"` // 若对发布也打 Producer span，可用
}

type KafkaConsumerCfg struct {
	Brokers      []string `yaml:"brokers"`
	Group        string   `yaml:"group"`
	Topic        string   `yaml:"topic"`
	Offset       string   `yaml:"offset"` // "first"/"earliest" | "last"/"latest"
	Conns        int      `yaml:"conns"`
	Consumers    int      `yaml:"consumers"`
	Processors   int      `yaml:"processors"`
	MinBytes     int      `yaml:"min_bytes"`
	MaxBytes     int      `yaml:"max_bytes"`
	Username     string   `yaml:"username"`
	Password     string   `yaml:"password"`
	TraceEnabled bool     `yaml:"trace_enabled"`
}

/* ===================== Pools / Notifier / TLS ===================== */

type PoolSpec struct {
	Size int `yaml:"size"`
}

type NotifierSpec struct {
	Lark     LarkNotifierSpec     `yaml:"lark"`
	Dingtalk DingtalkNotifierSpec `yaml:"dingtalk"`
	Wecom    WecomNotifierSpec    `yaml:"wecom"`
}

type LarkNotifierSpec struct {
	Enabled          bool          `yaml:"enabled"`
	ExceptionWebhook string        `yaml:"exception_webhook"`
	BusinessWebhook  string        `yaml:"business_webhook"`
	Timeout          time.Duration `yaml:"timeout"`
}

type DingtalkNotifierSpec struct {
	Enabled bool          `yaml:"enabled"`
	Webhook string        `yaml:"webhook"`
	Timeout time.Duration `yaml:"timeout"`
}

type WecomNotifierSpec struct {
	Enabled bool          `yaml:"enabled"`
	Webhook string        `yaml:"webhook"`
	Timeout time.Duration `yaml:"timeout"`
}

type TLSSpec struct {
	CAFile     string `yaml:"ca_file"`
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`
	ServerName string `yaml:"server_name"`
}

/* ===================== Loader ===================== */

func Load(path string) (*Cfg, error) {
	if path == "" {
		path = "configs/base.yaml"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Cfg
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
