#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
LOG_DIR="${ROOT_DIR}/logs"
RUN_DIR="${ROOT_DIR}/run"

LAYER_PORT=8081
PROXY_PORT=8080
ADMIN_PORT=8082

LAYER_BIN="${BIN_DIR}/layer"
PROXY_BIN="${BIN_DIR}/proxy"
ADMIN_BIN="${BIN_DIR}/admin"

LAYER_PID_FILE="${RUN_DIR}/layer.pid"
PROXY_PID_FILE="${RUN_DIR}/proxy.pid"
ADMIN_PID_FILE="${RUN_DIR}/admin.pid"

mkdir -p "${BIN_DIR}" "${LOG_DIR}" "${RUN_DIR}"

kill_by_pidfile() {
  local pid_file="$1"
  if [[ -f "${pid_file}" ]]; then
    local pid
    pid="$(cat "${pid_file}" || true)"
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
      kill "${pid}" || true
      sleep 0.3
      kill -9 "${pid}" 2>/dev/null || true
    fi
    rm -f "${pid_file}"
  fi
}

kill_by_port() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    local pids
    pids="$(lsof -ti tcp:${port} || true)"
    if [[ -n "${pids}" ]]; then
      kill ${pids} 2>/dev/null || true
      sleep 0.3
      kill -9 ${pids} 2>/dev/null || true
    fi
  elif command -v fuser >/dev/null 2>&1; then
    fuser -k "${port}"/tcp 2>/dev/null || true
  fi
}

build_all() {
  echo "[1/4] build layer/proxy/admin..."
  cd "${ROOT_DIR}"
  go build -o "${LAYER_BIN}" ./cmd/layer
  go build -o "${PROXY_BIN}" ./cmd/proxy
  go build -o "${ADMIN_BIN}" ./cmd/admin
}

stop_all() {
  echo "[2/4] stop old processes..."
  kill_by_pidfile "${LAYER_PID_FILE}"
  kill_by_pidfile "${PROXY_PID_FILE}"
  kill_by_pidfile "${ADMIN_PID_FILE}"

  kill_by_port "${LAYER_PORT}"
  kill_by_port "${PROXY_PORT}"
  kill_by_port "${ADMIN_PORT}"
}

start_one() {
  local name="$1"
  local bin="$2"
  local log_file="$3"
  local pid_file="$4"

  nohup "${bin}" > "${log_file}" 2>&1 &
  local pid=$!
  echo "${pid}" > "${pid_file}"
  echo "started ${name}: pid=${pid}, log=${log_file}"
}

wait_health() {
  local name="$1"
  local url="$2"

  for _ in $(seq 1 40); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      echo "${name} healthy: ${url}"
      return 0
    fi
    sleep 0.25
  done

  echo "${name} health check failed: ${url}"
  return 1
}

start_all() {
  echo "[3/4] start services..."
  start_one "layer" "${LAYER_BIN}" "${LOG_DIR}/layer.log" "${LAYER_PID_FILE}"
  start_one "proxy" "${PROXY_BIN}" "${LOG_DIR}/proxy.log" "${PROXY_PID_FILE}"
  start_one "admin" "${ADMIN_BIN}" "${LOG_DIR}/admin.log" "${ADMIN_PID_FILE}"

  echo "[4/4] health checks..."
  wait_health "layer" "http://127.0.0.1:${LAYER_PORT}/healthz"
  wait_health "proxy" "http://127.0.0.1:${PROXY_PORT}/healthz"
  wait_health "admin" "http://127.0.0.1:${ADMIN_PORT}/healthz"

  echo "all services are up"
  echo "logs: ${LOG_DIR}"
}

status_all() {
  echo "===== status ====="
  for f in "${LAYER_PID_FILE}" "${PROXY_PID_FILE}" "${ADMIN_PID_FILE}"; do
    if [[ -f "${f}" ]]; then
      name="$(basename "${f}" .pid)"
      pid="$(cat "${f}" || true)"
      if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
        echo "${name}: running (pid=${pid})"
      else
        echo "${name}: not running (stale pid file)"
      fi
    fi
  done
}

cmd="${1:-restart}"
case "${cmd}" in
  build)
    build_all
    ;;
  start)
    start_all
    ;;
  stop)
    stop_all
    ;;
  restart)
    build_all
    stop_all
    start_all
    ;;
  status)
    status_all
    ;;
  *)
    echo "usage: $0 [build|start|stop|restart|status]"
    exit 1
    ;;
esac