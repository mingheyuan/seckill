package service

import "seckill/internal/common/model"

type Store interface {
	TryReserve(activityID int64, userID string) (ok bool, msg string)
	RollbackReserve(activityID int64, userID string)
	SaveOrder(req model.SeckillRequest) error

	ListOrdersByUser(userID string) ([]model.SeckillRequest, error)
}
