package service

import (
	"context"
	"log"
	"sync"

	"seckill/internal/common/model"
	"seckill/internal/layer/queue"
)

type Core struct {
	mu 		sync.Mutex
	stock 	map[int64]int64
	orders 	[]model.SeckillRequest
	pool 	*queue.WorkerPool
}

func NewCore(ctx context.Context) *Core {
	c:= &Core {
		stock:map[int64]int64{
			1001:10,
		},
		orders:make([]model.SeckillRequest,0,128),
	}

	c.pool =queue.NewWorkerPool(1024,4,func(req model.SeckillRequest){
		c.mu.Lock()
		defer c.mu.Unlock()
		c.orders =append(c.orders,req)
		log.Printf("persist order user=%s activity=%d", req.UserID, req.ActivityID)
	})
	c.pool.Start(ctx)

	return c
}

func (c *Core) InitActivity(activityID,stock int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stock[activityID] =stock
}

func (c *Core) GetStock(activityID int64) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stock[activityID]
}

func (c *Core) TrySeckill(req model.SeckillRequest)(bool,string) {
	c.mu.Lock()
	left :=c.stock[req.ActivityID]
	if left<= 0 {
		c.mu.Unlock()
		return false,"sold out"
	}
	c.stock[req.ActivityID] =left -1
	c.mu.Unlock()

	if ok := c.pool.Submit(req);!ok {
		c.mu.Lock()
		c.stock[req.ActivityID]++
		c.mu.Unlock()
		return false,"system busy"
	}
	return true,"success"
}