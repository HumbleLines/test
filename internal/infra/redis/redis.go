// Package redis 文件：internal/infra/redis/redis.go
// 说明：支持 Standalone / Cluster / Sentinel，统一返回 redis.UniversalClient。
//
//	开启 Trace 时，为 client 注入 redisotel（命令级 span）。
package redis

import (
	"crypto/tls"
	"time"
	"trade-gateway/internal/pkg/format"

	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// Mode Redis 部署模式
type Mode string

const (
	ModeStandalone Mode = "standalone" // 单机：Addr 只需 1 个
	ModeCluster    Mode = "cluster"    // 集群：Addrs 为多个 7000/7001/...
	ModeSentinel   Mode = "sentinel"   // 哨兵：Addrs 为多个 26379/26380/...
)

// Option Redis 实例配置（可从 YAML 直接映射）
// 兼容：单机仍可只配 Addr；Cluster/Sentinel 用 Addrs。
type Option struct {
	Mode          Mode     `yaml:"mode"`     // standalone / cluster / sentinel（默认 standalone）
	Addr          string   `yaml:"addr"`     // 单机：host:port（向后兼容）
	Addrs         []string `yaml:"addrs"`    // Cluster 或 Sentinel：节点列表
	Username      string   `yaml:"username"` // 如需 ACL
	Password      string   `yaml:"password"`
	DB            int      `yaml:"db"`              // 仅 Standalone/Sentinel 生效；Cluster 固定 0
	TraceEnabled  bool     `yaml:"trace_enabled"`   // 是否注入 redisotel tracing hook
	MetricsEnable bool     `yaml:"metrics_enabled"` // 可选：是否注入 redisotel metrics hook

	// 连接池/超时（可选）
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	PoolSize     int           `yaml:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns"`

	// Sentinel 专用
	SentinelMasterName string `yaml:"sentinel_master_name"` // 如 "mymaster"

	TLSEnabled            bool `yaml:"tls_enabled"`
	TLSInsecureSkipVerify bool `yaml:"tls_insecure_skip_verify"`
	// 可选 TLS（若用内网明文可不配）
	TLS *tls.Config `yaml:"-"`
}

// New 创建 redis.UniversalClient：根据 Mode 选择具体实现
// - standalone: *redis.Client
// - cluster:    *redis.ClusterClient
// - sentinel:   *redis.Client（Failover 模式）
func New(opt Option) redis.UniversalClient {
	if opt.TLS == nil && opt.TLSEnabled {
		opt.TLS = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: opt.TLSInsecureSkipVerify, // 默认 false
		}
	}
	// 合法化：默认 standalone；兼容只给了 Addr 的场景
	mode := opt.Mode
	if mode == "" {
		mode = ModeStandalone
	}

	var cli redis.UniversalClient

	switch mode {
	case ModeCluster:
		// Cluster：Addrs 必须为多个；DB 在 Cluster 固定为 0
		copt := &redis.ClusterOptions{
			Addrs:        opt.Addrs,
			Username:     opt.Username,
			Password:     opt.Password,
			ReadTimeout:  format.DurOrDefault(opt.ReadTimeout, 3*time.Second),
			WriteTimeout: format.DurOrDefault(opt.WriteTimeout, 3*time.Second),
			DialTimeout:  format.DurOrDefault(opt.DialTimeout, 3*time.Second),
			PoolSize:     format.IntOrDefault(opt.PoolSize, 50),
			MinIdleConns: format.IntOrDefault(opt.MinIdleConns, 5),
			TLSConfig:    opt.TLS,
		}
		cli = redis.NewClusterClient(copt)

	case ModeSentinel:
		// Sentinel（Failover）：Addrs 为哨兵地址们，MasterName 必填
		fopt := &redis.FailoverOptions{
			MasterName:    opt.SentinelMasterName,
			SentinelAddrs: opt.Addrs,
			DB:            opt.DB,
			Username:      opt.Username,
			Password:      opt.Password,
			ReadTimeout:   format.DurOrDefault(opt.ReadTimeout, 3*time.Second),
			WriteTimeout:  format.DurOrDefault(opt.WriteTimeout, 3*time.Second),
			DialTimeout:   format.DurOrDefault(opt.DialTimeout, 3*time.Second),
			PoolSize:      format.IntOrDefault(opt.PoolSize, 50),
			MinIdleConns:  format.IntOrDefault(opt.MinIdleConns, 5),
			TLSConfig:     opt.TLS,
		}
		cli = redis.NewFailoverClient(fopt)

	default: // standalone
		addr := opt.Addr
		// 若没填 Addr，但传了 Addrs，取第一个以保持容错
		if addr == "" && len(opt.Addrs) > 0 {
			addr = opt.Addrs[0]
		}
		sopt := &redis.Options{
			Addr:         addr,
			DB:           opt.DB,
			Username:     opt.Username,
			Password:     opt.Password,
			ReadTimeout:  format.DurOrDefault(opt.ReadTimeout, 3*time.Second),
			WriteTimeout: format.DurOrDefault(opt.WriteTimeout, 3*time.Second),
			DialTimeout:  format.DurOrDefault(opt.DialTimeout, 3*time.Second),
			PoolSize:     format.IntOrDefault(opt.PoolSize, 20),
			MinIdleConns: format.IntOrDefault(opt.MinIdleConns, 2),
			TLSConfig:    opt.TLS,
		}
		cli = redis.NewClient(sopt)
	}

	// 链路追踪/指标（可选）
	if opt.TraceEnabled {
		_ = redisotel.InstrumentTracing(cli)
	}
	if opt.MetricsEnable {
		_ = redisotel.InstrumentMetrics(cli)
	}

	return cli
}
