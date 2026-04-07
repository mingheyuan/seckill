package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/redis/go-redis/v9"
	"seckill/internal/proxy/controller"
	"seckill/internal/proxy/middleware"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	r:=gin.Default()
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	reqPerSec := getEnvInt("PROXY_REQ_PER_SEC",50)
	ipWhite := getEnvCSV("PROXY_IP_WHITELIST")
	ipBlack := getEnvCSV("PROXY_IP_BLACKLIST")
	_ = ipWhite
	_ = ipBlack

	r.Use(middleware.RequestID())
	r.Use(middleware.IPAccessControl(rdb))
	r.Use(middleware.NewRateLimiter(reqPerSec).Handler())

	h:=controller.NewHandler(
		getEnvBool("PROXY_REQUIRE_SIGNATURE",false),
		getEnv("PROXY_SIGN_SECRET","seckill_sign"),
		int64(getEnvInt("PROXY_SIGN_MAX_SKEW_SEC",30)),
	)
	h.Register(r)

	
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
		Port:8080,
		ServiceName:"proxy-service",
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