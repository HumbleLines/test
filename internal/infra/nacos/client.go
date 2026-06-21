package nacos

import (
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

/*
 * Nacos 客户端封装：创建 ConfigClient（用于拉取配置、监听变化）
 * 说明：这里只做“配置中心”，服务注册/发现我们下一步再加 NamingClient。
 */

type Options struct {
	Addr      string // "host:port" 例如 "127.0.0.1:8848"
	Username  string
	Password  string
	Namespace string // 命名空间名或ID
	TimeoutMs uint64
}

// NewConfigClient 创建配置客户端
func NewConfigClient(opt Options) (config_client.IConfigClient, error) {
	host, port := splitHostPort(opt.Addr)
	sc := []constant.ServerConfig{
		{IpAddr: host, Port: port},
	}
	cc := constant.ClientConfig{
		NamespaceId:         opt.Namespace,
		Username:            opt.Username,
		Password:            opt.Password,
		TimeoutMs:           opt.TimeoutMs,
		NotLoadCacheAtStart: true,                 // 启动时不优先使用本地缓存
		LogDir:              "./runtime/nacoslog", // 可按需调整日志/缓存目录
		CacheDir:            "./runtime/nacoscache",
	}
	return clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  &cc,
		ServerConfigs: sc,
	})
}

func splitHostPort(addr string) (host string, port uint64) {
	host = addr
	port = 8848 // 默认
	if i := strings.LastIndex(addr, ":"); i > 0 && i < len(addr)-1 {
		host = addr[:i]
		if p, err := strconv.ParseUint(addr[i+1:], 10, 64); err == nil {
			port = p
		}
	}
	return
}
