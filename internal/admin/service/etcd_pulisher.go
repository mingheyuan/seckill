package service

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"seckill/internal/common/model"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdPublisher struct {
	enabled bool
	key 	string
	cli 	*clientv3.Client
}

func NewEtcdPublisherFromEnv() (*EtcdPublisher,error) {
	enabled := strings.EqualFold(strings.TrimSpace(os.Getenv("ADMIN_ETCD_ENABLED")),"true")
	if !enabled {
		return &EtcdPublisher{enabled:false},nil
	}

	endpointsRaw :=strings.TrimSpace(os.Getenv("ADMIN_ETCD_ENDPOINTS"))
    if endpointsRaw == "" {
        // 错误说明: 开了 ADMIN_ETCD_ENABLED 但没配 endpoints，降级为 disabled，避免 admin 启动失败
        return &EtcdPublisher{enabled: false}, nil
    }

    key := strings.TrimSpace(os.Getenv("ADMIN_ETCD_ACTIVITY_KEY"))
    if key == "" {
        key = "/seckill/activity/config"
    }

	parts :=strings.Split(endpointsRaw,",")
	endpoints :=make([]string,0,len(parts))
	for i:= range parts {
		p:=strings.TrimSpace(parts[i])
		if p!="" {
			endpoints =append(endpoints,p)
		}
	}

	cli,err:=clientv3.New(clientv3.Config{
		Endpoints: 	endpoints,
		DialTimeout:5*time.Second,
	})
	if err !=nil {
		return nil,err
	}

	return &EtcdPublisher{
        enabled: true,
        key:     key,
        cli:     cli,
    }, nil
}

func (p *EtcdPublisher) PublishActivity(cfg model.ActivityConfig) error {
    if p == nil || !p.enabled || p.cli == nil {
        return nil
    }

    raw, err := json.Marshal(cfg)
    if err != nil {
        return err
    }

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    _, err = p.cli.Put(ctx, p.key, string(raw))
    return err
}