package service

import (
	"fmt"
	"os"
	"strings"
)

const (
	StorageMemory ="memory"
	StorageMySQLRedis ="mysql-redis"
)

func NewStoreFromEnv() (Store,error) {
	engine := strings.TrimSpace(strings.ToLower(os.Getenv("LAYER_STORAGE_ENGINE")))
	if engine =="" ||engine ==StorageMemory {
		return NewMemoryStore(),nil
	}
	
	switch engine {
	case StorageMySQLRedis:
		s,err :=NewMySQLRedisStoreFromEnv()
		if err !=nil {
			return nil,fmt.Errorf("init mysql-redis store failed: %w",err)
		}
		return s,nil
	default:
	return nil,fmt.Errorf("unknown storage engine: %s",engine)
	}
}