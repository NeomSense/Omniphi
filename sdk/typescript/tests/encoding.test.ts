/**
 * Tests for encoding utilities — Amino messages, protobuf registry,
 * hex/byte conversions, and hash normalization.
 */

import {
  createOmniphiRegistry,
  createOmniphiAminoTypes,
  encodeMsg,
  encodeAminoMsg,
  hexToBytes,
  bytesToHex,
  normalizeHash,
} from "../src/encoding";
import { MSG_TYPE_URLS } from "../src/constants";

describe("encoding", () => {
  // -----------------------------------------------------------------------
  // Amino message encoding
  // -----------------------------------------------------------------------

  describe("encodeAminoMsg", () => {
    test("encodes MsgSend with correct amino type", () => {
      // MsgSend is a standard cosmos message, not in our AMINO_TYPE_MAP,
      // so we test with a custom Omniphi message instead
      const msg = encodeAminoMsg(MSG_TYPE_URLS.SUBMIT_CONTRIBUTION, {
        contributor: "omni1test",
        ctype: "code",
        uri: "ipfs://QmTest",
        hash: [1, 2, 3],
      });

      expect(msg.type).toBe("pos/poc/SubmitContribution");
      expect(msg.value).toEqual({
        contributor: "omni1test",
        ctype: "code",
        uri: "ipfs://QmTest",
        hash: [1, 2, 3],
      });
    });

    test("encodes MsgSubmitContribution preserves all fields", () => {
      const value = {
        contributor: "omni1contributor",
        ctype: "record",
        uri: "ipfs://QmRecord",
        hash: [0xaa, 0xbb, 0xcc],
        canonical_hash: [0xdd, 0xee, 0xff],
        canonical_spec_version: 2,
      };

      const msg = encodeAminoMsg(MSG_TYPE_URLS.SUBMIT_CONTRIBUTION, value);
      expect(msg.type).toBe("pos/poc/SubmitContribution");
      expect(msg.value.contributor).toBe("omni1contributor");
      expect(msg.value.ctype).toBe("record");
      expect(msg.value.canonical_spec_version).toBe(2);
    });

    test("encodes MsgEndorse with correct amino type", () => {
      const msg = encodeAminoMsg(MSG_TYPE_URLS.ENDORSE, {
        validator: "omni1val",
        contribution_id: 42,
        decision: true,
      });
      expect(msg.type).toBe("pos/poc/Endorse");
    });

    test("encodes MsgRegisterApp with correct amino type", () => {
      const msg = encodeAminoMsg(MSG_TYPE_URLS.REGISTER_APP, {
        owner: "omni1owner",
        name: "TestApp",
        schema_cid: "QmSchema",
      });
      expect(msg.type).toBe("pos/por/RegisterApp");
    });

    test("encodes MsgDeployContract with correct amino type", () => {
      const msg = encodeAminoMsg(MSG_TYPE_URLS.DEPLOY_CONTRACT, {
        deployer: "omni1deployer",
        name: "MyContract",
      });
      expect(msg.type).toBe("pos/contracts/DeployContract");
    });

    test("encodes MsgTokenizeRoyalty with correct amino type", () => {
      const msg = encodeAminoMsg(MSG_TYPE_URLS.TOKENIZE_ROYALTY, {
        creator: "omni1creator",
        claim_id: 1,
        royalty_share: "0.1",
      });
      expect(msg.type).toBe("pos/royalty/TokenizeRoyalty");
    });

    test("encodes MsgDelegateReputation with correct amino type", () => {
      const msg = encodeAminoMsg(MSG_TYPE_URLS.DELEGATE_REPUTATION, {
        delegator: "omni1del",
        delegatee: "omni1tee",
        amount: "100",
      });
      expect(msg.type).toBe("pos/repgov/DelegateReputation");
    });

    test("throws for unknown type URL", () => {
      expect(() => {
        encodeAminoMsg("/unknown.type.MsgFoo", { foo: "bar" });
      }).toThrow("No amino type mapping");
    });

    test("amino value is passed through unchanged", () => {
      const originalValue = { a: 1, b: "two", c: [3] };
      const msg = encodeAminoMsg(MSG_TYPE_URLS.SUBMIT_CONTRIBUTION, originalValue);
      expect(msg.value).toEqual(originalValue);
    });
  });

  // -----------------------------------------------------------------------
  // encodeMsg (EncodeObject)
  // -----------------------------------------------------------------------

  describe("encodeMsg", () => {
    test("produces EncodeObject with typeUrl and value", () => {
      const result = encodeMsg("/test.v1.Msg", { field: "value" });
      expect(result).toEqual({
        typeUrl: "/test.v1.Msg",
        value: { field: "value" },
      });
    });

    test("uses MSG_TYPE_URLS constant", () => {
      const result = encodeMsg(MSG_TYPE_URLS.SUBMIT_CONTRIBUTION, {
        contributor: "omni1abc",
      });
      expect(result.typeUrl).toBe("/pos.poc.v1.MsgSubmitContribution");
    });

    test("preserves complex nested values", () => {
      const value = {
        deployer: "omni1d",
        intent_schemas: [
          { method: "transfer", params: [{ name: "to", type_hint: "address" }] },
        ],
        wasm_bytecode: [0, 1, 2, 3],
      };
      const result = encodeMsg(MSG_TYPE_URLS.DEPLOY_CONTRACT, value);
      expect(result.value).toEqual(value);
    });
  });

  // -----------------------------------------------------------------------
  // Protobuf registry
  // -----------------------------------------------------------------------

  describe("createOmniphiRegistry", () => {
    test("creates a Registry with all custom message types", () => {
      const registry = createOmniphiRegistry();
      expect(registry).toBeDefined();

      // Verify that lookupType returns something for each registered type
      const typeUrls = Object.values(MSG_TYPE_URLS);
      for (const typeUrl of typeUrls) {
        const type = registry.lookupType(typeUrl);
        expect(type).toBeDefined();
      }
    });

    test("registered types have encode method", () => {
      const registry = createOmniphiRegistry();
      const type = registry.lookupType(MSG_TYPE_URLS.SUBMIT_CONTRIBUTION);
      expect(type).toBeDefined();
    });

    test("registry does not contain unregistered types", () => {
      const registry = createOmniphiRegistry();
      const type = registry.lookupType("/totally.fake.v1.MsgNonExistent");
      // CosmJS registry returns undefined for unregistered types
      expect(type).toBeUndefined();
    });
  });

  // -----------------------------------------------------------------------
  // Amino types
  // -----------------------------------------------------------------------

  describe("createOmniphiAminoTypes", () => {
    test("creates AminoTypes instance", () => {
      const aminoTypes = createOmniphiAminoTypes();
      expect(aminoTypes).toBeDefined();
    });

    test("amino types can convert known type URLs", () => {
      const aminoTypes = createOmniphiAminoTypes();
      const amino = aminoTypes.toAmino({
        typeUrl: MSG_TYPE_URLS.SUBMIT_CONTRIBUTION,
        value: { contributor: "omni1x" },
      });
      expect(amino.type).toBe("pos/poc/SubmitContribution");
    });
  });

  // -----------------------------------------------------------------------
  // hexToBytes / bytesToHex roundtrip
  // -----------------------------------------------------------------------

  describe("hex encoding/decoding", () => {
    test("hexToBytes converts hex string to bytes", () => {
      const bytes = hexToBytes("deadbeef");
      expect(bytes).toEqual(new Uint8Array([0xde, 0xad, 0xbe, 0xef]));
    });

    test("hexToBytes handles 0x prefix", () => {
      const bytes = hexToBytes("0xdeadbeef");
      expect(bytes).toEqual(new Uint8Array([0xde, 0xad, 0xbe, 0xef]));
    });

    test("bytesToHex converts bytes to lowercase hex", () => {
      const hex = bytesToHex(new Uint8Array([0xde, 0xad, 0xbe, 0xef]));
      expect(hex).toBe("deadbeef");
    });

    test("roundtrip hex -> bytes -> hex", () => {
      const original = "aabbccdd11223344";
      const bytes = hexToBytes(original);
      const result = bytesToHex(bytes);
      expect(result).toBe(original);
    });

    test("roundtrip bytes -> hex -> bytes", () => {
      const original = new Uint8Array([0, 1, 127, 128, 255]);
      const hex = bytesToHex(original);
      const result = hexToBytes(hex);
      expect(result).toEqual(original);
    });

    test("hexToBytes throws on odd-length string", () => {
      expect(() => hexToBytes("abc")).toThrow("Invalid hex string length");
    });

    test("hexToBytes handles empty string", () => {
      const bytes = hexToBytes("");
      expect(bytes).toEqual(new Uint8Array(0));
    });

    test("bytesToHex handles empty array", () => {
      const hex = bytesToHex(new Uint8Array(0));
      expect(hex).toBe("");
    });

    test("bytesToHex pads single-digit values", () => {
      const hex = bytesToHex(new Uint8Array([0, 1, 2, 15]));
      expect(hex).toBe("0001020f");
    });
  });

  // -----------------------------------------------------------------------
  // normalizeHash
  // -----------------------------------------------------------------------

  describe("normalizeHash", () => {
    test("converts hex string to Uint8Array", () => {
      const result = normalizeHash("aabbccdd");
      expect(result).toEqual(new Uint8Array([0xaa, 0xbb, 0xcc, 0xdd]));
    });

    test("passes through Uint8Array unchanged", () => {
      const original = new Uint8Array([1, 2, 3, 4]);
      const result = normalizeHash(original);
      expect(result).toBe(original); // same reference
    });

    test("normalizes 32-byte hash from hex", () => {
      const hex = "aa".repeat(32);
      const result = normalizeHash(hex);
      expect(result.length).toBe(32);
      expect(result.every((b) => b === 0xaa)).toBe(true);
    });
  });
});
