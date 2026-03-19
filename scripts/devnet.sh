#!/usr/bin/env bash
# scripts/devnet.sh — Start a local 3-node PoSeq devnet.
#
# Usage:
#   ./scripts/devnet.sh [start|stop|status|logs]
#
# Requirements:
#   - cargo build must have completed (poseq-node binary available)
#   - POSIX shell (bash/sh)
#
# What it does (start):
#   1. Creates per-node config dirs under /tmp/poseq_devnet/
#   2. Launches 3 poseq-node processes, each with:
#      - Unique node identity (deterministic hex keys)
#      - Peered to the other two nodes
#      - Prometheus metrics on ports 9091/9092/9093
#      - Export dir: /tmp/poseq_devnet/exports/
#      - Snapshot dir: /tmp/poseq_devnet/snapshots/
#      - State dump: /tmp/poseq_devnet/stateN.json
#   3. Waits for all 3 nodes to emit READY marker
#   4. Prints summary of endpoints

set -euo pipefail

POSEQ_BIN="${POSEQ_BIN:-$(dirname "$0")/../.poseq_target/debug/poseq-node}"
DEVNET_DIR="/tmp/poseq_devnet"
LOG_DIR="${DEVNET_DIR}/logs"
PID_FILE="${DEVNET_DIR}/pids"

# ── Node identities (deterministic 32-byte hex) ────────────────────────────────
NODE1_ID="0101010101010101010101010101010101010101010101010101010101010101"
NODE2_ID="0202020202020202020202020202020202020202020202020202020202020202"
NODE3_ID="0303030303030303030303030303030303030303030303030303030303030303"

NODE1_PORT=7001
NODE2_PORT=7002
NODE3_PORT=7003

start_devnet() {
    echo "[devnet] Building poseq-node binary..."
    (cd "$(dirname "$0")/.." && cargo build --bin poseq-node 2>&1) || {
        echo "[devnet] ERROR: build failed"
        exit 1
    }

    echo "[devnet] Creating devnet dirs..."
    mkdir -p "${DEVNET_DIR}" "${LOG_DIR}"
    mkdir -p "${DEVNET_DIR}/exports"
    mkdir -p "${DEVNET_DIR}/snapshots"
    mkdir -p "${DEVNET_DIR}/data1" "${DEVNET_DIR}/data2" "${DEVNET_DIR}/data3"
    rm -f "${PID_FILE}"

    echo "[devnet] Starting node 1 (port ${NODE1_PORT})..."
    "${POSEQ_BIN}" \
        --id "${NODE1_ID}" \
        --addr "127.0.0.1:${NODE1_PORT}" \
        --peer "127.0.0.1:${NODE2_PORT}" \
        --peer "127.0.0.1:${NODE3_PORT}" \
        --quorum 2 \
        --slot-ms 2000 \
        --data-dir "${DEVNET_DIR}/data1" \
        --metrics-addr "127.0.0.1:9091" \
        --state-dump-path "${DEVNET_DIR}/state1.json" \
        --export-dir "${DEVNET_DIR}/exports" \
        --snapshot-dir "${DEVNET_DIR}/snapshots" \
        > "${LOG_DIR}/node1.log" 2>&1 &
    echo "$!" >> "${PID_FILE}"

    echo "[devnet] Starting node 2 (port ${NODE2_PORT})..."
    "${POSEQ_BIN}" \
        --id "${NODE2_ID}" \
        --addr "127.0.0.1:${NODE2_PORT}" \
        --peer "127.0.0.1:${NODE1_PORT}" \
        --peer "127.0.0.1:${NODE3_PORT}" \
        --quorum 2 \
        --slot-ms 2000 \
        --data-dir "${DEVNET_DIR}/data2" \
        --metrics-addr "127.0.0.1:9092" \
        --state-dump-path "${DEVNET_DIR}/state2.json" \
        --export-dir "${DEVNET_DIR}/exports" \
        --snapshot-dir "${DEVNET_DIR}/snapshots" \
        > "${LOG_DIR}/node2.log" 2>&1 &
    echo "$!" >> "${PID_FILE}"

    echo "[devnet] Starting node 3 (port ${NODE3_PORT})..."
    "${POSEQ_BIN}" \
        --id "${NODE3_ID}" \
        --addr "127.0.0.1:${NODE3_PORT}" \
        --peer "127.0.0.1:${NODE1_PORT}" \
        --peer "127.0.0.1:${NODE2_PORT}" \
        --quorum 2 \
        --slot-ms 2000 \
        --data-dir "${DEVNET_DIR}/data3" \
        --metrics-addr "127.0.0.1:9093" \
        --state-dump-path "${DEVNET_DIR}/state3.json" \
        --export-dir "${DEVNET_DIR}/exports" \
        --snapshot-dir "${DEVNET_DIR}/snapshots" \
        > "${LOG_DIR}/node3.log" 2>&1 &
    echo "$!" >> "${PID_FILE}"

    echo "[devnet] Waiting for nodes to become ready..."
    local TIMEOUT=30
    local ELAPSED=0
    local READY=0
    while [ "${ELAPSED}" -lt "${TIMEOUT}" ] && [ "${READY}" -lt 3 ]; do
        sleep 1
        ELAPSED=$((ELAPSED + 1))
        READY=0
        grep -l "READY node_id=" "${LOG_DIR}/node1.log" "${LOG_DIR}/node2.log" "${LOG_DIR}/node3.log" 2>/dev/null | wc -l | read -r READY || READY=0
        READY=$(grep -c "READY node_id=" "${LOG_DIR}/node1.log" "${LOG_DIR}/node2.log" "${LOG_DIR}/node3.log" 2>/dev/null | grep -v "^0$" | wc -l)
    done

    if [ "${READY}" -ge 3 ]; then
        echo "[devnet] All 3 nodes ready."
    else
        echo "[devnet] WARNING: Only ${READY}/3 nodes reported ready within ${TIMEOUT}s"
        echo "[devnet] Check logs in ${LOG_DIR}/"
    fi

    echo ""
    echo "=== Omniphi PoSeq Devnet ==="
    echo "  Node 1: tcp://127.0.0.1:${NODE1_PORT}  metrics: http://127.0.0.1:9091/metrics"
    echo "  Node 2: tcp://127.0.0.1:${NODE2_PORT}  metrics: http://127.0.0.1:9092/metrics"
    echo "  Node 3: tcp://127.0.0.1:${NODE3_PORT}  metrics: http://127.0.0.1:9093/metrics"
    echo "  Logs:   ${LOG_DIR}/"
    echo "  State:  ${DEVNET_DIR}/state{1,2,3}.json"
    echo "  Exports: ${DEVNET_DIR}/exports/"
    echo "  Snapshots: ${DEVNET_DIR}/snapshots/"
    echo ""
    echo "  Stop:  ./scripts/devnet.sh stop"
    echo "  Logs:  ./scripts/devnet.sh logs"
    echo "  Status: ./scripts/devnet.sh status"
}

stop_devnet() {
    if [ ! -f "${PID_FILE}" ]; then
        echo "[devnet] No PID file found at ${PID_FILE}"
        return
    fi
    echo "[devnet] Stopping nodes..."
    while read -r pid; do
        if kill -0 "${pid}" 2>/dev/null; then
            kill "${pid}" && echo "  Stopped PID ${pid}"
        fi
    done < "${PID_FILE}"
    rm -f "${PID_FILE}"
    echo "[devnet] All nodes stopped."
}

status_devnet() {
    echo "=== PoSeq Devnet Status ==="
    for i in 1 2 3; do
        local state_file="${DEVNET_DIR}/state${i}.json"
        if [ -f "${state_file}" ]; then
            echo "  Node ${i}: $(cat "${state_file}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'epoch={d[\"current_epoch\"]} slot={d[\"current_slot\"]} exported={d[\"exported_epoch_count\"]} ready={d[\"ready\"]}')" 2>/dev/null || echo "parse error")"
        else
            echo "  Node ${i}: state file not found"
        fi
    done
    echo ""
    echo "  Exports: $(ls ${DEVNET_DIR}/exports/ 2>/dev/null | wc -l) files"
    echo "  Snapshots: $(ls ${DEVNET_DIR}/snapshots/ 2>/dev/null | wc -l) files"
}

logs_devnet() {
    if [ -d "${LOG_DIR}" ]; then
        echo "[devnet] Tailing logs from ${LOG_DIR}/ (Ctrl+C to stop)..."
        tail -f "${LOG_DIR}/node1.log" "${LOG_DIR}/node2.log" "${LOG_DIR}/node3.log"
    else
        echo "[devnet] No logs found at ${LOG_DIR}/"
    fi
}

CMD="${1:-start}"
case "${CMD}" in
    start)  start_devnet ;;
    stop)   stop_devnet ;;
    status) status_devnet ;;
    logs)   logs_devnet ;;
    *)
        echo "Usage: devnet.sh [start|stop|status|logs]"
        exit 1
        ;;
esac
