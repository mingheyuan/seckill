package main

import (
	"log"
	"os"
	"strings"
	"strconv"

	"github.com/gin-gonic/gin"
	"seckill/internal/proxy/controller"
	"seckill/internal/proxy/middleware"
	"seckill/internal/proxy/service"
)

func main() {
	r:=gin.Default()
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	reqPerSec := getEnvInt("PROXY_REQ_PER_SEC",50)
	ipWhite := getEnvCSV("PROXY_IP_WHITELIST")
	ipBlack := getEnvCSV("PROXY_IP_BLACKLIST")

	r.Use(middleware.RequestID())
	r.Use(middleware.IPAccessControl(ipWhite,ipBlack))
	r.Use(middleware.NewRateLimiter(reqPerSec).Handler())

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

func getEnvCSV(key string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return nil
	}
	parts := strings.Split(v,",")
	out := make([]string,0,len(parts))
	for i := range parts {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			out = append(out,p)
		}
	}
	return out
}