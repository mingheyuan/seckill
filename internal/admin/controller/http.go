package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	nacosmodel "github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/gin-gonic/gin"
	"seckill/internal/common/config"
	"seckill/internal/common/model"
	"seckill/internal/admin/service"
)

type Handler struct {
	client          *service.LayerAdminClient
	clientaddr      string
	nacosCfg        config.NacosConfig
	layerService    string
	refreshInterval time.Duration
	core            service.Core
	stockShards     int
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/healthz",h.Healthz)

	r.POST("/admin/activity", h.CreateActivity)           // 创建活动
	r.GET("/admin/activity/list", h.ListActivities)	 	  // 查询所有活动
	r.GET("/admin/activity/:id", h.GetActivity)           // 查询活动
	r.PUT("/admin/activity/:id", h.UpdateActivity)        // 更新活动
	r.DELETE("/admin/activity/:id", h.DeleteActivity)     // 删除活动

	// 库存管理
	r.POST("/admin/activity/:id/stock/init", h.InitStock)     // 初始化库存
	r.POST("/admin/activity/:id/stock/incr", h.IncrStock)     // 补充库存
	r.GET("/admin/activity/:id/stock", h.GetStock)            // 查询库存

	// 其他管理
	r.POST("/admin/activity/:id/stop", h.StopActivity)        // 停止活动
	r.POST("/admin/activity/:id/start", h.StartActivity) 
}

func NewHandler(nacosCfg config.NacosConfig, layerService string, intervalSec int, redisAddr string, stockShards int) *Handler {
	if strings.TrimSpace(layerService) == "" {
		layerService = "layer-service"
	}
	if intervalSec <= 0 {
		intervalSec = 10
	}
	if stockShards <= 0 {
		stockShards = 30
	}

	h:=&Handler{
		nacosCfg: nacosCfg,
		layerService: layerService,
		refreshInterval: time.Duration(intervalSec) * time.Second,
		core: service.NewCore(redisAddr, stockShards),
		stockShards: stockShards,
	}
	go h.ServiceConfigLoop()

	return h
}

// 活动管理
type createActivityReq struct {
	ActivityID int64 `json:"activity_id" binding:"required"`
	Stock      int64 `json:"stock" binding:"required"`
	model.ActivityConfig
}

func (h *Handler) CreateActivity(c *gin.Context) {
	var req createActivityReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad request"})
		return
	}
	err := h.core.CreateActivity(c.Request.Context(), req.ActivityID, req.Stock, req.ActivityConfig)
	if err != nil {
		switch err {
		case service.ErrInvalidActivity:
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "activity_id and stock must be positive"})
			return
		case service.ErrActivityExists:
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "activity already exists"})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis write failed"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":        0,
		"message":     "ok",
		"activity_id": req.ActivityID,
		"stock":       req.Stock,
		"shards":      h.stockShards,
	})

}      // 创建活动

func (h *Handler) GetActivity(c *gin.Context) {
	activityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		return
	}

	cfg, err := h.core.GetActivityConfig(c.Request.Context(), activityID)
	if err != nil {
		switch err {
		case service.ErrInvalidActivity:
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		case service.ErrActivityNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "activity not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis read failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":        0,
		"activity_id": activityID,
		"config":      cfg,
	})
}         // 查询活动
func (h *Handler) ListActivities(c *gin.Context) {
	items, err := h.core.ListActivities(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis read failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":       0,
		"activities": items,
	})
}      // 查询所有活动
func (h *Handler) UpdateActivity(c *gin.Context) {
	activityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		return
	}

	var req model.ActivityConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad request"})
		return
	}

	err = h.core.UpdateActivityConfig(c.Request.Context(), activityID, req)
	if err != nil {
		switch err {
		case service.ErrInvalidActivity:
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		case service.ErrActivityNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "activity not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis write failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "activity_id": activityID})
}      // 更新活动
func (h *Handler) DeleteActivity(c *gin.Context) {
	activityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		return
	}

	err = h.core.DeleteActivity(c.Request.Context(), activityID)
	if err != nil {
		switch err {
		case service.ErrInvalidActivity:
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		case service.ErrActivityNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "activity not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis write failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "activity_id": activityID})
}      // 删除活动

// 库存管理
func (h *Handler) InitStock(c *gin.Context) {
	activityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		return
	}

	var req struct {
		Stock int64 `json:"stock" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad request"})
		return
	}

	err = h.core.InitStock(c.Request.Context(), activityID, req.Stock)
	if err != nil {
		switch err {
		case service.ErrInvalidActivity:
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "activity_id and stock must be positive"})
		case service.ErrActivityNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "activity not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis write failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "activity_id": activityID, "stock": req.Stock, "shards": h.stockShards})
}           // 初始化库存
func (h *Handler) IncrStock(c *gin.Context) {
	activityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		return
	}

	var req struct {
		Stock int64 `json:"stock" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad request"})
		return
	}

	err = h.core.IncrStock(c.Request.Context(), activityID, req.Stock)
	if err != nil {
		switch err {
		case service.ErrInvalidActivity:
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "activity_id and stock must be positive"})
		case service.ErrActivityNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "activity not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis write failed"})
		}
		return
	}

	total, err := h.core.GetStock(c.Request.Context(), activityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis read failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "activity_id": activityID, "stock": total, "shards": h.stockShards})
}           // 补充库存
func (h *Handler) GetStock(c *gin.Context) {
	activityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		return
	}

	total, err := h.core.GetStock(c.Request.Context(), activityID)
	if err != nil {
		switch err {
		case service.ErrInvalidActivity:
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		case service.ErrActivityNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "activity not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis read failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "activity_id": activityID, "stock": total, "shards": h.stockShards})
}            // 查询库存

// 其他管理
func (h *Handler) StopActivity(c *gin.Context) {
	activityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		return
	}

	err = h.core.UpdateActivityStatus(c.Request.Context(), activityID, 2)
	if err != nil {
		switch err {
		case service.ErrInvalidActivity:
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		case service.ErrActivityNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "activity not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis write failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "activity_id": activityID, "status": 2})
}        // 停止活动
func (h *Handler) StartActivity(c *gin.Context) {
	activityID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		return
	}

	err = h.core.UpdateActivityStatus(c.Request.Context(), activityID, 2)
	if err != nil {
		switch err {
		case service.ErrInvalidActivity:
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "bad activity id"})
		case service.ErrActivityNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "activity not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "redis write failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "activity_id": activityID, "status": 2})
}       // 开始活动


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

	if addr, ok := firstInstanceAddr(instances); ok {
		h.client = service.NewLayerAdminClient(addr)
		h.clientaddr = addr
	}

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


func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK,gin.H{"status":"ok"})
}