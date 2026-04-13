linux
curl --noproxy '*' -X POST "http://127.0.0.1:8080/api/seckill" \
-H "Content-Type: application/json" \
-d '{"user_id":"user_1001","activity_id":1}'

win
curl --noproxy "*" -X POST "http://192.168.32.131:8080/api/seckill" -H "Content-Type: application/json" -d "{\"user_id\":\"user_1001\",\"activity_id\":1}"

win
curl --noproxy "*" -X POST "http://127.0.0.1:8080/api/seckill" -H "Content-Type: application/json" -d "{\"user_id\":\"user_1001\",\"activity_id\":1}"