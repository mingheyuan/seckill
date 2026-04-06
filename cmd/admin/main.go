package main

import (
	"fmt"
	"log"
	

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/gin-gonic/gin"
	"seckill/internal/admin/controller"
	"seckill/internal/admin/service"
)

func main() {
	r:=gin.Default()

    publisher, err := service.NewEtcdPublisherFromEnv()
    if err != nil {
        // 错误说明: 发布器初始化失败不阻塞 admin 启动，避免管理面不可用
        log.Printf("init etcd publisher failed: %v", err)
        publisher = &service.EtcdPublisher{}
    }
	controller.NewHandler(publisher).Register(r)

	
	//注册微服务
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
		Port:8082,
		ServiceName:"admin-service",
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
	

	log.Println("admin listening on :8082")
	if err:=r.Run(":8082");err!=nil {
		log.Fatal(err)
	}
}

