package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"seckill/internal/admin/service"
    "seckill/internal/common/model"
)

type Handler struct {
	client *service.LayerAdminClient
}

func NewHandler(client *service.LayerAdminClient) *Handler {
	return &Handler{client:client}
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