package config

import (
	"context"
	"fmt"
	"log"
	"os"

	myapollo "trade-gateway/internal/infra/apollo"

	"gopkg.in/yaml.v3"
)

/*
  对外入口（任选其一使用）：
  1 ResolveServer(ctx, cfgFlagVal, bootstrapFlagVal)  // server 默认: ./configs/api.yaml + ./configs/api-bootstrap.yaml
  2 ResolveWorker(ctx, cfgFlagVal, bootstrapFlagVal)  // worker 默认: ./configs/worker-local.yaml + ./configs/worker-bootstrap.yaml
  3 ResolveAndLoadWithDefault(ctx, cfgFlagVal, bootstrapFlagVal, defCfg, defBootstrap) // 自定义默认
  5 LoadWithFallback(ctx, bootstrapPath, fallbackPath) // 已有：优先 Apollo，失败回文件
*/

// ------- 公开的封装 -------

func ResolveServer(ctx context.Context, cfgFromFlag, bootstrapFromFlag string) (*Cfg, string, error) {
	return ResolveAndLoadWithDefault(ctx, cfgFromFlag, bootstrapFromFlag, defaultServerConfigPath, defaultServerBootstrapPath)
}

func ResolveWorker(ctx context.Context, cfgFromFlag, bootstrapFromFlag string) (*Cfg, string, error) {
	return ResolveAndLoadWithDefault(ctx, cfgFromFlag, bootstrapFromFlag, defaultWorkerConfigPath, defaultWorkerBootstrapPath)
}

// ResolveAndLoadWithDefault 自定义默认值的统一入口（给其它进程/工具使用）
func ResolveAndLoadWithDefault(ctx context.Context, cfgFromFlag, bootstrapFromFlag, defCfg, defBootstrap string) (*Cfg, string, error) {
	cfgPath := pickPath(cfgFromFlag, envConfigFile, defCfg)
	bootstrapPath := pickPath(bootstrapFromFlag, envBootstrapFile, defBootstrap)
	return LoadWithFallback(ctx, bootstrapPath, cfgPath)
}

// LoadWithFallback ------- Apollo 优先，失败回文件 -------
func LoadWithFallback(ctx context.Context, bootstrapPath, fallbackPath string) (*Cfg, string, error) {
	// 读引导配置
	bs, err := LoadBootstrap(bootstrapPath)
	if err == nil && ApolloEnabled(bs) {
		// 尝试 Apollo
		var cfg Cfg
		aerr := LoadCfgFromApolloOnce(ctx, bs, &cfg)
		if aerr == nil {
			return &cfg, "apollo", nil
		}
		log.Printf("[WARN] 从 Apollo 加载失败，将回退本地: %v", aerr)
	} else if err != nil {
		log.Printf("[INFO] 读取 bootstrap 失败或不存在，将回退本地: %v", err)
	}

	// 本地兜底
	cfg, ferr := Load(fallbackPath)
	if ferr != nil {
		return nil, "", fmt.Errorf("fallback to file(%s) failed: %w", fallbackPath, ferr)
	}
	return cfg, "file", nil
}

// ApolloEnabled 是否启用 Apollo（enabled=true 或关键字段齐全）
func ApolloEnabled(bs *Bootstrap) bool {
	if bs == nil {
		return false
	}
	if bs.Apollo.Enabled {
		return true
	}
	return bs.Apollo.IP != "" && bs.Apollo.AppID != "" && bs.Apollo.Namespace != ""
}

// loadFromApollo 内部：从 Apollo 拉取并反序列化
// 约定：namespace 下某个 key（默认 config.yaml）的 value 存整份 YAML 文本
func loadFromApollo(ctx context.Context, bs *Bootstrap) (*Cfg, error) {
	_ = ctx // agollo 不吃 ctx，这里保留签名一致

	key := bs.Apollo.Key
	if key == "" {
		key = "config.yaml"
	}

	client, ac, err := myapollo.NewClient(myapollo.Options{
		IP:             bs.Apollo.IP,
		AppID:          bs.Apollo.AppID,
		Cluster:        bs.Apollo.Cluster,
		Namespace:      bs.Apollo.Namespace,
		Secret:         bs.Apollo.Secret,
		IsBackupConfig: bs.Apollo.IsBackupConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("create apollo client: %w", err)
	}

	cache := client.GetConfigCache(ac.NamespaceName)
	valAny, err := cache.Get(key)
	if err != nil {
		return nil, fmt.Errorf("apollo cache get failed (ns=%s key=%s): %w", ac.NamespaceName, key, err)
	}
	val, ok := valAny.(string)
	if !ok || val == "" {
		return nil, fmt.Errorf("apollo config empty or not string (ns=%s key=%s)", ac.NamespaceName, key)
	}

	var cfg Cfg
	if err := yaml.Unmarshal([]byte(val), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal apollo yaml (ns=%s key=%s): %w", ac.NamespaceName, key, err)
	}
	return &cfg, nil
}

// ------- 默认路径 & 环境变量键 -------

const (
	// server 默认
	defaultServerConfigPath    = "./configs/api.yaml"
	defaultServerBootstrapPath = "./configs/api-bootstrap.yaml"

	// worker 默认
	defaultWorkerConfigPath    = "./configs/worker-local.yaml"
	defaultWorkerBootstrapPath = "./configs/worker-bootstrap.yaml"

	// 环境变量覆盖键（两类进程共用）
	envConfigFile    = "CONFIG_FILE"
	envBootstrapFile = "BOOTSTRAP_FILE"
)

func pickPath(flagVal, envKey, def string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return def
}
