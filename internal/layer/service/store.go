package service

import "seckill/internal/common/model"

type Store interface{
	InitActivity(activityID,stock int64)
	GetStock(activityID int64) int64
	TryReserve(activityID int64,userID string) (ok bool,msg string)
	RollbackReserve(activityID int64,userID string)
	SaveOrder(req model.SeckillRequest)
}