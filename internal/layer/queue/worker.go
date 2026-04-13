package queue

import (
	"context"
	"log"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"seckill/internal/common/redisx"
	"seckill/internal/common/model"
)

type OrderQueue interface {
	Start(ctx context.Context)
	// Submit(req model.SeckillRequest) bool
}

// type Job struct {
// 	Order model.SeckillRequest
// }

type PersistFunc func(req model.SeckillRequest) error

type WorkerPool struct {
	// jobs    chan Job
	rdb redis.UniversalClient
	workers int
	persist PersistFunc
}

func NewWorkerPool(redisAddr string, workers int, persist PersistFunc) (*WorkerPool, error) {
	rdb := redisx.NewClient(redisAddr, workers+5)

	err := rdb.XGroupCreateMkStream(context.Background(), "seckill-stream", "group1", "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return nil, err
	}

	return &WorkerPool{
		// jobs:    make(chan Job, size),
		rdb:     rdb,
		workers: workers,
		persist: persist,
	}, nil
}

func (w *WorkerPool) Start(ctx context.Context) {
	var wg sync.WaitGroup

	go w.solvePending(ctx)
	
	for i := 0; i < w.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("worker panic: %v\n%s", r, string(debug.Stack()))
							}
						}()

						msgs, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
							Group:    "group1",
							Consumer: strconv.Itoa(workerID),
							Streams:  []string{"seckill-stream", ">"},
							Count:    1,
							Block:    2 * time.Second,
						}).Result()

						if err != nil {
							if err == redis.Nil {
								return
							}
							log.Printf("读取失败: %v", err)
							time.Sleep(time.Second)
							return
						}

						for _, msg := range msgs {
							for _, m := range msg.Messages {
								req, ok := parseRequest(m.Values)
								if !ok {
									log.Printf("invalid message: %v", m.Values)
									continue
								}

								if err := w.persist(req); err != nil {
									log.Printf("worker persist failed: user=%s activity=%d err=%v", req.UserID, req.ActivityID, err)
									continue
								}

								if _, err := w.rdb.XAck(ctx, "seckill-stream", "group1", m.ID).Result(); err != nil {
									log.Printf("ack failed: id=%s err=%v", m.ID, err)
								}
							}
						}
					}()
				}
			}
		}(i)
	}

	go func() {
		<-ctx.Done()
		log.Println("worker pool stopping")
		wg.Wait()
		log.Println("all workers stopped")
	}()
}

func (w *WorkerPool) solvePending(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			pending, err := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
				Stream: "seckill-stream",
				Group:  "group1",
				Start:  "-",
				End:    "+",
				Count:  100,
			}).Result()
			if err != nil {
				log.Printf("query pending failed: %v", err)
				time.Sleep(60 * time.Second)
				continue
			}
			
			var deadIDs []string
			for _, p := range pending {
				// 空闲超过 60 秒的消息，认为是"死信"
				if p.Idle > 60*time.Second {
					deadIDs = append(deadIDs, p.ID)
				}
			}

			if len(deadIDs) > 0 {
				claimed, err := w.rdb.XClaim(ctx, &redis.XClaimArgs{
					Stream:   "seckill-stream",
					Group:    "group1",
					Consumer: "recovery-consumer",
					MinIdle:  60 * time.Second,
					Messages: deadIDs,
				}).Result()
				if err != nil {
					log.Printf("claim pending failed: %v", err)
					time.Sleep(60 * time.Second)
					continue
				}

				for _, m := range claimed {
					req, ok := parseRequest(m.Values)
					if !ok {
						log.Printf("invalid message: %v", m.Values)
						continue
					}

					if err := w.persist(req); err != nil {
						log.Printf("worker persist failed: user=%s activity=%d err=%v", req.UserID, req.ActivityID, err)
						continue
					}

					if _, err := w.rdb.XAck(ctx, "seckill-stream", "group1", m.ID).Result(); err != nil {
						log.Printf("ack failed: id=%s err=%v", m.ID, err)
					}
				}
			}
			time.Sleep(60 * time.Second)
		}
	}
}

func parseRequest(values map[string]any) (model.SeckillRequest, bool) {
	userID, ok := values["user_id"].(string)
	if !ok || userID == "" {
		return model.SeckillRequest{}, false
	}

	activityRaw, ok := values["activity_id"]
	if !ok {
		return model.SeckillRequest{}, false
	}

	var activityID int64
	switch v := activityRaw.(type) {
	case int64:
		activityID = v
	case int:
		activityID = int64(v)
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return model.SeckillRequest{}, false
		}
		activityID = parsed
	default:
		return model.SeckillRequest{}, false
	}

	return model.SeckillRequest{
		UserID:     userID,
		ActivityID: activityID,
	}, true
}

// func (w *WorkerPool) Submit(req model.SeckillRequest) bool {
// 	select {
// 	case w.jobs <- Job{Order: req}:
// 		return true
// 	default:
// 		return false
// 	}
// }
