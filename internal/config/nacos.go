package config

//
//import (
//	"context"
//	"fmt"
//	"log"
//	"os"
//
//	mynacos "trade-gateway/internal/infra/nacos"
//)
//
///*
//  对外入口（任选其一使用）：
//  1 ResolveServer(ctx, cfgFlagVal, bootstrapFlagVal)  // server 默认: ./configs/api.yaml + ./configs/api-bootstrap.yaml
//  2 ResolveWorker(ctx, cfgFlagVal, bootstrapFlagVal)  // worker 默认: ./configs/worker-local.yaml + ./configs/worker-bootstrap.yaml
//  3 ResolveAndLoadWithDefault(ctx, cfgFlagVal, bootstrapFlagVal, defCfg, defBootstrap) // 自定义默认
//  5 LoadWithFallback(ctx, bootstrapPath, fallbackPath) // 已有：优先 Nacos，失败回文件
//*/
//
//// ------- 公开的封装 -------
//
//func ResolveServer(ctx context.Context, cfgFromFlag, bootstrapFromFlag string) (*Cfg, string, error) {
//	return ResolveAndLoadWithDefault(ctx, cfgFromFlag, bootstrapFromFlag, defaultServerConfigPath, defaultServerBootstrapPath)
//}
//
//func ResolveWorker(ctx context.Context, cfgFromFlag, bootstrapFromFlag string) (*Cfg, string, error) {
//	return ResolveAndLoadWithDefault(ctx, cfgFromFlag, bootstrapFromFlag, defaultWorkerConfigPath, defaultWorkerBootstrapPath)
//}
//
//// ResolveAndLoadWithDefault 自定义默认值的统一入口（给其它进程/工具使用）
//func ResolveAndLoadWithDefault(ctx context.Context, cfgFromFlag, bootstrapFromFlag, defCfg, defBootstrap string) (*Cfg, string, error) {
//	cfgPath := pickPath(cfgFromFlag, envConfigFile, defCfg)
//	bootstrapPath := pickPath(bootstrapFromFlag, envBootstrapFile, defBootstrap)
//	return LoadWithFallback(ctx, bootstrapPath, cfgPath)
//}
//
//// LoadWithFallback ------- Nacos 优先，失败回文件 -------
//func LoadWithFallback(ctx context.Context, bootstrapPath, fallbackPath string) (*Cfg, string, error) {
//	//  读引导配置
//	bs, err := LoadBootstrap(bootstrapPath)
//	if err == nil && NacosEnabled(bs) {
//		//  尝试 Nacos
//		cfg, nerr := loadFromNacos(ctx, bs)
//		if nerr == nil {
//			return cfg, "nacos", nil
//		}
//		log.Printf("[WARN] 从 Nacos 加载失败，将回退本地: %v", nerr)
//	} else if err != nil {
//		log.Printf("[INFO] 读取 bootstrap 失败或不存在，将回退本地: %v", err)
//	}
//
//	// 本地兜底
//	cfg, ferr := Load(fallbackPath)
//	if ferr != nil {
//		return nil, "", fmt.Errorf("fallback to file(%s) failed: %w", fallbackPath, ferr)
//	}
//	return cfg, "file", nil
//}
//
//// NacosEnabled 是否启用 Nacos（addr 与 username 皆非空）
//func NacosEnabled(bs *Bootstrap) bool {
//	return bs != nil && bs.Nacos.Addr != "" && bs.Nacos.Username != ""
//}
//
//// loadFromNacos 内部：从 Nacos 拉取并反序列化
//func loadFromNacos(ctx context.Context, bs *Bootstrap) (*Cfg, error) {
//	cli, err := mynacos.NewConfigClient(mynacos.Options{
//		Addr:      bs.Nacos.Addr,
//		Username:  bs.Nacos.Username,
//		Password:  bs.Nacos.Password,
//		Namespace: bs.Nacos.Namespace,
//		TimeoutMs: bs.Nacos.TimeoutMs,
//	})
//	if err != nil {
//		return nil, fmt.Errorf("create nacos client: %w", err)
//	}
//	var cfg Cfg
//	if err := LoadCfgFromNacosOnce(ctx, cli, bs.Nacos.Group, bs.Nacos.DataID, &cfg); err != nil {
//		return nil, fmt.Errorf("get config from nacos (group=%s, dataID=%s): %w", bs.Nacos.Group, bs.Nacos.DataID, err)
//	}
//	return &cfg, nil
//}
//
//// ------- 默认路径 & 环境变量键 -------
//
//const (
//	// server 默认
//	defaultServerConfigPath    = "./configs/api.yaml"
//	defaultServerBootstrapPath = "./configs/api-bootstrap.yaml"
//
//	// worker 默认
//	defaultWorkerConfigPath    = "./configs/worker-local.yaml"
//	defaultWorkerBootstrapPath = "./configs/worker-bootstrap.yaml"
//
//	// 环境变量覆盖键（两类进程共用）
//	envConfigFile    = "CONFIG_FILE"
//	envBootstrapFile = "BOOTSTRAP_FILE"
//)
//
//func pickPath(flagVal, envKey, def string) string {
//	if flagVal != "" {
//		return flagVal
//	}
//	if v := os.Getenv(envKey); v != "" {
//		return v
//	}
//	return def
//}
