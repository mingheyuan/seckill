cd /home/yuan/test_sum_seckill_concurrence/seckill/scripts

# 1) 先重置库存（关键，不然会 sold out）
curl --noproxy '*' -s -X POST http://127.0.0.1:8082/admin/activity/init \
  -H 'Content-Type: application/json' \
  -d '{"activity_id":1001,"stock":20}'
echo

# 2) 下单（用新用户）
curl --noproxy '*' -s -X POST http://127.0.0.1:8080/api/seckill \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"phase5_persist_u1","activity_id":1001}'
echo

# 3) 查单（重启前）
curl --noproxy '*' -s 'http://127.0.0.1:8080/api/orders?user_id=phase5_persist_u1'
echo

# 4) 正确重启（你当前目录在 scripts）
LAYER_STORAGE_ENGINE=mysql-redis \
LAYER_MYSQL_DSN='root:123456@tcp(127.0.0.1:3306)/seckill?parseTime=true&loc=Local&charset=utf8mb4' \
LAYER_REDIS_ADDR='127.0.0.1:6379' \
bash ./start.sh restart

# 5) 查单（重启后）
curl --noproxy '*' -s 'http://127.0.0.1:8080/api/orders?user_id=phase5_persist_u1'
echo

# 6) 查引擎日志（比 tail+sed 更稳）
grep -E 'store selected|fallback|mysql-redis store enabled' ../logs/layer.log