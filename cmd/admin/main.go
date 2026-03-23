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
	controller.NewHandler(client).Register(r)

	log.Println("admin listening on :8082")
	if err:=r.Run(":8082");err!=nil {
		log.Fatal(err)
	}
}

