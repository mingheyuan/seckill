package service

import (
	"fmt"
	"strings"

	"seckill/internal/common/config"
)

const (
	StorageMySQLRedis ="mysql-redis"
)

func NewStore(cfg *config.Config) (Store,error) {
	engine := strings.TrimSpace(strings.ToLower(cfg.Storage.Engine))
	if engine =="" {
		engine = StorageMySQLRedis
	}
	
	switch engine {
	case StorageMySQLRedis:
		s,err :=NewMySQLRedisStore(cfg.Storage.MySQLDSN, cfg.Storage.RedisAddr)
		if err !=nil {
			return nil,fmt.Errorf("init mysql-redis store failed: %w",err)
		}
		return s,nil
	default:
	return nil,fmt.Errorf("unknown storage engine: %s",engine)
	}
}