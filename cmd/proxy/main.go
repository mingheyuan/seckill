package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"seckill/internal/proxy/controller"
	"seckill/internal/proxy/service"
)

func main() {
	r:=gin.Default()

	layerClient :=service.NewLayerClient("http://127.0.0.1:8081")
	controller.NewHandler(layerClient).Register(r)

	log.Println("proxy listening on :8080")
	if err := r.Run(":8080");err!=nil {
		log.Fatal(err)
	}
}