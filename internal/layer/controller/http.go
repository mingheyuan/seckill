package controller

import (
	"net/http"

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

	r.GET("/internal/admin/activity",h.GetActivity)
	r.GET("/internal/orders",h.OrdersByUser)
}

func (h *Handler) OrdersByUser(c *gin.Context) {
	userID :=c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest,gin.H{"code": 400, "message": "user_id required"})
		return
	}

	orders ,err:=h.core.ListOrdersByUser(userID)
	if err !=nil {
        c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "query failed"})
        return
	}

	c.JSON(http.StatusOK,gin.H{
		"code":0,
		"orders":orders,
	})
}

func (h *Handler) GetActivity(c *gin.Context) {
	c.JSON(http.StatusOK,h.core.GetActivity())
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
	if ok {
		c.JSON(http.StatusOK,model.InternalSeckillResponse{
			OK:true,
			Message:"success",
		})
		return
	}

	status:=http.StatusConflict
    switch msg {
    case service.ErrTooFrequent:
        status = http.StatusTooManyRequests
    case service.ErrSystemBusy:
        status = http.StatusTooManyRequests
    case service.ErrDuplicateOrder:
        status = http.StatusConflict
    case service.ErrSoldOut:
        status = http.StatusConflict
	case service.ErrActivityClosed:
		status =http.StatusConflict
    default:
        status = http.StatusInternalServerError
	}

	c.JSON(status,model.InternalSeckillResponse{
		OK:false,
		Message:msg,
	})
}