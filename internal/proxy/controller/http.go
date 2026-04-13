package controller

import (
	"net/http"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	nacosmodel "github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"github.com/gin-gonic/gin"
	"seckill/internal/common/config"
	"seckill/internal/common/model"
	proxysvc "seckill/internal/proxy/service"
	layersvc "seckill/internal/layer/service"
)

type Handler struct {
	layers 				[]*proxysvc.LayerClient
	layerAddrs 			[]string
	mu 					sync.RWMutex
	rr 					uint64
	requireSignature 	bool
	signSecret 			string
	maxSkewSec			int64
	nacosCfg			config.NacosConfig
	layerService		string
	refreshInterval		time.Duration
}


func (h *Handler) Register(r *gin.Engine) {
	r.GET("/healthz",h.Healthz)
	r.POST("/api/seckill",h.Seckill)
	r.GET("/api/orders",h.OrdersByUser)
}

func NewHandler(requireSignature bool,signSecret string,maxSkewSec int64,nacosCfg config.NacosConfig,layerService string,intervalSec int) *Handler {
	if maxSkewSec<=0 {
		maxSkewSec =30
	}
	if strings.TrimSpace(layerService) == "" {
		layerService = "layer-service"
	}
	if intervalSec <= 0 {
		intervalSec = 10
	}
	h:= &Handler{
		requireSignature:requireSignature,
		signSecret:signSecret,
		maxSkewSec:maxSkewSec,
		nacosCfg: nacosCfg,
		layerService: layerService,
		refreshInterval: time.Duration(intervalSec) * time.Second,
	}
	go h.ServiceConfigLoop()

	return h
}

func (h *Handler) ServiceConfigLoop(){
	sc := []constant.ServerConfig{
		{IpAddr: h.nacosCfg.ServerIP, Port: h.nacosCfg.ServerPort},
	}
	cc := constant.ClientConfig{
		NamespaceId: h.nacosCfg.NamespaceID,
		NotLoadCacheAtStart: true,
		LogDir: h.nacosCfg.LogDir,
		CacheDir: h.nacosCfg.CacheDir,
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
		ServiceName: h.layerService,
		HealthyOnly: true, // 只拿健康的
	})
	if err != nil {
		fmt.Println("刷新失败:", err)
	}

	h.setLayerClients(instances)

	//loop循环
	ticker:=time.NewTicker(h.refreshInterval)
	defer ticker.Stop()

	for{
		select {
		case <-ticker.C:
			instances, err :=client.SelectInstances(vo.SelectInstancesParam{
				ServiceName: h.layerService,
				HealthyOnly: true, // 只拿健康的
			})
			if err != nil {
				fmt.Println("刷新失败:", err)
				continue
			}
			h.setLayerClients(instances)
		}
	}
	
}

func (h *Handler) setLayerClients(instances []nacosmodel.Instance) {
	addrs := make([]string, 0, len(instances))
	clients := make([]*proxysvc.LayerClient, 0, len(instances))
	for i := range instances {
		ip := strings.TrimSpace(instances[i].Ip)
		if ip == "" || instances[i].Port <= 0 {
			continue
		}
		addr := fmt.Sprintf("http://%s:%d", ip, instances[i].Port)
		addrs = append(addrs, addr)
		clients = append(clients, proxysvc.NewLayerClient(addr))
	}

	h.mu.Lock()
	h.layerAddrs = addrs
	h.layers = clients
	h.mu.Unlock()
}

func (h *Handler) pickLayer() *proxysvc.LayerClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.layers) == 0 {
		return nil
	}
	idx := atomic.AddUint64(&h.rr, 1)
	return h.layers[idx%uint64(len(h.layers))]
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




func (h *Handler) OrdersByUser(c *gin.Context) {
	userID :=c.Query("user_id")
	if userID =="" {
		c.JSON(http.StatusBadRequest,gin.H{"code":400,"message":"user_id required"})
		return
	}

	layer := h.pickLayer()
	if layer == nil {
		c.JSON(http.StatusBadGateway,gin.H{"code":502,"message":"layer unavailable"})
		return
	}

	orders,err :=layer.OrdersByUser(userID)
	if err !=nil {
		log.Printf("proxy ordersByUser failed: user=%s err=%v", userID, err)
		c.JSON(http.StatusBadGateway,gin.H{"code":502,"message":"layer unavailable"})
		return
	}

	c.JSON(http.StatusOK,gin.H{
		"code":0,
		"orders":orders,
	})
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK,gin.H{"status":"ok"})
}

func (h *Handler) Seckill(c *gin.Context) {
	var req model.SeckillRequest
	if err :=c.ShouldBindJSON(&req);err !=nil {
		c.JSON(http.StatusBadRequest,model.SeckillResponse{Code:400,Message:"bad request"})
		return
	}

	if h.requireSignature{
		if !h.verifySignature(c,req) {
			c.JSON(http.StatusUnauthorized,model.SeckillResponse{Code:401,Message:"invalid signature"})
			return
		}
	}
	layer := h.pickLayer()
	if layer == nil {
		c.JSON(http.StatusBadGateway,model.SeckillResponse{Code:502,Message:"layer unavailable"})
		return
	}

	ret,err := layer.Seckill(req)
	if err != nil {
		log.Printf("proxy seckill failed: user=%s activity=%d err=%v", req.UserID, req.ActivityID, err)
		c.JSON(http.StatusBadGateway,model.SeckillResponse{Code:502,Message:"layer unavailable"})
		return
	}
	if !ret.OK{
		code := 1003
        switch ret.Message {
        case layersvc.ErrDuplicateOrder, layersvc.ErrSoldOut, layersvc.ErrActivityClosed:
            code = 1001
        case layersvc.ErrTooFrequent, layersvc.ErrSystemBusy:
            code = 1002
        }

		c.JSON(http.StatusOK,model.SeckillResponse{
			Code:code,
			Message:ret.Message,
		})
		return
	}
	c.JSON(http.StatusOK,model.SeckillResponse{Code:0,Message:"success"})
}

func (h *Handler) verifySignature(c *gin.Context,req model.SeckillRequest) bool {
	tsText :=c.GetHeader("X-Timestamp")
	signature :=c.GetHeader("X-Signature")
	if tsText == "" || signature == ""{
		return false
	}


	ts,err :=strconv.ParseInt(tsText,10,64)
	if err !=nil {
		return false
	}

	now :=time.Now().Unix()
	delta := now -ts
	if delta<0 {
		delta =-delta
	}
	if delta >h.maxSkewSec {
		return false
	}

	payload :=fmt.Sprintf("%s:%d:%d",req.UserID,req.ActivityID,ts)
	mac :=hmac.New(sha256.New,[]byte(h.signSecret))
	mac.Write([]byte(payload))
	expected:=hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected),[]byte(signature))
}