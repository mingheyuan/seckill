#!/usr/bin/env bash
set -euo pipefail

BASE_PROXY="http://127.0.0.1:8080"
BASE_LAYER="http://127.0.0.1:8081"
BASE_ADMIN="http://127.0.0.1:8082"

ACTIVITY_ID=1001
STOCK=5

echo "== init stock =="
curl -sS -X POST "${BASE_ADMIN}/admin/activity/init" \
  -H "Content-Type: application/json" \
  -d "{\"activity_id\":${ACTIVITY_ID},\"stock\":${STOCK}}"
echo
echo

echo "== stock before =="
curl -sS "${BASE_LAYER}/internal/stock?activity_id=${ACTIVITY_ID}"
echo
echo

echo "== duplicate user test (u1 twice) =="
curl -sS -X POST "${BASE_PROXY}/api/seckill" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"u1\",\"activity_id\":${ACTIVITY_ID}}"
echo
curl -sS -X POST "${BASE_PROXY}/api/seckill" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"u1\",\"activity_id\":${ACTIVITY_ID}}"
echo
echo

echo "== concurrent users test =="
tmpfile="$(mktemp)"
for i in $(seq 2 20); do
  (
    curl -sS -X POST "${BASE_PROXY}/api/seckill" \
      -H "Content-Type: application/json" \
      -d "{\"user_id\":\"u${i}\",\"activity_id\":${ACTIVITY_ID}}"
    echo
  ) >> "${tmpfile}" &
done
wait

cat "${tmpfile}"
echo
echo "success count:"
grep -c '"code":0' "${tmpfile}" || true
echo "fail count:"
grep -vc '"code":0' "${tmpfile}" || true

echo
echo "== stock after =="
curl -sS "${BASE_LAYER}/internal/stock?activity_id=${ACTIVITY_ID}"
echo

rm -f "${tmpfile}"