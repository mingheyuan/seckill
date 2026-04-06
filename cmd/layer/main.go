package main

import(
	"context"
	"log"
	"fmt"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"github.com/gin-gonic/gin"
	"seckill/internal/layer/controller"
	"seckill/internal/layer/service"
)

func main() {
	ctx :=context.Background()
	core :=service.NewCore(ctx)

	service.StartEtcdActivitySync(ctx,core)

	r:=gin.Default()
	controller.NewHandler(core).Register(r)
	

	sc:=[]constant.ServerConfig{{IpAddr:"127.0.0.1",Port:8848}}

	cc:=constant.ClientConfig{
		NamespaceId:"seckill",
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
		Ip:"127.0.0.1",
		Port:8081,
		ServiceName:"layer-service",
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
	log.Println("layer listening on :8081")
	if err := r.Run(":8081");err !=nil {
		log.Fatal(err)
	}

}