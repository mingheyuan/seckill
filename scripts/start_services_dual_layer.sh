#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
LOG_DIR="${ROOT_DIR}/logs"
RUN_DIR="${ROOT_DIR}/run"
TMP_DIR="${ROOT_DIR}/run/tmp"
BASE_CFG="${ROOT_DIR}/config/config.yaml"
LAYER2_CFG="${TMP_DIR}/config.layer2.yaml"

mkdir -p "${BIN_DIR}" "${LOG_DIR}" "${RUN_DIR}" "${TMP_DIR}"

if [[ ! -f "${BASE_CFG}" ]]; then
  echo "config not found: ${BASE_CFG}" >&2
  exit 1
fi

kill_by_pidfile() {
  local pid_file="$1"
  if [[ -f "${pid_file}" ]]; then
    local pid
    pid="$(cat "${pid_file}" 2>/dev/null || true)"
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
      kill "${pid}" 2>/dev/null || true
      sleep 0.2
      kill -9 "${pid}" 2>/dev/null || true
    fi
    rm -f "${pid_file}"
  fi
}

yaml_get() {
  local section="$1"
  local key="$2"
  local file="$3"
  awk -v sec="${section}" -v k="${key}" '
    $0 ~ "^"sec":[[:space:]]*$" { insec=1; next }
    insec && $0 ~ "^[A-Za-z0-9_]+:[[:space:]]*$" { insec=0 }
    insec && $0 ~ "^[[:space:]]*"k":[[:space:]]*" {
      sub("^[[:space:]]*"k":[[:space:]]*", "")
      gsub(/[[:space:]]+$/, "")
      print
      exit
    }
  ' "${file}"
}

cleanup_nacos_layer_instances() {
  local nacos_ip nacos_port namespace_id service_name register_ip base
  nacos_ip="$(yaml_get nacos serverIp "${BASE_CFG}")"
  nacos_port="$(yaml_get nacos serverPort "${BASE_CFG}")"
  namespace_id="$(yaml_get nacos namespaceId "${BASE_CFG}")"
  service_name="$(yaml_get layer serviceName "${BASE_CFG}")"
  register_ip="$(yaml_get layer registerIp "${BASE_CFG}")"

  [[ -n "${nacos_ip}" ]] || nacos_ip="127.0.0.1"
  [[ -n "${nacos_port}" ]] || nacos_port="8848"
  [[ -n "${namespace_id}" ]] || namespace_id="seckill"
  [[ -n "${service_name}" ]] || service_name="layer-service"
  [[ -n "${register_ip}" ]] || register_ip="127.0.0.1"

  base="http://${nacos_ip}:${nacos_port}/nacos/v1/ns/instance"
  for p in 8081 8083; do
    curl --noproxy '*' -fsS -X DELETE "${base}" \
      --data-urlencode "serviceName=${service_name}" \
      --data-urlencode "ip=${register_ip}" \
      --data-urlencode "port=${p}" \
      --data-urlencode "namespaceId=${namespace_id}" \
      --data-urlencode "ephemeral=true" >/dev/null 2>&1 || true
  done
}

wait_health() {
  local name="$1"
  local url="$2"
  for _ in $(seq 1 80); do
    if curl --noproxy '*' -fsS "${url}" >/dev/null 2>&1; then
      echo "healthy ${name}: ${url}"
      return 0
    fi
    sleep 0.25
  done
  echo "health check failed: ${name} ${url}" >&2
  return 1
}

start_proc() {
  local name="$1"
  local cfg="$2"
  local bin="$3"
  local log_file="$4"
  local pid_file="$5"

  nohup env SECKILL_CONFIG_PATH="${cfg}" "${bin}" >"${log_file}" 2>&1 &
  local pid=$!
  echo "${pid}" >"${pid_file}"
  echo "started ${name}: pid=${pid}"
}

echo "[1/5] build binaries"
(cd "${ROOT_DIR}" && go build -o "${BIN_DIR}/layer" ./cmd/layer)
(cd "${ROOT_DIR}" && go build -o "${BIN_DIR}/admin" ./cmd/admin)
(cd "${ROOT_DIR}" && go build -o "${BIN_DIR}/proxy" ./cmd/proxy)

echo "[2/5] stop old processes"
kill_by_pidfile "${RUN_DIR}/layer1.pid"
kill_by_pidfile "${RUN_DIR}/layer2.pid"
kill_by_pidfile "${RUN_DIR}/admin.pid"
kill_by_pidfile "${RUN_DIR}/proxy.pid"

echo "[2.5/5] cleanup nacos stale layer instances"
cleanup_nacos_layer_instances

echo "[3/5] generate layer2 yaml"
awk '
BEGIN { in_layer=0 }
/^layer:[[:space:]]*$/ { in_layer=1; print; next }
/^[A-Za-z0-9_]+:[[:space:]]*$/ {
  if ($0 !~ /^layer:[[:space:]]*$/) in_layer=0
}
{
  if (in_layer && $0 ~ /^[[:space:]]+listenAddr:[[:space:]]*/) {
    sub(/listenAddr:[[:space:]]*.*/, "listenAddr: :8083")
  }
  if (in_layer && $0 ~ /^[[:space:]]+registerPort:[[:space:]]*/) {
    sub(/registerPort:[[:space:]]*[0-9]+/, "registerPort: 8083")
  }
  print
}
' "${BASE_CFG}" > "${LAYER2_CFG}"

echo "[4/5] start services"
start_proc "layer-1" "${BASE_CFG}" "${BIN_DIR}/layer" "${LOG_DIR}/layer1.log" "${RUN_DIR}/layer1.pid"
start_proc "layer-2" "${LAYER2_CFG}" "${BIN_DIR}/layer" "${LOG_DIR}/layer2.log" "${RUN_DIR}/layer2.pid"
start_proc "admin" "${BASE_CFG}" "${BIN_DIR}/admin" "${LOG_DIR}/admin.log" "${RUN_DIR}/admin.pid"
start_proc "proxy" "${BASE_CFG}" "${BIN_DIR}/proxy" "${LOG_DIR}/proxy.log" "${RUN_DIR}/proxy.pid"

echo "[5/5] health checks"
wait_health "layer-1" "http://127.0.0.1:8081/healthz"
wait_health "layer-2" "http://127.0.0.1:8083/healthz"
wait_health "admin" "http://127.0.0.1:8082/healthz"
wait_health "proxy" "http://127.0.0.1:8080/healthz"

echo "all services started"
echo "layer1 config: ${BASE_CFG}"
echo "layer2 config: ${LAYER2_CFG}"
