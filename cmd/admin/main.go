package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"seckill/internal/admin/controller"
	"seckill/internal/admin/service"
)

func main() {
	r:=gin.Default()

	client:=service.NewLayerAdminClient("http://127.0.0.1:8081")

    publisher, err := service.NewEtcdPublisherFromEnv()
    if err != nil {
        // 错误说明: 发布器初始化失败不阻塞 admin 启动，避免管理面不可用
        log.Printf("init etcd publisher failed: %v", err)
        publisher = &service.EtcdPublisher{}
    }
	controller.NewHandler(client,publisher).Register(r)

	log.Println("admin listening on :8082")
	if err:=r.Run(":8082");err!=nil {
		log.Fatal(err)
	}
}

