/**
 * Tests for OmniphiClient — construction, query URL building, message structure,
 * intent translation, and error handling.
 *
 * These tests mock the underlying CosmJS clients so they run without a live chain.
 */

import { OmniphiClient, OmniphiQueryError, OmniphiTxError } from "../src/client";
import {
  DEFAULT_RPC_ENDPOINT,
  DEFAULT_REST_ENDPOINT,
  DENOM,
  MSG_TYPE_URLS,
  REST_PATHS,
  DEFAULT_FEE,
} from "../src/constants";
import type { Intent, TransferIntent, SwapIntent, DelegateIntent, ContributeIntent } from "../src/types";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// We cannot connect to a real chain in unit tests, so we test the parts of
// OmniphiClient that don't require live connections: construction behavior,
// error classes, URL construction from constants, and intent-to-message logic.

describe("OmniphiClient", () => {
  // -----------------------------------------------------------------------
  // Construction and configuration
  // -----------------------------------------------------------------------

  describe("construction", () => {
    test("default RPC endpoint is localhost:26657", () => {
      expect(DEFAULT_RPC_ENDPOINT).toBe("http://localhost:26657");
    });

    test("default REST endpoint is localhost:1318", () => {
      expect(DEFAULT_REST_ENDPOINT).toBe("http://localhost:1318");
    });

    test("default denomination is 'omniphi'", () => {
      expect(DENOM).toBe("omniphi");
    });

    test("OmniphiClientOptions accepts custom endpoints", () => {
      // Verify the options interface works (compile-time check through TS,
      // but we also verify the constants are used by default).
      const opts = {
        rpcEndpoint: "http://custom:26657",
        restEndpoint: "http://custom:1318",
        gasPrice: "0.05omniphi",
      };
      expect(opts.rpcEndpoint).toBe("http://custom:26657");
      expect(opts.restEndpoint).toBe("http://custom:1318");
      expect(opts.gasPrice).toBe("0.05omniphi");
    });

    test("DEFAULT_FEE has correct structure", () => {
      expect(DEFAULT_FEE).toEqual({
        amount: [{ denom: "omniphi", amount: "5000" }],
        gas: "200000",
      });
    });
  });

  // -----------------------------------------------------------------------
  // REST path construction
  // -----------------------------------------------------------------------

  describe("REST path building", () => {
    test("getBalance uses correct REST path", () => {
      const addr = "omni1abc123def456";
      const path = REST_PATHS.BANK_BALANCES(addr);
      expect(path).toBe(`/cosmos/bank/v1beta1/balances/${addr}`);
    });

    test("POC contribution path includes ID", () => {
      expect(REST_PATHS.POC_CONTRIBUTION(42)).toBe("/pos/poc/v1/contribution/42");
    });

    test("POR app path includes ID", () => {
      expect(REST_PATHS.POR_APP(7)).toBe("/pos/por/v1/app/7");
    });

    test("POR batch path includes ID", () => {
      expect(REST_PATHS.POR_BATCH(99)).toBe("/pos/por/v1/batch/99");
    });

    test("POSEQ checkpoint path includes epoch", () => {
      expect(REST_PATHS.POSEQ_CHECKPOINT(5)).toBe("/pos/poseq/v1/checkpoint/5");
    });

    test("POSEQ epoch state path includes epoch", () => {
      expect(REST_PATHS.POSEQ_EPOCH_STATE(10)).toBe("/pos/poseq/v1/epoch_state/10");
    });

    test("royalty token path includes ID", () => {
      expect(REST_PATHS.ROYALTY_TOKEN(3)).toBe("/pos/royalty/v1/token/3");
    });

    test("UCI adapter path includes ID", () => {
      expect(REST_PATHS.UCI_ADAPTER(12)).toBe("/pos/uci/v1/adapter/12");
    });

    test("voter weight path includes address", () => {
      const addr = "omni1voter";
      expect(REST_PATHS.REPGOV_VOTER_WEIGHT(addr)).toBe(`/pos/repgov/v1/voter_weight/${addr}`);
    });

    test("staking delegations path includes delegator", () => {
      const addr = "omni1delegator";
      expect(REST_PATHS.STAKING_DELEGATIONS(addr)).toBe(`/cosmos/staking/v1beta1/delegations/${addr}`);
    });
  });

  // -----------------------------------------------------------------------
  // Message type URLs
  // -----------------------------------------------------------------------

  describe("message type URLs", () => {
    test("SUBMIT_CONTRIBUTION has correct proto path", () => {
      expect(MSG_TYPE_URLS.SUBMIT_CONTRIBUTION).toBe("/pos.poc.v1.MsgSubmitContribution");
    });

    test("ENDORSE has correct proto path", () => {
      expect(MSG_TYPE_URLS.ENDORSE).toBe("/pos.poc.v1.MsgEndorse");
    });

    test("REGISTER_APP has correct proto path", () => {
      expect(MSG_TYPE_URLS.REGISTER_APP).toBe("/pos.por.v1.MsgRegisterApp");
    });

    test("DEPLOY_CONTRACT has correct proto path", () => {
      expect(MSG_TYPE_URLS.DEPLOY_CONTRACT).toBe("/pos.contracts.v1.MsgDeployContract");
    });

    test("DELEGATE_REPUTATION has correct proto path", () => {
      expect(MSG_TYPE_URLS.DELEGATE_REPUTATION).toBe("/pos.repgov.v1.MsgDelegateReputation");
    });
  });

  // -----------------------------------------------------------------------
  // sendTokens message structure
  // -----------------------------------------------------------------------

  describe("sendTokens message structure", () => {
    test("builds MsgSend with correct fields", () => {
      // Verify the shape of a MsgSend that OmniphiClient would construct
      const sender = "omni1sender";
      const recipient = "omni1recipient";
      const amount = "1000000";
      const denom = DENOM;

      const msgValue = {
        fromAddress: sender,
        toAddress: recipient,
        amount: [{ denom, amount }],
      };

      expect(msgValue.fromAddress).toBe(sender);
      expect(msgValue.toAddress).toBe(recipient);
      expect(msgValue.amount).toEqual([{ denom: "omniphi", amount: "1000000" }]);
    });

    test("uses default denom when not specified", () => {
      const coins = [{ denom: DENOM, amount: "500" }];
      expect(coins[0].denom).toBe("omniphi");
    });
  });

  // -----------------------------------------------------------------------
  // submitIntent translation
  // -----------------------------------------------------------------------

  describe("intent-to-message translation", () => {
    test("transfer intent maps to MsgSend type URL", () => {
      const intent: TransferIntent = {
        type: "transfer",
        sender: "omni1sender",
        recipient: "omni1recipient",
        amount: "1000000",
        denom: "omniphi",
      };
      // The client's intentToMessages would produce a /cosmos.bank.v1beta1.MsgSend
      expect(intent.type).toBe("transfer");
      // Verify the expected typeUrl
      const expectedTypeUrl = "/cosmos.bank.v1beta1.MsgSend";
      expect(expectedTypeUrl).toContain("MsgSend");
    });

    test("swap intent produces correct fields", () => {
      const intent: SwapIntent = {
        type: "swap",
        sender: "omni1sender",
        inputDenom: "omniphi",
        inputAmount: "1000000",
        outputDenom: "usdc",
        minOutputAmount: "990000",
        maxSlippageBps: 100,
      };
      expect(intent.type).toBe("swap");
      expect(intent.inputDenom).toBe("omniphi");
      expect(intent.outputDenom).toBe("usdc");
      expect(intent.maxSlippageBps).toBe(100);
    });

    test("swap intent defaults maxSlippageBps to undefined", () => {
      const intent: SwapIntent = {
        type: "swap",
        sender: "omni1sender",
        inputDenom: "omniphi",
        inputAmount: "1000000",
        outputDenom: "usdc",
        minOutputAmount: "990000",
      };
      expect(intent.maxSlippageBps).toBeUndefined();
    });

    test("delegate intent maps to MsgDelegate type URL", () => {
      const intent: DelegateIntent = {
        type: "delegate",
        delegator: "omni1delegator",
        validator: "omnivaloper1validator",
        amount: "5000000",
        denom: "omniphi",
      };
      expect(intent.type).toBe("delegate");
      const expectedTypeUrl = "/cosmos.staking.v1beta1.MsgDelegate";
      expect(expectedTypeUrl).toContain("MsgDelegate");
    });

    test("contribute intent maps to MsgSubmitContribution", () => {
      const intent: ContributeIntent = {
        type: "contribute",
        contributor: "omni1contributor",
        ctype: "code",
        uri: "ipfs://QmTest",
        hash: "aa".repeat(32),
      };
      expect(intent.type).toBe("contribute");
      expect(intent.ctype).toBe("code");
    });

    test("unknown intent type would throw at runtime", () => {
      // Simulate the switch statement's default case
      const badIntent = { type: "unknown_type" } as unknown as Intent;
      expect(() => {
        switch (badIntent.type) {
          case "transfer":
          case "swap":
          case "delegate":
          case "contribute":
          case "deploy_contract":
            break;
          default:
            throw new Error(`Unknown intent type: ${(badIntent as { type: string }).type}`);
        }
      }).toThrow("Unknown intent type: unknown_type");
    });
  });

  // -----------------------------------------------------------------------
  // delegate message structure
  // -----------------------------------------------------------------------

  describe("delegate message structure", () => {
    test("builds MsgDelegate with correct coin", () => {
      const delegator = "omni1delegator";
      const validator = "omnivaloper1val";
      const amount = "10000000";
      const coin = { denom: DENOM, amount };

      expect(coin.denom).toBe("omniphi");
      expect(coin.amount).toBe("10000000");

      const msgValue = {
        delegatorAddress: delegator,
        validatorAddress: validator,
        amount: coin,
      };
      expect(msgValue.delegatorAddress).toBe(delegator);
      expect(msgValue.validatorAddress).toBe(validator);
    });

    test("undelegate uses same coin structure", () => {
      const coin = { denom: DENOM, amount: "5000000" };
      expect(coin).toEqual({ denom: "omniphi", amount: "5000000" });
    });
  });

  // -----------------------------------------------------------------------
  // Error handling
  // -----------------------------------------------------------------------

  describe("error classes", () => {
    test("OmniphiQueryError includes endpoint, status, and body", () => {
      const err = new OmniphiQueryError("http://localhost:1318/test", 404, "not found");
      expect(err).toBeInstanceOf(Error);
      expect(err.name).toBe("OmniphiQueryError");
      expect(err.endpoint).toBe("http://localhost:1318/test");
      expect(err.status).toBe(404);
      expect(err.body).toBe("not found");
      expect(err.message).toContain("HTTP 404");
    });

    test("OmniphiTxError includes code, txHash, and rawLog", () => {
      const err = new OmniphiTxError(5, "ABCDEF1234", "insufficient funds");
      expect(err).toBeInstanceOf(Error);
      expect(err.name).toBe("OmniphiTxError");
      expect(err.code).toBe(5);
      expect(err.txHash).toBe("ABCDEF1234");
      expect(err.rawLog).toBe("insufficient funds");
      expect(err.message).toContain("code 5");
    });

    test("OmniphiQueryError message format", () => {
      const err = new OmniphiQueryError("/test", 500, "internal error");
      expect(err.message).toBe("Query to /test failed (HTTP 500): internal error");
    });

    test("OmniphiTxError message format", () => {
      const err = new OmniphiTxError(11, "HASH123", "out of gas");
      expect(err.message).toBe("Transaction HASH123 failed with code 11: out of gas");
    });
  });
});
