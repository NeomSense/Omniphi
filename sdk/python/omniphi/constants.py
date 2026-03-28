"""Chain constants and well-known paths for the Omniphi blockchain."""

from __future__ import annotations

# ── Address encoding ──────────────────────────────────────────────────
BECH32_PREFIX: str = "omni"
BECH32_VAL_PREFIX: str = "omnivaloper"

# ── Token ─────────────────────────────────────────────────────────────
DENOM: str = "omniphi"

# ── HD key derivation (EIP-155 compatible coin type 60) ───────────────
COIN_TYPE: int = 60
HD_PATH: str = "m/44'/60'/0'/0/0"

# ── Default transaction parameters ────────────────────────────────────
DEFAULT_GAS: int = 200_000
DEFAULT_FEE: int = 5_000  # in smallest denomination
DEFAULT_FEE_DENOM: str = DENOM
DEFAULT_MEMO: str = ""

# ── Network defaults ──────────────────────────────────────────────────
DEFAULT_RPC_URL: str = "http://localhost:26657"
DEFAULT_REST_URL: str = "http://localhost:1317"

# ── Custom module names ──────────────────────────────────────────────
MODULE_NAMES: tuple[str, ...] = (
    "poc",
    "por",
    "poseq",
    "tokenomics",
    "feemarket",
    "guard",
    "timelock",
    "rewardmult",
    "repgov",
    "royalty",
    "uci",
    "contracts",
)

# ── Cosmos SDK message type URLs ─────────────────────────────────────
MSG_TYPE_URLS: dict[str, str] = {
    # Bank
    "send": "/cosmos.bank.v1beta1.MsgSend",
    # Staking
    "delegate": "/cosmos.staking.v1beta1.MsgDelegate",
    "undelegate": "/cosmos.staking.v1beta1.MsgUndelegate",
    "redelegate": "/cosmos.staking.v1beta1.MsgBeginRedelegate",
    # Governance
    "submit_proposal": "/cosmos.gov.v1.MsgSubmitProposal",
    "vote": "/cosmos.gov.v1.MsgVote",
    # PoC (Proof of Contribution)
    "submit_contribution": "/pos.poc.v1.MsgSubmitContribution",
    "endorse": "/pos.poc.v1.MsgEndorse",
    "withdraw_poc_rewards": "/pos.poc.v1.MsgWithdrawPOCRewards",
    "poc_update_params": "/pos.poc.v1.MsgUpdateParams",
    # Guard
    "guard_update_params": "/pos.guard.v1.MsgUpdateParams",
    "confirm_execution": "/pos.guard.v1.MsgConfirmExecution",
    "update_ai_model": "/pos.guard.v1.MsgUpdateAIModel",
    "submit_advisory_link": "/pos.guard.v1.MsgSubmitAdvisoryLink",
}

# ── REST API query paths ─────────────────────────────────────────────
REST_PATHS: dict[str, str] = {
    # Cosmos SDK standard
    "balances": "/cosmos/bank/v1beta1/balances/{address}",
    "balance": "/cosmos/bank/v1beta1/balances/{address}/by_denom",
    "supply": "/cosmos/bank/v1beta1/supply",
    "validators": "/cosmos/staking/v1beta1/validators",
    "node_info": "/cosmos/base/tendermint/v1beta1/node_info",
    "latest_block": "/cosmos/base/tendermint/v1beta1/blocks/latest",
    "account": "/cosmos/auth/v1beta1/accounts/{address}",
    "tx_broadcast": "/cosmos/tx/v1beta1/txs",
    "simulate": "/cosmos/tx/v1beta1/simulate",
    # PoC module
    "poc_params": "/pos/poc/v1/params",
    "poc_contribution": "/pos/poc/v1/contribution/{id}",
    "poc_contributions": "/pos/poc/v1/contributions",
    "poc_credits": "/pos/poc/v1/credits/{address}",
    "poc_fee_metrics": "/pos/poc/v1/fee_metrics",
    "poc_contributor_fee_stats": "/pos/poc/v1/contributor_fee_stats/{address}",
    # Tokenomics module
    "tokenomics_params": "/pos/tokenomics/v1/params",
    "tokenomics_supply": "/pos/tokenomics/v1/supply",
    "tokenomics_inflation": "/pos/tokenomics/v1/inflation",
    "tokenomics_emissions": "/pos/tokenomics/v1/emissions",
    "tokenomics_burns": "/pos/tokenomics/v1/burns",
    "tokenomics_treasury": "/pos/tokenomics/v1/treasury",
    "tokenomics_projections": "/pos/tokenomics/v1/projections",
    "tokenomics_fee_stats": "/pos/tokenomics/v1/fees/stats",
    "tokenomics_burn_rate": "/pos/tokenomics/v1/burn-rate",
    # Feemarket module
    "feemarket_params": "/pos/feemarket/v1/params",
    "feemarket_base_fee": "/pos/feemarket/v1/base_fee",
    "feemarket_utilization": "/pos/feemarket/v1/block_utilization",
    "feemarket_burn_tier": "/pos/feemarket/v1/burn_tier",
    "feemarket_fee_stats": "/pos/feemarket/v1/fee_stats",
    # Guard module
    "guard_params": "/omniphi/guard/v1/params",
    "guard_risk_report": "/omniphi/guard/v1/risk_report/{proposal_id}",
    "guard_queued": "/omniphi/guard/v1/queued/{proposal_id}",
    "guard_advisory": "/omniphi/guard/v1/advisory/{proposal_id}",
    # Timelock module
    "timelock_params": "/pos/timelock/v1/params",
    "timelock_operation": "/pos/timelock/v1/operation/{operation_id}",
    "timelock_operations": "/pos/timelock/v1/operations",
    "timelock_queued": "/pos/timelock/v1/queued",
    "timelock_executable": "/pos/timelock/v1/executable",
}

# ── Module param REST path mapping ───────────────────────────────────
MODULE_PARAM_PATHS: dict[str, str] = {
    "poc": "/pos/poc/v1/params",
    "tokenomics": "/pos/tokenomics/v1/params",
    "feemarket": "/pos/feemarket/v1/params",
    "guard": "/omniphi/guard/v1/params",
    "timelock": "/pos/timelock/v1/params",
    # Standard Cosmos SDK modules
    "staking": "/cosmos/staking/v1beta1/params",
    "gov": "/cosmos/gov/v1/params/tallying",
    "distribution": "/cosmos/distribution/v1beta1/params",
    "slashing": "/cosmos/slashing/v1beta1/params",
    "mint": "/cosmos/mint/v1beta1/params",
}
