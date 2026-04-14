package service

import (
	"errors"
	"log"
	"context"
	"time"
	"fmt"
	"database/sql"
	"hash/fnv"

	"seckill/internal/common/model"
	"seckill/internal/common/redisx"

	"github.com/redis/go-redis/v9"
	_ "github.com/go-sql-driver/mysql"
)

var ErrMySQLRedisNotReady = errors.New("mysql-redis store is not ready")

const orderShardCount = 2

type MySQLRedisStore struct {
	mysqlDSN string
	redisAddr string

	redis 	redis.UniversalClient
	db 		*sql.DB
}

func NewMySQLRedisStore(mysqlDSN, redisAddr string) (*MySQLRedisStore,error) {

	if mysqlDSN =="" || redisAddr =="" {
		return nil,ErrMySQLRedisNotReady
	}

	rdb := redisx.NewClient(redisAddr, 0)

	ctx,cancel := context.WithTimeout(context.Background(),2*time.Second)
	defer cancel()
	if err :=rdb.Ping(ctx).Err();err!=nil {
		return nil,fmt.Errorf("redis ping failed: %w",err)
	}

	db,err :=sql.Open("mysql",mysqlDSN)
	if err !=nil {
		return nil,fmt.Errorf("mysql open failed: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 *time.Minute)

	mctx,mcancel:= context.WithTimeout(context.Background(),2*time.Second)
	defer mcancel()
	if err :=db.PingContext(mctx);err !=nil {
		_=db.Close()
		return nil,fmt.Errorf("mysql ping failed: %w",err)
	}

	if err :=ensureOrderSchema(context.Background(),db);err!=nil {
		_=db.Close()
		return nil,fmt.Errorf("ensure mysql schema failed: %w", err)
	}

	s:=&MySQLRedisStore{
		mysqlDSN: mysqlDSN,
		redisAddr:redisAddr,
		redis:rdb,
		db:db,
	}

    log.Printf("mysql-redis store enabled: redis=%s", redisAddr)
    return s, nil
}

func ensureOrderSchema(ctx context.Context,db *sql.DB) error {
	tctx,cancel := context.WithTimeout(ctx,3*time.Second)
	defer cancel()

	for i := 0; i < orderShardCount; i++ {
		table := orderTableByIndex(i)
		ddl := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  activity_id BIGINT NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_activity_user (activity_id,user_id),
  KEY idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=UTF8MB4;
`, table)

		if _,err := db.ExecContext(tctx,ddl); err != nil {
			return err
		}
	}

	return nil
}

func (s *MySQLRedisStore) TryReserve(activityID int64, userID string) (bool, string) {
	keys := []string{s.activeShardsKey(activityID), s.boughtKey(activityID,userID)}
	res,err := s.redis.Eval(context.Background(),acquireScript,keys,600,activityID).Int64()
	if err !=nil {
		log.Printf("redis eval failed: %v", err)
		return false,ErrSystemBusy
	}

	s.redis.XAdd(context.Background(),&redis.XAddArgs{
		Stream:"seckill-stream",
		Values:map[string]interface{}{
			"user_id":userID,
			"activity_id":activityID,
			"timestamp":time.Now().Unix(),
		},
	}).Result()

	switch res {
	case 0:
		return false,ErrSoldOut
	case 1:
		return false,ErrDuplicateOrder
	case 2:
		// continue
	default:
		return false,ErrSystemBusy
	}
	return true,"success"
}

func (s *MySQLRedisStore) RollbackReserve(activityID int64, userID string) {
	_ =activityID
	_=s.redis.Del(context.Background(),s.boughtKey(activityID,userID)).Err()
}

func (s *MySQLRedisStore) SaveOrder(req model.SeckillRequest) error {
	ctx,cancel :=context.WithTimeout(context.Background(),2*time.Second)
	defer cancel()

	table := orderTableByUser(req.UserID)
	
	_,err:=s.db.ExecContext(
		ctx,
		fmt.Sprintf("INSERT INTO %s(activity_id, user_id) VALUES (?,?)", table),
		req.ActivityID,
		req.UserID,
	)
	if err !=nil {
		log.Printf("mysql save order failed: user=%s activity=%d err=%v", req.UserID, req.ActivityID, err)
		return err
	}

	return nil
}

func (s *MySQLRedisStore) ListOrdersByUser(userID string) ([]model.SeckillRequest,error) {
	ctx,cancel := context.WithTimeout(context.Background(),2*time.Second)
	defer cancel()

	table := orderTableByUser(userID)

	rows,err :=s.db.QueryContext(
		ctx,
		fmt.Sprintf("SELECT activity_id,user_id FROM %s WHERE user_id=? ORDER BY id DESC LIMIT 200", table),
		userID,
	)
	if err !=nil {
		return nil,err
	}
	defer rows.Close()

	out :=make([]model.SeckillRequest,0,8)
	for rows.Next() {
		var req model.SeckillRequest
		if err :=rows.Scan(&req.ActivityID,&req.UserID);err !=nil{
			continue
		}
		out =append(out,req)
	}
	return out,nil
}

func (s *MySQLRedisStore) boughtKey(activityID int64,userID string) string {
	return fmt.Sprintf("seckill:{%d}:bought:%s",activityID,userID)
}

func (s *MySQLRedisStore) activeShardsKey(activityID int64) string {
	return fmt.Sprintf("seckill:{%d}:shards:active", activityID)
}

func orderTableByUser(userID string) string {
	idx := userServerIndex(userID)
	return orderTableByIndex(idx)
}

func orderTableByIndex(idx int) string {
	return fmt.Sprintf("orders_%d", idx)
}

func userServerIndex(userID string) int {
	if orderShardCount <= 1 {
		return 0
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(userID))
	return int(h.Sum32() % uint32(orderShardCount))
}

const acquireScript = `
if redis.call('EXISTS', KEYS[2]) == 1 then
	return 1
end

while true do
	local shard_id = redis.call('SRANDMEMBER', KEYS[1])
	if (not shard_id) then
		return 0
	end

	local stock_key = 'seckill:{' .. tostring(ARGV[2]) .. '}:stock:' .. tostring(shard_id)
	local stock = tonumber(redis.call('GET', stock_key) or '0')
	if stock <= 0 then
		redis.call('SREM', KEYS[1], shard_id)
	else
		local left = redis.call('DECR', stock_key)
		if tonumber(left) == 0 then
			redis.call('SREM', KEYS[1], shard_id)
		end
		redis.call('SET', KEYS[2], '1', 'EX', tonumber(ARGV[1]))
		return 2
	end
end
`