package service

import (
	"errors"
	"os"
	"strings"
	"log"
	"strconv"
	"context"
	"time"
	"fmt"
	"database/sql"

	"seckill/internal/common/model"

	"github.com/redis/go-redis/v9"
	_ "github.com/go-sql-driver/mysql"
)

var ErrMySQLRedisNotReady = errors.New("mysql-redis store is not ready")

type MySQLRedisStore struct {
	mysqlDSN string
	redisAddr string

	redis 	*redis.Client
	db 		*sql.DB
	ctx 	context.Context
    fallback *MemoryStore
}

func NewMySQLRedisStoreFromEnv() (*MySQLRedisStore,error) {
	mysqlDSN := strings.TrimSpace(os.Getenv("LAYER_MYSQL_DSN"))
	redisAddr := strings.TrimSpace(os.Getenv("LAYER_REDIS_ADDR"))

	if mysqlDSN =="" || redisAddr =="" {
		return nil,ErrMySQLRedisNotReady
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:redisAddr,
		DB:0,
	})

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
		ctx:context.Background(),
		fallback:  NewMemoryStore(),
	}

    log.Printf("mysql-redis store enabled: redis=%s", redisAddr)
    return s, nil
}

func ensureOrderSchema(ctx context.Context,db *sql.DB) error {
	ddl:= `
CREATE TABLE IF NOT EXISTS orders (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  activity_id BIGINT NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_activity_user (activity_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=UTF8MB4;
`
	tctx,cancel := context.WithTimeout(ctx,3*time.Second)
	defer cancel()

	_,err :=db.ExecContext(tctx,ddl)
	return err
}

func (s *MySQLRedisStore) InitActivity(activityID, stock int64) {
	if err :=s.redis.Set(s.ctx,s.stockKey(activityID),stock,0).Err();err !=nil {
		log.Printf("redis init stock failed, fallback memory: activity=%d err=%v", activityID, err)
		s.fallback.InitActivity(activityID,stock)
		return
	}


    s.fallback.InitActivity(activityID, stock)
}

func (s *MySQLRedisStore) GetStock(activityID int64) int64 {
	v,err := s.redis.Get(s.ctx,s.stockKey(activityID)).Result()
	if err !=nil {
		return s.fallback.GetStock(activityID)
	}

	n,convErr :=strconv.ParseInt(v,10,64)
	if convErr !=nil {
		return	s.fallback.GetStock(activityID)
	}
    return n
}

func (s *MySQLRedisStore) TryReserve(activityID int64, userID string) (bool, string) {
	keys := []string{s.stockKey(activityID), s.boughtKey(activityID,userID)}
	res,err := s.redis.Eval(s.ctx,acquireScript,keys,600).Int64()
	if err !=nil {
		log.Printf("redis eval failed, fallback memory: %v", err)
		return s.fallback.TryReserve(activityID, userID)
	}

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

	_,_=s.fallback.TryReserve(activityID,userID)
	return true,"success"
}

func (s *MySQLRedisStore) RollbackReserve(activityID int64, userID string) {
	_ =s.redis.Incr(s.ctx,s.stockKey(activityID)).Err()
	_=s.redis.Del(s.ctx,s.boughtKey(activityID,userID)).Err()

    s.fallback.RollbackReserve(activityID, userID)
}

func (s *MySQLRedisStore) SaveOrder(req model.SeckillRequest) {
    if s.db==nil {
		s.fallback.SaveOrder(req)
		return
	}

	ctx,cancel :=context.WithTimeout(s.ctx,2*time.Second)
	defer cancel()
	
	_,err:=s.db.ExecContext(
		ctx,
		`INSERT INTO orders(activity_id, user_id) VALUES (?,?)`,
		req.ActivityID,
		req.UserID,
	)
	if err !=nil {
        log.Printf("mysql save order failed, fallback memory: user=%s activity=%d err=%v", req.UserID, req.ActivityID, err)
        s.fallback.SaveOrder(req)
        return
	}

}

func (s *MySQLRedisStore) ListOrdersByUser(userID string) ([]model.SeckillRequest,error) {
	if s.db == nil {
		return s.fallback.ListOrdersByUser(userID)
	}

	ctx,cancel := context.WithTimeout(s.ctx,2*time.Second)
	defer cancel()

	rows,err :=s.db.QueryContext(
		ctx,
		`SELECT activity_id,user_id FROM orders WHERE user_id=? ORDER BY id DESC LIMIT 200`,
		userID,
	)
	if err !=nil {
		return s.fallback.ListOrdersByUser(userID)
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

func (s *MySQLRedisStore) stockKey(activityID int64) string {
	return fmt.Sprintf("seckill:stock:%d",activityID)
}

func (s *MySQLRedisStore) boughtKey(activityID int64,userID string) string {
	return fmt.Sprintf("seckill:bought:%d:%s",activityID,userID)
}

const acquireScript = `
if redis.call('EXISTS', KEYS[2]) == 1 then
	return 1
end

local stock = redis.call('GET', KEYS[1])
if (not stock) then
	return 0
end

stock = tonumber(stock)
if stock <= 0 then
	return 0
end

redis.call('DECR', KEYS[1])
redis.call('SET', KEYS[2], '1', 'EX', tonumber(ARGV[1]))
return 2
`