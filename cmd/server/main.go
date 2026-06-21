package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	sboot "trade-gateway/internal/bootstrap/server"
	"trade-gateway/internal/config"
)

func main() {
	// 参数：本地兜底 + Nacos 引导
	cfgFlag := "configs/api-pro.yaml" // 默认生产配置
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	switch env {
	case "local":
		cfgFlag = "configs/api-local.yaml"
	case "dev", "test", "qa":
		cfgFlag = "configs/api-test.yaml"
	case "staging":
		cfgFlag = "configs/api-pre.yaml"
	case "prod":
		cfgFlag = "configs/api-pro.yaml"
	default:
		cfgFlag = "configs/api-test.yaml"
	}
	ctx := context.Background()
	appCfg, err := config.Load(cfgFlag)
	if err != nil {
		log.Fatalf("[FATAL] 配置加载失败: %v", err)
	}
	log.Printf("[OK] trade-gateway success api 配置加载来源: %s", "file")

	app, err := sboot.Bootstrap(ctx, appCfg)
	if err != nil {
		log.Fatalf("[FATAL] Bootstrap 失败: %v", err)
	}
	app.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGKILL, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	app.Stop(shutdownCtx)
}
