package config

import "time"

// ====================== 通用 Trace 配置 ======================

// TraceCfg 全局 Trace 配置（仅一处控制即可）
type TraceCfg struct {
	Enabled bool    `yaml:"enabled"` // 是否开启链路追踪
	Sample  float64 `yaml:"sample"`  // 采样率 0~1，建议：线上 0.01~0.1
}

// ====================== Worker 总配置 ======================

type WorkerCfg struct {
	Trace      TraceCfg          `yaml:"trace"`      // 全局 trace 开关+采样
	MQ         WorkerMQCfg       `yaml:"mq"`         // MQ 消费者
	Blocking   WorkerBlockingCfg `yaml:"blocking"`   // 自定义阻塞型任务
	XXL        WorkerXXLCfg      `yaml:"xxl"`        // XXL-JOB 执行器
	Marketdata MarketdataCfg     `yaml:"marketdata"` // 新增
}

// MarketdataCfg 行情服务配置
type MarketdataCfg struct {
	Binance   CommonMarketdataCfg `yaml:"binance"`
	Okx       CommonMarketdataCfg `yaml:"okx"`
	SiriStars CommonMarketdataCfg `yaml:"siristars"`

	// WS 连接池：每条连接订阅上限、qps日志、failover去抖等
	Pool WsPoolSpec `yaml:"pool"`

	Staleness StalenessCfg `yaml:"staleness"`

	// Leader 单活 TTL（建议 >= 30s）
	LeaderTTL time.Duration `yaml:"leader_ttl"`

	DepthTopN int `yaml:"depth_top_n"`

	DepthWarmupConfig DepthWarmupConfig `yaml:"depth_warmup"`

	RestLimitPerMin      int64 `yaml:"rest_limit_per_min"`
	RestLimitConcurrency int   `yaml:"rest_limit_concurrency"`
}

// DepthWarmupConfig
// 本地深度 warmup（按批次建簿）的控制参数。
// 主要目标：
//   - 避免一次性对大量 symbol 拉 REST snapshot 导致限频/打爆
//   - 避免 WS diff 缓存过多导致内存压力
//   - 控制 warmup 整体耗时与失败重试策略
//
// 说明：
//   - 所有 duration 字段建议在 yaml 中使用 Go duration 格式（如 "3s", "800ms"）
//   - 若某字段为 0，WarmupInBatches 内部会使用默认值兜底
type DepthWarmupConfig struct {

	// BatchSize
	// 每批 warmup 的 symbol 数量。
	// 建议：
	//   - 生产环境：10（较稳妥，避免 REST 突刺）
	//   - 币种极少时可适当调大
	BatchSize int `yaml:"batch_size" json:"batch_size"`

	// BatchInterval
	// 每批之间的间隔时间。
	// 作用：
	//   - 避免 REST / WS / 内存瞬间突刺
	//   - 给前一批 symbol 时间完成 bridge 进入 RUNNING
	// 建议：
	//   - 3s 为较安全默认值
	BatchInterval time.Duration `yaml:"batch_interval" json:"batch_interval"`

	// SnapshotRetry
	// 单个 symbol snapshot 拉取失败后的重试次数。
	// 注意：
	//   - 实际尝试次数 = SnapshotRetry + 1
	//   - 只用于短暂网络抖动/偶发超时
	//   - 不建议设置过大，否则 warmup 总耗时会拉长
	SnapshotRetry int `yaml:"snapshot_retry" json:"snapshot_retry"`

	// RetryInterval
	// snapshot 重试之间的等待时间。
	// 可后续替换为指数退避（backoff）。
	RetryInterval time.Duration `yaml:"retry_interval" json:"retry_interval"`

	// SnapshotTimeout
	// 单次 snapshot REST 请求的超时时间。
	// 作用：
	//   - 防止某个 symbol 卡住导致整批阻塞
	// 建议：
	//   - 3~5s 较合理
	SnapshotTimeout time.Duration `yaml:"snapshot_timeout" json:"snapshot_timeout"`

	// BatchTimeout
	// 单批最大执行时间。
	// 若超过该时间，则当前批次提前结束，进入下一批。
	// 作用：
	//   - 防止某些 symbol 长时间 bridge_wait 导致 warmup 卡死
	// 建议：
	//   - 60s 左右
	BatchTimeout time.Duration `yaml:"batch_timeout" json:"batch_timeout"`
}
type StalenessCfg struct {
	StalenessEnabled    bool          `yaml:"staleness_enabled"`
	StalenessThreshold  time.Duration `yaml:"staleness_threshold"`
	StalenessCheckEvery time.Duration `yaml:"staleness_check_every"`
}

type CommonMarketdataCfg struct {
	Enabled bool `yaml:"enabled"`
	// Kline 周期：1m/5m/15m/1h...
	KlineInterval string `yaml:"kline_interval"`

	// 现货订阅币对列表（大写，如 BTCUSDT）
	SpotSymbols    []string `yaml:"spot_symbols"`
	SpotWssHost    []string `yaml:"spot_wss_host"`
	FutureWssHost  []string `yaml:"future_wss_host"`
	SpotRestHost   []string `yaml:"spot_rest_host"`
	FutureRestHost []string `yaml:"future_rest_host"`
	// 合约订阅币对列表（大写，如 BTCUSDT）
	FuturesSymbols []string `yaml:"futures_symbols"`
}

// WsPoolSpec 对应 WsPoolConfig（配置文件里可写 duration）
type WsPoolSpec struct {
	MaxSymbolsPerConn      int           `yaml:"max_symbols_per_conn"`
	FailoverTTL            time.Duration `yaml:"failover_ttl"`
	QPSLogInterval         time.Duration `yaml:"qps_log_interval"`
	FailoverDebounce       time.Duration `yaml:"failover_debounce"`
	MaxConsecutiveDialFail int           `yaml:"max_consecutive_dial_fail"`
	MaxDialFailDuration    time.Duration `yaml:"max_dial_fail_duration"`
	DialAlertCoolDown      time.Duration `yaml:"dial_alert_cool_down"`
	DepthQueueSize         int64         `yaml:"depth_queue_size"`
	FastQueueSize          int64         `yaml:"fast_queue_size"`
	FastWorkers            int           `yaml:"fast_workers"`
}

// ====================== MQ ======================

type WorkerMQCfg struct {
	Enabled bool           `yaml:"enabled"`
	Items   []WorkerMQItem `yaml:"items"`
}

type WorkerMQItem struct {
	Driver      string   `yaml:"driver"` // kafka | rocketmq | rabbit...
	Name        string   `yaml:"name"`
	Brokers     []string `yaml:"brokers"`
	Topic       string   `yaml:"topic"`
	Group       string   `yaml:"group"`
	Concurrency int      `yaml:"concurrency"`
	Handler     string   `yaml:"handler"`
}

// ====================== Blocking ======================

type WorkerBlockingCfg struct {
	Enabled bool                 `yaml:"enabled"`
	Items   []WorkerBlockingItem `yaml:"items"`
}

type WorkerBlockingItem struct {
	Name           string        `yaml:"name"`
	Handler        string        `yaml:"handler"`
	Concurrency    int           `yaml:"concurrency"`
	RestartBackoff time.Duration `yaml:"restart_backoff"`
	MaxBackoff     time.Duration `yaml:"max_backoff"`
}

// ====================== XXL ======================

type WorkerXXLCfg struct {
	Addr     string `yaml:"addr"`
	Token    string `yaml:"access_token"`
	Executor struct {
		AppName string `yaml:"app_name"`
		IP      string `yaml:"ip"`
		Port    int    `yaml:"port"`
	} `yaml:"executor"`
}
