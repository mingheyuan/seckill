package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"seckill/internal/common/model"
	"seckill/internal/proxy/service"
)

type Handler struct {
	layer *service.LayerClient
}

func NewHandler(layer *service.LayerClient) *Handler {
	return &Handler{layer:layer}
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/healthz",h.Healthz)
	r.POST("/api/seckill",h.Seckill)
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

	ret,err := h.layer.Seckill(req)
	if err != nil {
		c.JSON(http.StatusBadGateway,model.SeckillResponse{Code:502,Message:"layer unavailable"})
		return
	}
	if !ret.OK{
		c.JSON(http.StatusOK,model.SeckillResponse{Code:1001,Message:ret.Message})
		return
	}
	c.JSON(http.StatusOK,model.SeckillResponse{Code:0,Message:"success"})
}