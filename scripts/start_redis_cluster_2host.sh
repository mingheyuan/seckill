#!/usr/bin/env bash
set -euo pipefail

# Two-host Redis Cluster launcher.
# Host A runs ports 17001-17003, Host B runs 17004-17006.
# Usage examples:
#   ROLE=A SELF_IP=192.168.32.131 PEER_IP=192.168.32.132 scripts/start_redis_cluster_2host.sh up
#   ROLE=B SELF_IP=192.168.32.132 PEER_IP=192.168.32.131 scripts/start_redis_cluster_2host.sh up
#   ROLE=A SELF_IP=192.168.32.131 PEER_IP=192.168.32.132 scripts/start_redis_cluster_2host.sh create

ACTION="${1:-up}"
ROLE="${ROLE:-}"
SELF_IP="${SELF_IP:-}"
PEER_IP="${PEER_IP:-}"
IMAGE="${REDIS_IMAGE:-redis:7.2-alpine}"
DATA_ROOT="${DATA_ROOT:-/tmp/seckill-redis-cluster}"

if [[ -z "${ROLE}" || ("${ROLE}" != "A" && "${ROLE}" != "B") ]]; then
  echo "ROLE must be A or B" >&2
  exit 1
fi

if [[ -z "${SELF_IP}" || -z "${PEER_IP}" ]]; then
  echo "SELF_IP and PEER_IP are required" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found" >&2
  exit 1
fi

if [[ "${ROLE}" == "A" ]]; then
  PORTS=(17001 17002 17003)
else
  PORTS=(17004 17005 17006)
fi

start_local_nodes() {
  mkdir -p "${DATA_ROOT}"
  for p in "${PORTS[@]}"; do
    name="seckill-redis-${p}"
    dir="${DATA_ROOT}/${p}"
    mkdir -p "${dir}"
    docker rm -f "${name}" >/dev/null 2>&1 || true

    docker run -d \
      --name "${name}" \
      --network host \
      -v "${dir}:/data" \
      "${IMAGE}" \
      redis-server \
      --port "${p}" \
      --cluster-enabled yes \
      --cluster-config-file /data/nodes.conf \
      --cluster-node-timeout 5000 \
      --appendonly no \
      --protected-mode no \
      --cluster-announce-ip "${SELF_IP}" \
      --cluster-announce-port "${p}" \
      --cluster-announce-bus-port "$((p+10000))" \
      >/dev/null
  done

  echo "local redis nodes started: ${PORTS[*]}"
}

create_cluster() {
  if [[ "${ROLE}" != "A" ]]; then
    echo "cluster create should be executed on ROLE=A host" >&2
    exit 1
  fi

  docker exec seckill-redis-17001 redis-cli --cluster create \
    "${SELF_IP}:17001" "${SELF_IP}:17002" "${SELF_IP}:17003" \
    "${PEER_IP}:17004" "${PEER_IP}:17005" "${PEER_IP}:17006" \
    --cluster-replicas 1 --cluster-yes >/dev/null

  echo "redis cluster created"
}

status_cluster() {
  docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' | grep 'seckill-redis-17' || true
  if docker ps --format '{{.Names}}' | grep -q '^seckill-redis-17001$'; then
    docker exec seckill-redis-17001 redis-cli -p 17001 cluster nodes | head -n 12
  fi
}

stop_local_nodes() {
  for p in "${PORTS[@]}"; do
    docker rm -f "seckill-redis-${p}" >/dev/null 2>&1 || true
  done
  echo "local redis nodes removed: ${PORTS[*]}"
}

case "${ACTION}" in
  up)
    start_local_nodes
    ;;
  create)
    create_cluster
    ;;
  status)
    status_cluster
    ;;
  down)
    stop_local_nodes
    ;;
  *)
    echo "usage: ROLE=A|B SELF_IP=<ip> PEER_IP=<ip> $0 [up|create|status|down]" >&2
    exit 1
    ;;
esac
