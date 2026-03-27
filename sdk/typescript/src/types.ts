/**
 * Core type definitions for the Omniphi blockchain SDK.
 *
 * Every type mirrors its Go counterpart in `chain/x/<module>/types/`.
 * Field names use camelCase (JS convention) with the original Go/JSON names
 * noted in doc comments where they differ.
 */

import type { Coin } from "@cosmjs/stargate";

// ============================================================================
// Common
// ============================================================================

/** A standard Cosmos SDK fee object. */
export interface StdFee {
  readonly amount: readonly Coin[];
  readonly gas: string;
}

// ============================================================================
// x/poc — Proof of Contribution
// ============================================================================

/** Contribution types recognized by the PoC module. */
export type ContributionType = "code" | "record" | "relay" | "green" | string;

/** Unified pipeline status across all 5 PoC layers. */
export enum ClaimStatus {
  SUBMITTED = 0,
  ENDORSED = 1,
  VERIFIED = 2,
  REWARDED = 3,
  REJECTED = 4,
}

/** Review lifecycle status. */
export enum ReviewStatus {
  NONE = 0,
  PENDING = 1,
  IN_REVIEW = 2,
  ACCEPTED = 3,
  REJECTED = 4,
  APPEALED = 5,
}

/** A single validator endorsement on a contribution. */
export interface Endorsement {
  /** Validator bech32 address (`omni...`). */
  valAddr: string;
  /** Whether the validator endorsed (true) or rejected (false). */
  decision: boolean;
  /** Validator voting power at time of endorsement. */
  power: string;
}

/** A Proof-of-Contribution record stored on-chain. */
export interface Contribution {
  id: number;
  contributor: string;
  ctype: ContributionType;
  uri: string;
  /** Hex-encoded content hash. */
  hash: string;
  endorsements: Endorsement[];
  verified: boolean;
  blockHeight: number;
  blockTime: number;
  rewarded: boolean;
  /** Hex-encoded canonical hash for deduplication. */
  canonicalHash: string;
  canonicalSpecVersion: number;
  /** Non-zero if this is a duplicate of another contribution. */
  duplicateOf: number;
  isDerivative: boolean;
  reviewStatus: ReviewStatus;
  parentClaimId: number;
  claimStatus: ClaimStatus;
}

/** Message to submit a new contribution. */
export interface MsgSubmitContribution {
  contributor: string;
  ctype: ContributionType;
  uri: string;
  /** Raw bytes or hex string of the content hash. */
  hash: Uint8Array | string;
  canonicalHash?: Uint8Array | string;
  canonicalSpecVersion?: number;
}

/** Message to endorse a contribution. */
export interface MsgEndorse {
  validator: string;
  contributionId: number;
  decision: boolean;
}

/** Message to withdraw PoC rewards. */
export interface MsgWithdrawPOCRewards {
  contributor: string;
}

// ============================================================================
// x/por — Proof of Record
// ============================================================================

export enum BatchStatus {
  SUBMITTED = 0,
  PENDING = 1,
  FINALIZED = 2,
  REJECTED = 3,
}

export enum ChallengeType {
  INVALID_ROOT = 0,
  DOUBLE_INCLUSION = 1,
  MISSING_RECORD = 2,
  INVALID_SCHEMA = 3,
}

export enum ChallengeStatus {
  OPEN = 0,
  RESOLVED_VALID = 1,
  RESOLVED_INVALID = 2,
}

export enum AppStatus {
  ACTIVE = 0,
  SUSPENDED = 1,
  DEREGISTERED = 2,
}

/** A registered application in the PoR module. */
export interface App {
  appId: number;
  name: string;
  owner: string;
  schemaCid: string;
  challengePeriod: number;
  minVerifiers: number;
  status: AppStatus;
  createdAt: number;
}

/** A batch commitment anchored on-chain. */
export interface BatchCommitment {
  batchId: number;
  epoch: number;
  recordMerkleRoot: string;
  recordCount: number;
  appId: number;
  verifierSetId: number;
  submitter: string;
  challengeEndTime: number;
  status: BatchStatus;
  submittedAt: number;
  finalizedAt: number;
  daCommitmentHash?: string;
  poseqCommitmentHash?: string;
}

/** A verifier attestation on a batch. */
export interface Attestation {
  batchId: number;
  verifierAddress: string;
  signature: string;
  confidenceWeight: string;
  timestamp: number;
}

/** A fraud proof challenge against a batch. */
export interface Challenge {
  challengeId: number;
  batchId: number;
  challenger: string;
  challengeType: ChallengeType;
  proofData: string;
  status: ChallengeStatus;
  timestamp: number;
  resolvedAt: number;
  resolvedBy: string;
  bondAmount: string;
}

// ============================================================================
// x/poseq — Proof of Sequencing
// ============================================================================

export type EvidenceKind =
  | "Equivocation"
  | "UnauthorizedProposal"
  | "UnfairSequencing"
  | "ReplayAbuse"
  | "BridgeAbuse"
  | "PersistentAbsence"
  | "StaleAuthority"
  | "InvalidProposalSpam";

export type MisbehaviorSeverity = "Minor" | "Moderate" | "Severe" | "Critical";

/** An evidence packet from the PoSeq sequencer. */
export interface EvidencePacket {
  packetHash: string;
  kind: EvidenceKind;
  offenderNodeId: string;
  epoch: number;
  slot: number;
  severity: MisbehaviorSeverity;
  proposedSlashBps: number;
  evidenceHashes: string[];
  requiresGovernance: boolean;
  recommendSuspension: boolean;
  linkedBatchId?: string;
}

/** Non-binding slash recommendation from PoSeq. */
export interface PenaltyRecommendationRecord {
  nodeId: string;
  epoch: number;
  slashBps: number;
  reason: string;
  packetHash: string;
}

/** Chain-side epoch summary from PoSeq. */
export interface EpochStateReference {
  epoch: number;
  committeeHash: string;
  finalizedBatchCount: number;
  misbehaviorCount: number;
  evidencePacketCount: number;
  governanceEscalations: number;
  epochStateHash: string;
}

/** On-chain anchor for a PoSeq checkpoint. */
export interface CheckpointAnchorRecord {
  checkpointId: string;
  epoch: number;
  slot: number;
  epochStateHash: string;
  bridgeStateHash: string;
  misbehaviorCount: number;
  finalitySummary: BatchFinalityReference;
  anchorHash: string;
}

/** A finalized PoSeq batch reference. */
export interface BatchFinalityReference {
  batchId: string;
  slot: number;
  epoch: number;
  finalizationHash: string;
  submissionCount: number;
  quorumApprovals: number;
  committeeSize: number;
}

/** Committed batch record from the execution layer. */
export interface CommittedBatchRecord {
  batchId: string;
  finalizationHash: string;
  epoch: number;
  slot: number;
  orderedSubmissionIds: string[];
  leaderId: string;
  approvals: number;
  committeeSize: number;
  submitterAddress: string;
}

/** QC signature entry for HotStuff consensus verification. */
export interface QCSignatureEntry {
  nodeId: string;
  signature: string;
}

/** Liveness observation for a PoSeq node within an epoch. */
export interface LivenessEvent {
  nodeId: string;
  epoch: number;
  lastSeenSlot: number;
  wasProposer: boolean;
  wasAttestor: boolean;
}

/** Per-node performance attribution for an epoch. */
export interface NodePerformanceRecord {
  nodeId: string;
  epoch: number;
  proposalsCount: number;
  attestationsCount: number;
  missedAttestations: number;
}

/** Inactivity observation for a PoSeq node. */
export interface InactivityEvent {
  nodeId: string;
  epoch: number;
  missedEpochs: number;
}

/** Lifecycle status recommendation from PoSeq. */
export interface StatusRecommendation {
  nodeId: string;
  recommendedStatus: string;
  reason: string;
  epoch: number;
}

// ============================================================================
// x/guard — Safety Guard
// ============================================================================

export type RiskTier = "LOW" | "MED" | "HIGH" | "CRITICAL";

export type ExecutionGate =
  | "VISIBILITY"
  | "SHOCK_ABSORBER"
  | "CONDITIONAL_EXECUTION"
  | "READY"
  | "EXECUTED";

// ============================================================================
// x/timelock
// ============================================================================

/** Timelock operation lifecycle. */
export enum TimelockStatus {
  QUEUED = 0,
  EXECUTED = 1,
  CANCELLED = 2,
}

// ============================================================================
// x/tokenomics
// ============================================================================

/** Token supply information. */
export interface TokenSupply {
  totalSupply: string;
  circulatingSupply: string;
  bondedTokens: string;
  mintedThisEpoch: string;
  burnedThisEpoch: string;
}

/** Inflation rate information. */
export interface InflationInfo {
  annualRate: string;
  epochRate: string;
  targetBondRatio: string;
  currentBondRatio: string;
}

// ============================================================================
// x/royalty — Royalty Tokens
// ============================================================================

export type TokenStatus = "ACTIVE" | "FROZEN" | "CLAWED_BACK" | "EXPIRED";

/** A tokenized royalty stream backed by a PoC contribution. */
export interface RoyaltyToken {
  tokenId: number;
  claimId: number;
  owner: string;
  originalCreator: string;
  royaltyShare: string;
  status: TokenStatus;
  createdAtHeight: number;
  isFractionalized: boolean;
  fractionCount: number;
  totalPayouts: string;
  metadata: string;
}

// ============================================================================
// x/uci — Universal Contribution Interface
// ============================================================================

export type AdapterStatus = "ACTIVE" | "SUSPENDED" | "DEREGISTERED";

/** A registered DePIN network adapter. */
export interface Adapter {
  adapterId: number;
  name: string;
  owner: string;
  schemaCid: string;
  oracleAllowlist: string[];
  status: AdapterStatus;
  networkType: string;
  createdAtHeight: number;
  totalContributions: number;
  totalRewardsDistributed: string;
  rewardShare: string;
  description: string;
}

/** Maps an external DePIN contribution to a PoC contribution. */
export interface ContributionMapping {
  adapterId: number;
  externalId: string;
  pocContributionId: number;
  contributor: string;
  mappedAtHeight: number;
  rewardAmount: string;
  oracleVerified: boolean;
}

// ============================================================================
// x/contracts
// ============================================================================

export type ContractStatus = "ACTIVE" | "SUSPENDED" | "DEPRECATED";

/** A single intent method supported by a contract schema. */
export interface IntentSchema {
  method: string;
  params: IntentSchemaField[];
  capabilities: string[];
}

export interface IntentSchemaField {
  name: string;
  typeHint: string;
}

/** On-chain registration of a deployed contract. */
export interface ContractSchema {
  schemaId: string;
  deployer: string;
  version: number;
  name: string;
  description: string;
  domainTag: string;
  intentSchemas: IntentSchema[];
  maxGasPerCall: number;
  maxStateBytes: number;
  validatorHash: string;
  wasmSize: number;
  status: ContractStatus;
  deployedAt: number;
}

/** A deployed instance of a contract schema. */
export interface ContractInstance {
  instanceId: number;
  schemaId: string;
  creator: string;
  admin: string;
  label: string;
  createdAt: number;
}

// ============================================================================
// x/repgov — Reputation Governance
// ============================================================================

/** Governance weight for an address, computed from reputation scores. */
export interface VoterWeight {
  address: string;
  epoch: number;
  reputationScore: string;
  cScore: string;
  endorsementRate: string;
  originalityAvg: string;
  uptimeScore: string;
  longevityScore: string;
  compositeWeight: string;
  effectiveWeight: string;
  delegatedWeight: string;
  lastVoteHeight: number;
}

// ============================================================================
// Intent types — cross-module intent-based transaction model
// ============================================================================

/** An intent to transfer tokens between two addresses. */
export interface TransferIntent {
  type: "transfer";
  sender: string;
  recipient: string;
  amount: string;
  denom: string;
  memo?: string;
}

/** An intent to swap one token for another. */
export interface SwapIntent {
  type: "swap";
  sender: string;
  inputDenom: string;
  inputAmount: string;
  outputDenom: string;
  minOutputAmount: string;
  maxSlippageBps?: number;
}

/** An intent to delegate stake to a validator. */
export interface DelegateIntent {
  type: "delegate";
  delegator: string;
  validator: string;
  amount: string;
  denom: string;
}

/** An intent to submit a proof-of-contribution. */
export interface ContributeIntent {
  type: "contribute";
  contributor: string;
  ctype: ContributionType;
  uri: string;
  hash: string;
}

/** An intent to deploy a contract. */
export interface DeployContractIntent {
  type: "deploy_contract";
  deployer: string;
  name: string;
  description: string;
  domainTag: string;
  intentSchemas: IntentSchema[];
  maxGasPerCall: number;
  maxStateBytes: number;
  wasmBytecode: Uint8Array;
}

/**
 * Union of all recognized intent types.
 *
 * The `type` discriminant lets callers use narrowing:
 * ```ts
 * if (intent.type === "transfer") { ... }
 * ```
 */
export type Intent =
  | TransferIntent
  | SwapIntent
  | DelegateIntent
  | ContributeIntent
  | DeployContractIntent;

/**
 * An intent transaction wrapping one or more intents for atomic execution.
 * Submitted to the chain via `MsgSubmitIntent`.
 */
export interface IntentTransaction {
  /** The account submitting the intent bundle. */
  sender: string;
  /** One or more intents to execute atomically. */
  intents: Intent[];
  /** Optional memo attached to the transaction. */
  memo?: string;
  /** Maximum gas the sender is willing to pay. */
  maxGas?: number;
  /** Deadline as a UNIX timestamp (seconds). Zero means no deadline. */
  deadline?: number;
}

// ============================================================================
// Query response wrappers
// ============================================================================

/** Generic paginated response from REST queries. */
export interface PaginatedResponse<T> {
  data: T[];
  pagination: {
    nextKey: string | null;
    total: string;
  };
}

/** Balance query response. */
export interface BalanceResponse {
  balance: Coin;
}

/** Node info from the Tendermint RPC. */
export interface NodeInfo {
  defaultNodeInfo: {
    network: string;
    version: string;
    moniker: string;
  };
  applicationVersion: {
    name: string;
    appName: string;
    version: string;
    buildDeps: string[];
  };
}
