package redisx

import (
	"strings"

	"github.com/redis/go-redis/v9"
)

func NewClient(addrsRaw string, poolSize int) redis.UniversalClient {
	parts := strings.Split(addrsRaw, ",")
	addrs := make([]string, 0, len(parts))
	for i := range parts {
		addr := strings.TrimSpace(parts[i])
		if addr != "" {
			addrs = append(addrs, addr)
		}
	}
	if len(addrs) == 0 {
		addrs = []string{"127.0.0.1:6379"}
	}

	return redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    addrs,
		DB:       0,
		PoolSize: poolSize,
	})
}