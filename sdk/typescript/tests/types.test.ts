/**
 * Tests for type definitions — intent construction, contribution types,
 * enum values, and structural correctness.
 *
 * These are compile-time + runtime correctness checks ensuring that the
 * type system accurately models the Omniphi chain state.
 */

import {
  ClaimStatus,
  ReviewStatus,
  BatchStatus,
  ChallengeType,
  ChallengeStatus,
  AppStatus,
  TimelockStatus,
} from "../src/types";

import type {
  TransferIntent,
  SwapIntent,
  DelegateIntent,
  ContributeIntent,
  DeployContractIntent,
  Intent,
  IntentTransaction,
  Contribution,
  Endorsement,
  App,
  BatchCommitment,
  RoyaltyToken,
  Adapter,
  VoterWeight,
  TokenSupply,
  InflationInfo,
  EpochStateReference,
  CheckpointAnchorRecord,
  StdFee,
  IntentSchema,
  IntentSchemaField,
  PaginatedResponse,
  BalanceResponse,
  NodeInfo,
} from "../src/types";

describe("types", () => {
  // -----------------------------------------------------------------------
  // Intent type construction
  // -----------------------------------------------------------------------

  describe("TransferIntent", () => {
    test("has correct type discriminant", () => {
      const intent: TransferIntent = {
        type: "transfer",
        sender: "omni1sender",
        recipient: "omni1recipient",
        amount: "1000000",
        denom: "omniphi",
      };
      expect(intent.type).toBe("transfer");
    });

    test("allows optional memo", () => {
      const intent: TransferIntent = {
        type: "transfer",
        sender: "omni1s",
        recipient: "omni1r",
        amount: "100",
        denom: "omniphi",
        memo: "test transfer",
      };
      expect(intent.memo).toBe("test transfer");
    });

    test("memo is undefined by default", () => {
      const intent: TransferIntent = {
        type: "transfer",
        sender: "omni1s",
        recipient: "omni1r",
        amount: "100",
        denom: "omniphi",
      };
      expect(intent.memo).toBeUndefined();
    });
  });

  describe("SwapIntent", () => {
    test("has correct type discriminant", () => {
      const intent: SwapIntent = {
        type: "swap",
        sender: "omni1sender",
        inputDenom: "omniphi",
        inputAmount: "1000000",
        outputDenom: "usdc",
        minOutputAmount: "990000",
      };
      expect(intent.type).toBe("swap");
      expect(intent.inputDenom).toBe("omniphi");
      expect(intent.outputDenom).toBe("usdc");
    });

    test("maxSlippageBps is optional", () => {
      const intent: SwapIntent = {
        type: "swap",
        sender: "omni1s",
        inputDenom: "omniphi",
        inputAmount: "100",
        outputDenom: "usdc",
        minOutputAmount: "99",
      };
      expect(intent.maxSlippageBps).toBeUndefined();
    });

    test("maxSlippageBps can be set", () => {
      const intent: SwapIntent = {
        type: "swap",
        sender: "omni1s",
        inputDenom: "omniphi",
        inputAmount: "100",
        outputDenom: "usdc",
        minOutputAmount: "99",
        maxSlippageBps: 50,
      };
      expect(intent.maxSlippageBps).toBe(50);
    });
  });

  describe("DelegateIntent", () => {
    test("has correct type discriminant and validator field", () => {
      const intent: DelegateIntent = {
        type: "delegate",
        delegator: "omni1delegator",
        validator: "omnivaloper1validator",
        amount: "5000000",
        denom: "omniphi",
      };
      expect(intent.type).toBe("delegate");
      expect(intent.validator).toMatch(/^omnivaloper/);
    });
  });

  describe("ContributeIntent", () => {
    test("has correct type discriminant and ctype", () => {
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

    test("supports all contribution types", () => {
      const types: string[] = ["code", "record", "relay", "green", "custom_type"];
      for (const ctype of types) {
        const intent: ContributeIntent = {
          type: "contribute",
          contributor: "omni1c",
          ctype,
          uri: "ipfs://Qm",
          hash: "bb".repeat(32),
        };
        expect(intent.ctype).toBe(ctype);
      }
    });
  });

  describe("DeployContractIntent", () => {
    test("has correct structure with schemas and bytecode", () => {
      const intent: DeployContractIntent = {
        type: "deploy_contract",
        deployer: "omni1deployer",
        name: "TestContract",
        description: "A test contract",
        domainTag: "defi",
        intentSchemas: [
          {
            method: "swap",
            params: [
              { name: "amount", typeHint: "uint256" },
              { name: "to", typeHint: "address" },
            ],
            capabilities: ["transfer", "query_balance"],
          },
        ],
        maxGasPerCall: 500000,
        maxStateBytes: 1048576,
        wasmBytecode: new Uint8Array([0, 97, 115, 109]),
      };
      expect(intent.type).toBe("deploy_contract");
      expect(intent.intentSchemas.length).toBe(1);
      expect(intent.intentSchemas[0].params.length).toBe(2);
      expect(intent.wasmBytecode).toBeInstanceOf(Uint8Array);
    });
  });

  // -----------------------------------------------------------------------
  // Intent union type
  // -----------------------------------------------------------------------

  describe("Intent union", () => {
    test("narrowing works with type discriminant", () => {
      const intent: Intent = {
        type: "transfer",
        sender: "omni1s",
        recipient: "omni1r",
        amount: "100",
        denom: "omniphi",
      };

      if (intent.type === "transfer") {
        expect(intent.recipient).toBe("omni1r");
      } else {
        fail("Should have narrowed to TransferIntent");
      }
    });
  });

  // -----------------------------------------------------------------------
  // IntentTransaction
  // -----------------------------------------------------------------------

  describe("IntentTransaction", () => {
    test("wraps multiple intents", () => {
      const tx: IntentTransaction = {
        sender: "omni1sender",
        intents: [
          { type: "transfer", sender: "omni1s", recipient: "omni1r", amount: "100", denom: "omniphi" },
          { type: "delegate", delegator: "omni1s", validator: "omnivaloper1v", amount: "200", denom: "omniphi" },
        ],
      };
      expect(tx.intents.length).toBe(2);
      expect(tx.intents[0].type).toBe("transfer");
      expect(tx.intents[1].type).toBe("delegate");
    });

    test("has optional memo, maxGas, and deadline", () => {
      const tx: IntentTransaction = {
        sender: "omni1sender",
        intents: [],
        memo: "batch intent",
        maxGas: 500000,
        deadline: 1711500000,
      };
      expect(tx.memo).toBe("batch intent");
      expect(tx.maxGas).toBe(500000);
      expect(tx.deadline).toBe(1711500000);
    });
  });

  // -----------------------------------------------------------------------
  // Contribution type
  // -----------------------------------------------------------------------

  describe("Contribution", () => {
    test("has all required fields", () => {
      const contribution: Contribution = {
        id: 1,
        contributor: "omni1c",
        ctype: "code",
        uri: "ipfs://Qm",
        hash: "aabb",
        endorsements: [],
        verified: false,
        blockHeight: 100,
        blockTime: 1711500000,
        rewarded: false,
        canonicalHash: "ccdd",
        canonicalSpecVersion: 1,
        duplicateOf: 0,
        isDerivative: false,
        reviewStatus: ReviewStatus.NONE,
        parentClaimId: 0,
        claimStatus: ClaimStatus.SUBMITTED,
      };
      expect(contribution.id).toBe(1);
      expect(contribution.claimStatus).toBe(ClaimStatus.SUBMITTED);
    });
  });

  // -----------------------------------------------------------------------
  // PoSeq types
  // -----------------------------------------------------------------------

  describe("PoSeq types", () => {
    test("EpochStateReference has all fields", () => {
      const state: EpochStateReference = {
        epoch: 5,
        committeeHash: "aabbccdd",
        finalizedBatchCount: 10,
        misbehaviorCount: 0,
        evidencePacketCount: 0,
        governanceEscalations: 0,
        epochStateHash: "eeff0011",
      };
      expect(state.epoch).toBe(5);
      expect(state.finalizedBatchCount).toBe(10);
    });

    test("CheckpointAnchorRecord includes finality reference", () => {
      const checkpoint: CheckpointAnchorRecord = {
        checkpointId: "ckpt_1",
        epoch: 5,
        slot: 100,
        epochStateHash: "hash1",
        bridgeStateHash: "hash2",
        misbehaviorCount: 0,
        finalitySummary: {
          batchId: "batch_1",
          slot: 99,
          epoch: 5,
          finalizationHash: "fhash",
          submissionCount: 3,
          quorumApprovals: 2,
          committeeSize: 3,
        },
        anchorHash: "ahash",
      };
      expect(checkpoint.finalitySummary.quorumApprovals).toBe(2);
    });
  });

  // -----------------------------------------------------------------------
  // Enum values
  // -----------------------------------------------------------------------

  describe("enum values", () => {
    test("ClaimStatus has correct numeric values", () => {
      expect(ClaimStatus.SUBMITTED).toBe(0);
      expect(ClaimStatus.ENDORSED).toBe(1);
      expect(ClaimStatus.VERIFIED).toBe(2);
      expect(ClaimStatus.REWARDED).toBe(3);
      expect(ClaimStatus.REJECTED).toBe(4);
    });

    test("ReviewStatus has correct numeric values", () => {
      expect(ReviewStatus.NONE).toBe(0);
      expect(ReviewStatus.PENDING).toBe(1);
      expect(ReviewStatus.IN_REVIEW).toBe(2);
      expect(ReviewStatus.ACCEPTED).toBe(3);
      expect(ReviewStatus.REJECTED).toBe(4);
      expect(ReviewStatus.APPEALED).toBe(5);
    });

    test("BatchStatus has correct numeric values", () => {
      expect(BatchStatus.SUBMITTED).toBe(0);
      expect(BatchStatus.PENDING).toBe(1);
      expect(BatchStatus.FINALIZED).toBe(2);
      expect(BatchStatus.REJECTED).toBe(3);
    });

    test("ChallengeType has correct numeric values", () => {
      expect(ChallengeType.INVALID_ROOT).toBe(0);
      expect(ChallengeType.DOUBLE_INCLUSION).toBe(1);
      expect(ChallengeType.MISSING_RECORD).toBe(2);
      expect(ChallengeType.INVALID_SCHEMA).toBe(3);
    });

    test("ChallengeStatus has correct numeric values", () => {
      expect(ChallengeStatus.OPEN).toBe(0);
      expect(ChallengeStatus.RESOLVED_VALID).toBe(1);
      expect(ChallengeStatus.RESOLVED_INVALID).toBe(2);
    });

    test("AppStatus has correct numeric values", () => {
      expect(AppStatus.ACTIVE).toBe(0);
      expect(AppStatus.SUSPENDED).toBe(1);
      expect(AppStatus.DEREGISTERED).toBe(2);
    });

    test("TimelockStatus has correct numeric values", () => {
      expect(TimelockStatus.QUEUED).toBe(0);
      expect(TimelockStatus.EXECUTED).toBe(1);
      expect(TimelockStatus.CANCELLED).toBe(2);
    });
  });

  // -----------------------------------------------------------------------
  // Query response types
  // -----------------------------------------------------------------------

  describe("query response types", () => {
    test("StdFee has amount and gas", () => {
      const fee: StdFee = {
        amount: [{ denom: "omniphi", amount: "5000" }],
        gas: "200000",
      };
      expect(fee.amount[0].denom).toBe("omniphi");
      expect(fee.gas).toBe("200000");
    });

    test("TokenSupply has all supply fields", () => {
      const supply: TokenSupply = {
        totalSupply: "1000000000",
        circulatingSupply: "500000000",
        bondedTokens: "300000000",
        mintedThisEpoch: "1000",
        burnedThisEpoch: "500",
      };
      expect(supply.totalSupply).toBe("1000000000");
    });

    test("InflationInfo has rate fields", () => {
      const info: InflationInfo = {
        annualRate: "0.05",
        epochRate: "0.0001",
        targetBondRatio: "0.67",
        currentBondRatio: "0.60",
      };
      expect(info.annualRate).toBe("0.05");
    });

    test("PaginatedResponse structure", () => {
      const response: PaginatedResponse<{ id: number }> = {
        data: [{ id: 1 }, { id: 2 }],
        pagination: { nextKey: "abc", total: "100" },
      };
      expect(response.data.length).toBe(2);
      expect(response.pagination.total).toBe("100");
    });

    test("BalanceResponse structure", () => {
      const response: BalanceResponse = {
        balance: { denom: "omniphi", amount: "1000000" },
      };
      expect(response.balance.denom).toBe("omniphi");
    });

    test("IntentSchema and IntentSchemaField structure", () => {
      const schema: IntentSchema = {
        method: "transfer",
        params: [
          { name: "to", typeHint: "address" },
          { name: "amount", typeHint: "uint256" },
        ],
        capabilities: ["transfer"],
      };
      expect(schema.method).toBe("transfer");
      expect(schema.params.length).toBe(2);
    });
  });

  // -----------------------------------------------------------------------
  // Module-specific types
  // -----------------------------------------------------------------------

  describe("module types", () => {
    test("App type has correct structure", () => {
      const app: App = {
        appId: 1,
        name: "TestApp",
        owner: "omni1owner",
        schemaCid: "QmSchema",
        challengePeriod: 100,
        minVerifiers: 3,
        status: AppStatus.ACTIVE,
        createdAt: 1711500000,
      };
      expect(app.status).toBe(AppStatus.ACTIVE);
    });

    test("RoyaltyToken type has correct structure", () => {
      const token: RoyaltyToken = {
        tokenId: 1,
        claimId: 42,
        owner: "omni1owner",
        originalCreator: "omni1creator",
        royaltyShare: "0.10",
        status: "ACTIVE",
        createdAtHeight: 500,
        isFractionalized: false,
        fractionCount: 0,
        totalPayouts: "0",
        metadata: "test token",
      };
      expect(token.status).toBe("ACTIVE");
      expect(token.royaltyShare).toBe("0.10");
    });

    test("Adapter type has correct structure", () => {
      const adapter: Adapter = {
        adapterId: 1,
        name: "TestAdapter",
        owner: "omni1owner",
        schemaCid: "QmAdapter",
        oracleAllowlist: ["omni1oracle1", "omni1oracle2"],
        status: "ACTIVE",
        networkType: "filecoin",
        createdAtHeight: 100,
        totalContributions: 50,
        totalRewardsDistributed: "10000",
        rewardShare: "0.05",
        description: "Test DePIN adapter",
      };
      expect(adapter.oracleAllowlist.length).toBe(2);
      expect(adapter.networkType).toBe("filecoin");
    });

    test("VoterWeight type has all governance fields", () => {
      const weight: VoterWeight = {
        address: "omni1voter",
        epoch: 10,
        reputationScore: "85.5",
        cScore: "90.0",
        endorsementRate: "0.95",
        originalityAvg: "0.80",
        uptimeScore: "1.0",
        longevityScore: "0.70",
        compositeWeight: "88.2",
        effectiveWeight: "88.2",
        delegatedWeight: "10.0",
        lastVoteHeight: 1000,
      };
      expect(weight.compositeWeight).toBe("88.2");
    });

    test("Endorsement type", () => {
      const endorsement: Endorsement = {
        valAddr: "omni1validator",
        decision: true,
        power: "1000000",
      };
      expect(endorsement.decision).toBe(true);
    });

    test("BatchCommitment type", () => {
      const batch: BatchCommitment = {
        batchId: 1,
        epoch: 5,
        recordMerkleRoot: "aabbccdd",
        recordCount: 100,
        appId: 1,
        verifierSetId: 1,
        submitter: "omni1sub",
        challengeEndTime: 1711600000,
        status: BatchStatus.FINALIZED,
        submittedAt: 1711500000,
        finalizedAt: 1711500100,
      };
      expect(batch.status).toBe(BatchStatus.FINALIZED);
    });
  });
});
