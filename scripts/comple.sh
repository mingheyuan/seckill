go run ./cmd/layer

go run ./cmd/proxy

go run ./cmd/admin

初始化活动库存：
curl -X POST http://127.0.0.1:8082/admin/activity/init \
  -H "Content-Type: application/json" \
  -d '{"activity_id":1001,"stock":5}'

查询库存（查 layer
curl "http://127.0.0.1:8081/internal/stock?activity_id=1001"

走真实链路秒杀（打 proxy
curl -X POST http://127.0.0.1:8080/api/seckill \
  -H "Content-Type: application/json" \
  -d '{"user_id":"u1","activity_id":1001}'
