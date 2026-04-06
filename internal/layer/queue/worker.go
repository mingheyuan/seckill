package queue

import (
	"context"
	"log"
	"runtime/debug"
	"sync"

	"seckill/internal/common/model"
)

type OrderQueue interface {
	Start(ctx context.Context)
	Submit(req model.SeckillRequest) bool
}

type Job struct {
	Order model.SeckillRequest
}

type PersistFunc func(req model.SeckillRequest) error

type WorkerPool struct {
	jobs    chan Job
	workers int
	persist PersistFunc
}

func NewWorkerPool(size, workers int, persist PersistFunc) *WorkerPool {
	return &WorkerPool{
		jobs:    make(chan Job, size),
		workers: workers,
		persist: persist,
	}
}

func (w *WorkerPool) Start(ctx context.Context) {
	var wg sync.WaitGroup

	for i := 0; i < w.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-w.jobs:
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("worker panic: %v\n%s", r, string(debug.Stack()))
							}
						}()
						if err := w.persist(job.Order); err != nil {
							log.Printf("worker persist failed: user=%s activity=%d err=%v", job.Order.UserID, job.Order.ActivityID, err)
						}
					}()
				}
			}
		}()
	}

	go func() {
		<-ctx.Done()
		log.Println("worker pool stopping")
		wg.Wait()
		log.Println("all workers stopped")
	}()
}

func (w *WorkerPool) Submit(req model.SeckillRequest) bool {
	select {
	case w.jobs <- Job{Order: req}:
		return true
	default:
		return false
	}
}
