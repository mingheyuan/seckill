package controller

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	nacosmodel "github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/gin-gonic/gin"
	"seckill/internal/admin/service"
    "seckill/internal/common/model"
)

type Handler struct {
	client *service.LayerAdminClient
	clientaddr string
	publisher *service.EtcdPublisher
}

func NewHandler(publisher *service.EtcdPublisher) *Handler {
	h:=&Handler{publisher:publisher}
	go h.ServiceConfigLoop()

	return h
}


func (h *Handler) ServiceConfigLoop(){
	sc := []constant.ServerConfig{
		{IpAddr: "127.0.0.1", Port: 8848}, 
	}
	cc := constant.ClientConfig{
		NamespaceId:         "seckill", 
		NotLoadCacheAtStart:  true,   
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
	}

	client, err := clients.CreateNamingClient(map[string]interface{}{
		"serverConfigs": sc,
		"clientConfig":  cc,
	})
	if err != nil {
		fmt.Println("创建失败：", err)
		return
	}

	instances, err :=client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: "layer-service",
		HealthyOnly: true, // 只拿健康的
	})
	if err != nil {
		fmt.Println("刷新失败:", err)
	}

	if addr, ok := firstInstanceAddr(instances); ok {
		h.client = service.NewLayerAdminClient(addr)
		h.clientaddr = addr
	}

	//loop循环
	ticker:=time.NewTicker(10 *time.Second)
	defer ticker.Stop()

	for{
		select {
		case <-ticker.C:
			instances, err :=client.SelectInstances(vo.SelectInstancesParam{
				ServiceName: "layer-service",
				HealthyOnly: true, // 只拿健康的
			})
			if err != nil {
				fmt.Println("刷新失败:", err)
				continue
			}
			if addr, ok := firstInstanceAddr(instances); ok && h.clientaddr != addr {
				h.client = service.NewLayerAdminClient(addr)
				h.clientaddr = addr
			}
		}
	}
	
}

func firstInstanceAddr(instances []nacosmodel.Instance) (string, bool) {
	if len(instances) == 0 {
		return "", false
	}
	ip := strings.TrimSpace(instances[0].Ip)
	if ip == "" || instances[0].Port <= 0 {
		return "", false
	}
	return fmt.Sprintf("http://%s:%d", ip, instances[0].Port), true
}


type initReq struct {
	ActivityID 	int64 	`json:"activity_id" binding:"required"`
	Stock 		int64 	`json:"stock" binding:"required"`
} 

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/healthz",h.Healthz)
	r.POST("/admin/activity/init",h.Init)
	r.GET("/admin/activity",h.GetActivity)
	r.POST("/admin/activity",h.UpdateActivity)
	r.GET("/admin/activity/sync/stats",h.SyncStats)
}

func (h *Handler) SyncStats(c *gin.Context) {
	c.JSON(http.StatusOK,h.publisher.Stats())
}

func (h *Handler) GetActivity(c *gin.Context) {
	cfg,err :=h.client.GetActivity()
	if err !=nil {
		c.JSON(http.StatusBadGateway,gin.H{"code":502,"message":"layer unvailable"})
		return
	}
	c.JSON(http.StatusOK,cfg)
}

func (h *Handler) UpdateActivity(c *gin.Context) {
	var req model.ActivityConfig
	if err :=c.ShouldBindJSON(&req);err !=nil {
		c.JSON(http.StatusBadRequest,gin.H{"code":400,"message":"bad request"})
		return
	}
	if err :=h.client.UpdateActivity(req);err !=nil {
		c.JSON(http.StatusBadGateway,gin.H{"code":502,"message":"layer unavailable"})
		return
	}

    // 错误说明: etcd 同步失败不应让主流程失败，否则 admin 配置会因为 etcd 短抖动不可用
    if err := h.publisher.PublishActivity(req); err != nil {
        log.Printf("publish activity to etcd failed: %v", err)
    }
	c.JSON(http.StatusOK,gin.H{"code":0,"message":"ok"})
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK,gin.H{"status":"ok"})
}

func (h *Handler) Init(c *gin.Context) {
	var req initReq
	if err :=c.ShouldBindJSON(&req);err !=nil {
		c.JSON(http.StatusBadRequest,gin.H{"code":400,"message":"bad request"})
		return
	}
	if err :=h.client.Init(req.ActivityID,req.Stock);err !=nil {
		c.JSON(http.StatusBadGateway,gin.H{"code":502,"message":"layer unvailable"})
		return
	}
	c.JSON(http.StatusOK,gin.H{"code":0,"message":"ok"})
}