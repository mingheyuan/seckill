package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"seckill/internal/common/redisx"
	"seckill/internal/common/model"
)

var (
	ErrInvalidActivity = errors.New("invalid activity")
	ErrActivityExists = errors.New("activity already exists")
	ErrActivityNotFound = errors.New("activity not found")
)

type Core interface {
	CreateActivity(ctx context.Context, activityID, stock int64, cfg model.ActivityConfig) error
	GetActivityConfig(ctx context.Context, activityID int64) (model.ActivityConfig, error)
	UpdateActivityConfig(ctx context.Context, activityID int64, cfg model.ActivityConfig) error
	UpdateActivityStatus(ctx context.Context, activityID int64, status int64) error
	DeleteActivity(ctx context.Context, activityID int64) error
	InitStock(ctx context.Context, activityID, stock int64) error
	IncrStock(ctx context.Context, activityID, delta int64) error
	GetStock(ctx context.Context, activityID int64) (int64, error)
	ListActivities(ctx context.Context) ([]ActivityListItem, error)
}

type ActivityListItem struct {
	ActivityID int64 `json:"activity_id"`
	CreatedAt  int64 `json:"created_at"`
}

type core struct {
	redisClient redis.UniversalClient
	stockShards int
}

func NewCore(redisAddr string, stockShards int) Core {
	if stockShards <= 0 {
		stockShards = 30
	}
	return &core{
		redisClient: redisx.NewClient(redisAddr, 0),
		stockShards: stockShards,
	}
}

func (c *core) CreateActivity(ctx context.Context, activityID, stock int64, cfg model.ActivityConfig) error {
	if activityID <= 0 || stock <= 0 {
		return ErrInvalidActivity
	}

	exists, err := c.activityExists(ctx, activityID)
	if err != nil {
		return err
	}
	if exists {
		return ErrActivityExists
	}
	return c.setShardedStockAndConfig(ctx, activityID, stock, cfg, time.Now().Unix())
}

func (c *core) GetActivityConfig(ctx context.Context, activityID int64) (model.ActivityConfig, error) {
	var cfg model.ActivityConfig
	if activityID <= 0 {
		return cfg, ErrInvalidActivity
	}

	v, err := c.redisClient.HGetAll(ctx, activityConfigKey(activityID)).Result()
	if err != nil {
		return cfg, err
	}
	if len(v) == 0 {
		return cfg, ErrActivityNotFound
	}

	enabled, err := strconv.ParseBool(v["enabled"])
	if err != nil {
		return cfg, err
	}
	startAt, err := strconv.ParseInt(v["start_at_unix"], 10, 64)
	if err != nil {
		return cfg, err
	}
	endAt, err := strconv.ParseInt(v["end_at_unix"], 10, 64)
	if err != nil {
		return cfg, err
	}
	limit, err := strconv.Atoi(v["user_product_limit"])
	if err != nil {
		return cfg, err
	}

	cfg.Enabled = enabled
	cfg.StartAtUnix = startAt
	cfg.EndAtUnix = endAt
	cfg.UserProductLimit = limit
	return cfg, nil
}

func (c *core) UpdateActivityConfig(ctx context.Context, activityID int64, cfg model.ActivityConfig) error {
	if activityID <= 0 {
		return ErrInvalidActivity
	}

	exists, err := c.redisClient.Exists(ctx, activityConfigKey(activityID)).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrActivityNotFound
	}

	_, err = c.redisClient.HSet(ctx, activityConfigKey(activityID), map[string]interface{}{
		"enabled":            cfg.Enabled,
		"start_at_unix":      cfg.StartAtUnix,
		"end_at_unix":        cfg.EndAtUnix,
		"user_product_limit": cfg.UserProductLimit,
	}).Result()
	return err
}

func (c *core) UpdateActivityStatus(ctx context.Context, activityID int64, status int64) error {
	if activityID <= 0 {
		return ErrInvalidActivity
	}

	exists, err := c.redisClient.Exists(ctx, activityConfigKey(activityID)).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrActivityNotFound
	}

	return c.redisClient.Set(ctx, activityStatusKey(activityID), strconv.FormatInt(status, 10), 0).Err()
}

func (c *core) DeleteActivity(ctx context.Context, activityID int64) error {
	if activityID <= 0 {
		return ErrInvalidActivity
	}

	exists, err := c.redisClient.Exists(ctx, activityConfigKey(activityID)).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrActivityNotFound
	}

	pipe := c.redisClient.Pipeline()
	pipe.Del(ctx, activityConfigKey(activityID))
	pipe.Del(ctx, activityStatusKey(activityID))
	pipe.ZRem(ctx, activityListKey(), strconv.FormatInt(activityID, 10))
	_, err = pipe.Exec(ctx)
	return err
}

func (c *core) InitStock(ctx context.Context, activityID, stock int64) error {
	if activityID <= 0 || stock <= 0 {
		return ErrInvalidActivity
	}

	keys := make([]string, 0, c.stockShards+1)
	for i := 0; i < c.stockShards; i++ {
		keys = append(keys, stockShardKey(activityID, i))
	}
	keys = append(keys, activeShardsKey(activityID))

	res, err := c.redisClient.Eval(ctx, initStockLuaScript, keys, stock).Int64()
	if err != nil {
		return err
	}
	if res == -1 {
		return ErrActivityNotFound
	}
	return nil
}

func (c *core) IncrStock(ctx context.Context, activityID, delta int64) error {
	if activityID <= 0 || delta <= 0 {
		return ErrInvalidActivity
	}

	keys := make([]string, 0, c.stockShards+1)
	for i := 0; i < c.stockShards; i++ {
		keys = append(keys, stockShardKey(activityID, i))
	}
	keys = append(keys, activeShardsKey(activityID))

	res, err := c.redisClient.Eval(ctx, incrStockLuaScript, keys, delta).Int64()
	if err != nil {
		return err
	}
	if res == -1 {
		return ErrActivityNotFound
	}
	return nil
}

func (c *core) GetStock(ctx context.Context, activityID int64) (int64, error) {
	if activityID <= 0 {
		return 0, ErrInvalidActivity
	}

	exists, err := c.activityExists(ctx, activityID)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, ErrActivityNotFound
	}

	keys := make([]string, 0, c.stockShards)
	pattern := fmt.Sprintf("seckill:{%d}:stock:*", activityID)
	it := c.redisClient.Scan(ctx, 0, pattern, 100).Iterator()
	for it.Next(ctx) {
		keys = append(keys, it.Val())
	}
	if err := it.Err(); err != nil {
		return 0, err
	}

	if len(keys) == 0 {
		return 0, nil
	}

	vals, err := c.redisClient.MGet(ctx, keys...).Result()
	if err != nil {
		return 0, err
	}

	var total int64
	for _, raw := range vals {
		if raw == nil {
			continue
		}
		s, ok := raw.(string)
		if !ok {
			continue
		}
		n, convErr := strconv.ParseInt(s, 10, 64)
		if convErr != nil {
			continue
		}
		total += n
	}
	return total, nil
}

func (c *core) ListActivities(ctx context.Context) ([]ActivityListItem, error) {
	rows, err := c.redisClient.ZRangeWithScores(ctx, activityListKey(), 0, -1).Result()
	if err != nil {
		return nil, err
	}

	out := make([]ActivityListItem, 0, len(rows))
	for _, row := range rows {
		var activityID int64
		switch v := row.Member.(type) {
		case string:
			n, convErr := strconv.ParseInt(v, 10, 64)
			if convErr != nil {
				continue
			}
			activityID = n
		case int64:
			activityID = v
		case int:
			activityID = int64(v)
		default:
			continue
		}

		out = append(out, ActivityListItem{
			ActivityID: activityID,
			CreatedAt:  int64(row.Score),
		})
	}

	return out, nil
}

func (c *core) activityExists(ctx context.Context, activityID int64) (bool, error) {
	exists, err := c.redisClient.Exists(ctx, activityConfigKey(activityID)).Result()
	if err != nil {
		return false, err
	}
	if exists > 0 {
		return true, nil
	}

	pattern := fmt.Sprintf("seckill:{%d}:stock:*", activityID)
	it := c.redisClient.Scan(ctx, 0, pattern, 1).Iterator()
	if it.Next(ctx) {
		return true, nil
	}
	if err := it.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func (c *core) setShardedStockAndConfig(ctx context.Context, activityID, stock int64, cfg model.ActivityConfig, createdAt int64) error {
	parts := splitStock(stock, c.stockShards)

	pipe := c.redisClient.Pipeline()
	for i, v := range parts {
		pipe.Set(ctx, stockShardKey(activityID, i), strconv.FormatInt(v, 10), 0)
	}
	pipe.HSet(ctx, activityConfigKey(activityID), map[string]interface{}{
		"enabled":            cfg.Enabled,
		"start_at_unix":      cfg.StartAtUnix,
		"end_at_unix":        cfg.EndAtUnix,
		"user_product_limit": cfg.UserProductLimit,
	})
	pipe.Set(ctx, activityStatusKey(activityID), "0", 0)
	pipe.ZAdd(ctx, activityListKey(), redis.Z{
		Score:  float64(createdAt),
		Member: strconv.FormatInt(activityID, 10),
	})

	_, err := pipe.Exec(ctx)
	return err
}

func (c *core) setShardedStock(ctx context.Context, activityID, stock int64) error {
	parts := splitStock(stock, c.stockShards)

	pipe := c.redisClient.Pipeline()
	for i, v := range parts {
		pipe.Set(ctx, stockShardKey(activityID, i), strconv.FormatInt(v, 10), 0)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func splitStock(total int64, shards int) []int64 {
	if shards <= 0 {
		shards = 1
	}
	out := make([]int64, shards)
	base := total / int64(shards)
	rem := total % int64(shards)
	for i := 0; i < shards; i++ {
		out[i] = base
		if int64(i) < rem {
			out[i]++
		}
	}
	return out
}

func stockShardKey(activityID int64, shardID int) string {
	return fmt.Sprintf("seckill:{%d}:stock:%d", activityID, shardID)
}

func activityConfigKey(activityID int64) string {
	return fmt.Sprintf("activity:config:%d", activityID)
}

func activityListKey() string {
	return "activitylist"
}

func activityStatusKey(activityID int64) string {
	return fmt.Sprintf("seckill:{%d}:activity:status", activityID)
}

func activeShardsKey(activityID int64) string {
	return fmt.Sprintf("seckill:{%d}:shards:active", activityID)
}

const initStockLuaScript = `
if redis.call('EXISTS', KEYS[1]) == 0 then
	return -1
end

local total = tonumber(ARGV[1])
local shards = #KEYS - 1
local base = math.floor(total / shards)
local rem = total % shards

redis.call('DEL', KEYS[#KEYS])
for i = 1, shards do
	local v = base
	if i <= rem then
		v = v + 1
	end
	redis.call('SET', KEYS[i], v)
	redis.call('SADD', KEYS[#KEYS], tostring(i - 1))
end

return 1
`

const incrStockLuaScript = `
if redis.call('EXISTS', KEYS[1]) == 0 then
	return -1
end

local total = tonumber(ARGV[1])
local shards = #KEYS - 1
local base = math.floor(total / shards)
local rem = total % shards

for i = 1, shards do
	local inc = base
	if i <= rem then
		inc = inc + 1
	end
	if inc > 0 then
		redis.call('INCRBY', KEYS[i], inc)
		redis.call('SADD', KEYS[#KEYS], tostring(i - 1))
	end
end

return 1
`
