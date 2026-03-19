.PHONY: build-poseq check-poseq test-poseq test-poseq-doc \
        build-chain test-chain test-all build-all \
        run-node1 run-node2 run-node3 run-devnet \
        test-live-cluster proof-phase7b \
        devnet-start devnet-stop devnet-status devnet-logs \
        test-devnet-chaos \
        build-poseq-release \
        testnet-start testnet-stop testnet-status testnet-logs testnet-validate \
        test-network-hardening proof-phase7d \
        test-state-sync proof-phase8

CARGO    := $(HOME)/.cargo/bin/cargo
POSEQ_DIR := poseq
CHAIN_DIR := chain

# ── Build ──────────────────────────────────────────────────────────────────────

build-poseq:
	cd $(POSEQ_DIR) && $(CARGO) build --all-targets

check-poseq:
	cd $(POSEQ_DIR) && $(CARGO) check --all-targets

build-chain:
	cd $(CHAIN_DIR) && go build ./...

build-all: build-poseq build-chain

# ── Test ───────────────────────────────────────────────────────────────────────

test-poseq:
	cd $(POSEQ_DIR) && $(CARGO) test

test-poseq-doc:
	cd $(POSEQ_DIR) && $(CARGO) test --doc

test-chain:
	cd $(CHAIN_DIR) && go test ./x/poseq/... -v -count=1 -timeout 120s

test-all: test-poseq test-chain
	@echo "All tests passed."

# ── Run (local devnet) ────────────────────────────────────────────────────────
# Run each node in a separate terminal. Requires `cargo build` first.

POSEQ_BIN := $(POSEQ_DIR)/../.poseq_target/debug/poseq-node

run-node1: build-poseq
	$(POSEQ_BIN) --config $(POSEQ_DIR)/config/node1.toml

run-node2: build-poseq
	$(POSEQ_BIN) --config $(POSEQ_DIR)/config/node2.toml

run-node3: build-poseq
	$(POSEQ_BIN) --config $(POSEQ_DIR)/config/node3.toml

# Convenience: launch all 3 nodes in the background (requires a POSIX shell).
# Each node writes logs to /tmp/poseq-nodeN.log.
run-devnet: build-poseq
	@echo "Starting 3-node devnet (logs → /tmp/poseq-nodeN.log)..."
	$(POSEQ_BIN) --config $(POSEQ_DIR)/config/node1.toml > /tmp/poseq-node1.log 2>&1 &
	$(POSEQ_BIN) --config $(POSEQ_DIR)/config/node2.toml > /tmp/poseq-node2.log 2>&1 &
	$(POSEQ_BIN) --config $(POSEQ_DIR)/config/node3.toml > /tmp/poseq-node3.log 2>&1 &
	@echo "Nodes started. Logs: /tmp/poseq-node{1,2,3}.log"
	@echo "Stop with: pkill poseq-node"

# ── Phase 7B Live Cluster Tests ───────────────────────────────────────────────
# Run the real-process cluster tests (requires binary to be built first).
# These tests spawn real poseq-node OS processes and require a compiled binary.
test-live-cluster: build-poseq
	cd $(POSEQ_DIR) && $(CARGO) test --test test_live_cluster -- --include-ignored

# Final Phase 7B operational proof: all tests including live cluster.
# This is the definitive "operationally ready" verification target.
proof-phase7b: build-poseq build-chain
	@echo "=== Phase 7B Operational Readiness Proof ==="
	@echo "--- Step 1: Rust unit + integration tests ---"
	cd $(POSEQ_DIR) && $(CARGO) test
	@echo "--- Step 2: Go chain tests ---"
	cd $(CHAIN_DIR) && go test ./x/poseq/... -v -count=1 -timeout 120s
	@echo "--- Step 3: Live cluster tests (real processes) ---"
	cd $(POSEQ_DIR) && $(CARGO) test --test test_live_cluster -- --include-ignored
	@echo "=== Phase 7B COMPLETE: All tests passed. ==="

# ── Phase 7C — Devnet Activation ─────────────────────────────────────────────

# Run devnet chaos + failure tests (in-process, no binary required).
test-devnet-chaos:
	cd $(POSEQ_DIR) && $(CARGO) test --test test_devnet_chaos

# Start the 3-node local devnet.
devnet-start: build-poseq
	bash scripts/devnet.sh start

# Stop the running devnet.
devnet-stop:
	bash scripts/devnet.sh stop

# Show current devnet node status from state dump files.
devnet-status:
	bash scripts/devnet.sh status

# Tail devnet logs (Ctrl+C to exit).
devnet-logs:
	bash scripts/devnet.sh logs

# Phase 7C proof: all Rust tests including chaos tests.
proof-phase7c: build-poseq build-chain
	@echo "=== Phase 7C Devnet Activation Proof ==="
	@echo "--- Step 1: Full Rust test suite (including chaos) ---"
	cd $(POSEQ_DIR) && $(CARGO) test
	@echo "--- Step 2: Go chain tests ---"
	cd $(CHAIN_DIR) && go test ./x/poseq/... -v -count=1 -timeout 120s
	@echo "--- Step 3: Live cluster tests ---"
	cd $(POSEQ_DIR) && $(CARGO) test --test test_live_cluster -- --include-ignored
	@echo "=== Phase 7C COMPLETE: All tests passed. ==="

# ── Phase 7D — Testnet Readiness & Network Hardening ─────────────────────────

# Build release binary for testnet (optimized, no debug symbols).
build-poseq-release:
	cd $(POSEQ_DIR) && $(CARGO) build --bin poseq-node --release

# Run network hardening tests (peer lifecycle, config validation, version roundtrip).
test-network-hardening:
	cd $(POSEQ_DIR) && $(CARGO) test --test test_network_hardening

# Start the 5-node local testnet (release build, 3/5 quorum, 5s slots).
testnet-start: build-poseq-release
	bash scripts/testnet.sh start

# Stop the running testnet.
testnet-stop:
	bash scripts/testnet.sh stop

# Show current testnet node status from state dump files.
testnet-status:
	bash scripts/testnet.sh status

# Tail testnet logs (Ctrl+C to exit).
testnet-logs:
	bash scripts/testnet.sh logs

# Validate testnet config (port availability, binary existence) without starting.
testnet-validate:
	bash scripts/testnet.sh validate

# Phase 7D proof: all tests including network hardening + chaos.
proof-phase7d: build-poseq build-chain
	@echo "=== Phase 7D Testnet Readiness Proof ==="
	@echo "--- Step 1: Full Rust test suite ---"
	cd $(POSEQ_DIR) && $(CARGO) test
	@echo "--- Step 2: Network hardening tests ---"
	cd $(POSEQ_DIR) && $(CARGO) test --test test_network_hardening
	@echo "--- Step 3: Devnet chaos tests ---"
	cd $(POSEQ_DIR) && $(CARGO) test --test test_devnet_chaos
	@echo "--- Step 4: Go chain tests ---"
	cd $(CHAIN_DIR) && go test ./x/poseq/... -v -count=1 -timeout 120s
	@echo "--- Step 5: Live cluster tests ---"
	cd $(POSEQ_DIR) && $(CARGO) test --test test_live_cluster -- --include-ignored
	@echo "=== Phase 7D COMPLETE: All tests passed. ==="

# ── Phase 8 — State Sync, Catch-Up, and Long-Run Reliability ─────────────────

# Run state sync + soak tests.
test-state-sync:
	cd $(POSEQ_DIR) && $(CARGO) test --test test_state_sync

# Phase 8 proof: full test suite including state sync and soak tests.
proof-phase8: build-poseq build-chain
	@echo "=== Phase 8 State Sync & Long-Run Reliability Proof ==="
	@echo "--- Step 1: Full Rust test suite (unit + integration) ---"
	cd $(POSEQ_DIR) && $(CARGO) test
	@echo "--- Step 2: State sync and soak tests ---"
	cd $(POSEQ_DIR) && $(CARGO) test --test test_state_sync
	@echo "--- Step 3: Network hardening tests ---"
	cd $(POSEQ_DIR) && $(CARGO) test --test test_network_hardening
	@echo "--- Step 4: Devnet chaos tests ---"
	cd $(POSEQ_DIR) && $(CARGO) test --test test_devnet_chaos
	@echo "--- Step 5: Go chain tests ---"
	cd $(CHAIN_DIR) && go test ./x/poseq/... -v -count=1 -timeout 120s
	@echo "--- Step 6: Live cluster tests ---"
	cd $(POSEQ_DIR) && $(CARGO) test --test test_live_cluster -- --include-ignored
	@echo "=== Phase 8 COMPLETE: All tests passed. ==="
