package main

import(
	"context"
	"log"
	"fmt"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"github.com/gin-gonic/gin"
	"seckill/internal/common/config"
	"seckill/internal/layer/controller"
	"seckill/internal/layer/service"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	ctx :=context.Background()
	core :=service.NewCore(ctx, cfg)

	r:=gin.Default()
	controller.NewHandler(core).Register(r)
	

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
		Ip: cfg.Layer.RegisterIP,
		Port: cfg.Layer.RegisterPort,
		ServiceName: cfg.Layer.ServiceName,
		Weight:1.0,
		Enable:true,
		Healthy:true,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = success

	fmt.Println("layer-service注册成功")
	log.Printf("layer listening on %s", cfg.Layer.ListenAddr)
	if err := r.Run(cfg.Layer.ListenAddr);err !=nil {
		log.Fatal(err)
	}

}