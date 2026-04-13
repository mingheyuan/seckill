#!/usr/bin/env bash
set -euo pipefail

NETWORK="seckill-redis-net"
IMAGE="redis:7.2-alpine"
PORTS=(17001 17002 17003 17004 17005 17006)

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found" >&2
  exit 1
fi

docker network inspect "${NETWORK}" >/dev/null 2>&1 || docker network create "${NETWORK}" >/dev/null

for p in "${PORTS[@]}"; do
  name="seckill-redis-${p}"
  docker rm -f "${name}" >/dev/null 2>&1 || true
  docker run -d --name "${name}" --network "${NETWORK}" -p "${p}:${p}" "${IMAGE}" \
    redis-server --port "${p}" --cluster-enabled yes --cluster-config-file nodes.conf \
    --cluster-node-timeout 5000 --appendonly no --protected-mode no >/dev/null
done

docker exec seckill-redis-17001 redis-cli --cluster create \
  seckill-redis-17001:17001 seckill-redis-17002:17002 seckill-redis-17003:17003 \
  seckill-redis-17004:17004 seckill-redis-17005:17005 seckill-redis-17006:17006 \
  --cluster-replicas 1 --cluster-yes >/dev/null

echo "redis cluster started"
echo "nodes: 127.0.0.1:17001,127.0.0.1:17002,127.0.0.1:17003,127.0.0.1:17004,127.0.0.1:17005,127.0.0.1:17006"
