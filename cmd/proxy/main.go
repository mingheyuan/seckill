package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"seckill/internal/common/config"
	"seckill/internal/common/redisx"
	"seckill/internal/proxy/controller"
	"seckill/internal/proxy/middleware"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	rdb := redisx.NewClient(cfg.Storage.RedisAddr, 0)

	r:=gin.Default()
	_ = r.SetTrustedProxies(cfg.Proxy.TrustedProxies)


	r.Use(middleware.RequestID())
	r.Use(middleware.IPAccessControl(rdb))
	r.Use(middleware.NewRateLimiter(cfg.Proxy.ReqPerSec).Handler())

	h:=controller.NewHandler(
		cfg.Proxy.RequireSignature,
		cfg.Proxy.SignSecret,
		cfg.Proxy.SignMaxSkewSec,
		cfg.Nacos,
		cfg.Proxy.LayerServiceName,
		cfg.Proxy.DiscoveryInterval,
	)
	h.Register(r)

	
	//注册微服务
	sc:=[]constant.ServerConfig{{IpAddr:cfg.Nacos.ServerIP,Port:cfg.Nacos.ServerPort}}

	cc:=constant.ClientConfig{
		NamespaceId: cfg.Nacos.NamespaceID,
		NotLoadCacheAtStart: true,
		LogDir: cfg.Nacos.LogDir,
		CacheDir: cfg.Nacos.CacheDir,
	}

	namingClient,err:=clients.CreateNamingClient(map[string]interface{}{
		"serverConfigs":sc,
		"clientConfig":cc,
	})
	if err!=nil {
		fmt.Println(err)
		return
	}

	success,err:=namingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip: cfg.Proxy.RegisterIP,
		Port: cfg.Proxy.RegisterPort,
		ServiceName: cfg.Proxy.ServiceName,
		Weight:1.0,
		Enable:true,
		Healthy:true,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = success

	fmt.Println("proxy-service注册成功")
	
	log.Printf("proxy listening on %s", cfg.Proxy.ListenAddr)
	if err := r.Run(cfg.Proxy.ListenAddr);err!=nil {
		log.Fatal(err)
	}

}