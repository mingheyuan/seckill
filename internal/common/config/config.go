package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "config/config.yaml"

type Config struct {
	Nacos   NacosConfig   `yaml:"nacos"`
	Storage StorageConfig `yaml:"storage"`
	Proxy   ProxyConfig   `yaml:"proxy"`
	Layer   LayerConfig   `yaml:"layer"`
	Admin   AdminConfig   `yaml:"admin"`
}

type NacosConfig struct {
	ServerIP    string `yaml:"serverIp"`
	ServerPort  uint64 `yaml:"serverPort"`
	NamespaceID string `yaml:"namespaceId"`
	LogDir      string `yaml:"logDir"`
	CacheDir    string `yaml:"cacheDir"`
}

type StorageConfig struct {
	Engine    string `yaml:"engine"`
	MySQLDSN  string `yaml:"mysqlDsn"`
	RedisAddr string `yaml:"redisAddr"`
}

type ProxyConfig struct {
	ListenAddr        string   `yaml:"listenAddr"`
	RegisterIP        string   `yaml:"registerIp"`
	RegisterPort      uint64   `yaml:"registerPort"`
	ServiceName       string   `yaml:"serviceName"`
	LayerServiceName  string   `yaml:"layerServiceName"`
	ReqPerSec         int      `yaml:"reqPerSec"`
	RequireSignature  bool     `yaml:"requireSignature"`
	SignSecret        string   `yaml:"signSecret"`
	SignMaxSkewSec    int64    `yaml:"signMaxSkewSec"`
	TrustedProxies    []string `yaml:"trustedProxies"`
	DiscoveryInterval int      `yaml:"discoveryIntervalSec"`
}

type LayerConfig struct {
	ListenAddr        string `yaml:"listenAddr"`
	RegisterIP        string `yaml:"registerIp"`
	RegisterPort      uint64 `yaml:"registerPort"`
	ServiceName       string `yaml:"serviceName"`
	UserLimitPerSec   int    `yaml:"userLimitPerSec"`
	WorkerCount       int    `yaml:"workerCount"`
	DiscoveryInterval int    `yaml:"discoveryIntervalSec"`
}

type AdminConfig struct {
	ListenAddr        string `yaml:"listenAddr"`
	RegisterIP        string `yaml:"registerIp"`
	RegisterPort      uint64 `yaml:"registerPort"`
	ServiceName       string `yaml:"serviceName"`
	LayerServiceName  string `yaml:"layerServiceName"`
	DiscoveryInterval int    `yaml:"discoveryIntervalSec"`
	StockShards       int    `yaml:"stockShards"`
}

func Load(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		path = strings.TrimSpace(os.Getenv("SECKILL_CONFIG_PATH"))
	}
	if strings.TrimSpace(path) == "" {
		path = DefaultPath
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config failed: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config failed: %w", err)
	}

	cfg.applyEnvOverrides()
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyEnvOverrides() {
	applyString := func(dst *string, key string) {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			*dst = v
		}
	}

	applyInt := func(dst *int, key string) {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			n, err := strconv.Atoi(v)
			if err == nil {
				*dst = n
			}
		}
	}

	applyUint64 := func(dst *uint64, key string) {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			n, err := strconv.ParseUint(v, 10, 64)
			if err == nil {
				*dst = n
			}
		}
	}

	applyString(&c.Storage.Engine, "STORAGE_ENGINE")
	applyString(&c.Storage.MySQLDSN, "STORAGE_MYSQL_DSN")
	applyString(&c.Storage.RedisAddr, "STORAGE_REDIS_ADDR")

	applyString(&c.Nacos.ServerIP, "NACOS_SERVER_IP")
	applyUint64(&c.Nacos.ServerPort, "NACOS_SERVER_PORT")
	applyString(&c.Nacos.NamespaceID, "NACOS_NAMESPACE_ID")
	applyString(&c.Nacos.LogDir, "NACOS_LOG_DIR")
	applyString(&c.Nacos.CacheDir, "NACOS_CACHE_DIR")

	applyString(&c.Proxy.ListenAddr, "PROXY_LISTEN_ADDR")
	applyString(&c.Proxy.RegisterIP, "PROXY_REGISTER_IP")
	applyUint64(&c.Proxy.RegisterPort, "PROXY_REGISTER_PORT")
	applyInt(&c.Proxy.ReqPerSec, "PROXY_REQ_PER_SEC")

	applyString(&c.Layer.ListenAddr, "LAYER_LISTEN_ADDR")
	applyString(&c.Layer.RegisterIP, "LAYER_REGISTER_IP")
	applyUint64(&c.Layer.RegisterPort, "LAYER_REGISTER_PORT")
	applyInt(&c.Layer.UserLimitPerSec, "LAYER_USER_LIMIT_PER_SEC")
	applyInt(&c.Layer.WorkerCount, "LAYER_WORKER_COUNT")

	applyString(&c.Admin.ListenAddr, "ADMIN_LISTEN_ADDR")
	applyString(&c.Admin.RegisterIP, "ADMIN_REGISTER_IP")
	applyUint64(&c.Admin.RegisterPort, "ADMIN_REGISTER_PORT")
	applyInt(&c.Admin.StockShards, "ADMIN_STOCK_SHARDS")
}

func (c *Config) applyDefaults() {
	if c.Nacos.ServerIP == "" {
		c.Nacos.ServerIP = "127.0.0.1"
	}
	if c.Nacos.ServerPort == 0 {
		c.Nacos.ServerPort = 8848
	}
	if c.Nacos.NamespaceID == "" {
		c.Nacos.NamespaceID = "seckill"
	}
	if c.Nacos.LogDir == "" {
		c.Nacos.LogDir = "/tmp/nacos/log"
	}
	if c.Nacos.CacheDir == "" {
		c.Nacos.CacheDir = "/tmp/nacos/cache"
	}

	if c.Storage.Engine == "" {
		c.Storage.Engine = "mysql-redis"
	}

	if c.Proxy.ListenAddr == "" {
		c.Proxy.ListenAddr = ":8080"
	}
	if c.Proxy.RegisterIP == "" {
		c.Proxy.RegisterIP = "127.0.0.1"
	}
	if c.Proxy.RegisterPort == 0 {
		c.Proxy.RegisterPort = 8080
	}
	if c.Proxy.ServiceName == "" {
		c.Proxy.ServiceName = "proxy-service"
	}
	if c.Proxy.LayerServiceName == "" {
		c.Proxy.LayerServiceName = "layer-service"
	}
	if c.Proxy.ReqPerSec <= 0 {
		c.Proxy.ReqPerSec = 50
	}
	if c.Proxy.SignSecret == "" {
		c.Proxy.SignSecret = "seckill_sign"
	}
	if c.Proxy.SignMaxSkewSec <= 0 {
		c.Proxy.SignMaxSkewSec = 30
	}
	if len(c.Proxy.TrustedProxies) == 0 {
		c.Proxy.TrustedProxies = []string{"127.0.0.1", "::1"}
	}
	if c.Proxy.DiscoveryInterval <= 0 {
		c.Proxy.DiscoveryInterval = 10
	}

	if c.Layer.ListenAddr == "" {
		c.Layer.ListenAddr = ":8081"
	}
	if c.Layer.RegisterIP == "" {
		c.Layer.RegisterIP = "127.0.0.1"
	}
	if c.Layer.RegisterPort == 0 {
		c.Layer.RegisterPort = 8081
	}
	if c.Layer.ServiceName == "" {
		c.Layer.ServiceName = "layer-service"
	}
	if c.Layer.UserLimitPerSec <= 0 {
		c.Layer.UserLimitPerSec = 20
	}
	if c.Layer.WorkerCount <= 0 {
		c.Layer.WorkerCount = 4
	}

	if c.Admin.ListenAddr == "" {
		c.Admin.ListenAddr = ":8082"
	}
	if c.Admin.RegisterIP == "" {
		c.Admin.RegisterIP = "127.0.0.1"
	}
	if c.Admin.RegisterPort == 0 {
		c.Admin.RegisterPort = 8082
	}
	if c.Admin.ServiceName == "" {
		c.Admin.ServiceName = "admin-service"
	}
	if c.Admin.LayerServiceName == "" {
		c.Admin.LayerServiceName = "layer-service"
	}
	if c.Admin.DiscoveryInterval <= 0 {
		c.Admin.DiscoveryInterval = 10
	}
	if c.Admin.StockShards <= 0 {
		c.Admin.StockShards = 30
	}
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.Storage.MySQLDSN) == "" {
		return fmt.Errorf("config storage.mysqlDsn is required")
	}
	if strings.TrimSpace(c.Storage.RedisAddr) == "" {
		return fmt.Errorf("config storage.redisAddr is required")
	}
	return nil
}
