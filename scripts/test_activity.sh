#!/usr/bin/env bash
set -euo pipefail

#关闭活动
curl --noproxy '*' -s -X POST http://127.0.0.1:8082/admin/activity \
-H 'Content-Type: application/json' \
-d '{"enabled":false,"start_at_unix":0,"end_at_unix":4102444800,"user_product_limit":1}'
echo

#尝试秒杀（预期失败，活动未开启）
curl --noproxy '*' -s -X POST http://127.0.0.1:8080/api/seckill \
-H 'Content-Type: application/json' \
-d '{"user_id":"phase6_user_1","activity_id":1001}'
echo

#重新开启活动
NOW=$(date +%s)
END=$((NOW+3600))
curl --noproxy '*' -s -X POST http://127.0.0.1:8082/admin/activity \
-H 'Content-Type: application/json' \
-d "{\"enabled\":true,\"start_at_unix\":$((NOW-10)),\"end_at_unix\":$END,\"user_product_limit\":1}"
echo

#再次秒杀（预期成功）
curl --noproxy '*' -s -X POST http://127.0.0.1:8080/api/seckill \
-H 'Content-Type: application/json' \
-d '{"user_id":"phase6_user_2","activity_id":1001}'
echo