package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"seckill/internal/common/model"
	"seckill/internal/layer/service"
)

type Handler struct {
	core *service.Core
}

func NewHandler(core *service.Core) *Handler {
	return &Handler{core: core}
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/healthz",h.Healthz)
	r.POST("/internal/seckill",h.Seckill)
	r.GET("/internal/stock",h.Stock)
	r.POST("/internal/admin/init",h.Init)
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK,gin.H{"status":"ok"})
}

func (h *Handler) Seckill(c *gin.Context) {
	var req model.SeckillRequest
	if err :=c.ShouldBindJSON(&req);err !=nil{
		c.JSON(http.StatusBadRequest,model.InternalSeckillResponse{OK:false,Message:"bad request"})
		return
	}

	ok,msg:=h.core.TrySeckill(req)
	c.JSON(http.StatusBadRequest,model.InternalSeckillResponse{OK:ok,Message:msg})
}

func (h *Handler)Stock(c *gin.Context) {
	activityID,_ :=strconv.ParseInt(c.DefaultQuery("activity_id","1001"),10,64)
	c.JSON(http.StatusOK,gin.H{
		"activity_id":activityID,
		"stock":	h.core.GetStock(activityID),
	})
}

type initReq struct {
	ActivityID 	int64 `json:"activity_id" binding:"required"`
	Stock 		int64 `json:"stock" binding:"required"` 
}

func (h *Handler) Init(c *gin.Context) {
	var req initReq
	if err :=c.ShouldBindJSON(&req);err!=nil {
		c.JSON(http.StatusBadRequest,gin.H{"code":400,"message": "bad request"})
		return
	}
	h.core.InitActivity(req.ActivityID,req.Stock)
	c.JSON(http.StatusOK,gin.H{"code":0,"message":"ok"})
}