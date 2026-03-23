package main

import(
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"seckill/internal/layer/controller"
	"seckill/internal/layer/service"
)

func main() {
	ctx :=context.Background()
	core :=service.NewCore(ctx)

	r:=gin.Default()
	controller.NewHandler(core).Register(r)
	
	log.Println("layer listening on :8081")
	if err := r.Run(":8081");err !=nil {
		log.Fatal(err)
	}
}