/**
 * Omniphi DAO with Reputation-Weighted Governance
 *
 * This example demonstrates a governance model that goes beyond plutocratic
 * token voting. Traditional DAOs (Compound Governor, Snapshot) weight votes
 * purely by token holdings: 1 token = 1 vote. This leads to:
 *
 *   - Whale dominance: a few large holders control all decisions
 *   - Vote buying: tokens can be borrowed just to influence votes
 *   - Flash loan attacks: borrow millions of tokens, vote, repay in one block
 *   - Apathy: small holders have no meaningful influence, so they do not vote
 *
 * Omniphi's reputation-weighted governance addresses these problems by
 * integrating the chain's Proof of Contribution (PoC) scores into vote
 * weight calculation. Your vote weight is a composite of:
 *
 *   1. Contribution score (cScore): value of verified PoC contributions
 *   2. Endorsement rate: percentage of your contributions endorsed by validators
 *   3. Originality average: uniqueness of your contributions (dedup-aware)
 *   4. Uptime score: consistency of participation over time
 *   5. Longevity score: how long you have been an active contributor
 *   6. Delegated weight: reputation delegated to you by others
 *
 * This creates a meritocratic system where influence is earned through
 * genuine contributions, not just purchased with tokens.
 *
 * The x/repgov module computes these weights on-chain every epoch, using
 * data from x/poc, x/poseq, and the staking module.
 */

import {
  OmniphiClient,
  createWallet,
  fromMnemonic,
  DENOM,
  DEFAULT_FEE,
  DEFAULT_RPC_ENDPOINT,
  DEFAULT_REST_ENDPOINT,
  MSG_TYPE_URLS,
} from "@omniphi/sdk";
import type {
  IntentTransaction,
  VoterWeight,
  StdFee,
} from "@omniphi/sdk";

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

const CONFIG = {
  rpcEndpoint: process.env.OMNIPHI_RPC || DEFAULT_RPC_ENDPOINT,
  restEndpoint: process.env.OMNIPHI_REST || DEFAULT_REST_ENDPOINT,
  /** Minimum weight to create a proposal */
  minProposalWeight: "100",
  /** Voting period in seconds (5 minutes for testnet, 7 days for mainnet) */
  votingPeriod: 300,
  /** Quorum: minimum percentage of total weight that must vote (30%) */
  quorumPercentage: 30,
  /** Threshold: minimum percentage of yes votes to pass (66.7%) */
  passThreshold: 66.7,
};

// ---------------------------------------------------------------------------
// DAO types
// ---------------------------------------------------------------------------

type ProposalStatus =
  | "DEPOSIT_PERIOD"   // Collecting deposits
  | "VOTING_PERIOD"    // Active voting
  | "PASSED"           // Passed quorum + threshold
  | "REJECTED"         // Failed quorum or threshold
  | "EXECUTED"         // Actions have been executed
  | "FAILED";          // Execution failed

type VoteOption = "YES" | "NO" | "ABSTAIN" | "NO_WITH_VETO";

/** A governance proposal. */
interface Proposal {
  proposalId: number;
  title: string;
  description: string;
  proposer: string;
  /** On-chain actions to execute if passed */
  actions: ProposalAction[];
  status: ProposalStatus;
  /** Tally at current state */
  tally: VoteTally;
  /** Submission block height */
  submitHeight: number;
  /** Voting end time (UNIX) */
  votingEndTime: number;
  /** Deposit amount */
  depositAmount: string;
}

/** An action that a proposal will execute if passed. */
interface ProposalAction {
  /** Message type URL */
  typeUrl: string;
  /** Message body */
  value: Record<string, unknown>;
  /** Human-readable description of what this action does */
  description: string;
}

/** Vote tally for a proposal. */
interface VoteTally {
  yesWeight: string;
  noWeight: string;
  abstainWeight: string;
  vetoWeight: string;
  totalWeight: string;
  /** Number of unique voters */
  voterCount: number;
  /** Quorum reached? */
  quorumMet: boolean;
  /** Pass threshold met? */
  thresholdMet: boolean;
}

/** A single vote record. */
interface Vote {
  proposalId: number;
  voter: string;
  option: VoteOption;
  /** The voter's effective weight at time of voting */
  weight: string;
  /** Block height when vote was cast */
  height: number;
}

/** Result of a DAO operation. */
interface DaoResult {
  txHash: string;
  action: string;
  proposalId?: number;
}

/** Breakdown of a voter's reputation-derived weight. */
interface WeightBreakdown {
  address: string;
  /** Raw contribution score from x/poc */
  cScore: number;
  /** Endorsement rate (0-1) */
  endorsementRate: number;
  /** Originality average (0-1) */
  originalityAvg: number;
  /** Uptime score (0-1) */
  uptimeScore: number;
  /** Longevity score (0-1) */
  longevityScore: number;
  /** Weight delegated from other addresses */
  delegatedWeight: number;
  /** Final composite weight */
  effectiveWeight: number;
  /** Explanation of how the weight was calculated */
  formula: string;
}

// ---------------------------------------------------------------------------
// OmniphiDAO -- reputation-weighted governance client
// ---------------------------------------------------------------------------

class OmniphiDAO {
  private client: OmniphiClient;
  private address: string;

  private constructor(client: OmniphiClient, address: string) {
    this.client = client;
    this.address = address;
  }

  static async connect(mnemonic: string): Promise<OmniphiDAO> {
    const wallet = await fromMnemonic(mnemonic);
    const client = await OmniphiClient.connectWithSigner(
      CONFIG.rpcEndpoint,
      wallet.signer,
      { restEndpoint: CONFIG.restEndpoint },
    );
    console.log(`[DAO] Connected as ${wallet.address}`);
    return new OmniphiDAO(client, wallet.address);
  }

  get walletAddress(): string {
    return this.address;
  }

  // -------------------------------------------------------------------------
  // Proposal operations
  // -------------------------------------------------------------------------

  /**
   * Create a governance proposal.
   *
   * TRADITIONAL DAO (Compound Governor):
   *   - Proposer must hold N tokens (e.g., 100,000 COMP)
   *   - Proposal threshold is purely based on token balance
   *   - A whale with tokens but zero contributions can propose anything
   *   - Flash loan: borrow 100K COMP, propose, repay -- proposal stands
   *
   * OMNIPHI DAO:
   *   - Proposer must have sufficient reputation weight (not just tokens)
   *   - Weight is computed from verified contributions, not token holdings
   *   - Cannot be flash-loaned: reputation is earned over time
   *   - Sybil-resistant: duplicate contributions are detected by x/poc
   *   - The Guard module's safety pipeline applies to sensitive proposals
   *
   * @param title       - Proposal title
   * @param description - Detailed proposal description
   * @param actions     - On-chain actions to execute if passed
   */
  async createProposal(
    title: string,
    description: string,
    actions: ProposalAction[],
  ): Promise<Proposal> {
    console.log(`[DAO] Creating proposal:`);
    console.log(`      Title: ${title}`);
    console.log(`      Actions: ${actions.length}`);

    // Check that the proposer has sufficient reputation weight.
    // This prevents token-only whales from creating proposals.
    const weight = await this.queryVoterWeight(this.address);
    console.log(`      Proposer weight: ${weight.effectiveWeight}`);

    if (parseFloat(weight.effectiveWeight) < parseFloat(CONFIG.minProposalWeight)) {
      throw new Error(
        `Insufficient reputation weight: ${weight.effectiveWeight} < ${CONFIG.minProposalWeight}. ` +
        `Earn more contribution score to create proposals.`,
      );
    }

    // Submit the proposal. On Omniphi, proposals that affect chain parameters
    // automatically go through the Guard module's safety pipeline, adding
    // time-delay and risk assessment before execution.
    const result = await this.client.signAndBroadcast(
      this.address,
      "/pos.repgov.v1.MsgSubmitProposal",
      {
        proposer: this.address,
        title,
        description,
        actions: actions.map((a) => ({
          type_url: a.typeUrl,
          value: a.value,
          description: a.description,
        })),
        initial_deposit: { denom: DENOM, amount: "10000000" },
      },
      DEFAULT_FEE,
      `dao:create_proposal:${title}`,
    );

    console.log(`      TX hash: ${result.transactionHash}`);

    // Parse the proposal ID from events (in production)
    const proposalId = 1; // extracted from result.events

    const proposal: Proposal = {
      proposalId,
      title,
      description,
      proposer: this.address,
      actions,
      status: "VOTING_PERIOD",
      tally: {
        yesWeight: "0",
        noWeight: "0",
        abstainWeight: "0",
        vetoWeight: "0",
        totalWeight: "0",
        voterCount: 0,
        quorumMet: false,
        thresholdMet: false,
      },
      submitHeight: 0,
      votingEndTime: Math.floor(Date.now() / 1000) + CONFIG.votingPeriod,
      depositAmount: "10000000",
    };

    console.log(`      Proposal ID: ${proposal.proposalId}`);
    console.log(`      Status: ${proposal.status}`);
    console.log(`      Voting ends: ${new Date(proposal.votingEndTime * 1000).toISOString()}`);

    return proposal;
  }

  /**
   * Vote on a proposal with reputation-weighted voting power.
   *
   * TRADITIONAL DAO:
   *   - Vote weight = number of tokens held
   *   - Weight can be manipulated by borrowing tokens
   *   - Delegation is token-based (delegate all your tokens' votes)
   *
   * OMNIPHI DAO:
   *   - Vote weight = composite reputation score from x/repgov
   *   - Weight includes: cScore, endorsements, originality, uptime, longevity
   *   - Cannot be borrowed or flash-loaned
   *   - Delegation is reputation-based (delegate your contribution reputation)
   *   - Weight is snapshotted at proposal creation to prevent gaming
   *
   * @param proposalId - The proposal to vote on
   * @param vote       - YES, NO, ABSTAIN, or NO_WITH_VETO
   * @param weight     - Override weight (if the voter wants to use partial weight)
   */
  async voteOnProposal(
    proposalId: number,
    vote: VoteOption,
    weight?: string,
  ): Promise<DaoResult> {
    console.log(`[DAO] Voting on proposal ${proposalId}:`);
    console.log(`      Option: ${vote}`);

    // Query the voter's current weight from x/repgov
    const voterWeight = await this.queryVoterWeight(this.address);
    const effectiveWeight = weight || voterWeight.effectiveWeight;

    console.log(`      Voter weight: ${effectiveWeight}`);
    console.log(`      Breakdown:`);
    console.log(`        cScore:          ${voterWeight.cScore}`);
    console.log(`        Endorsement:     ${voterWeight.endorsementRate}`);
    console.log(`        Originality:     ${voterWeight.originalityAvg}`);
    console.log(`        Uptime:          ${voterWeight.uptimeScore}`);
    console.log(`        Longevity:       ${voterWeight.longevityScore}`);
    console.log(`        Delegated:       ${voterWeight.delegatedWeight}`);

    const result = await this.client.signAndBroadcast(
      this.address,
      "/pos.repgov.v1.MsgVote",
      {
        voter: this.address,
        proposal_id: proposalId,
        option: vote,
        // Weight is automatically computed by the chain from x/repgov state.
        // Specifying a weight here would be used for partial voting only.
      },
      DEFAULT_FEE,
      `dao:vote:${proposalId}:${vote}`,
    );

    console.log(`      TX hash: ${result.transactionHash}`);

    return {
      txHash: result.transactionHash,
      action: `vote_${vote.toLowerCase()}`,
      proposalId,
    };
  }

  /**
   * Execute a passed proposal.
   *
   * For standard proposals, anyone can trigger execution once the voting
   * period ends and the proposal has passed.
   *
   * For proposals that modify chain parameters (Guard risk thresholds,
   * tokenomics parameters, etc.), execution is routed through the Guard
   * module's time-lock mechanism. This adds a delay proportional to the
   * risk tier of the change.
   *
   * @param proposalId - The proposal to execute
   */
  async executeProposal(proposalId: number): Promise<DaoResult> {
    console.log(`[DAO] Executing proposal ${proposalId}...`);

    const result = await this.client.signAndBroadcast(
      this.address,
      "/pos.repgov.v1.MsgExecuteProposal",
      {
        sender: this.address,
        proposal_id: proposalId,
      },
      DEFAULT_FEE,
      `dao:execute:${proposalId}`,
    );

    console.log(`      TX hash: ${result.transactionHash}`);
    console.log(`      Proposal actions executed on-chain`);

    return {
      txHash: result.transactionHash,
      action: "execute",
      proposalId,
    };
  }

  // -------------------------------------------------------------------------
  // Query operations
  // -------------------------------------------------------------------------

  /**
   * Query all proposals (active and historical).
   */
  async queryProposals(): Promise<Proposal[]> {
    console.log(`[DAO] Querying proposals...`);

    // In production: GET /pos/repgov/v1/proposals
    const proposals: Proposal[] = [
      {
        proposalId: 1,
        title: "Increase PoC reward multiplier bounds",
        description: "Expand the reward multiplier range from [0.85, 1.15] to [0.80, 1.20] to better incentivize high-quality contributions.",
        proposer: "omni1contributor_alice",
        actions: [{
          typeUrl: "/pos.rewardmult.v1.MsgUpdateParams",
          value: { min_multiplier: "0.80", max_multiplier: "1.20" },
          description: "Update rewardmult params",
        }],
        status: "VOTING_PERIOD",
        tally: {
          yesWeight: "15000",
          noWeight: "3000",
          abstainWeight: "2000",
          vetoWeight: "500",
          totalWeight: "20500",
          voterCount: 47,
          quorumMet: true,
          thresholdMet: true,
        },
        submitHeight: 100000,
        votingEndTime: Math.floor(Date.now() / 1000) + 120,
        depositAmount: "10000000",
      },
      {
        proposalId: 2,
        title: "Fund community developer grants program",
        description: "Allocate 500,000 OMNI from the community pool to fund developer grants for ecosystem tooling and documentation.",
        proposer: "omni1developer_bob",
        actions: [{
          typeUrl: "/cosmos.distribution.v1beta1.MsgCommunityPoolSpend",
          value: {
            authority: "omni10d07y265gmmuvt4z0w9aw880jnsr700j2gz0r8",
            recipient: "omni1grants_multisig",
            amount: [{ denom: DENOM, amount: "500000000000" }],
          },
          description: "Transfer from community pool to grants multisig",
        }],
        status: "PASSED",
        tally: {
          yesWeight: "25000",
          noWeight: "5000",
          abstainWeight: "3000",
          vetoWeight: "200",
          totalWeight: "33200",
          voterCount: 89,
          quorumMet: true,
          thresholdMet: true,
        },
        submitHeight: 95000,
        votingEndTime: Math.floor(Date.now() / 1000) - 3600,
        depositAmount: "10000000",
      },
      {
        proposalId: 3,
        title: "Register new DePIN adapter for solar network",
        description: "Add a UCI adapter for the SolarGrid network, enabling solar energy contributions to earn PoC rewards.",
        proposer: "omni1green_carol",
        actions: [{
          typeUrl: "/pos.uci.v1.MsgRegisterAdapter",
          value: {
            owner: "omni1solargrid_operator",
            name: "SolarGrid",
            network_type: "energy",
            reward_share: "0.15",
          },
          description: "Register SolarGrid DePIN adapter",
        }],
        status: "REJECTED",
        tally: {
          yesWeight: "8000",
          noWeight: "12000",
          abstainWeight: "1000",
          vetoWeight: "4000",
          totalWeight: "25000",
          voterCount: 62,
          quorumMet: true,
          thresholdMet: false,
        },
        submitHeight: 90000,
        votingEndTime: Math.floor(Date.now() / 1000) - 86400,
        depositAmount: "10000000",
      },
    ];

    console.log(`      Found ${proposals.length} proposals:`);
    for (const p of proposals) {
      console.log(`      [${p.proposalId}] ${p.status}: ${p.title}`);
      console.log(`           YES: ${p.tally.yesWeight} | NO: ${p.tally.noWeight} | ABSTAIN: ${p.tally.abstainWeight} | VETO: ${p.tally.vetoWeight}`);
    }

    return proposals;
  }

  /**
   * Query a voter's reputation-derived weight.
   *
   * This queries the x/repgov module, which computes weights from
   * Proof of Contribution data. The weight formula is:
   *
   *   effectiveWeight = cScore * endorsementRate * originalityAvg
   *                   * (uptimeScore * 0.3 + longevityScore * 0.2 + 0.5)
   *                   + delegatedWeight
   *
   * Each component is normalized to [0, 1] except cScore (unbounded)
   * and delegatedWeight (sum of delegations received).
   *
   * @param address - The voter's bech32 address
   */
  async queryVoterWeight(address: string): Promise<VoterWeight> {
    console.log(`[DAO] Querying voter weight: ${address.slice(0, 16)}...`);

    // In production, this calls the SDK's queryVoterWeight method
    // which hits: GET /pos/repgov/v1/voter_weight/{address}
    //
    // The weight is recomputed every epoch by the x/repgov EndBlocker,
    // using data from x/poc (contribution scores), x/poseq (node
    // performance), and staking (delegation state).

    const weight: VoterWeight = {
      address,
      epoch: 42,
      reputationScore: "850.5",
      cScore: "750.0",
      endorsementRate: "0.92",
      originalityAvg: "0.85",
      uptimeScore: "0.98",
      longevityScore: "0.75",
      compositeWeight: "520.35",
      effectiveWeight: "620.35",
      delegatedWeight: "100.0",
      lastVoteHeight: 99500,
    };

    return weight;
  }

  /**
   * Get a detailed breakdown of how a voter's weight is calculated.
   *
   * This is useful for transparency: voters can understand exactly
   * why they have the weight they do, and what they can do to earn more.
   */
  async getWeightBreakdown(address: string): Promise<WeightBreakdown> {
    const weight = await this.queryVoterWeight(address);

    const cScore = parseFloat(weight.cScore);
    const endorsement = parseFloat(weight.endorsementRate);
    const originality = parseFloat(weight.originalityAvg);
    const uptime = parseFloat(weight.uptimeScore);
    const longevity = parseFloat(weight.longevityScore);
    const delegated = parseFloat(weight.delegatedWeight);

    // Reproduce the on-chain formula
    const activityMultiplier = uptime * 0.3 + longevity * 0.2 + 0.5;
    const baseWeight = cScore * endorsement * originality * activityMultiplier;
    const effective = baseWeight + delegated;

    return {
      address,
      cScore,
      endorsementRate: endorsement,
      originalityAvg: originality,
      uptimeScore: uptime,
      longevityScore: longevity,
      delegatedWeight: delegated,
      effectiveWeight: effective,
      formula: `(${cScore} * ${endorsement} * ${originality} * (${uptime}*0.3 + ${longevity}*0.2 + 0.5)) + ${delegated} = ${effective.toFixed(2)}`,
    };
  }

  /**
   * Delegate reputation to another address.
   *
   * Unlike token delegation (which transfers voting power proportional
   * to tokens), reputation delegation transfers a portion of your
   * contribution-derived influence. This is useful for:
   *
   *   - Delegating to a trusted expert in a specific domain
   *   - Collective bargaining (pool reputation for larger voice)
   *   - Passive governance (delegate to someone who votes actively)
   *
   * Reputation delegation does NOT transfer tokens. Your token stake
   * remains unchanged. Only your vote weight in x/repgov is affected.
   *
   * @param delegatee - Address to delegate reputation to
   * @param amount    - Amount of reputation weight to delegate
   */
  async delegateReputation(
    delegatee: string,
    amount: string,
  ): Promise<DaoResult> {
    console.log(`[DAO] Delegating reputation:`);
    console.log(`      To:     ${delegatee.slice(0, 20)}...`);
    console.log(`      Amount: ${amount}`);

    const result = await this.client.delegateReputation(
      this.address,
      delegatee,
      amount,
      DEFAULT_FEE,
    );

    console.log(`      TX hash: ${result.transactionHash}`);

    return {
      txHash: result.transactionHash,
      action: "delegate_reputation",
    };
  }

  disconnect(): void {
    this.client.disconnect();
  }
}

// ---------------------------------------------------------------------------
// Main demonstration
// ---------------------------------------------------------------------------

async function main(): Promise<void> {
  console.log("============================================================");
  console.log("  Omniphi DAO -- Reputation-Weighted Governance");
  console.log("============================================================");
  console.log("");

  // ----- The problem with token-weighted voting -----
  console.log("--- The Problem with Token-Weighted Governance ---");
  console.log("");
  console.log("  Traditional DAOs: 1 token = 1 vote");
  console.log("");
  console.log("  ATTACK 1: Flash Loan Governance");
  console.log("    1. Borrow 1M COMP tokens via flash loan");
  console.log("    2. Submit proposal to drain treasury");
  console.log("    3. Vote YES with 1M tokens");
  console.log("    4. Repay flash loan -- all in one transaction");
  console.log("    Result: attacker controls governance with zero skin in the game");
  console.log("");
  console.log("  ATTACK 2: Whale Dominance");
  console.log("    - Top 10 addresses hold 60% of governance tokens");
  console.log("    - Small holders have no meaningful influence");
  console.log("    - Whales may have zero contribution to the protocol");
  console.log("    Result: plutocracy, not meritocracy");
  console.log("");
  console.log("  ATTACK 3: Vote Buying");
  console.log("    - Bribe platform: \"Vote YES on Prop 42, get 0.1 ETH\"");
  console.log("    - Token-weighted voting makes this economically rational");
  console.log("    Result: governance captured by highest bidder");
  console.log("");

  // ----- Omniphi's solution -----
  console.log("--- Omniphi's Solution: Reputation-Weighted Voting ---");
  console.log("");
  console.log("  Vote weight = f(contributions, endorsements, originality,");
  console.log("                   uptime, longevity) + delegated_reputation");
  console.log("");
  console.log("  Weight formula:");
  console.log("    effective = (cScore * endorsementRate * originalityAvg");
  console.log("               * (uptime*0.3 + longevity*0.2 + 0.5))");
  console.log("               + delegatedWeight");
  console.log("");
  console.log("  Why this works:");
  console.log("    - cScore: measures value of your verified contributions");
  console.log("    - endorsementRate: validators vouch for your work quality");
  console.log("    - originalityAvg: prevents copy-paste contribution spam");
  console.log("    - uptime: consistent participation, not one-time dumps");
  console.log("    - longevity: long-term commitment, not fly-by-night");
  console.log("    - Cannot be flash-loaned: reputation is earned over time");
  console.log("");

  console.log("--- Step 1: Create Wallet ---");
  const wallet = await createWallet();
  console.log(`  Address: ${wallet.address}`);
  console.log("");

  // ----- Voter weight breakdown -----
  console.log("--- Step 2: Check Voter Weight ---");
  console.log("");
  console.log("  const weight = await dao.queryVoterWeight(address);");
  console.log("");
  console.log("  Example voter weight breakdown:");
  console.log("    +---------------------+--------+");
  console.log("    | Component           | Value  |");
  console.log("    +---------------------+--------+");
  console.log("    | cScore              | 750.0  |");
  console.log("    | endorsementRate     | 0.92   |");
  console.log("    | originalityAvg      | 0.85   |");
  console.log("    | uptimeScore         | 0.98   |");
  console.log("    | longevityScore      | 0.75   |");
  console.log("    | delegatedWeight     | 100.0  |");
  console.log("    +---------------------+--------+");
  console.log("    | effectiveWeight     | 620.35 |");
  console.log("    +---------------------+--------+");
  console.log("");
  console.log("  Formula applied:");
  console.log("    (750 * 0.92 * 0.85 * (0.98*0.3 + 0.75*0.2 + 0.5)) + 100");
  console.log("    = (750 * 0.92 * 0.85 * 0.944) + 100");
  console.log("    = 520.35 + 100 = 620.35");
  console.log("");

  // ----- Create proposal -----
  console.log("--- Step 3: Create a Proposal ---");
  console.log("");
  console.log("  const proposal = await dao.createProposal(");
  console.log('    "Increase PoC reward multiplier bounds",');
  console.log('    "Expand the reward multiplier range from [0.85, 1.15] to...",');
  console.log("    [{");
  console.log('      typeUrl: "/pos.rewardmult.v1.MsgUpdateParams",');
  console.log('      value: { min_multiplier: "0.80", max_multiplier: "1.20" },');
  console.log('      description: "Widen reward multiplier bounds",');
  console.log("    }],");
  console.log("  );");
  console.log("");
  console.log("  Note: the proposer must have sufficient reputation weight.");
  console.log("  This prevents token-only whales from spamming proposals.");
  console.log("  Reputation is earned through contributions, not purchased.");
  console.log("");

  // ----- Vote -----
  console.log("--- Step 4: Vote on a Proposal ---");
  console.log("");
  console.log("  await dao.voteOnProposal(1, 'YES');");
  console.log("");
  console.log("  Your vote is weighted by your reputation score (620.35),");
  console.log("  not by how many tokens you hold.");
  console.log("");
  console.log("  Comparison for the same voter with 10,000 OMNI staked:");
  console.log("    Traditional DAO: vote weight = 10,000 (token count)");
  console.log("    Omniphi DAO:     vote weight = 620.35 (reputation)");
  console.log("");
  console.log("  A whale with 1M OMNI but zero contributions:");
  console.log("    Traditional DAO: vote weight = 1,000,000");
  console.log("    Omniphi DAO:     vote weight = 0 (no contributions)");
  console.log("");

  // ----- Execute -----
  console.log("--- Step 5: Execute a Passed Proposal ---");
  console.log("");
  console.log("  await dao.executeProposal(1);");
  console.log("");
  console.log("  For proposals that modify chain parameters, execution is");
  console.log("  routed through the Guard module's safety pipeline:");
  console.log("    VISIBILITY -> SHOCK_ABSORBER -> CONDITIONAL -> EXECUTED");
  console.log("");
  console.log("  High-risk changes (e.g., modifying Guard thresholds)");
  console.log("  trigger longer delays, giving the community time to react");
  console.log("  if a malicious proposal somehow passes.");
  console.log("");

  // ----- Reputation delegation -----
  console.log("--- Step 6: Delegate Reputation ---");
  console.log("");
  console.log("  await dao.delegateReputation('omni1expert...', '200');");
  console.log("");
  console.log("  Unlike token delegation:");
  console.log("    - Tokens stay in your wallet (no transfer)");
  console.log("    - Only governance weight is delegated");
  console.log("    - Delegatee's effective weight increases by 200");
  console.log("    - Your effective weight decreases by 200");
  console.log("    - Revocable at any time");
  console.log("");

  // ----- Query proposals -----
  console.log("--- Step 7: Query Proposals ---");
  console.log("");
  console.log("  const proposals = await dao.queryProposals();");
  console.log("");
  console.log("  Active proposals:");
  console.log("    [1] VOTING: Increase PoC reward multiplier bounds");
  console.log("        YES: 15000 | NO: 3000 | ABSTAIN: 2000 | VETO: 500");
  console.log("        Quorum: YES | Threshold: YES");
  console.log("");
  console.log("    [2] PASSED: Fund community developer grants program");
  console.log("        YES: 25000 | NO: 5000 | ABSTAIN: 3000 | VETO: 200");
  console.log("");
  console.log("    [3] REJECTED: Register SolarGrid DePIN adapter");
  console.log("        YES: 8000 | NO: 12000 | ABSTAIN: 1000 | VETO: 4000");
  console.log("        (Failed: NO votes exceeded threshold)");
  console.log("");

  // ----- Architecture -----
  console.log("============================================================");
  console.log("  Architecture: PoC-Integrated Governance");
  console.log("============================================================");
  console.log("");
  console.log("  x/poc                 x/repgov                 Governance");
  console.log("  (contributions)       (weight computation)     (proposals)");
  console.log("       |                       |                      |");
  console.log("       | cScore, endorsements  |                      |");
  console.log("       | originality, uptime   |                      |");
  console.log("       +-----> epoch weight -->+----> vote weight --->+");
  console.log("                               |                      |");
  console.log("  x/poseq                      |                      |");
  console.log("  (node perf)                  |                      |");
  console.log("       |                       |                      |");
  console.log("       | attestations, uptime  |                      |");
  console.log("       +-----> multiplier ---->+                      |");
  console.log("                               |                      |");
  console.log("  Delegations                  |                      |");
  console.log("       |                       |                      |");
  console.log("       +-----> add weight ---->+                      |");
  console.log("                                                      |");
  console.log("  x/guard                                             |");
  console.log("  (safety)                                            |");
  console.log("       |                                              |");
  console.log("       +---- risk assessment on param changes ------->+");
  console.log("");
  console.log("  Weight is recomputed every epoch (configurable, e.g., daily).");
  console.log("  Votes use the weight snapshot at proposal creation time");
  console.log("  to prevent gaming during the voting period.");
  console.log("");
  console.log("============================================================");
  console.log("  Example complete. See README.md for full documentation.");
  console.log("============================================================");
}

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
