#!/usr/bin/env bash
set -euo pipefail

# Wrapper for two-host deployment.
# Example on 192.168.32.131:
#   SELF_IP=192.168.32.131 NACOS_IP=192.168.32.131 scripts/start_services_dual_layer_2host.sh
# Example on 192.168.32.132:
#   SELF_IP=192.168.32.132 NACOS_IP=192.168.32.131 scripts/start_services_dual_layer_2host.sh

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SELF_IP="${SELF_IP:-}"
NACOS_IP="${NACOS_IP:-192.168.32.131}"
NACOS_PORT="${NACOS_PORT:-8848}"

if [[ -z "${SELF_IP}" ]]; then
  echo "SELF_IP is required" >&2
  exit 1
fi

export STORAGE_REDIS_ADDR="${STORAGE_REDIS_ADDR:-192.168.32.131:17001,192.168.32.131:17002,192.168.32.131:17003,192.168.32.132:17004,192.168.32.132:17005,192.168.32.132:17006}"
export NACOS_SERVER_IP="${NACOS_IP}"
export NACOS_SERVER_PORT="${NACOS_PORT}"

export LAYER_REGISTER_IP="${SELF_IP}"
export PROXY_REGISTER_IP="${SELF_IP}"
export ADMIN_REGISTER_IP="${SELF_IP}"

"${ROOT_DIR}/scripts/start_services_dual_layer.sh"
