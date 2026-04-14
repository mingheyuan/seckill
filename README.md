linux
curl --noproxy '*' -X POST "http://127.0.0.1:8080/api/seckill" \
-H "Content-Type: application/json" \
-d '{"user_id":"user_1001","activity_id":1}'

nginx 入口负载均衡（解决单 proxy 单点）
1. 准备两台 proxy（示例）
	- 192.168.32.131:8080
	- 192.168.32.132:8080

2. 启动 nginx LB（默认监听 18080）
PROXY_UPSTREAMS='192.168.32.131:8080,192.168.32.132:8080' \
NGINX_LISTEN_PORT=18080 \
bash ./scripts/start_nginx_lb.sh up

3. 检查状态
bash ./scripts/start_nginx_lb.sh status
curl --noproxy '*' -s http://127.0.0.1:18080/healthz

4. 压测或业务入口改为 nginx
	- BASE_URL=http://127.0.0.1:18080

5. 更新上游后热重载
PROXY_UPSTREAMS='192.168.32.131:8080,192.168.32.132:8080' \
NGINX_LISTEN_PORT=18080 \
bash ./scripts/start_nginx_lb.sh reload

6. 停止 nginx LB
bash ./scripts/start_nginx_lb.sh down

win
curl --noproxy "*" -X POST "http://192.168.32.131:8080/api/seckill" -H "Content-Type: application/json" -d "{\"user_id\":\"user_1001\",\"activity_id\":1}"

win
curl --noproxy "*" -X POST "http://127.0.0.1:8080/api/seckill" -H "Content-Type: application/json" -d "{\"user_id\":\"user_1001\",\"activity_id\":1}"