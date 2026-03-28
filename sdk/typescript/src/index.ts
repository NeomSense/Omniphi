/**
 * @omniphi/sdk — TypeScript SDK for the Omniphi blockchain.
 *
 * @example
 * ```ts
 * import { OmniphiClient, createWallet, DENOM, DEFAULT_FEE } from "@omniphi/sdk";
 *
 * // Create a wallet
 * const wallet = await createWallet();
 * console.log("Address:", wallet.address);
 *
 * // Connect with signer
 * const client = await OmniphiClient.connectWithSigner(
 *   "http://localhost:26657",
 *   wallet.signer,
 * );
 *
 * // Query balance
 * const balance = await client.getBalance(wallet.address);
 * console.log("Balance:", balance);
 *
 * // Send tokens
 * await client.sendTokens(wallet.address, "omni1recipient...", "1000000");
 * ```
 *
 * @packageDocumentation
 */

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

export { OmniphiClient, OmniphiQueryError, OmniphiTxError } from "./client";
export type { OmniphiClientOptions } from "./client";

// ---------------------------------------------------------------------------
// Wallet
// ---------------------------------------------------------------------------

export {
  createWallet,
  fromMnemonic,
  fromPrivateKey,
  getAddress,
  createAminoSigner,
  isValidAddress,
  isValidMnemonicLength,
} from "./wallet";
export type { WalletInfo, WalletOptions } from "./wallet";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type {
  // Common
  StdFee,

  // x/poc
  ContributionType,
  Endorsement,
  Contribution,
  MsgSubmitContribution,
  MsgEndorse,
  MsgWithdrawPOCRewards,

  // x/por
  App,
  BatchCommitment,
  Attestation,
  Challenge,

  // x/poseq
  EvidencePacket,
  PenaltyRecommendationRecord,
  EpochStateReference,
  CheckpointAnchorRecord,
  BatchFinalityReference,
  CommittedBatchRecord,
  QCSignatureEntry,
  LivenessEvent,
  NodePerformanceRecord,
  InactivityEvent,
  StatusRecommendation,

  // x/tokenomics
  TokenSupply,
  InflationInfo,

  // x/royalty
  RoyaltyToken,

  // x/uci
  Adapter,
  ContributionMapping,

  // x/contracts
  ContractSchema,
  ContractInstance,
  IntentSchema,
  IntentSchemaField,

  // x/repgov
  VoterWeight,

  // Intents
  TransferIntent,
  SwapIntent,
  DelegateIntent,
  ContributeIntent,
  DeployContractIntent,
  Intent,
  IntentTransaction,

  // Query helpers
  PaginatedResponse,
  BalanceResponse,
  NodeInfo,
} from "./types";

export {
  ClaimStatus,
  ReviewStatus,
  BatchStatus,
  ChallengeType,
  ChallengeStatus,
  AppStatus,
  TimelockStatus,
} from "./types";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

export {
  BECH32_PREFIX,
  BECH32_PREFIX_VALOPER,
  BECH32_PREFIX_CONS,
  COIN_TYPE,
  HD_PATH,
  DENOM,
  DISPLAY_DENOM,
  DEFAULT_RPC_ENDPOINT,
  DEFAULT_REST_ENDPOINT,
  DEFAULT_GRPC_ENDPOINT,
  DEFAULT_GAS_LIMIT,
  DEFAULT_GAS_PRICE,
  DEFAULT_FEE,
  MODULE_NAMES,
  MSG_TYPE_URLS,
  REST_PATHS,
} from "./constants";

// ---------------------------------------------------------------------------
// Subscriptions
// ---------------------------------------------------------------------------

export { OmniphiSubscriber } from "./subscriptions";
export type {
  NewBlockEvent,
  NewTxEvent,
  EventData,
  NewBlockCallback,
  NewTxCallback,
  EventCallback,
  ReconnectOptions,
  IWebSocket,
  WebSocketFactory,
} from "./subscriptions";

// ---------------------------------------------------------------------------
// Encoding
// ---------------------------------------------------------------------------

export {
  createOmniphiRegistry,
  createOmniphiAminoTypes,
  encodeMsg,
  encodeAminoMsg,
  hexToBytes,
  bytesToHex,
  normalizeHash,
} from "./encoding";
