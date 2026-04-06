package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"seckill/internal/common/model"
)

type RedisStreamOptions struct {
	Addr           string
	Stream         string
	Group          string
	ConsumerPrefix string
	Workers        int
	BlockTimeout   time.Duration
}

type RedisStreamQueue struct {
	client         *redis.Client
	stream         string
	group          string
	consumerPrefix string
	workers        int
	blockTimeout   time.Duration
	persist        PersistFunc
}

func NewRedisStreamQueue(opts RedisStreamOptions, persist PersistFunc) (*RedisStreamQueue, error) {
	if opts.Addr == "" {
		return nil, errors.New("redis addr is empty")
	}
	if opts.Stream == "" {
		opts.Stream = "seckill:orders:stream"
	}
	if opts.Group == "" {
		opts.Group = "seckill-layer"
	}
	if opts.ConsumerPrefix == "" {
		opts.ConsumerPrefix = "layer"
	}
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	if opts.BlockTimeout <= 0 {
		opts.BlockTimeout = time.Second
	}

	client := redis.NewClient(&redis.Options{Addr: opts.Addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisStreamQueue{
		client:         client,
		stream:         opts.Stream,
		group:          opts.Group,
		consumerPrefix: opts.ConsumerPrefix,
		workers:        opts.Workers,
		blockTimeout:   opts.BlockTimeout,
		persist:        persist,
	}, nil
}

func (q *RedisStreamQueue) Start(ctx context.Context) {
	if err := q.ensureGroup(ctx); err != nil {
		log.Printf("redis stream ensure group failed: %v", err)
		return
	}

	for i := 0; i < q.workers; i++ {
		consumer := fmt.Sprintf("%s-%d-%d", q.consumerPrefix, i, time.Now().UnixNano())
		go q.consumeLoop(ctx, consumer)
	}
}

func (q *RedisStreamQueue) Submit(req model.SeckillRequest) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	_, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{
			"user_id":     req.UserID,
			"activity_id": strconv.FormatInt(req.ActivityID, 10),
			"created_at":  time.Now().UnixMilli(),
		},
		MaxLen: 100000,
		Approx: true,
	}).Result()
	if err != nil {
		log.Printf("redis stream enqueue failed: user=%s activity=%d err=%v", req.UserID, req.ActivityID, err)
		return false
	}
	return true
}

func (q *RedisStreamQueue) ensureGroup(ctx context.Context) error {
	err := q.client.XGroupCreateMkStream(ctx, q.stream, q.group, "0").Err()
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

func (q *RedisStreamQueue) consumeLoop(ctx context.Context, consumer string) {
	for {
		if ctx.Err() != nil {
			return
		}

		streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    q.group,
			Consumer: consumer,
			Streams:  []string{q.stream, ">"},
			Count:    10,
			Block:    q.blockTimeout,
			NoAck:    false,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || ctx.Err() != nil {
				continue
			}
			log.Printf("redis stream consume failed: consumer=%s err=%v", consumer, err)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				req, ok := decodeRequest(msg.Values)
				if !ok {
					_ = q.client.XAck(ctx, q.stream, q.group, msg.ID).Err()
					continue
				}

				persistOK := true
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("redis stream worker panic: %v", r)
							persistOK = false
						}
					}()
					if err := q.persist(req); err != nil {
						persistOK = false
						log.Printf("redis stream persist failed: consumer=%s id=%s user=%s activity=%d err=%v", consumer, msg.ID, req.UserID, req.ActivityID, err)
					}
				}()

				if !persistOK {
					continue
				}
				if err := q.client.XAck(ctx, q.stream, q.group, msg.ID).Err(); err != nil {
					log.Printf("redis stream ack failed: consumer=%s id=%s err=%v", consumer, msg.ID, err)
				}
			}
		}
	}
}

func decodeRequest(values map[string]any) (model.SeckillRequest, bool) {
	uidRaw, ok := values["user_id"]
	if !ok {
		return model.SeckillRequest{}, false
	}
	actRaw, ok := values["activity_id"]
	if !ok {
		return model.SeckillRequest{}, false
	}

	uid := fmt.Sprint(uidRaw)
	activityID, err := strconv.ParseInt(fmt.Sprint(actRaw), 10, 64)
	if err != nil || uid == "" {
		return model.SeckillRequest{}, false
	}

	return model.SeckillRequest{UserID: uid, ActivityID: activityID}, true
}
