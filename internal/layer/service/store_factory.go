package service

import (
	"fmt"
	"os"
	"strings"
)

const (
	StorageMemory ="memory"
)

func NewStoreFromEnv() (Store,error) {
	engine := strings.TrimSpace(strings.ToLower(os.Getenv("LAYER_STORAGE_ENGINE")))
	if engine =="" ||engine ==StorageMemory {
		return NewMemoryStore(),nil
	}

	return nil,fmt.Errorf("unknow storage engine: %s",engine)
}