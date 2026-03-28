/**
 * Omniphi chain constants.
 *
 * These values correspond to the chain configuration used by the Omniphi
 * Cosmos SDK node (`posd`). Bech32 prefix, coin type, and denomination are
 * compiled into the binary and cannot change without a coordinated upgrade.
 */

// ---------------------------------------------------------------------------
// Address / key derivation
// ---------------------------------------------------------------------------

/** Bech32 human-readable prefix for account addresses. */
export const BECH32_PREFIX = "omni";

/** Bech32 prefix for validator operator addresses. */
export const BECH32_PREFIX_VALOPER = "omnivaloper";

/** Bech32 prefix for consensus node addresses. */
export const BECH32_PREFIX_CONS = "omnivalcons";

/**
 * SLIP-44 coin type.
 * Omniphi uses coin type 60 (same as Ethereum) for HD key derivation.
 */
export const COIN_TYPE = 60;

/** Default BIP-44 HD derivation path. */
export const HD_PATH = "m/44'/60'/0'/0/0";

// ---------------------------------------------------------------------------
// Native denomination
// ---------------------------------------------------------------------------

/** The base denomination of the Omniphi chain. */
export const DENOM = "omniphi";

/** Display denomination (same as base for now; no exponent split). */
export const DISPLAY_DENOM = "OMNI";

// ---------------------------------------------------------------------------
// Default endpoints
// ---------------------------------------------------------------------------

/** Default Tendermint RPC endpoint. */
export const DEFAULT_RPC_ENDPOINT = "http://localhost:26657";

/** Default REST (LCD / gRPC-gateway) endpoint. */
export const DEFAULT_REST_ENDPOINT = "http://localhost:1318";

/** Default gRPC endpoint. */
export const DEFAULT_GRPC_ENDPOINT = "localhost:9090";

// ---------------------------------------------------------------------------
// Default fee
// ---------------------------------------------------------------------------

/** Reasonable default gas limit for simple transactions. */
export const DEFAULT_GAS_LIMIT = 200_000;

/** Default gas price string (used when building StdFee). */
export const DEFAULT_GAS_PRICE = "0.025omniphi";

/**
 * Pre-built default fee suitable for most transactions.
 * 200 000 gas * 0.025 = 5 000 omniphi.
 */
export const DEFAULT_FEE: { readonly amount: readonly { readonly denom: string; readonly amount: string }[]; readonly gas: string } = {
  amount: [{ denom: DENOM, amount: "5000" }],
  gas: String(DEFAULT_GAS_LIMIT),
};

// ---------------------------------------------------------------------------
// Module names (match Go `ModuleName` constants)
// ---------------------------------------------------------------------------

export const MODULE_NAMES = {
  POC: "poc",
  POR: "por",
  POSEQ: "poseq",
  TOKENOMICS: "tokenomics",
  FEEMARKET: "feemarket",
  GUARD: "guard",
  TIMELOCK: "timelock",
  REWARDMULT: "rewardmult",
  REPGOV: "repgov",
  ROYALTY: "royalty",
  UCI: "uci",
  CONTRACTS: "contracts",
} as const;

// ---------------------------------------------------------------------------
// Proto type URLs for custom messages
// ---------------------------------------------------------------------------

/**
 * Cosmos SDK message type URLs.
 *
 * These must match the proto package path registered by each module's
 * `RegisterInterfaces`. The format is `/<proto_package>.<MsgName>`.
 */
export const MSG_TYPE_URLS = {
  // x/poc
  SUBMIT_CONTRIBUTION: "/pos.poc.v1.MsgSubmitContribution",
  ENDORSE: "/pos.poc.v1.MsgEndorse",
  WITHDRAW_POC_REWARDS: "/pos.poc.v1.MsgWithdrawPOCRewards",
  SUBMIT_SIMILARITY_COMMITMENT: "/pos.poc.v1.MsgSubmitSimilarityCommitment",
  START_REVIEW: "/pos.poc.v1.MsgStartReview",
  CAST_REVIEW_VOTE: "/pos.poc.v1.MsgCastReviewVote",
  FINALIZE_REVIEW: "/pos.poc.v1.MsgFinalizeReview",
  APPEAL_REVIEW: "/pos.poc.v1.MsgAppealReview",
  RESOLVE_APPEAL: "/pos.poc.v1.MsgResolveAppeal",

  // x/por
  REGISTER_APP: "/pos.por.v1.MsgRegisterApp",
  CREATE_VERIFIER_SET: "/pos.por.v1.MsgCreateVerifierSet",
  SUBMIT_BATCH: "/pos.por.v1.MsgSubmitBatch",
  SUBMIT_ATTESTATION: "/pos.por.v1.MsgSubmitAttestation",
  CHALLENGE_BATCH: "/pos.por.v1.MsgChallengeBatch",

  // x/poseq
  SUBMIT_EXPORT_BATCH: "/pos.poseq.v1.MsgSubmitExportBatch",
  SUBMIT_EVIDENCE_PACKET: "/pos.poseq.v1.MsgSubmitEvidencePacket",
  SUBMIT_CHECKPOINT_ANCHOR: "/pos.poseq.v1.MsgSubmitCheckpointAnchor",
  COMMIT_EXECUTION: "/pos.poseq.v1.MsgCommitExecution",
  EXECUTE_SLASH: "/pos.poseq.v1.MsgExecuteSlash",

  // x/guard
  CONFIRM_EXECUTION: "/pos.guard.v1.MsgConfirmExecution",
  SUBMIT_ADVISORY_LINK: "/pos.guard.v1.MsgSubmitAdvisoryLink",

  // x/timelock
  EXECUTE_OPERATION: "/pos.timelock.v1.MsgExecuteOperation",
  CANCEL_OPERATION: "/pos.timelock.v1.MsgCancelOperation",
  EMERGENCY_EXECUTE: "/pos.timelock.v1.MsgEmergencyExecute",

  // x/tokenomics
  MINT_TOKENS: "/pos.tokenomics.v1.MsgMintTokens",
  BURN_TOKENS: "/pos.tokenomics.v1.MsgBurnTokens",

  // x/royalty
  TOKENIZE_ROYALTY: "/pos.royalty.v1.MsgTokenizeRoyalty",
  TRANSFER_TOKEN: "/pos.royalty.v1.MsgTransferToken",
  CLAIM_ROYALTIES: "/pos.royalty.v1.MsgClaimRoyalties",
  FRACTIONALIZE_TOKEN: "/pos.royalty.v1.MsgFractionalizeToken",
  LIST_TOKEN: "/pos.royalty.v1.MsgListToken",
  BUY_TOKEN: "/pos.royalty.v1.MsgBuyToken",
  DELIST_TOKEN: "/pos.royalty.v1.MsgDelistToken",

  // x/uci
  REGISTER_ADAPTER: "/pos.uci.v1.MsgRegisterAdapter",
  SUSPEND_ADAPTER: "/pos.uci.v1.MsgSuspendAdapter",
  SUBMIT_DEPIN_CONTRIBUTION: "/pos.uci.v1.MsgSubmitDePINContribution",
  SUBMIT_ORACLE_ATTESTATION: "/pos.uci.v1.MsgSubmitOracleAttestation",

  // x/contracts
  DEPLOY_CONTRACT: "/pos.contracts.v1.MsgDeployContract",
  INSTANTIATE_CONTRACT: "/pos.contracts.v1.MsgInstantiateContract",

  // x/repgov
  DELEGATE_REPUTATION: "/pos.repgov.v1.MsgDelegateReputation",
  UNDELEGATE_REPUTATION: "/pos.repgov.v1.MsgUndelegateReputation",
} as const;

// ---------------------------------------------------------------------------
// REST query paths
// ---------------------------------------------------------------------------

/**
 * REST API paths for querying module state via the gRPC-gateway (port 1318).
 * Append query parameters as needed (e.g. `?contributor_id=1`).
 */
export const REST_PATHS = {
  // Standard Cosmos
  BANK_BALANCES: (address: string) => `/cosmos/bank/v1beta1/balances/${address}`,
  STAKING_VALIDATORS: "/cosmos/staking/v1beta1/validators",
  STAKING_DELEGATIONS: (delegator: string) =>
    `/cosmos/staking/v1beta1/delegations/${delegator}`,
  NODE_INFO: "/cosmos/base/tendermint/v1beta1/node_info",
  LATEST_BLOCK: "/cosmos/base/tendermint/v1beta1/blocks/latest",

  // x/poc
  POC_PARAMS: "/pos/poc/v1/params",
  POC_CONTRIBUTION: (id: string | number) => `/pos/poc/v1/contribution/${id}`,
  POC_CONTRIBUTIONS: "/pos/poc/v1/contributions",

  // x/por
  POR_PARAMS: "/pos/por/v1/params",
  POR_APP: (id: string | number) => `/pos/por/v1/app/${id}`,
  POR_BATCH: (id: string | number) => `/pos/por/v1/batch/${id}`,

  // x/poseq
  POSEQ_PARAMS: "/pos/poseq/v1/params",
  POSEQ_CHECKPOINT: (epoch: string | number) => `/pos/poseq/v1/checkpoint/${epoch}`,
  POSEQ_EPOCH_STATE: (epoch: string | number) => `/pos/poseq/v1/epoch_state/${epoch}`,

  // x/guard
  GUARD_PARAMS: "/pos/guard/v1/params",
  GUARD_RISK_REPORT: "/pos/guard/v1/risk_report",

  // x/tokenomics
  TOKENOMICS_PARAMS: "/pos/tokenomics/v1/params",
  TOKENOMICS_SUPPLY: "/pos/tokenomics/v1/supply",
  TOKENOMICS_INFLATION: "/pos/tokenomics/v1/inflation",

  // x/rewardmult
  REWARDMULT_PARAMS: "/pos/rewardmult/v1/params",

  // x/repgov
  REPGOV_PARAMS: "/pos/repgov/v1/params",
  REPGOV_VOTER_WEIGHT: (address: string) => `/pos/repgov/v1/voter_weight/${address}`,

  // x/royalty
  ROYALTY_PARAMS: "/pos/royalty/v1/params",
  ROYALTY_TOKEN: (id: string | number) => `/pos/royalty/v1/token/${id}`,

  // x/uci
  UCI_PARAMS: "/pos/uci/v1/params",
  UCI_ADAPTER: (id: string | number) => `/pos/uci/v1/adapter/${id}`,
} as const;
