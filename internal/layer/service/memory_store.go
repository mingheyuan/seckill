package service

import (
	"sync"

	"seckill/internal/common/model"
)

type MemoryStore struct {
	mu     sync.Mutex
	stock  map[int64]int64
	bought map[int64]map[string]bool
	orders []model.SeckillRequest
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		stock: map[int64]int64{
			1001: 10,
		},
		bought: make(map[int64]map[string]bool),
		orders: make([]model.SeckillRequest, 0, 128),
	}
}

func (s *MemoryStore) InitActivity(activityID, stock int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stock[activityID] = stock
	s.bought[activityID] = make(map[string]bool)
}

func (s *MemoryStore) GetStock(activityID int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stock[activityID]
}

func (s *MemoryStore) TryReserve(activityID int64, userID string) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.bought[activityID]; !ok {
		s.bought[activityID] = make(map[string]bool)
	}
	if s.bought[activityID][userID] {
		return false, ErrDuplicateOrder
	}

	left := s.stock[activityID]
	if left <= 0 {
		return false, ErrSoldOut
	}

	s.stock[activityID] = left - 1
	s.bought[activityID][userID] = true
	return true, "success"
}

func (s *MemoryStore) RollbackReserve(activityID int64, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stock[activityID]++
	delete(s.bought[activityID], userID)
}

func (s *MemoryStore) SaveOrder(req model.SeckillRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders = append(s.orders, req)
	return nil
}

func (s *MemoryStore) ListOrdersByUser(userID string) ([]model.SeckillRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]model.SeckillRequest, 0, 8)
	for i := range s.orders {
		if s.orders[i].UserID == userID {
			out = append(out, s.orders[i])
		}
	}
	return out, nil
}
