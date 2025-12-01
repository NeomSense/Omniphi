/**
 * Omniphi Local Validator - Cryptographic Utilities
 * Provides secure encryption/decryption for validator keys
 */

const crypto = require('crypto');

// Encryption configuration
const ALGORITHM = 'aes-256-gcm';
const KEY_LENGTH = 32; // 256 bits
const IV_LENGTH = 16; // 128 bits
const AUTH_TAG_LENGTH = 16; // 128 bits
const SALT_LENGTH = 32;
const PBKDF2_ITERATIONS = 100000;

/**
 * Derive encryption key from password using PBKDF2
 * @param {string} password - User password
 * @param {Buffer} salt - Salt for key derivation
 * @returns {Buffer} Derived key
 */
function deriveKey(password, salt) {
  return crypto.pbkdf2Sync(
    password,
    salt,
    PBKDF2_ITERATIONS,
    KEY_LENGTH,
    'sha512'
  );
}

/**
 * Encrypt data with a password
 * @param {string} data - Data to encrypt (JSON string or plaintext)
 * @param {string} password - Encryption password
 * @returns {string} Encrypted data as base64 string with salt, iv, authTag embedded
 */
function encrypt(data, password) {
  if (!data || !password) {
    throw new Error('Data and password are required for encryption');
  }

  // Generate random salt and IV
  const salt = crypto.randomBytes(SALT_LENGTH);
  const iv = crypto.randomBytes(IV_LENGTH);

  // Derive key from password
  const key = deriveKey(password, salt);

  // Create cipher
  const cipher = crypto.createCipheriv(ALGORITHM, key, iv);

  // Encrypt data
  let encrypted = cipher.update(data, 'utf8', 'base64');
  encrypted += cipher.final('base64');

  // Get auth tag
  const authTag = cipher.getAuthTag();

  // Combine salt + iv + authTag + encrypted into single buffer
  const combined = Buffer.concat([
    salt,
    iv,
    authTag,
    Buffer.from(encrypted, 'base64')
  ]);

  return combined.toString('base64');
}

/**
 * Decrypt data with a password
 * @param {string} encryptedData - Base64 encrypted data
 * @param {string} password - Decryption password
 * @returns {string} Decrypted data
 */
function decrypt(encryptedData, password) {
  if (!encryptedData || !password) {
    throw new Error('Encrypted data and password are required for decryption');
  }

  try {
    // Decode combined buffer
    const combined = Buffer.from(encryptedData, 'base64');

    // Extract components
    const salt = combined.subarray(0, SALT_LENGTH);
    const iv = combined.subarray(SALT_LENGTH, SALT_LENGTH + IV_LENGTH);
    const authTag = combined.subarray(
      SALT_LENGTH + IV_LENGTH,
      SALT_LENGTH + IV_LENGTH + AUTH_TAG_LENGTH
    );
    const encrypted = combined.subarray(SALT_LENGTH + IV_LENGTH + AUTH_TAG_LENGTH);

    // Derive key from password
    const key = deriveKey(password, salt);

    // Create decipher
    const decipher = crypto.createDecipheriv(ALGORITHM, key, iv);
    decipher.setAuthTag(authTag);

    // Decrypt data
    let decrypted = decipher.update(encrypted, undefined, 'utf8');
    decrypted += decipher.final('utf8');

    return decrypted;
  } catch (error) {
    if (error.message.includes('Unsupported state') || error.message.includes('bad decrypt')) {
      throw new Error('Invalid password or corrupted data');
    }
    throw error;
  }
}

/**
 * Generate a random password/key
 * @param {number} length - Length of password (default 32)
 * @returns {string} Random password as hex string
 */
function generateRandomPassword(length = 32) {
  return crypto.randomBytes(length).toString('hex');
}

/**
 * Hash a value using SHA-256
 * @param {string} value - Value to hash
 * @returns {string} Hash as hex string
 */
function hash(value) {
  return crypto.createHash('sha256').update(value).digest('hex');
}

/**
 * Verify data integrity using HMAC
 * @param {string} data - Data to verify
 * @param {string} secret - Secret for HMAC
 * @param {string} expectedHmac - Expected HMAC value
 * @returns {boolean} Whether HMAC matches
 */
function verifyHmac(data, secret, expectedHmac) {
  const hmac = crypto.createHmac('sha256', secret);
  hmac.update(data);
  const computedHmac = hmac.digest('hex');

  // Use timing-safe comparison
  return crypto.timingSafeEqual(
    Buffer.from(computedHmac, 'hex'),
    Buffer.from(expectedHmac, 'hex')
  );
}

/**
 * Create HMAC for data
 * @param {string} data - Data to sign
 * @param {string} secret - Secret for HMAC
 * @returns {string} HMAC as hex string
 */
function createHmac(data, secret) {
  const hmac = crypto.createHmac('sha256', secret);
  hmac.update(data);
  return hmac.digest('hex');
}

module.exports = {
  encrypt,
  decrypt,
  generateRandomPassword,
  hash,
  verifyHmac,
  createHmac
};
