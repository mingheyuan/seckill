package service

import (
	"context"
	"log"
	"time"
	"sync"
	"os"

	"seckill/internal/common/model"
	"seckill/internal/layer/queue"
)

var (
	ErrDuplicateOrder = "duplicate order"
	ErrSoldOut 			="sold out"
	ErrSystemBusy 		="system busy"
	ErrTooFrequent 		="too many requests"
	ErrActivityClosed 	="activity not open"
)

type userBucket struct {
	sec 		int64
	count  		int
}



type Core struct {
	pool 	*queue.WorkerPool

	userLimit 	int
	userMu 	sync.Mutex
	userSec map[string]userBucket

	activity 	model.ActivityConfig
	actMu 		sync.RWMutex

	snapshotPath 	string
	dirty 			bool

	store 			Store
}

func NewCore(ctx context.Context) *Core {
	now :=time.Now().Unix()
	s,err:=NewStoreFromEnv()
	if err !=nil {
        log.Printf("init store failed, fallback to memory: %v", err)
		s =NewMemoryStore()
		log.Printf("store selected: %s (fallback)", StorageMemory)
	} else {
		log.Printf("store selected: %s", os.Getenv("LAYER_STORAGE_ENGINE"))
	}
	c:= &Core{
		store:s,
		userLimit:20,
		userSec:make(map[string]userBucket,1024),
		activity:model.ActivityConfig{
			Enabled: 	true,
			StartAtUnix:now -3600,
			EndAtUnix: 	now +86400,
			UserProductLimit:1,
		},
	}

	c.pool =queue.NewWorkerPool(1024,4,func(req model.SeckillRequest){
		c.store.SaveOrder(req)
		log.Printf("persist order user=%s activity=%d", req.UserID, req.ActivityID)
	})
	c.pool.Start(ctx)
	return c
}

func (c *Core) GetActivity() model.ActivityConfig {
	c.actMu.RLock()
	defer c.actMu.RUnlock()
	return c.activity
}

func (c *Core) UpdateActivity(cfg model.ActivityConfig) {
	c.actMu.Lock()
	defer c.actMu.Unlock()

	if cfg.EndAtUnix < cfg.StartAtUnix {
		// 错误说明: 非法时间窗直接拒绝，避免活动状态进入坏数据
		return
	}

	if cfg.UserProductLimit<=0 {
		cfg.UserProductLimit=1
	}
	c.activity=cfg
}

func (c *Core) isActivityOpen(nowUnix int64) bool {
	c.actMu.RLock()
	defer c.actMu.RUnlock()

	if !c.activity.Enabled {
		return false
	}

	if nowUnix< c.activity.StartAtUnix {
		return false
	}

	if nowUnix> c.activity.EndAtUnix {
		return false
	}
	return true
}

func (c *Core) InitActivity(activityID,stock int64) {
	c.store.InitActivity(activityID,stock)
}

func (c *Core) GetStock(activityID int64) int64 {
	return c.store.GetStock(activityID)
}

func (c *Core) TrySeckill(req model.SeckillRequest)(bool,string) {
	if !c.isActivityOpen(time.Now().Unix()) {
		return false,ErrActivityClosed
	}
	
	if !c.allowUser(req.UserID) {
		return false,ErrTooFrequent
	}
	
	ok,msg :=c.store.TryReserve(req.ActivityID,req.UserID)
	if !ok {
		return false,msg
	}

	if ok := c.pool.Submit(req);!ok {
		c.store.RollbackReserve(req.ActivityID,req.UserID)
		return false,ErrSystemBusy
	}
	return true,"success"
}

func (c *Core) allowUser(userID string) bool {
	if c.userLimit <= 0{
		return true
	}
	nowSec :=time.Now().Unix()

	c.userMu.Lock()
	defer c.userMu.Unlock()

	b:=c.userSec[userID]
	if b.sec !=nowSec{
		c.userSec[userID] =userBucket{sec:nowSec,count:1}
		return true
	}
	b.count++
	c.userSec[userID]=b
	return b.count<=c.userLimit
}

func (c *Core) ListOrdersByUser(userID string) ([]model.SeckillRequest,error) {
	return c.store.ListOrdersByUser(userID)
}