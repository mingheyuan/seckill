package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"seckill/internal/common/model"
	proxysvc "seckill/internal/proxy/service"
	layersvc "seckill/internal/layer/service"
)

type Handler struct {
	layer *proxysvc.LayerClient
}

func NewHandler(layer *proxysvc.LayerClient) *Handler {
	return &Handler{layer:layer}
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