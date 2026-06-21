package consts

import "time"

// Tracing 导出器的“语义值”常量（与配置里的 exporter 对应）
const (
	TraceExporterOTLPHTTP    = "otlphttp" // 在配置里使用这个值
	TraceExporterOTLPGRPC    = "otlpgrpc" // gRPC 导出器
	TraceExporterOTLP        = "otlp"     // 兼容别名：按 gRPC 处理
	SdkTraceWithBatchTimeout = 3 * time.Second
)

// 常用的 OTel attribute key（可选，用于统一打点键名）
const (
	AttributeDBNameKey               = "db.name"
	AttrDBInstance                   = "db.instance"
	CKAttributeKey                   = "db.system"
	CKAttributeValue                 = "clickhouse"
	CKAttributeStatementKey          = "db.statement"
	CKAttributeOperationKey          = "db.operation"
	CKAttributeOperationValue        = "clickhouse.exec"
	PGDriverName                     = "pgx"
	PGAttributeSystemKey             = "db.system"
	PGAttributeSystemValue           = "postgresql"
	CKTracerName                     = "clickhouse-native"
	CkBatchSpanName                  = "clickhouse.batch.send"
	CKAttributeBatchOperationValue   = "batch"
	CKAttributeBatchRowsOperationKey = "clickhouse.batch.rows"
)

const (
	HttpPrefix       = "http://"
	HttpsPrefix      = "https://"
	Localhost        = "localhost"
	LocalhostIP      = "127.0.0.1"
	TraceSuffix      = "/v1/traces"
	TraceSuffixSlash = "/v1/traces/"
)

const (
	KATracerName              = "trade-gateway/mq/kafka"
	KASpanName                = "kafka.consume"
	KAAttributeSystemKey      = "messaging.system"
	KAAttributeSystemValue    = "kafka"
	KAAttributeOperationKey   = "messaging.operation"
	KAAttributeOperationValue = "process"
	KAAttributeMessageKey     = "messaging.kafka.message.key"
	KAAttributeSizeKey        = "payload.size"
)
