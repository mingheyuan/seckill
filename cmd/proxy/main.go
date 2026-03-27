package main

import (
	"log"
	"os"
	"strings"
	"strconv"

	"github.com/gin-gonic/gin"
	"seckill/internal/proxy/controller"
	"seckill/internal/proxy/service"
)

func main() {
	r:=gin.Default()

	layerClient :=service.NewLayerClient("http://127.0.0.1:8081")
	h:=controller.NewHandler(
		layerClient,
		getEnvBool("PROXY_REQUIRE_SIGNATURE",false),
		getEnv("PROXY_SIGN_SECRET","seckill_sign"),
		int64(getEnvInt("PROXY_SIGN_MAX_SKEW_SEC",30)),
	)
	h.Register(r)

	log.Println("proxy listening on :8080")
	if err := r.Run(":8080");err!=nil {
		log.Fatal(err)
	}
}

func getEnv(key,fallback string) string {
	v:=strings.TrimSpace(os.Getenv(key))
	if v=="" {
		return fallback
	}
	return v
}

func getEnvInt(key string,fallback int) int {
	v:=strings.TrimSpace(os.Getenv(key))
	if v=="" {
		return fallback
	}
	n,err :=strconv.Atoi(v)
	if err !=nil {
		return fallback
	}
	return n
}

func getEnvBool(key string,fallback bool) bool {
	v:=strings.TrimSpace(os.Getenv(key))
	if v=="" {
		return fallback
	}
	b,err :=strconv.ParseBool(v)
	if err !=nil {
		return fallback
	}
	return b
}