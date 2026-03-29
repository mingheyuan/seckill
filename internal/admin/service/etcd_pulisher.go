package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"seckill/internal/common/model"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdPublisher struct {
	enabled bool
	key     string
	cli     *clientv3.Client
	queue   chan model.ActivityConfig
	maxRetries int
	retryInterval time.Duration
	queuedTotal   atomic.Uint64
	publishedTotal atomic.Uint64
	failedTotal   atomic.Uint64
	droppedTotal  atomic.Uint64
	mu            sync.RWMutex
	lastError     string
	lastErrorAt   time.Time
	lastSuccessAt time.Time
}

type PublishStats struct {
	Enabled        bool      `json:"enabled"`
	QueueLen       int       `json:"queueLen"`
	QueueCap       int       `json:"queueCap"`
	QueuedTotal    uint64    `json:"queuedTotal"`
	PublishedTotal uint64    `json:"publishedTotal"`
	FailedTotal    uint64    `json:"failedTotal"`
	DroppedTotal   uint64    `json:"droppedTotal"`
	LastError      string    `json:"lastError,omitempty"`
	LastErrorAt    time.Time `json:"lastErrorAt,omitempty"`
	LastSuccessAt  time.Time `json:"lastSuccessAt,omitempty"`
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

	queueSize := envIntOr("ADMIN_ETCD_QUEUE_SIZE",256)
	if queueSize <= 0 {
		queueSize = 256
	}
	retries := envIntOr("ADMIN_ETCD_PUBLISH_RETRIES",3)
	if retries < 0 {
		retries = 0
	}
	interval := time.Duration(envIntOr("ADMIN_ETCD_RETRY_INTERVAL_MS",300)) * time.Millisecond
	if interval <= 0 {
		interval = 300 * time.Millisecond
	}

	p := &EtcdPublisher{
        enabled: true,
        key:     key,
        cli:     cli,
		queue:   make(chan model.ActivityConfig, queueSize),
		maxRetries: retries,
		retryInterval: interval,
    }

	go p.startWorker(context.Background())
	return p, nil
}

func (p *EtcdPublisher) PublishActivity(cfg model.ActivityConfig) error {
    if p == nil || !p.enabled || p.cli == nil {
        return nil
    }

	select {
	case p.queue <- cfg:
		p.queuedTotal.Add(1)
		return nil
	default:
		p.droppedTotal.Add(1)
		p.setLastError("etcd publish queue full")
		return fmt.Errorf("etcd publish queue full")
	}
}

func (p *EtcdPublisher) Stats() PublishStats {
	if p == nil || !p.enabled {
		return PublishStats{Enabled:false}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	return PublishStats{
		Enabled: true,
		QueueLen: len(p.queue),
		QueueCap: cap(p.queue),
		QueuedTotal: p.queuedTotal.Load(),
		PublishedTotal: p.publishedTotal.Load(),
		FailedTotal: p.failedTotal.Load(),
		DroppedTotal: p.droppedTotal.Load(),
		LastError: p.lastError,
		LastErrorAt: p.lastErrorAt,
		LastSuccessAt: p.lastSuccessAt,
	}
}

func (p *EtcdPublisher) startWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case cfg := <-p.queue:
			p.publishWithRetry(cfg)
		}
	}
}

func (p *EtcdPublisher) publishWithRetry(cfg model.ActivityConfig) {
    raw, err := json.Marshal(cfg)
    if err != nil {
		p.failedTotal.Add(1)
		p.setLastError(fmt.Sprintf("marshal activity config failed: %v", err))
        return
    }

	ctx := context.Background()
	var lastErr error
	for i := 0; i <= p.maxRetries; i++ {
		_, err = p.cli.Put(ctx, p.key, string(raw))
		if err == nil {
			p.publishedTotal.Add(1)
			p.setLastSuccess()
			return
		}
		lastErr = err
		if i < p.maxRetries {
			time.Sleep(p.retryInterval)
		}
	}

	p.failedTotal.Add(1)
	p.setLastError(fmt.Sprintf("publish activity failed after retries: %v", lastErr))
}

func (p *EtcdPublisher) setLastError(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = msg
	p.lastErrorAt = time.Now()
}

func (p *EtcdPublisher) setLastSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastSuccessAt = time.Now()
}

func envIntOr(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return fallback
	}
	return n
}