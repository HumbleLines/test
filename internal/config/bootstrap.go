package config

import (
	"context"
	"fmt"
	"log"
	"os"
	myapollo "trade-gateway/internal/infra/apollo"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

/*
 * 用于启动阶段连接 Nacos 的最小配置（只含 Nacos 连接信息）
 */

// Bootstrap 根结构
type Bootstrap struct {
	Nacos  NacosBootstrap  `yaml:"nacos"`
	Apollo ApolloBootstrap `yaml:"apollo"`
}

// NacosBootstrap Nacos 连接参数（来自 configs/bootstrap.yaml）
type NacosBootstrap struct {
	Addr           string `yaml:"addr"` // 例如 "127.0.0.1:8848"
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	Namespace      string `yaml:"namespace"`          // 例如 "public"
	Group          string `yaml:"group"`              // 例如 "DEFAULT_GROUP"
	DataID         string `yaml:"data_id"`            // 例如 "trade-gateway-api"
	TimeoutMs      uint64 `yaml:"timeout_ms"`         // 请求超时
	ListenInterval uint64 `yaml:"listen_interval_ms"` // (可选) 监听间隔
}

// LoadBootstrap 读取引导配置
func LoadBootstrap(path string) (*Bootstrap, error) {
	bs := &Bootstrap{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(raw, bs); err != nil {
		log.Printf("failed to unmarshal bootstrap config:%v", zap.Error(err))
		return nil, err
	}
	fmt.Println("bootstrap config:", bs)
	return bs, nil
}

// LoadCfgFromNacosOnce
/*
 * 从 Nacos 拉取 DataID 对应的 YAML 配置，并反序列化为 Cfg。
 * 这里只做“启动时拉取一次”。热更新监听留在第 6 步可选。
 */
func LoadCfgFromNacosOnce(_ context.Context, cli config_client.IConfigClient,
	group, dataID string, out *Cfg) error {

	content, err := cli.GetConfig(vo.ConfigParam{
		DataId: dataID,
		Group:  group,
	})
	if err != nil {
		return err
	}
	return yaml.Unmarshal([]byte(content), out)
}

// LoadCfgFromApolloOnce
/*
 * 从 Apollo 拉取 Key 对应的“整份 YAML 文本”，并反序列化为 Cfg。
 * 语义与 LoadCfgFromNacosOnce 完全对齐：启动时拉取一次。
 */
func LoadCfgFromApolloOnce(_ context.Context, bs *Bootstrap, out *Cfg) error {
	if bs == nil {
		return fmt.Errorf("bootstrap is nil")
	}
	a := bs.Apollo

	if a.IP == "" || a.AppID == "" || a.Namespace == "" {
		return fmt.Errorf("apollo bootstrap incomplete: ip/app_id/namespace required")
	}

	key := a.Key
	if key == "" {
		key = "config.yaml" // 默认 key
	}

	cli, ac, err := myapollo.NewClient(myapollo.Options{
		IP:             a.IP,
		AppID:          a.AppID,
		Cluster:        a.Cluster,
		Namespace:      a.Namespace,
		Secret:         a.Secret,
		IsBackupConfig: a.IsBackupConfig,
	})
	if err != nil {
		return fmt.Errorf("create apollo client: %w", err)
	}

	cache := cli.GetConfigCache(ac.NamespaceName)
	valAny, err := cache.Get(key)
	if err != nil {
		return fmt.Errorf("apollo get failed (ns=%s key=%s): %w", ac.NamespaceName, key, err)
	}

	content, ok := valAny.(string)
	if !ok || content == "" {
		return fmt.Errorf("apollo config empty or not string (ns=%s key=%s)", ac.NamespaceName, key)
	}

	return yaml.Unmarshal([]byte(content), out)
}

type ApolloBootstrap struct {
	Enabled        bool   `yaml:"enabled"`
	IP             string `yaml:"ip"` // Apollo config service address
	AppID          string `yaml:"app_id"`
	Cluster        string `yaml:"cluster"`
	Namespace      string `yaml:"namespace"`
	Key            string `yaml:"key"`
	Secret         string `yaml:"secret"`
	TimeoutMs      int    `yaml:"timeout_ms"`
	IsBackupConfig bool   `yaml:"is_backup_config"`
}
