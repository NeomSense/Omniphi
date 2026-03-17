# Phase 4: Protocol Parameter Guide

## Configuration

All parameters are defined in `poseq/src/intent_pool/constants.rs` with validation in `poseq/src/intent_pool/config.rs`.

Use `IntentProtocolConfig::default_config()` for mainnet defaults or `IntentProtocolConfig::devnet_config()` for testing.

## Timing Parameters

| Parameter | Default | Devnet | Range | Description |
|-----------|---------|--------|-------|-------------|
| `commit_phase_blocks` | 5 | 2 | ≥1 | Duration of commit phase |
| `reveal_phase_blocks` | 3 | 2 | ≥1 | Duration of reveal phase |
| `min_intent_lifetime` | 10 | 3 | ≥1 | Minimum blocks intent must live |
| `max_pool_residence` | 1000 | 1000 | ≥min_intent_lifetime | Max blocks in pool |
| `fast_dispute_window` | 100 | 10 | ≥1 | Fast dispute window |
| `extended_dispute_window` | 50400 | 100 | >fast_dispute_window | Extended dispute window |
| `unbonding_period_blocks` | 50400 | 50 | ≥1 | Bond unbonding period |
| `expiry_check_interval` | 5 | 5 | ≥1 | Blocks between expiry checks |

## Economic Parameters

| Parameter | Default | Devnet | Range | Description |
|-----------|---------|--------|-------|-------------|
| `min_intent_fee_bps` | 10 | 10 | 0-10000 | Min fee in basis points |
| `min_solver_bond` | 10000 | 100 | ≥1 | Min bond to register |
| `active_solver_bond` | 50000 | 500 | ≥min_solver_bond | Min bond for auctions |
| `fast_dispute_bond` | 1000 | 10 | ≥1 | Bond for fast disputes |
| `commit_without_reveal_penalty_bps` | 100 | 100 | 0-10000 | No-reveal penalty |

## Limit Parameters

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `max_intents_per_block_per_user` | 10 | ≥1 | Rate limit per user |
| `max_nonce_gap` | 3 | ≥1 | Max nonce gap allowed |
| `max_intent_size` | 4096 | ≥1 | Max intent size (bytes) |
| `max_pool_size` | 50000 | ≥1 | Max intents in pool |
| `max_commitments_per_solver_per_window` | 10 | ≥1 | Max commits per solver |
| `max_bundle_steps` | 64 | ≥1 | Max execution steps |

## Reputation Parameters

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `max_violation_score` | 9500 | 0-10000 | Auto-deactivation threshold |
| `performance_score_init` | 5000 | 0-10000 | Initial solver performance |
| `da_failure_threshold` | 3 | ≥1 | DA failures before epoch transition |

## Validation

Call `config.validate()` at startup. It returns `Err(Vec<ConfigValidationError>)` if any parameter is out of safe range.
