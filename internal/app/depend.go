package app

import (
	"trade-gateway/internal/config"
	ihttp "trade-gateway/internal/infra/client/http"
	"trade-gateway/internal/infra/gopool"
	ilog "trade-gateway/internal/infra/log"
	"trade-gateway/internal/infra/notifier"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	clickhx "trade-gateway/internal/infra/clickhouse"
)

// AppDepend 定义“应用依赖容器”的对外接口
// 业务模块通过它按实例名获取各种基础设施（DB/MQ/HTTP 等）
type AppDepend interface {
	// 数据库
	MySQL(name string) (*gorm.DB, bool)    // 例如 name="master"
	Postgres(name string) (*gorm.DB, bool) // 与 MySQL 对齐，返回 *gorm.DB
	Redis(name string) (redis.UniversalClient, bool)
	ClickHouseNative(name string) (*clickhx.Native, bool)
	HTTPClient(name string) (*ihttp.Client, bool)
	GRPCClient(name string) (*grpc.ClientConn, bool)
	Logger() *ilog.Logger
	Config() *config.Cfg
	Notifier(name string) (notifier.Notifier, bool)
	GoPool(name string) (gopool.Pool, bool)
	KafkaProducer(name string) (*kafka.Writer, bool)
	CloseKafkaWriters()
}
