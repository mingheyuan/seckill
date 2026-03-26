package service

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"seckill/internal/common/model"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const defaultActivityKey ="/seckill/activity/config"

func StartEtcdActivitySync(ctx context.Context,core *Core) {
	enabled :=strings.EqualFold(strings.TrimSpace(os.Getenv("LAYER_ETCD_ENABLED")),"true")
	if !enabled {
		return
	}

	endpointsRaw:=strings.TrimSpace(os.Getenv("LAYER_ETCD_ENDPOINTS"))
	if endpointsRaw == "" {
 		log.Printf("etcd sync skipped: empty LAYER_ETCD_ENDPOINTS")
        return
	}

	endpoints:=splitCSV(endpointsRaw)
	key := strings.TrimSpace(os.Getenv("LAYER_ETCD_ACTIVITY_KEY"))
	if key =="" {
		key =defaultActivityKey
	}

	go watchActivityLoop(ctx,endpoints,key,core)
}

func watchActivityLoop(ctx context.Context,endpoints []string,key string,core *Core) {
	for{
		if err :=watchActivityOnce(ctx,endpoints,key,core);err !=nil {
            log.Printf("etcd watch disconnected: %v", err)
		}
		if ctx.Err() !=nil {
			return
		}
		time.Sleep(1*time.Second)
	}
}

func watchActivityOnce(ctx context.Context,endpoints []string,key string,core *Core) error {
	cli,err := clientv3.New(clientv3.Config{
		Endpoints: 	endpoints,
		DialTimeout:5*time.Second,
	})

	if err !=nil {
		return err
	}
	defer cli.Close()

	if err :=loadActivityOnce(ctx,cli,key,core);err !=nil {
        log.Printf("etcd bootstrap apply failed: %v", err)
	}

	wch :=cli.Watch(ctx,key)
	for wr :=range wch {
		if wr.Err() !=nil {
			return wr.Err()
		}
		for i:=range wr.Events {
			ev :=wr.Events[i]
			if ev.Kv ==nil || len(ev.Kv.Value)==0 {
				continue
			}
			if err :=applyActivityJSON(ev.Kv.Value,core);err !=nil {
                log.Printf("etcd activity update ignored: %v", err)
                continue
			}
			log.Printf("etcd activity applied from key=%s", key)
		}
	}

	return context.Canceled
}

func loadActivityOnce(ctx context.Context,cli *clientv3.Client,key string,core *Core) error {
	resp,err := cli.Get(ctx,key)
	if err !=nil {
		return err
	}
	if len(resp.Kvs) == 0 {
		return nil
	}
	return applyActivityJSON(resp.Kvs[0].Value,core)
}

func applyActivityJSON(raw []byte,core *Core) error {
	var cfg model.ActivityConfig
	if err :=json.Unmarshal(raw,&cfg);err !=nil {
		return err
	}

    if cfg.EndAtUnix < cfg.StartAtUnix {
        return nil
    }
    if cfg.UserProductLimit <= 0 {
        cfg.UserProductLimit = 1
    }
    core.UpdateActivity(cfg)

	return nil
}

func splitCSV(s string) []string {
	parts :=strings.Split(s,",")
	out :=make([]string,0,len(parts))
	for i:=range parts {
		p:=strings.TrimSpace(parts[i])
		if p!="" {
			out=append(out,p)
		}
	}
	return out
}