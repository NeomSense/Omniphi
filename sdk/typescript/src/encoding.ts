/**
 * Amino and Protobuf encoding helpers for Omniphi custom message types.
 *
 * The Omniphi chain registers its custom messages with both Amino (legacy)
 * and Protobuf (modern) codecs. This module provides the type registry
 * entries needed by `@cosmjs/stargate` to encode and decode these messages.
 */

import { type GeneratedType, Registry } from "@cosmjs/proto-signing";
import { AminoTypes } from "@cosmjs/stargate";
import type { AminoMsg } from "@cosmjs/amino";
import type { EncodeObject } from "@cosmjs/proto-signing";
import { MSG_TYPE_URLS } from "./constants";

// ---------------------------------------------------------------------------
// Protobuf Registry
// ---------------------------------------------------------------------------

/**
 * Creates a GeneratedType shim that JSON-encodes/decodes message values.
 *
 * CosmJS's `Registry` expects a `GeneratedType` with `encode`, `decode`, and
 * `fromPartial`. For Omniphi's custom messages (which lack generated protobuf
 * code on the JS side), we use JSON serialization as the wire format. This
 * works with SIGN_MODE_LEGACY_AMINO_JSON and with the chain's gRPC-gateway
 * REST interface.
 */
function createJsonPassthroughType(): GeneratedType {
  // GeneratedType's exact shape varies across CosmJS versions, so we build
  // the minimum viable object and cast it. The three methods are the ones
  // that the Registry actually calls at runtime.
  const shim = {
    encode: (message: unknown) =>
      new TextEncoder().encode(JSON.stringify(message)),
    decode: (input: Uint8Array) =>
      JSON.parse(new TextDecoder().decode(input)) as Record<string, unknown>,
    fromPartial: (obj: Record<string, unknown>) => ({ ...obj }),
  };
  return shim as unknown as GeneratedType;
}

/**
 * Returns a `Registry` populated with all Omniphi custom message types.
 *
 * Because we are using generic JSON-encoded messages (the chain's modules
 * use manual Go types, not full protoc-generated types), we register each
 * type URL with a passthrough encoder that serializes the value object
 * directly. The Cosmos SDK's `SigningStargateClient` uses this registry
 * to build the `TxBody.messages` array.
 *
 * For modules that DO have real .proto definitions (poc, guard, timelock,
 * tokenomics, feemarket), the registry entries still work because the
 * chain accepts JSON-encoded messages via the REST API and amino-JSON
 * signing mode.
 */
export function createOmniphiRegistry(): Registry {
  const registry = new Registry();

  // Register each custom message type URL.
  // We use a generic encoder: the message value is a plain JS object that
  // will be JSON-serialized by the Amino signer or passed through to the
  // protobuf encoder as `google.protobuf.Any`.
  const typeUrls = Object.values(MSG_TYPE_URLS);
  for (const typeUrl of typeUrls) {
    // Register a JSON-passthrough GeneratedType for each custom message.
    // This encodes message values as JSON bytes, which is compatible with the
    // chain's manual (non-protoc) type registration. The Amino signing path
    // uses the AminoTypes converter below instead of this encoder.
    registry.register(typeUrl, createJsonPassthroughType());
  }

  return registry;
}

// ---------------------------------------------------------------------------
// Amino type mappings
// ---------------------------------------------------------------------------

/**
 * Amino type name mappings for Omniphi custom messages.
 *
 * These match the strings passed to `legacy.RegisterAminoMsg` in each
 * module's `codec.go`. Format: `"pos/<module>/<MsgName>"`.
 */
const AMINO_TYPE_MAP: Record<string, string> = {
  // x/poc
  [MSG_TYPE_URLS.SUBMIT_CONTRIBUTION]: "pos/poc/SubmitContribution",
  [MSG_TYPE_URLS.ENDORSE]: "pos/poc/Endorse",
  [MSG_TYPE_URLS.WITHDRAW_POC_REWARDS]: "pos/poc/WithdrawPOCRewards",
  [MSG_TYPE_URLS.SUBMIT_SIMILARITY_COMMITMENT]: "pos/poc/SubmitSimilarityCommitment",
  [MSG_TYPE_URLS.START_REVIEW]: "pos/poc/StartReview",
  [MSG_TYPE_URLS.CAST_REVIEW_VOTE]: "pos/poc/CastReviewVote",
  [MSG_TYPE_URLS.FINALIZE_REVIEW]: "pos/poc/FinalizeReview",
  [MSG_TYPE_URLS.APPEAL_REVIEW]: "pos/poc/AppealReview",
  [MSG_TYPE_URLS.RESOLVE_APPEAL]: "pos/poc/ResolveAppeal",

  // x/por
  [MSG_TYPE_URLS.REGISTER_APP]: "pos/por/RegisterApp",
  [MSG_TYPE_URLS.CREATE_VERIFIER_SET]: "pos/por/CreateVerifierSet",
  [MSG_TYPE_URLS.SUBMIT_BATCH]: "pos/por/SubmitBatch",
  [MSG_TYPE_URLS.SUBMIT_ATTESTATION]: "pos/por/SubmitAttestation",
  [MSG_TYPE_URLS.CHALLENGE_BATCH]: "pos/por/ChallengeBatch",

  // x/poseq
  [MSG_TYPE_URLS.SUBMIT_EXPORT_BATCH]: "pos/poseq/SubmitExportBatch",
  [MSG_TYPE_URLS.SUBMIT_EVIDENCE_PACKET]: "pos/poseq/SubmitEvidencePacket",
  [MSG_TYPE_URLS.SUBMIT_CHECKPOINT_ANCHOR]: "pos/poseq/SubmitCheckpointAnchor",
  [MSG_TYPE_URLS.COMMIT_EXECUTION]: "pos/poseq/CommitExecution",
  [MSG_TYPE_URLS.EXECUTE_SLASH]: "pos/poseq/ExecuteSlash",

  // x/guard
  [MSG_TYPE_URLS.CONFIRM_EXECUTION]: "pos/guard/ConfirmExecution",
  [MSG_TYPE_URLS.SUBMIT_ADVISORY_LINK]: "pos/guard/SubmitAdvisoryLink",

  // x/timelock
  [MSG_TYPE_URLS.EXECUTE_OPERATION]: "pos/timelock/ExecuteOperation",
  [MSG_TYPE_URLS.CANCEL_OPERATION]: "pos/timelock/CancelOperation",
  [MSG_TYPE_URLS.EMERGENCY_EXECUTE]: "pos/timelock/EmergencyExecute",

  // x/tokenomics
  [MSG_TYPE_URLS.MINT_TOKENS]: "pos/tokenomics/MintTokens",
  [MSG_TYPE_URLS.BURN_TOKENS]: "pos/tokenomics/BurnTokens",

  // x/royalty
  [MSG_TYPE_URLS.TOKENIZE_ROYALTY]: "pos/royalty/TokenizeRoyalty",
  [MSG_TYPE_URLS.TRANSFER_TOKEN]: "pos/royalty/TransferToken",
  [MSG_TYPE_URLS.CLAIM_ROYALTIES]: "pos/royalty/ClaimRoyalties",
  [MSG_TYPE_URLS.FRACTIONALIZE_TOKEN]: "pos/royalty/FractionalizeToken",
  [MSG_TYPE_URLS.LIST_TOKEN]: "pos/royalty/ListToken",
  [MSG_TYPE_URLS.BUY_TOKEN]: "pos/royalty/BuyToken",
  [MSG_TYPE_URLS.DELIST_TOKEN]: "pos/royalty/DelistToken",

  // x/uci
  [MSG_TYPE_URLS.REGISTER_ADAPTER]: "pos/uci/RegisterAdapter",
  [MSG_TYPE_URLS.SUSPEND_ADAPTER]: "pos/uci/SuspendAdapter",
  [MSG_TYPE_URLS.SUBMIT_DEPIN_CONTRIBUTION]: "pos/uci/SubmitDePINContribution",
  [MSG_TYPE_URLS.SUBMIT_ORACLE_ATTESTATION]: "pos/uci/SubmitOracleAttestation",

  // x/contracts
  [MSG_TYPE_URLS.DEPLOY_CONTRACT]: "pos/contracts/DeployContract",
  [MSG_TYPE_URLS.INSTANTIATE_CONTRACT]: "pos/contracts/InstantiateContract",

  // x/repgov
  [MSG_TYPE_URLS.DELEGATE_REPUTATION]: "pos/repgov/DelegateReputation",
  [MSG_TYPE_URLS.UNDELEGATE_REPUTATION]: "pos/repgov/UndelegateReputation",
};

/**
 * Build a converter entry for the AminoTypes map.
 *
 * The converter simply passes the value through unchanged, since the
 * Omniphi chain's amino codec expects the same field names as protobuf.
 */
function passthroughConverter(aminoType: string) {
  return {
    aminoType,
    toAmino: (value: Record<string, unknown>): Record<string, unknown> => value,
    fromAmino: (value: Record<string, unknown>): Record<string, unknown> => value,
  };
}

/**
 * Returns an `AminoTypes` instance with converters for all Omniphi
 * custom message types.
 */
export function createOmniphiAminoTypes(): AminoTypes {
  const converters: Record<string, ReturnType<typeof passthroughConverter>> = {};
  for (const [typeUrl, aminoType] of Object.entries(AMINO_TYPE_MAP)) {
    converters[typeUrl] = passthroughConverter(aminoType);
  }
  return new AminoTypes(converters);
}

// ---------------------------------------------------------------------------
// EncodeObject helpers
// ---------------------------------------------------------------------------

/**
 * Creates an `EncodeObject` for a custom Omniphi message.
 *
 * @param typeUrl - The proto type URL (use values from `MSG_TYPE_URLS`).
 * @param value   - The message body as a plain JS object.
 *
 * @example
 * ```ts
 * import { MSG_TYPE_URLS } from "./constants";
 * import { encodeMsg } from "./encoding";
 *
 * const msg = encodeMsg(MSG_TYPE_URLS.SUBMIT_CONTRIBUTION, {
 *   contributor: "omni1abc...",
 *   ctype: "code",
 *   uri: "ipfs://Qm...",
 *   hash: new Uint8Array(32),
 * });
 * ```
 */
export function encodeMsg(
  typeUrl: string,
  value: Record<string, unknown>,
): EncodeObject {
  return { typeUrl, value };
}

/**
 * Creates an Amino-JSON message from a type URL and value.
 *
 * @param typeUrl - The proto type URL.
 * @param value   - The message body.
 * @returns An `AminoMsg` with the corresponding amino type string.
 * @throws If the type URL has no known amino mapping.
 */
export function encodeAminoMsg(
  typeUrl: string,
  value: Record<string, unknown>,
): AminoMsg {
  const aminoType = AMINO_TYPE_MAP[typeUrl];
  if (!aminoType) {
    throw new Error(
      `No amino type mapping for typeUrl "${typeUrl}". ` +
        "Register it in AMINO_TYPE_MAP or use direct protobuf encoding.",
    );
  }
  return { type: aminoType, value };
}

// ---------------------------------------------------------------------------
// Byte helpers
// ---------------------------------------------------------------------------

/**
 * Converts a hex string to a `Uint8Array`.
 * Accepts with or without `0x` prefix.
 */
export function hexToBytes(hex: string): Uint8Array {
  const clean = hex.startsWith("0x") ? hex.slice(2) : hex;
  if (clean.length % 2 !== 0) {
    throw new Error(`Invalid hex string length: ${clean.length}`);
  }
  const bytes = new Uint8Array(clean.length / 2);
  for (let i = 0; i < clean.length; i += 2) {
    bytes[i / 2] = parseInt(clean.substring(i, i + 2), 16);
  }
  return bytes;
}

/**
 * Converts a `Uint8Array` to a lowercase hex string (no prefix).
 */
export function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

/**
 * Normalizes a hash value to a `Uint8Array`.
 * Accepts hex strings or existing `Uint8Array`.
 */
export function normalizeHash(hash: Uint8Array | string): Uint8Array {
  if (typeof hash === "string") {
    return hexToBytes(hash);
  }
  return hash;
}
