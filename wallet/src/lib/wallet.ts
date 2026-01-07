/**
 * Omniphi Wallet Core
 * Secure key management and wallet operations
 */

import { DirectSecp256k1HdWallet, OfflineSigner } from '@cosmjs/proto-signing';
import { SigningStargateClient, StargateClient, GasPrice } from '@cosmjs/stargate';
import { Secp256k1HdWallet } from '@cosmjs/amino';
import { stringToPath, EnglishMnemonic } from '@cosmjs/crypto';
import { Wallet as EthersWallet } from 'ethers';
import {
  BECH32_PREFIX,
  CHAIN_ID,
  DENOM,
  GAS_PRICE,
  RPC_ENDPOINTS,
} from './constants';

// HD derivation paths
// Omniphi uses coin type 60 (Ethereum compatible) for HD wallet compatibility
const COSMOS_HD_PATH = "m/44'/60'/0'/0/0";

export interface WalletAccount {
  address: string;
  pubkey: Uint8Array;
}

export interface OmniphiWallet {
  cosmos: {
    address: string;
    signer: OfflineSigner;
  };
  evm: {
    address: string;
    displayAddress: string; // 1x format
  };
  mnemonic: string;
}

export interface EncryptedWallet {
  encryptedMnemonic: string;
  address: string;
  evmAddress: string;
  createdAt: number;
}

/**
 * Validate a BIP-39 mnemonic using CosmJS
 */
export function validateMnemonic(mnemonic: string): boolean {
  try {
    // EnglishMnemonic validates the mnemonic
    new EnglishMnemonic(mnemonic);
    return true;
  } catch {
    return false;
  }
}

/**
 * Create a wallet from mnemonic
 */
export async function createWalletFromMnemonic(
  mnemonic: string
): Promise<OmniphiWallet> {
  if (!validateMnemonic(mnemonic)) {
    throw new Error('Invalid mnemonic phrase');
  }

  // Create Cosmos wallet using coin type 60 (Ethereum compatible) HD path
  const cosmosWallet = await DirectSecp256k1HdWallet.fromMnemonic(mnemonic, {
    prefix: BECH32_PREFIX,
    hdPaths: [stringToPath(COSMOS_HD_PATH)],
  });

  const [cosmosAccount] = await cosmosWallet.getAccounts();

  // Create EVM wallet (for future PoSeQ integration)
  const evmWallet = EthersWallet.fromPhrase(mnemonic);

  return {
    cosmos: {
      address: cosmosAccount.address,
      signer: cosmosWallet,
    },
    evm: {
      address: evmWallet.address,
      displayAddress: to1x(evmWallet.address),
    },
    mnemonic,
  };
}

/**
 * Create a new wallet with fresh mnemonic
 */
export async function createNewWallet(): Promise<OmniphiWallet> {
  // Generate a new wallet which creates a fresh mnemonic using coin type 60 HD path
  const cosmosWallet = await DirectSecp256k1HdWallet.generate(24, {
    prefix: BECH32_PREFIX,
    hdPaths: [stringToPath(COSMOS_HD_PATH)],
  });

  // Get the mnemonic from the wallet
  const mnemonic = cosmosWallet.mnemonic;
  const [cosmosAccount] = await cosmosWallet.getAccounts();

  // Create EVM wallet from same mnemonic
  const evmWallet = EthersWallet.fromPhrase(mnemonic);

  return {
    cosmos: {
      address: cosmosAccount.address,
      signer: cosmosWallet,
    },
    evm: {
      address: evmWallet.address,
      displayAddress: to1x(evmWallet.address),
    },
    mnemonic,
  };
}

/**
 * Convert 0x address to 1x display format
 */
export function to1x(address: string): string {
  if (address.startsWith('0x')) {
    return '1x' + address.slice(2);
  }
  return address;
}

/**
 * Convert 1x display format to 0x address
 */
export function from1x(address: string): string {
  if (address.startsWith('1x')) {
    return '0x' + address.slice(2);
  }
  return address;
}

/**
 * Encrypt mnemonic with password using Web Crypto API
 */
export async function encryptMnemonic(
  mnemonic: string,
  password: string
): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(mnemonic);

  // Derive key from password
  const keyMaterial = await crypto.subtle.importKey(
    'raw',
    encoder.encode(password),
    'PBKDF2',
    false,
    ['deriveBits', 'deriveKey']
  );

  const salt = crypto.getRandomValues(new Uint8Array(16));
  const iv = crypto.getRandomValues(new Uint8Array(12));

  const key = await crypto.subtle.deriveKey(
    {
      name: 'PBKDF2',
      salt,
      iterations: 100000,
      hash: 'SHA-256',
    },
    keyMaterial,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt']
  );

  const encrypted = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    key,
    data
  );

  // Combine salt + iv + encrypted data
  const combined = new Uint8Array(
    salt.length + iv.length + encrypted.byteLength
  );
  combined.set(salt, 0);
  combined.set(iv, salt.length);
  combined.set(new Uint8Array(encrypted), salt.length + iv.length);

  return btoa(String.fromCharCode(...combined));
}

/**
 * Decrypt mnemonic with password
 */
export async function decryptMnemonic(
  encryptedData: string,
  password: string
): Promise<string> {
  const encoder = new TextEncoder();
  const decoder = new TextDecoder();

  const combined = Uint8Array.from(atob(encryptedData), (c) => c.charCodeAt(0));

  const salt = combined.slice(0, 16);
  const iv = combined.slice(16, 28);
  const data = combined.slice(28);

  const keyMaterial = await crypto.subtle.importKey(
    'raw',
    encoder.encode(password),
    'PBKDF2',
    false,
    ['deriveBits', 'deriveKey']
  );

  const key = await crypto.subtle.deriveKey(
    {
      name: 'PBKDF2',
      salt,
      iterations: 100000,
      hash: 'SHA-256',
    },
    keyMaterial,
    { name: 'AES-GCM', length: 256 },
    false,
    ['decrypt']
  );

  try {
    const decrypted = await crypto.subtle.decrypt(
      { name: 'AES-GCM', iv },
      key,
      data
    );
    return decoder.decode(decrypted);
  } catch {
    throw new Error('Invalid password or corrupted data');
  }
}

/**
 * Create a signing client for transactions
 */
export async function createSigningClient(
  signer: OfflineSigner,
  rpcEndpoint: string = RPC_ENDPOINTS.primary
): Promise<SigningStargateClient> {
  return SigningStargateClient.connectWithSigner(rpcEndpoint, signer, {
    gasPrice: GasPrice.fromString(`${GAS_PRICE}${DENOM}`),
  });
}

/**
 * Create a query-only client
 */
export async function createQueryClient(
  rpcEndpoint: string = RPC_ENDPOINTS.primary
): Promise<StargateClient> {
  return StargateClient.connect(rpcEndpoint);
}

/**
 * Format address for display (truncate middle)
 */
export function formatAddress(address: string, prefixLen = 10, suffixLen = 6): string {
  if (address.length <= prefixLen + suffixLen + 3) {
    return address;
  }
  return `${address.slice(0, prefixLen)}...${address.slice(-suffixLen)}`;
}

/**
 * Format amount from base units to display units
 */
export function formatAmount(
  amount: string | bigint,
  decimals = 6,
  displayDecimals = 2
): string {
  const value = typeof amount === 'string' ? BigInt(amount) : amount;
  const divisor = BigInt(10 ** decimals);
  const whole = value / divisor;
  const fraction = value % divisor;

  const fractionStr = fraction.toString().padStart(decimals, '0');
  const displayFraction = fractionStr.slice(0, displayDecimals);

  if (displayDecimals === 0 || BigInt(displayFraction) === 0n) {
    return whole.toLocaleString();
  }

  return `${whole.toLocaleString()}.${displayFraction}`;
}

/**
 * Parse display amount to base units
 */
export function parseAmount(displayAmount: string, decimals = 6): string {
  const [whole, fraction = ''] = displayAmount.split('.');
  const paddedFraction = fraction.padEnd(decimals, '0').slice(0, decimals);
  const combined = `${whole}${paddedFraction}`.replace(/^0+/, '') || '0';
  return combined;
}

/**
 * Convert string path to Cosmos SDK HD path
 */
function stringToHdPath(path: string) {
  return stringToPath(path);
}

/**
 * Check if address is valid Omniphi address
 */
export function isValidOmniphiAddress(address: string): boolean {
  if (!address) return false;

  // Cosmos address
  if (address.startsWith(BECH32_PREFIX + '1')) {
    return address.length === 43;
  }

  // EVM address (0x or 1x)
  if (address.startsWith('0x') || address.startsWith('1x')) {
    return address.length === 42;
  }

  return false;
}

/**
 * Get address type
 */
export function getAddressType(address: string): 'cosmos' | 'evm' | 'unknown' {
  if (address.startsWith(BECH32_PREFIX)) return 'cosmos';
  if (address.startsWith('0x') || address.startsWith('1x')) return 'evm';
  return 'unknown';
}
