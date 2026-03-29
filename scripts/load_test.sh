#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
ACTIVITY_ID="${ACTIVITY_ID:-1001}"
VUS="${VUS:-200}"
DURATION="${DURATION:-30s}"

if ! command -v k6 >/dev/null 2>&1; then
  echo "k6 not found. install from https://k6.io/docs/get-started/installation/"
  exit 1
fi

echo "Running k6: base=${BASE_URL} activity=${ACTIVITY_ID} vus=${VUS} duration=${DURATION}"
k6 run \
  -e BASE_URL="${BASE_URL}" \
  -e ACTIVITY_ID="${ACTIVITY_ID}" \
  -e VUS="${VUS}" \
  -e DURATION="${DURATION}" \
  "${ROOT_DIR}/bench/k6_seckill.js"
