package cmd

import (
	"time"

	cmtcfg "github.com/cometbft/cometbft/config"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
)

// ===============================================================================
// OMNIPHI ANCHOR LANE CONFIGURATION
// ===============================================================================
// Target: ~4 second block time for PoS + PoC anchor chain
// Design: Conservative, decentralization-friendly parameters
// NOT optimized for: sub-second blocks or high-end-only validators
//
// The anchor lane prioritizes security and stability over raw throughput.
// High-speed execution is handled by PoSeq (sequencer layer).
// ===============================================================================

// initCometBFTConfig configures CometBFT for Omniphi's anchor lane.
// Targets ~4 second block time with conservative timeouts that tolerate
// lower-end validators and geographic distribution.
func initCometBFTConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()

	// =========================================================================
	// CONSENSUS TIMEOUTS (Anchor Lane - 4s Target Block Time)
	// =========================================================================
	// These values are tuned for decentralization, not speed.
	// They must tolerate validators with varying hardware and network conditions.

	// timeout_propose: Time to wait for a proposal from the block proposer
	// Conservative: 2.5s allows slow proposers without excessive delays
	// Rationale: If proposer is slow, we don't want to skip to next round too fast
	cfg.Consensus.TimeoutPropose = 2500 * time.Millisecond

	// timeout_propose_delta: Additional time per round for proposal timeout
	// 500ms per round escalation balances patience with liveness
	cfg.Consensus.TimeoutProposeDelta = 500 * time.Millisecond

	// timeout_prevote: Time to wait for 2/3+ prevotes before timing out
	// 1.5s is generous - allows geographically distributed validators
	// Lower values can cause unnecessary round skips with high latency peers
	cfg.Consensus.TimeoutPrevote = 1500 * time.Millisecond

	// timeout_prevote_delta: Escalation per round for prevote collection
	cfg.Consensus.TimeoutPrevoteDelta = 500 * time.Millisecond

	// timeout_precommit: Time to wait for 2/3+ precommits
	// Matches prevote - both voting phases need similar tolerance
	cfg.Consensus.TimeoutPrecommit = 1500 * time.Millisecond

	// timeout_precommit_delta: Escalation per round for precommit collection
	cfg.Consensus.TimeoutPrecommitDelta = 500 * time.Millisecond

	// timeout_commit: CRITICAL - Primary driver of block time
	// After consensus, wait before starting the next height
	// 2s commit + ~2s consensus rounds = ~4s effective block time
	//
	// IMPORTANT: This is intentionally conservative. The anchor lane
	// does NOT need sub-second latency. PoSeq handles that.
	cfg.Consensus.TimeoutCommit = 2000 * time.Millisecond

	// =========================================================================
	// BLOCK SIZE & TRANSACTION LIMITS
	// =========================================================================
	// Note: max_bytes (10MB) and max_gas (60M) are set in genesis.json,
	// not in CometBFT config. See TESTNET_GENESIS_TEMPLATE.json.

	// create_empty_blocks: Allow empty blocks to maintain steady block time
	// Essential for predictable finality timing
	cfg.Consensus.CreateEmptyBlocks = true

	// create_empty_blocks_interval: If true, how often to create empty blocks
	// Default 0 means every block interval - keeps timing predictable
	cfg.Consensus.CreateEmptyBlocksInterval = 0 * time.Second

	// =========================================================================
	// MEMPOOL CONFIGURATION (Anchor Lane)
	// =========================================================================
	// Tuned for bursty anchor activity (governance votes, PoC submissions)

	// mempool size: Number of transactions to hold
	// 5000 handles PoC and governance spikes without excessive memory
	cfg.Mempool.Size = 5000

	// max_txs_bytes: Total size of transactions in mempool (256MB)
	// Generous to handle PoC data submissions
	cfg.Mempool.MaxTxsBytes = 256 * 1024 * 1024

	// max_tx_bytes: Maximum size per transaction (1MB)
	// PoC contributions may include moderate data payloads
	cfg.Mempool.MaxTxBytes = 1024 * 1024

	// cache_size: Rejected transaction cache
	// Helps prevent reprocessing invalid transactions
	cfg.Mempool.CacheSize = 10000

	// broadcast: Gossip transactions to peers
	cfg.Mempool.Broadcast = true

	// =========================================================================
	// P2P CONFIGURATION
	// =========================================================================
	// Conservative peer limits to support decentralized validator set
	// Not tuned for maximum throughput - stability over speed

	// Inbound/outbound peer limits: Conservative for stability
	// Lower-end validators should be able to maintain these connections
	cfg.P2P.MaxNumInboundPeers = 40
	cfg.P2P.MaxNumOutboundPeers = 10

	// handshake_timeout: Time to complete peer handshake
	// Generous to tolerate high-latency peers
	cfg.P2P.HandshakeTimeout = 20 * time.Second

	// dial_timeout: Time to establish connection
	cfg.P2P.DialTimeout = 5 * time.Second

	// flush_throttle_timeout: Throttle for message flushing
	// 100ms balances latency with batching efficiency
	cfg.P2P.FlushThrottleTimeout = 100 * time.Millisecond

	// =========================================================================
	// RPC CONFIGURATION
	// =========================================================================

	// Broadcast timeout: Allow generous time for anchor transactions
	// Governance and PoC submissions may take longer to propagate
	cfg.RPC.TimeoutBroadcastTxCommit = 15 * time.Second

	// Max open connections: Moderate - anchor lane isn't high-RPC-throughput
	cfg.RPC.MaxOpenConnections = 450

	return cfg
}

// initAppConfig configures Omniphi application-level settings.
// Sets conservative defaults suitable for the anchor lane.
func initAppConfig() (string, interface{}) {
	// =========================================================================
	// ANCHOR LANE APPLICATION CONFIGURATION
	// =========================================================================

	type CustomAppConfig struct {
		serverconfig.Config `mapstructure:",squash"`
	}

	srvCfg := serverconfig.DefaultConfig()

	// =========================================================================
	// MINIMUM GAS PRICES (Anchor Lane Anti-Spam)
	// =========================================================================
	// Set a reasonable floor to prevent spam while keeping anchor transactions
	// affordable. The fee market module handles dynamic pricing above this floor.
	//
	// 0.025 omniphi = 25,000 uomniphi per gas unit
	// For a 150,000 gas transaction: 0.025 * 150,000 = 3,750 omniphi minimum
	//
	// This is LOW enough for legitimate anchor traffic (staking, governance, PoC)
	// but provides a baseline spam deterrent.
	srvCfg.MinGasPrices = "0.025omniphi"

	// =========================================================================
	// PRUNING CONFIGURATION
	// =========================================================================
	// Default pruning for anchor lane - balance storage with query capability
	// Validators can override for archive nodes
	srvCfg.Pruning = "default"

	// =========================================================================
	// STATE SYNC SNAPSHOTS
	// =========================================================================
	// Enable snapshots for faster new validator onboarding
	// Critical for decentralization - makes it easier to join the network
	srvCfg.StateSync.SnapshotInterval = 1000   // Every ~4000 seconds (1.1 hours)
	srvCfg.StateSync.SnapshotKeepRecent = 2    // Keep 2 recent snapshots

	// =========================================================================
	// API CONFIGURATION
	// =========================================================================
	// Enable API for ecosystem tooling (explorers, wallets, PoC clients)
	srvCfg.API.Enable = true
	srvCfg.API.Swagger = false                  // Disable swagger in production
	srvCfg.API.MaxOpenConnections = 1000        // Generous for ecosystem tools
	srvCfg.API.RPCReadTimeout = 15              // 15 second read timeout
	srvCfg.API.RPCWriteTimeout = 15             // 15 second write timeout
	srvCfg.API.RPCMaxBodyBytes = 2 * 1024 * 1024 // 2MB max body (PoC data)

	// =========================================================================
	// gRPC CONFIGURATION
	// =========================================================================
	srvCfg.GRPC.Enable = true
	srvCfg.GRPC.MaxRecvMsgSize = 10 * 1024 * 1024  // 10MB max message (PoC)
	srvCfg.GRPC.MaxSendMsgSize = 10 * 1024 * 1024  // 10MB max message

	customAppConfig := CustomAppConfig{
		Config: *srvCfg,
	}

	// Custom template with anchor lane documentation
	customAppTemplate := serverconfig.DefaultConfigTemplate + `
###############################################################################
###                    OMNIPHI ANCHOR LANE NOTES                            ###
###############################################################################
#
# This node operates on the Omniphi anchor lane (PoS + PoC).
#
# Target specifications:
#   - Block time: ~4 seconds
#   - Sustained TPS: 50-150 (target: 100)
#   - Primary traffic: staking, governance, PoC submissions
#
# The anchor lane is NOT optimized for high-speed smart contracts.
# PoSeq handles high-throughput execution with periodic commits here.
#
# Gas parameters (set at genesis/consensus level):
#   - Max gas per block: 60,000,000
#   - Max gas per tx: 5,000,000
#   - Target block utilization: 33%
#
# For validators:
#   - Ensure minimum-gas-prices is set (default: 0.025omniphi)
#   - Monitor block execution time (should be < 2.5s typically)
#   - Watch for sustained >70% gas utilization (may indicate issues)
#
###############################################################################
`

	return customAppTemplate, customAppConfig
}
