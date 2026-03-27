#!/usr/bin/env bash
set -euo pipefail

BASE_PROXY="http://127.0.0.1:8080"
BASE_LAYER="http://127.0.0.1:8081"
BASE_ADMIN="http://127.0.0.1:8082"

ACTIVITY_ID=1001
STOCK=5

REQUIRE_SIGNATURE="${PROXY_REQUIRE_SIGNATURE:-false}"
SIGN_SECRET="${PROXY_SIGN_SECRET:-seckill_sign}"

post_seckill() {
  local uid="$1"
  local body="{\"user_id\":\"${uid}\",\"activity_id\":${ACTIVITY_ID}}"
  local headers=(-H "Content-Type: application/json")

  if [[ "${REQUIRE_SIGNATURE}" == "true" ]]; then
    local ts payload sig
    ts="$(date +%s)"
    payload="${uid}:${ACTIVITY_ID}:${ts}"
    sig="$(printf "%s" "${payload}" | openssl dgst -sha256 -hmac "${SIGN_SECRET}" | awk '{print $2}')"
    headers+=(-H "X-Timestamp: ${ts}" -H "X-Signature: ${sig}")
  fi

  curl -sS -X POST "${BASE_PROXY}/api/seckill" "${headers[@]}" -d "${body}"
}

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
post_seckill "u1"
echo
post_seckill "u1"
echo

echo "== concurrent users test =="
tmpfile="$(mktemp)"
for i in $(seq 2 20); do
  (
    post_seckill "u${i}"
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