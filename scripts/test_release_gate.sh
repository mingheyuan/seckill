#!/usr/bin/env bash
set -euo pipefail

BASE_PROXY="http://127.0.0.1:8080"
BASE_ADMIN="http://127.0.0.1:8082"
BASE_LAYER="http://127.0.0.1:8081"
ACTIVITY_ID=1001
STOCK=100

# 1) 初始化库存
curl --noproxy '*' -s -X POST "${BASE_ADMIN}/admin/activity/init" \
  -H "Content-Type: application/json" \
  -d "{\"activity_id\":${ACTIVITY_ID},\"stock\":${STOCK}}"

# 2) 开启活动（确保窗口命中）
NOW=$(date +%s); END=$((NOW+3600))
curl --noproxy '*' -s -X POST "${BASE_ADMIN}/admin/activity" \
  -H "Content-Type: application/json" \
  -d "{\"enabled\":true,\"start_at_unix\":$((NOW-10)),\"end_at_unix\":${END},\"user_product_limit\":1}"

# 3) 并发请求
tmpfile="$(mktemp)"
for i in $(seq 1 300); do
  (
    curl --noproxy '*' -s -X POST "${BASE_PROXY}/api/seckill" \
      -H "Content-Type: application/json" \
      -d "{\"user_id\":\"gate_u${i}\",\"activity_id\":${ACTIVITY_ID}}"
    echo
  ) >> "${tmpfile}" &
done
wait

success=$(grep -c '"code":0' "${tmpfile}" || true)
fail=$(grep -vc '"code":0' "${tmpfile}" || true)

echo "success=${success} fail=${fail}"

# 4) 一致性检查
stock_after=$(curl --noproxy '*' -s "${BASE_LAYER}/internal/stock?activity_id=${ACTIVITY_ID}" | sed -n 's/.*"stock":[ ]*\([0-9-]\+\).*/\1/p')
echo "stock_after=${stock_after}"

# 门禁规则：成功数不能超过初始库存，库存不能为负
if [[ "${success}" -gt "${STOCK}" ]]; then
  echo "gate failed: oversell"
  exit 1
fi
if [[ "${stock_after}" -lt 0 ]]; then
  echo "gate failed: negative stock"
  exit 1
fi

echo "gate passed"
rm -f "${tmpfile}"