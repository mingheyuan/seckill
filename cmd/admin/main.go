package main

import (
	"fmt"
	"log"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/gin-gonic/gin"
	"seckill/internal/common/config"
	"seckill/internal/admin/controller"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	r:=gin.Default()
	controller.NewHandler(
		cfg.Nacos,
		cfg.Admin.LayerServiceName,
		cfg.Admin.DiscoveryInterval,
		cfg.Storage.RedisAddr,
		cfg.Admin.StockShards,
	).Register(r)

	
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
		Ip: cfg.Admin.RegisterIP,
		Port: cfg.Admin.RegisterPort,
		ServiceName: cfg.Admin.ServiceName,
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
	

	log.Printf("admin listening on %s", cfg.Admin.ListenAddr)
	if err:=r.Run(cfg.Admin.ListenAddr);err!=nil {
		log.Fatal(err)
	}
}

