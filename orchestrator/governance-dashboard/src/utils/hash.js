/**
 * Browser-native SHA-256 hashing via Web Crypto API.
 * Zero dependencies — no external crypto libraries.
 */

/**
 * Convert a Uint8Array to a lowercase hex string.
 */
export function bytesToHex(bytes) {
  const arr = new Uint8Array(bytes);
  let hex = '';
  for (let i = 0; i < arr.length; i++) {
    hex += arr[i].toString(16).padStart(2, '0');
  }
  return hex;
}

/**
 * Compute SHA-256 of raw bytes and return lowercase hex.
 * @param {ArrayBuffer|Uint8Array} data
 * @returns {Promise<string>} 64-char lowercase hex
 */
export async function sha256Hex(data) {
  const buf = await crypto.subtle.digest('SHA-256', data);
  return bytesToHex(new Uint8Array(buf));
}

/**
 * Normalize an on-chain hash for comparison.
 * Strips optional 0x prefix, lowercases.
 */
export function normalizeHash(hash) {
  if (!hash) return '';
  return hash.replace(/^0x/i, '').toLowerCase();
}
