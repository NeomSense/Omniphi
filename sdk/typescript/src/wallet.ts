/**
 * Wallet utilities for the Omniphi blockchain.
 *
 * Provides helpers to create wallets, derive addresses, and build signers
 * compatible with `@cosmjs/proto-signing`'s `OfflineSigner` interface.
 *
 * All HD key derivation uses SLIP-44 coin type 60 and the "omni" bech32
 * prefix by default.
 */

import {
  DirectSecp256k1HdWallet,
  DirectSecp256k1Wallet,
  type OfflineDirectSigner,
} from "@cosmjs/proto-signing";
import { Secp256k1HdWallet } from "@cosmjs/amino";
import { fromHex } from "@cosmjs/encoding";
import {
  BECH32_PREFIX,
  HD_PATH,
} from "./constants";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** Result of wallet creation, containing the signer and derived address. */
export interface WalletInfo {
  /** The offline signer for signing transactions. */
  signer: OfflineDirectSigner;
  /** The bech32-encoded account address. */
  address: string;
  /** The BIP-39 mnemonic (only present for newly created wallets). */
  mnemonic?: string;
}

/** Options for wallet creation. */
export interface WalletOptions {
  /** BIP-44 HD derivation path. Defaults to `m/44'/60'/0'/0/0`. */
  hdPath?: string;
  /** Bech32 address prefix. Defaults to `"omni"`. */
  prefix?: string;
  /** Number of mnemonic words (12, 15, 18, 21, or 24). Defaults to 24. */
  mnemonicLength?: 12 | 15 | 18 | 21 | 24;
}

// ---------------------------------------------------------------------------
// Wallet creation
// ---------------------------------------------------------------------------

/**
 * Creates a brand-new wallet with a randomly generated mnemonic.
 *
 * @param options - Optional HD path, prefix, and mnemonic length overrides.
 * @returns A `WalletInfo` including the mnemonic. **Store the mnemonic
 *          securely** -- it is the only way to recover the wallet.
 *
 * @example
 * ```ts
 * const wallet = await createWallet();
 * console.log("Address:", wallet.address);
 * console.log("Mnemonic:", wallet.mnemonic);
 * ```
 */
export async function createWallet(
  options: WalletOptions = {},
): Promise<WalletInfo> {
  const {
    hdPath = HD_PATH,
    prefix = BECH32_PREFIX,
    mnemonicLength = 24,
  } = options;

  const wallet = await DirectSecp256k1HdWallet.generate(mnemonicLength, {
    hdPaths: [stringToHdPath(hdPath)],
    prefix,
  });

  const [account] = await wallet.getAccounts();
  if (!account) {
    throw new Error("Failed to derive account from generated mnemonic");
  }

  return {
    signer: wallet,
    address: account.address,
    mnemonic: wallet.mnemonic,
  };
}

/**
 * Restores a wallet from an existing BIP-39 mnemonic.
 *
 * @param mnemonic - The BIP-39 mnemonic phrase (12-24 words).
 * @param options  - Optional HD path and prefix overrides.
 * @returns A `WalletInfo` (mnemonic is echoed back for convenience).
 *
 * @example
 * ```ts
 * const wallet = await fromMnemonic("word1 word2 ... word24");
 * console.log("Restored address:", wallet.address);
 * ```
 */
export async function fromMnemonic(
  mnemonic: string,
  options: WalletOptions = {},
): Promise<WalletInfo> {
  const {
    hdPath = HD_PATH,
    prefix = BECH32_PREFIX,
  } = options;

  const trimmed = mnemonic.trim();
  if (!trimmed) {
    throw new Error("Mnemonic cannot be empty");
  }

  const wordCount = trimmed.split(/\s+/).length;
  if (![12, 15, 18, 21, 24].includes(wordCount)) {
    throw new Error(
      `Invalid mnemonic word count: ${wordCount}. Expected 12, 15, 18, 21, or 24.`,
    );
  }

  const wallet = await DirectSecp256k1HdWallet.fromMnemonic(trimmed, {
    hdPaths: [stringToHdPath(hdPath)],
    prefix,
  });

  const [account] = await wallet.getAccounts();
  if (!account) {
    throw new Error("Failed to derive account from mnemonic");
  }

  return {
    signer: wallet,
    address: account.address,
    mnemonic: trimmed,
  };
}

/**
 * Creates a wallet from a raw secp256k1 private key.
 *
 * @param privateKey - The 32-byte private key as hex string or `Uint8Array`.
 * @param prefix     - Bech32 address prefix. Defaults to `"omni"`.
 * @returns A `WalletInfo` (no mnemonic, since it was derived from a raw key).
 *
 * @example
 * ```ts
 * const wallet = await fromPrivateKey("deadbeef...");
 * console.log("Address:", wallet.address);
 * ```
 */
export async function fromPrivateKey(
  privateKey: string | Uint8Array,
  prefix: string = BECH32_PREFIX,
): Promise<WalletInfo> {
  const keyBytes =
    typeof privateKey === "string" ? fromHex(privateKey) : privateKey;

  if (keyBytes.length !== 32) {
    throw new Error(
      `Invalid private key length: ${keyBytes.length} bytes. Expected 32.`,
    );
  }

  const wallet = await DirectSecp256k1Wallet.fromKey(keyBytes, prefix);

  const [account] = await wallet.getAccounts();
  if (!account) {
    throw new Error("Failed to derive account from private key");
  }

  return {
    signer: wallet,
    address: account.address,
  };
}

/**
 * Derives the bech32 address from a mnemonic without retaining the signer.
 *
 * Useful for address-only lookups (e.g., checking balances) where you do
 * not need signing capability.
 *
 * @param mnemonic - The BIP-39 mnemonic phrase.
 * @param options  - Optional HD path and prefix overrides.
 * @returns The bech32-encoded address string.
 */
export async function getAddress(
  mnemonic: string,
  options: WalletOptions = {},
): Promise<string> {
  const info = await fromMnemonic(mnemonic, options);
  return info.address;
}

/**
 * Creates an Amino-compatible signer (for `SIGN_MODE_LEGACY_AMINO_JSON`).
 *
 * Some Cosmos SDK modules only support amino signing. Use this when you
 * need to sign with the legacy codec.
 *
 * @param mnemonic - The BIP-39 mnemonic phrase.
 * @param options  - Optional HD path and prefix overrides.
 */
export async function createAminoSigner(
  mnemonic: string,
  options: WalletOptions = {},
): Promise<{
  signer: Secp256k1HdWallet;
  address: string;
}> {
  const {
    hdPath = HD_PATH,
    prefix = BECH32_PREFIX,
  } = options;

  const wallet = await Secp256k1HdWallet.fromMnemonic(mnemonic.trim(), {
    hdPaths: [stringToHdPath(hdPath)],
    prefix,
  });

  const [account] = await wallet.getAccounts();
  if (!account) {
    throw new Error("Failed to derive amino account from mnemonic");
  }

  return { signer: wallet, address: account.address };
}

// ---------------------------------------------------------------------------
// HD path parsing
// ---------------------------------------------------------------------------

/**
 * Parses a BIP-44 path string like `"m/44'/60'/0'/0/0"` into the
 * `HdPath` (number array) format that CosmJS expects.
 *
 * Each component ending with `'` is marked as hardened (bit 31 set).
 */
function stringToHdPath(path: string): import("@cosmjs/crypto").HdPath {
  // Dynamically import to avoid top-level dep on @cosmjs/crypto
  // CosmJS HdPath is just a readonly number[]
  const parts = path
    .replace(/^m\//, "")
    .split("/")
    .map((component) => {
      const hardened = component.endsWith("'");
      const index = parseInt(component.replace("'", ""), 10);
      if (isNaN(index)) {
        throw new Error(`Invalid HD path component: "${component}"`);
      }
      // Hardened derivation uses index + 2^31
      return hardened ? index + 0x80000000 : index;
    });

  // CosmJS Slip10RawIndex type is just number, and HdPath is readonly number[]
  return parts as unknown as import("@cosmjs/crypto").HdPath;
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

/**
 * Checks whether a string looks like a valid Omniphi bech32 address.
 *
 * This performs a prefix check and basic length validation. It does NOT
 * verify the bech32 checksum (use `@cosmjs/encoding`'s `fromBech32` for that).
 *
 * @param address - The address string to validate.
 * @param prefix  - Expected prefix. Defaults to `"omni"`.
 */
export function isValidAddress(
  address: string,
  prefix: string = BECH32_PREFIX,
): boolean {
  if (!address.startsWith(prefix + "1")) {
    return false;
  }
  // Cosmos SDK addresses are typically 39-59 chars (20 or 32 byte payloads)
  if (address.length < prefix.length + 7 || address.length > prefix.length + 90) {
    return false;
  }
  // Bech32 charset: qpzry9x8gf2tvdw0s3jn54khce6mua7l
  const bech32Chars = /^[qpzry9x8gf2tvdw0s3jn54khce6mua7l]+$/;
  const dataPart = address.slice(prefix.length + 1);
  return bech32Chars.test(dataPart);
}

/**
 * Validates that a mnemonic has the correct word count.
 * Does NOT check word validity against the BIP-39 wordlist.
 */
export function isValidMnemonicLength(mnemonic: string): boolean {
  const wordCount = mnemonic.trim().split(/\s+/).length;
  return [12, 15, 18, 21, 24].includes(wordCount);
}
