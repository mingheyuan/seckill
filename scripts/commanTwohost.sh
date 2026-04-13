#!/usr/bin/env bash
set -euo pipefail

# Two-host diagnostics collector.
# Example:
#   SELF_IP=192.168.32.131 PEER_IP=192.168.32.132 NACOS_IP=192.168.32.131 scripts/commanTwohost.sh

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SELF_IP="${SELF_IP:-}"
PEER_IP="${PEER_IP:-}"
NACOS_IP="${NACOS_IP:-192.168.32.131}"
NACOS_PORT="${NACOS_PORT:-8848}"
SELF_ROLE="${SELF_ROLE:-}"
LOCAL_REDIS_PORTS=()
PEER_REDIS_PORTS=()

if [[ -z "${SELF_IP}" || -z "${PEER_IP}" ]]; then
	echo "SELF_IP and PEER_IP are required" >&2
	exit 1
fi

detect_ports_from_docker() {
	mapfile -t detected < <(docker ps --format '{{.Names}}' 2>/dev/null \
		| sed -n 's/^seckill-redis-\([0-9]\{5\}\)$/\1/p' \
		| sort -n)
	if [[ "${#detected[@]}" -eq 3 ]]; then
		LOCAL_REDIS_PORTS=("${detected[@]}")
		if [[ "${LOCAL_REDIS_PORTS[0]}" == "17001" ]]; then
			PEER_REDIS_PORTS=(17004 17005 17006)
			SELF_ROLE="A"
			return 0
		fi
		if [[ "${LOCAL_REDIS_PORTS[0]}" == "17004" ]]; then
			PEER_REDIS_PORTS=(17001 17002 17003)
			SELF_ROLE="B"
			return 0
		fi
	fi
	return 1
}

if ! detect_ports_from_docker; then
	if [[ -z "${SELF_ROLE}" ]]; then
		case "${SELF_IP}" in
			*.131) SELF_ROLE="A" ;;
			*.132) SELF_ROLE="B" ;;
			*)
				echo "Cannot auto-detect Redis ports; set SELF_ROLE=A|B" >&2
				exit 1
				;;
		esac
	fi

	if [[ "${SELF_ROLE}" == "A" ]]; then
		LOCAL_REDIS_PORTS=(17001 17002 17003)
		PEER_REDIS_PORTS=(17004 17005 17006)
	elif [[ "${SELF_ROLE}" == "B" ]]; then
		LOCAL_REDIS_PORTS=(17004 17005 17006)
		PEER_REDIS_PORTS=(17001 17002 17003)
	else
		echo "SELF_ROLE must be A or B" >&2
		exit 1
	fi
fi

OUT_FILE="/tmp/seckill_diag_${SELF_IP}_$(date +%Y%m%d_%H%M%S).log"

section() {
	echo
	echo "===== $1 ====="
}

run() {
	local desc="$1"
	shift
	echo
	echo "--- ${desc} ---"
	"$@" || true
}

{
	echo "diag_time=$(date '+%F %T %z')"
	echo "self_ip=${SELF_IP}"
	echo "peer_ip=${PEER_IP}"
	echo "self_role=${SELF_ROLE}"
	echo "local_redis_ports=${LOCAL_REDIS_PORTS[*]}"
	echo "peer_redis_ports=${PEER_REDIS_PORTS[*]}"
	echo "nacos=${NACOS_IP}:${NACOS_PORT}"

	section "Docker Redis Containers"
	run "docker ps redis" docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 

	section "Redis Cluster Info"
	LOCAL_PORT="${LOCAL_REDIS_PORTS[0]}"
	LOCAL_CONTAINER="seckill-redis-${LOCAL_PORT}"
	if docker ps --format '{{.Names}}' | grep -q "^${LOCAL_CONTAINER}$"; then
		run "cluster nodes" docker exec "${LOCAL_CONTAINER}" redis-cli -p "${LOCAL_PORT}" cluster nodes
		run "cluster info" docker exec "${LOCAL_CONTAINER}" redis-cli -p "${LOCAL_PORT}" cluster info
	else
		echo "${LOCAL_CONTAINER} not found on this host"
	fi

	section "TCP Reachability To Peer Redis"
	for p in "${PEER_REDIS_PORTS[@]}"; do
		bus_p="$((p+10000))"
		if timeout 2 bash -lc "</dev/tcp/${PEER_IP}/${p}" 2>/dev/null; then
			echo "${PEER_IP}:${p} -> ok"
		else
			echo "${PEER_IP}:${p} -> fail"
		fi
		if timeout 2 bash -lc "</dev/tcp/${PEER_IP}/${bus_p}" 2>/dev/null; then
			echo "${PEER_IP}:${bus_p} -> ok"
		else
			echo "${PEER_IP}:${bus_p} -> fail"
		fi
	done

	section "Service Health (Local + Peer)"
	for u in \
		"http://127.0.0.1:8080/healthz" \
		"http://127.0.0.1:8081/healthz" \
		"http://127.0.0.1:8082/healthz" \
		"http://127.0.0.1:8083/healthz" \
		"http://${PEER_IP}:8081/healthz" \
		"http://${PEER_IP}:8083/healthz"
	do
		code="$(curl --noproxy '*' -m 2 -s -o /dev/null -w '%{http_code}' "${u}" || true)"
		echo "${u} -> ${code}"
	done

	section "Nacos Layer Instances"
	run "nacos instance list" curl --noproxy '*' -s "http://${NACOS_IP}:${NACOS_PORT}/nacos/v1/ns/instance/list?serviceName=layer-service&namespaceId=seckill"

	section "Recent Error Logs"
	run "proxy errors" bash -lc "grep -E ' 502 |timeout|layer unavailable' '${ROOT_DIR}/logs/proxy.log' | tail -n 120"
	run "layer1 errors" bash -lc "grep -E 'CLUSTERDOWN|timeout|redis eval failed|读取失败' '${ROOT_DIR}/logs/layer1.log' | tail -n 120"
	run "layer2 errors" bash -lc "grep -E 'CLUSTERDOWN|timeout|redis eval failed|读取失败' '${ROOT_DIR}/logs/layer2.log' | tail -n 120"
} | tee "${OUT_FILE}"

echo
echo "diag file: ${OUT_FILE}"