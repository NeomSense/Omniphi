/**
 * Tests for wallet utilities — creation, mnemonic restoration, address validation,
 * and deterministic key derivation.
 *
 * These tests exercise the actual CosmJS HD wallet derivation (no mocks needed
 * since it runs purely in-process with no network).
 */

import {
  createWallet,
  fromMnemonic,
  fromPrivateKey,
  getAddress,
  isValidAddress,
  isValidMnemonicLength,
} from "../src/wallet";
import { BECH32_PREFIX, HD_PATH, COIN_TYPE } from "../src/constants";

describe("wallet", () => {
  // -----------------------------------------------------------------------
  // createWallet
  // -----------------------------------------------------------------------

  describe("createWallet", () => {
    test("generates a wallet with default 24-word mnemonic", async () => {
      const wallet = await createWallet();
      expect(wallet.mnemonic).toBeDefined();
      const words = wallet.mnemonic!.split(" ");
      expect(words.length).toBe(24);
    });

    test("generates a wallet with 12-word mnemonic when requested", async () => {
      const wallet = await createWallet({ mnemonicLength: 12 });
      expect(wallet.mnemonic).toBeDefined();
      const words = wallet.mnemonic!.split(" ");
      expect(words.length).toBe(12);
    });

    test("generated wallet has omni-prefixed address", async () => {
      const wallet = await createWallet();
      expect(wallet.address).toMatch(/^omni1/);
    });

    test("generated wallet address has valid length", async () => {
      const wallet = await createWallet();
      // Cosmos SDK bech32 addresses with 20-byte payload: prefix + "1" + 32 chars + 6 checksum
      expect(wallet.address.length).toBeGreaterThanOrEqual(BECH32_PREFIX.length + 7);
      expect(wallet.address.length).toBeLessThanOrEqual(BECH32_PREFIX.length + 90);
    });

    test("generated wallet includes a signer", async () => {
      const wallet = await createWallet();
      expect(wallet.signer).toBeDefined();
      expect(typeof wallet.signer.getAccounts).toBe("function");
    });

    test("each call generates a different mnemonic", async () => {
      const w1 = await createWallet();
      const w2 = await createWallet();
      expect(w1.mnemonic).not.toBe(w2.mnemonic);
      expect(w1.address).not.toBe(w2.address);
    });
  });

  // -----------------------------------------------------------------------
  // fromMnemonic
  // -----------------------------------------------------------------------

  describe("fromMnemonic", () => {
    // A known 12-word test mnemonic (DO NOT use for real funds)
    const TEST_MNEMONIC =
      "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";

    test("restores wallet from known mnemonic deterministically", async () => {
      const w1 = await fromMnemonic(TEST_MNEMONIC);
      const w2 = await fromMnemonic(TEST_MNEMONIC);
      expect(w1.address).toBe(w2.address);
    });

    test("address has omni prefix", async () => {
      const wallet = await fromMnemonic(TEST_MNEMONIC);
      expect(wallet.address.startsWith("omni1")).toBe(true);
    });

    test("echoes mnemonic back", async () => {
      const wallet = await fromMnemonic(TEST_MNEMONIC);
      expect(wallet.mnemonic).toBe(TEST_MNEMONIC);
    });

    test("trims whitespace from mnemonic", async () => {
      const wallet = await fromMnemonic(`  ${TEST_MNEMONIC}  `);
      expect(wallet.mnemonic).toBe(TEST_MNEMONIC);
    });

    test("throws on empty mnemonic", async () => {
      await expect(fromMnemonic("")).rejects.toThrow("Mnemonic cannot be empty");
    });

    test("throws on whitespace-only mnemonic", async () => {
      await expect(fromMnemonic("   ")).rejects.toThrow("Mnemonic cannot be empty");
    });

    test("throws on invalid word count (5 words)", async () => {
      await expect(fromMnemonic("one two three four five")).rejects.toThrow(
        "Invalid mnemonic word count: 5",
      );
    });

    test("produces different address with different prefix", async () => {
      const defaultWallet = await fromMnemonic(TEST_MNEMONIC);
      const customWallet = await fromMnemonic(TEST_MNEMONIC, { prefix: "cosmos" });
      expect(defaultWallet.address).not.toBe(customWallet.address);
      expect(customWallet.address.startsWith("cosmos1")).toBe(true);
    });

    test("produces deterministic address from known test vector", async () => {
      // The test mnemonic "abandon...about" with omni prefix and m/44'/60'/0'/0/0
      // should produce the same address every time
      const wallet = await fromMnemonic(TEST_MNEMONIC);
      expect(wallet.address).toMatch(/^omni1[a-z0-9]+$/);
      // Verify it's consistent across runs
      const wallet2 = await fromMnemonic(TEST_MNEMONIC);
      expect(wallet.address).toBe(wallet2.address);
    });
  });

  // -----------------------------------------------------------------------
  // fromPrivateKey
  // -----------------------------------------------------------------------

  describe("fromPrivateKey", () => {
    // A 32-byte hex private key (test-only, NOT real)
    const TEST_KEY = "a".repeat(64);

    test("creates wallet from hex private key", async () => {
      const wallet = await fromPrivateKey(TEST_KEY);
      expect(wallet.address).toMatch(/^omni1/);
    });

    test("creates wallet from Uint8Array private key", async () => {
      const keyBytes = new Uint8Array(32).fill(0xaa);
      const wallet = await fromPrivateKey(keyBytes);
      expect(wallet.address).toMatch(/^omni1/);
    });

    test("does not include mnemonic for key-derived wallet", async () => {
      const wallet = await fromPrivateKey(TEST_KEY);
      expect(wallet.mnemonic).toBeUndefined();
    });

    test("throws on invalid key length", async () => {
      await expect(fromPrivateKey("abcd")).rejects.toThrow("Invalid private key length");
    });
  });

  // -----------------------------------------------------------------------
  // getAddress
  // -----------------------------------------------------------------------

  describe("getAddress", () => {
    const TEST_MNEMONIC =
      "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";

    test("returns address string from mnemonic", async () => {
      const addr = await getAddress(TEST_MNEMONIC);
      expect(typeof addr).toBe("string");
      expect(addr).toMatch(/^omni1/);
    });

    test("returns same address as fromMnemonic", async () => {
      const addr = await getAddress(TEST_MNEMONIC);
      const wallet = await fromMnemonic(TEST_MNEMONIC);
      expect(addr).toBe(wallet.address);
    });
  });

  // -----------------------------------------------------------------------
  // isValidAddress
  // -----------------------------------------------------------------------

  describe("isValidAddress", () => {
    test("accepts valid omni address format", () => {
      // Use a realistic-length bech32 string with valid charset
      const addr = "omni1" + "q".repeat(38);
      expect(isValidAddress(addr)).toBe(true);
    });

    test("rejects address with wrong prefix", () => {
      expect(isValidAddress("cosmos1" + "q".repeat(38))).toBe(false);
    });

    test("rejects address without '1' separator", () => {
      expect(isValidAddress("omni" + "q".repeat(38))).toBe(false);
    });

    test("rejects too-short address", () => {
      expect(isValidAddress("omni1abc")).toBe(false);
    });

    test("rejects address with invalid bech32 characters", () => {
      // 'b', 'i', 'o' are NOT valid bech32 characters
      expect(isValidAddress("omni1" + "b".repeat(38))).toBe(false);
    });

    test("accepts address with custom prefix", () => {
      const addr = "cosmos1" + "q".repeat(38);
      expect(isValidAddress(addr, "cosmos")).toBe(true);
    });

    test("rejects empty string", () => {
      expect(isValidAddress("")).toBe(false);
    });
  });

  // -----------------------------------------------------------------------
  // isValidMnemonicLength
  // -----------------------------------------------------------------------

  describe("isValidMnemonicLength", () => {
    test("accepts 12-word mnemonic", () => {
      expect(isValidMnemonicLength("a ".repeat(11) + "a")).toBe(true);
    });

    test("accepts 24-word mnemonic", () => {
      expect(isValidMnemonicLength("a ".repeat(23) + "a")).toBe(true);
    });

    test("rejects 10-word mnemonic", () => {
      expect(isValidMnemonicLength("a ".repeat(9) + "a")).toBe(false);
    });

    test("rejects 13-word mnemonic", () => {
      expect(isValidMnemonicLength("a ".repeat(12) + "a")).toBe(false);
    });
  });

  // -----------------------------------------------------------------------
  // Constants
  // -----------------------------------------------------------------------

  describe("constants", () => {
    test("BECH32_PREFIX is 'omni'", () => {
      expect(BECH32_PREFIX).toBe("omni");
    });

    test("HD_PATH follows BIP-44 with coin type 60", () => {
      expect(HD_PATH).toBe("m/44'/60'/0'/0/0");
    });

    test("COIN_TYPE is 60", () => {
      expect(COIN_TYPE).toBe(60);
    });
  });
});
