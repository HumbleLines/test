package apollo

import (
	"fmt"
	"time"

	"github.com/apolloconfig/agollo/v4"
	apolloConf "github.com/apolloconfig/agollo/v4/env/config"
)

type Options struct {
	IP             string
	AppID          string
	Cluster        string
	Namespace      string
	Secret         string
	IsBackupConfig bool
	Timeout        time.Duration
}

func NewClient(opt Options) (agollo.Client, *apolloConf.AppConfig, error) {
	if opt.IP == "" || opt.AppID == "" || opt.Namespace == "" {
		return nil, nil, fmt.Errorf("apollo options invalid: ip/appId/namespace required")
	}
	if opt.Cluster == "" {
		opt.Cluster = "default"
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 10 * time.Second
	}

	ac := &apolloConf.AppConfig{
		AppID:          opt.AppID,
		Cluster:        opt.Cluster,
		IP:             opt.IP,
		NamespaceName:  opt.Namespace,
		IsBackupConfig: opt.IsBackupConfig,
		Secret:         opt.Secret,
	}

	client, err := agollo.StartWithConfig(func() (*apolloConf.AppConfig, error) {
		return ac, nil
	})
	if err != nil {
		return nil, nil, err
	}
	return client, ac, nil
}
