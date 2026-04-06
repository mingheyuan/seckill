package controller

import (
	"net/http"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	nacosmodel "github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"github.com/gin-gonic/gin"
	"seckill/internal/common/model"
	proxysvc "seckill/internal/proxy/service"
	layersvc "seckill/internal/layer/service"
)

type Handler struct {
	layer 				*proxysvc.LayerClient
	layerAddr 			string
	requireSignature 	bool
	signSecret 			string
	maxSkewSec			int64
}

func NewHandler(requireSignature bool,signSecret string,maxSkewSec int64) *Handler {
	if maxSkewSec<=0 {
		maxSkewSec =30
	}
	h:= &Handler{
		requireSignature:requireSignature,
		signSecret:signSecret,
		maxSkewSec:maxSkewSec,
	}
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
		h.layer = proxysvc.NewLayerClient(addr)
		h.layerAddr = addr
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
			if addr, ok := firstInstanceAddr(instances); ok && h.layerAddr != addr {
				h.layer=proxysvc.NewLayerClient(addr)
				h.layerAddr=addr
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



func (h *Handler) Register(r *gin.Engine) {
	r.GET("/healthz",h.Healthz)
	r.POST("/api/seckill",h.Seckill)
	r.GET("/api/orders",h.OrdersByUser)
}

func (h *Handler) OrdersByUser(c *gin.Context) {
	userID :=c.Query("user_id")
	if userID =="" {
		c.JSON(http.StatusBadRequest,gin.H{"code":400,"message":"user_id required"})
		return
	}

	orders,err :=h.layer.OrdersByUser(userID)
	if err !=nil {
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
	ret,err := h.layer.Seckill(req)
	if err != nil {
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