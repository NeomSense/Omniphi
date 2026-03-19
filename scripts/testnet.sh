#!/usr/bin/env bash
# scripts/testnet.sh — Start a local 5-node PoSeq testnet with hardening features.
#
# Usage:
#   ./scripts/testnet.sh [start|stop|status|logs|validate]
#
# Differences from devnet.sh:
#   - 5 nodes (testnet-grade quorum 3/5)
#   - Longer slot interval (5s) for realistic conditions
#   - Config validation runs before start
#   - Release build instead of debug
#   - Structured log filtering via SLOG_LEVEL
#   - Hard fault injection support (kill/revive via testnet.sh kill-node <N>)

set -euo pipefail

POSEQ_BIN="${POSEQ_BIN:-$(dirname "$0")/../.poseq_target/release/poseq-node}"
TESTNET_DIR="/tmp/poseq_testnet"
LOG_DIR="${TESTNET_DIR}/logs"
PID_FILE="${TESTNET_DIR}/pids"

# ── Node identities (deterministic 32-byte hex) ────────────────────────────────
NODE1_ID="1111111111111111111111111111111111111111111111111111111111111111"
NODE2_ID="2222222222222222222222222222222222222222222222222222222222222222"
NODE3_ID="3333333333333333333333333333333333333333333333333333333333333333"
NODE4_ID="4444444444444444444444444444444444444444444444444444444444444444"
NODE5_ID="5555555555555555555555555555555555555555555555555555555555555555"

NODE1_PORT=7101
NODE2_PORT=7102
NODE3_PORT=7103
NODE4_PORT=7104
NODE5_PORT=7105

METRICS_BASE=9191

validate_config() {
    echo "[testnet] Validating config..."
    # Check binary exists
    if [ ! -f "${POSEQ_BIN}" ]; then
        echo "[testnet] ERROR: poseq-node binary not found at ${POSEQ_BIN}"
        echo "[testnet]        Run: make build-poseq-release"
        exit 1
    fi

    # Check port availability
    for port in ${NODE1_PORT} ${NODE2_PORT} ${NODE3_PORT} ${NODE4_PORT} ${NODE5_PORT}; do
        if lsof -i ":${port}" >/dev/null 2>&1; then
            echo "[testnet] ERROR: port ${port} already in use"
            exit 1
        fi
    done
    echo "[testnet] Config OK."
}

start_node() {
    local n="$1"
    local node_id="$2"
    local port="$3"
    local metrics_port="$4"
    shift 4
    local peer_args=("$@")

    echo "[testnet] Starting node ${n} (port ${port}, metrics :${metrics_port})..."
    "${POSEQ_BIN}" \
        --id "${node_id}" \
        --addr "127.0.0.1:${port}" \
        "${peer_args[@]}" \
        --quorum 3 \
        --slot-ms 5000 \
        --data-dir "${TESTNET_DIR}/data${n}" \
        --metrics-addr "127.0.0.1:${metrics_port}" \
        --state-dump-path "${TESTNET_DIR}/state${n}.json" \
        --export-dir "${TESTNET_DIR}/exports" \
        --snapshot-dir "${TESTNET_DIR}/snapshots" \
        > "${LOG_DIR}/node${n}.log" 2>&1 &
    echo "$!" >> "${PID_FILE}"
}

start_testnet() {
    validate_config

    echo "[testnet] Building poseq-node (release)..."
    (cd "$(dirname "$0")/.." && cargo build --bin poseq-node --release 2>&1) || {
        echo "[testnet] ERROR: build failed"
        exit 1
    }

    echo "[testnet] Creating testnet dirs..."
    mkdir -p "${TESTNET_DIR}" "${LOG_DIR}"
    mkdir -p "${TESTNET_DIR}/exports" "${TESTNET_DIR}/snapshots"
    for i in 1 2 3 4 5; do
        mkdir -p "${TESTNET_DIR}/data${i}"
    done
    rm -f "${PID_FILE}"

    # All peers for each node
    P1="--peer 127.0.0.1:${NODE2_PORT} --peer 127.0.0.1:${NODE3_PORT} --peer 127.0.0.1:${NODE4_PORT} --peer 127.0.0.1:${NODE5_PORT}"
    P2="--peer 127.0.0.1:${NODE1_PORT} --peer 127.0.0.1:${NODE3_PORT} --peer 127.0.0.1:${NODE4_PORT} --peer 127.0.0.1:${NODE5_PORT}"
    P3="--peer 127.0.0.1:${NODE1_PORT} --peer 127.0.0.1:${NODE2_PORT} --peer 127.0.0.1:${NODE4_PORT} --peer 127.0.0.1:${NODE5_PORT}"
    P4="--peer 127.0.0.1:${NODE1_PORT} --peer 127.0.0.1:${NODE2_PORT} --peer 127.0.0.1:${NODE3_PORT} --peer 127.0.0.1:${NODE5_PORT}"
    P5="--peer 127.0.0.1:${NODE1_PORT} --peer 127.0.0.1:${NODE2_PORT} --peer 127.0.0.1:${NODE3_PORT} --peer 127.0.0.1:${NODE4_PORT}"

    start_node 1 "${NODE1_ID}" "${NODE1_PORT}" $((METRICS_BASE + 0)) ${P1}
    start_node 2 "${NODE2_ID}" "${NODE2_PORT}" $((METRICS_BASE + 1)) ${P2}
    start_node 3 "${NODE3_ID}" "${NODE3_PORT}" $((METRICS_BASE + 2)) ${P3}
    start_node 4 "${NODE4_ID}" "${NODE4_PORT}" $((METRICS_BASE + 3)) ${P4}
    start_node 5 "${NODE5_ID}" "${NODE5_PORT}" $((METRICS_BASE + 4)) ${P5}

    echo "[testnet] Waiting for nodes to become ready (up to 60s)..."
    local TIMEOUT=60
    local ELAPSED=0
    local READY=0
    while [ "${ELAPSED}" -lt "${TIMEOUT}" ] && [ "${READY}" -lt 5 ]; do
        sleep 1
        ELAPSED=$((ELAPSED + 1))
        READY=$(grep -rl "READY node_id=" "${LOG_DIR}/node1.log" "${LOG_DIR}/node2.log" \
            "${LOG_DIR}/node3.log" "${LOG_DIR}/node4.log" "${LOG_DIR}/node5.log" 2>/dev/null | \
            xargs grep -l "READY node_id=" 2>/dev/null | wc -l || echo 0)
    done

    if [ "${READY}" -ge 5 ]; then
        echo "[testnet] All 5 nodes ready."
    else
        echo "[testnet] WARNING: Only ${READY}/5 nodes ready within ${TIMEOUT}s"
        echo "[testnet] Check logs in ${LOG_DIR}/"
    fi

    echo ""
    echo "=== Omniphi PoSeq Testnet ==="
    echo "  Node 1: tcp://127.0.0.1:${NODE1_PORT}  metrics: http://127.0.0.1:$((METRICS_BASE+0))/metrics"
    echo "  Node 2: tcp://127.0.0.1:${NODE2_PORT}  metrics: http://127.0.0.1:$((METRICS_BASE+1))/metrics"
    echo "  Node 3: tcp://127.0.0.1:${NODE3_PORT}  metrics: http://127.0.0.1:$((METRICS_BASE+2))/metrics"
    echo "  Node 4: tcp://127.0.0.1:${NODE4_PORT}  metrics: http://127.0.0.1:$((METRICS_BASE+3))/metrics"
    echo "  Node 5: tcp://127.0.0.1:${NODE5_PORT}  metrics: http://127.0.0.1:$((METRICS_BASE+4))/metrics"
    echo ""
    echo "  Quorum: 3/5"
    echo "  Slot interval: 5s"
    echo "  Logs:   ${LOG_DIR}/"
    echo "  Exports: ${TESTNET_DIR}/exports/"
    echo "  Snapshots: ${TESTNET_DIR}/snapshots/"
    echo ""
    echo "  Stop:         ./scripts/testnet.sh stop"
    echo "  Logs:         ./scripts/testnet.sh logs"
    echo "  Status:       ./scripts/testnet.sh status"
    echo "  Kill node N:  ./scripts/testnet.sh kill-node <N>"
    echo "  Revive PID:   ./scripts/testnet.sh revive-node <N>"
}

stop_testnet() {
    if [ ! -f "${PID_FILE}" ]; then
        echo "[testnet] No PID file found at ${PID_FILE}"
        return
    fi
    echo "[testnet] Stopping nodes..."
    while read -r pid; do
        if kill -0 "${pid}" 2>/dev/null; then
            kill "${pid}" && echo "  Stopped PID ${pid}"
        fi
    done < "${PID_FILE}"
    rm -f "${PID_FILE}"
    echo "[testnet] All nodes stopped."
}

status_testnet() {
    echo "=== PoSeq Testnet Status ==="
    for i in 1 2 3 4 5; do
        local state_file="${TESTNET_DIR}/state${i}.json"
        if [ -f "${state_file}" ]; then
            echo "  Node ${i}: $(python3 -c "
import sys,json
d=json.load(open('${state_file}'))
print(f'epoch={d[\"current_epoch\"]} slot={d[\"current_slot\"]} exported={d[\"exported_epoch_count\"]} peers={d[\"peer_count\"]} ready={d[\"ready\"]}')
" 2>/dev/null || echo "parse error")"
        else
            echo "  Node ${i}: state file not found"
        fi
    done
    echo ""
    echo "  Exports: $(ls ${TESTNET_DIR}/exports/ 2>/dev/null | wc -l) files"
    echo "  Snapshots: $(ls ${TESTNET_DIR}/snapshots/ 2>/dev/null | wc -l) files"
}

logs_testnet() {
    if [ -d "${LOG_DIR}" ]; then
        echo "[testnet] Tailing logs from ${LOG_DIR}/ (Ctrl+C to stop)..."
        tail -f "${LOG_DIR}/node1.log" "${LOG_DIR}/node2.log" "${LOG_DIR}/node3.log" \
             "${LOG_DIR}/node4.log" "${LOG_DIR}/node5.log"
    else
        echo "[testnet] No logs found at ${LOG_DIR}/"
    fi
}

kill_node() {
    local n="${1:-}"
    if [ -z "${n}" ]; then
        echo "Usage: testnet.sh kill-node <1-5>"
        exit 1
    fi
    local line=$n
    local pid
    pid=$(sed -n "${line}p" "${PID_FILE}" 2>/dev/null || echo "")
    if [ -z "${pid}" ]; then
        echo "[testnet] No PID found for node ${n}"
        return
    fi
    if kill -0 "${pid}" 2>/dev/null; then
        kill -9 "${pid}"
        echo "[testnet] Killed node ${n} (PID ${pid})"
    else
        echo "[testnet] Node ${n} (PID ${pid}) is already dead"
    fi
}

CMD="${1:-start}"
case "${CMD}" in
    start)       start_testnet ;;
    stop)        stop_testnet ;;
    status)      status_testnet ;;
    logs)        logs_testnet ;;
    validate)    validate_config ;;
    kill-node)   kill_node "${2:-}" ;;
    *)
        echo "Usage: testnet.sh [start|stop|status|logs|validate|kill-node <N>]"
        exit 1
        ;;
esac
