#!/usr/bin/env bash
set -euo pipefail

# Nginx LB wrapper for proxy horizontal ingress.
# Usage examples:
#   PROXY_UPSTREAMS='192.168.32.131:8080,192.168.32.132:8080' scripts/start_nginx_lb.sh up
#   PROXY_UPSTREAMS='192.168.32.131:8080,192.168.32.132:8080' NGINX_LISTEN_PORT=18080 scripts/start_nginx_lb.sh reload
#   scripts/start_nginx_lb.sh status
#   scripts/start_nginx_lb.sh down

ACTION="${1:-up}"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TEMPLATE="${ROOT_DIR}/config/nginx/lb.conf.tmpl"
RUNTIME_DIR="${ROOT_DIR}/run/nginx-lb"
CONF_FILE="${RUNTIME_DIR}/nginx.conf"
LOG_DIR="${RUNTIME_DIR}/logs"

NGINX_BIN="${NGINX_BIN:-$(command -v nginx || true)}"
NGINX_LISTEN_PORT="${NGINX_LISTEN_PORT:-18080}"
PROXY_UPSTREAMS="${PROXY_UPSTREAMS:-192.168.32.131:8080,192.168.32.132:8080}"

if [[ -z "${NGINX_BIN}" ]]; then
  echo "nginx not found, please install nginx first" >&2
  exit 1
fi

if [[ ! -f "${TEMPLATE}" ]]; then
  echo "template not found: ${TEMPLATE}" >&2
  exit 1
fi

mkdir -p "${RUNTIME_DIR}" "${LOG_DIR}"

build_upstream_servers() {
  local raw="$1"
  local out=""
  IFS=',' read -r -a arr <<< "${raw}"
  for i in "${!arr[@]}"; do
    local addr
    addr="$(echo "${arr[$i]}" | xargs)"
    if [[ -z "${addr}" ]]; then
      continue
    fi
    out+="        server ${addr} max_fails=3 fail_timeout=2s;"
    out+=$'\n'
  done

  if [[ -z "${out}" ]]; then
    echo "PROXY_UPSTREAMS is empty" >&2
    exit 1
  fi

  printf "%s" "${out}"
}

generate_conf() {
  local servers
  servers="$(build_upstream_servers "${PROXY_UPSTREAMS}")"

  sed \
    -e "s#__LISTEN_PORT__#${NGINX_LISTEN_PORT}#g" \
    -e "/__UPSTREAM_SERVERS__/ { r /dev/stdin\n d; }" \
    "${TEMPLATE}" > "${CONF_FILE}" <<< "${servers}"
}

nginx_cmd() {
  "${NGINX_BIN}" -p "${RUNTIME_DIR}/" -c "${CONF_FILE}" "$@"
}

case "${ACTION}" in
  up)
    generate_conf
    nginx_cmd -t
    if [[ -f "${RUNTIME_DIR}/logs/nginx.pid" ]]; then
      nginx_cmd -s reload
      echo "nginx lb reloaded on :${NGINX_LISTEN_PORT}" 
    else
      nginx_cmd
      echo "nginx lb started on :${NGINX_LISTEN_PORT}" 
    fi
    ;;
  reload)
    generate_conf
    nginx_cmd -t
    nginx_cmd -s reload
    echo "nginx lb reloaded on :${NGINX_LISTEN_PORT}" 
    ;;
  down)
    if [[ -f "${RUNTIME_DIR}/logs/nginx.pid" ]]; then
      nginx_cmd -s quit || true
      echo "nginx lb stopped"
    else
      echo "nginx lb not running"
    fi
    ;;
  status)
    if [[ -f "${RUNTIME_DIR}/logs/nginx.pid" ]]; then
      pid="$(cat "${RUNTIME_DIR}/logs/nginx.pid" || true)"
      if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
        echo "nginx lb running pid=${pid} listen=:${NGINX_LISTEN_PORT}"
        echo "upstreams=${PROXY_UPSTREAMS}"
        exit 0
      fi
    fi
    echo "nginx lb not running"
    ;;
  *)
    echo "usage: $0 [up|reload|down|status]" >&2
    exit 1
    ;;
esac
