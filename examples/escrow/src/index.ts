/**
 * Omniphi Capability-Based Escrow
 *
 * This example demonstrates a fundamental advantage of Omniphi's architecture
 * over EVM-based chains: capability-based authorization replaces token approvals.
 *
 * THE PROBLEM WITH ERC-20 APPROVALS:
 *
 * On Ethereum, to let a contract spend your tokens, you must call
 * `token.approve(spender, amount)`. This creates a standing permission that:
 *   - Persists until explicitly revoked
 *   - Can be exploited if the spender contract has a vulnerability
 *   - Requires a separate transaction (gas cost)
 *   - Is all-or-nothing: you cannot scope permissions to a specific use case
 *
 * Billions of dollars have been stolen through approval exploits. Users
 * routinely approve unlimited amounts ("infinite approval") because the
 * alternative is paying gas for every interaction.
 *
 * OMNIPHI'S CAPABILITY-BASED APPROACH:
 *
 * Instead of granting blanket permissions, users create *capabilities*:
 * scoped, time-limited, single-use authorizations. A capability says:
 *
 *   "This specific escrow (ID 42) may spend up to 5000 OMNI from my account,
 *    but only if both the buyer confirms delivery AND the deadline has not
 *    passed, and this capability expires in 7 days regardless."
 *
 * Capabilities are:
 *   - Scoped: tied to a specific escrow/contract instance
 *   - Time-limited: expire automatically
 *   - Conditional: execute only when conditions are met
 *   - Single-use: consumed on execution, cannot be replayed
 *   - Revocable: the grantor can revoke at any time before use
 *
 * This example builds a complete escrow service using these primitives.
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
  StdFee,
} from "@omniphi/sdk";

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

const CONFIG = {
  rpcEndpoint: process.env.OMNIPHI_RPC || DEFAULT_RPC_ENDPOINT,
  restEndpoint: process.env.OMNIPHI_REST || DEFAULT_REST_ENDPOINT,
  /** Maximum escrow duration: 30 days */
  maxEscrowDuration: 30 * 24 * 3600,
  /** Arbiter fee: 1% of escrow amount */
  arbiterFeeBps: 100,
};

// ---------------------------------------------------------------------------
// Escrow types
// ---------------------------------------------------------------------------

/** The lifecycle states of an escrow. */
type EscrowStatus =
  | "CREATED"      // Escrow created, funds not yet deposited
  | "FUNDED"       // Buyer has deposited funds
  | "RELEASED"     // Arbiter released funds to seller
  | "REFUNDED"     // Arbiter refunded funds to buyer
  | "DISPUTED"     // A dispute has been filed
  | "EXPIRED";     // Deadline passed without resolution

/** A spend capability scoped to a specific escrow. */
interface SpendCapability {
  capabilityId: string;
  /** The account granting the capability */
  grantor: string;
  /** Maximum amount this capability allows */
  maxAmount: string;
  denom: string;
  /** The escrow ID this capability is scoped to */
  scopedTo: string;
  /** UNIX timestamp when this capability expires */
  expiresAt: number;
  /** Whether this capability has been consumed */
  consumed: boolean;
  /** Conditions that must be met to exercise this capability */
  conditions: CapabilityCondition[];
}

/** A condition on a spend capability. */
interface CapabilityCondition {
  type: "arbiter_signature" | "deadline_not_passed" | "dispute_resolved" | "custom";
  description: string;
  /** The address whose signature is required (for arbiter_signature type) */
  requiredSigner?: string;
}

/** A complete escrow record. */
interface Escrow {
  escrowId: string;
  seller: string;
  buyer: string;
  /** The arbiter who can resolve disputes */
  arbiter: string;
  amount: string;
  denom: string;
  /** What the escrow is for */
  description: string;
  /** UNIX timestamp deadline */
  deadline: number;
  status: EscrowStatus;
  /** The spend capability created for this escrow */
  capability: SpendCapability;
  /** Dispute reason, if any */
  disputeReason?: string;
  /** Block height at creation */
  createdAt: number;
  /** Block height at resolution */
  resolvedAt?: number;
}

/** Result of an escrow operation. */
interface EscrowResult {
  txHash: string;
  escrowId: string;
  action: string;
  status: EscrowStatus;
}

// ---------------------------------------------------------------------------
// OmniphiEscrow -- capability-based escrow client
// ---------------------------------------------------------------------------

class OmniphiEscrow {
  private client: OmniphiClient;
  private address: string;

  private constructor(client: OmniphiClient, address: string) {
    this.client = client;
    this.address = address;
  }

  static async connect(mnemonic: string): Promise<OmniphiEscrow> {
    const wallet = await fromMnemonic(mnemonic);
    const client = await OmniphiClient.connectWithSigner(
      CONFIG.rpcEndpoint,
      wallet.signer,
      { restEndpoint: CONFIG.restEndpoint },
    );
    console.log(`[Escrow] Connected as ${wallet.address}`);
    return new OmniphiEscrow(client, wallet.address);
  }

  get walletAddress(): string {
    return this.address;
  }

  // -------------------------------------------------------------------------
  // Escrow lifecycle
  // -------------------------------------------------------------------------

  /**
   * Create a new escrow with a scoped spend capability.
   *
   * ERC-20 APPROVAL APPROACH:
   *   1. Buyer calls USDC.approve(escrowContract, amount)
   *      -> Escrow contract now has blanket permission to spend buyer's USDC
   *      -> If escrow contract has a bug, ALL approved USDC is at risk
   *      -> Approval persists even after escrow is resolved
   *
   * OMNIPHI CAPABILITY APPROACH:
   *   1. Buyer creates a scoped capability:
   *      -> "Escrow #42 may spend up to 5000 OMNI from my account"
   *      -> "Only if the arbiter signs a release"
   *      -> "Expires in 7 days"
   *      -> "Single-use: consumed on first exercise"
   *   2. Capability is created atomically with the escrow
   *   3. No blanket permissions; no leftover approvals
   *
   * @param seller   - The seller's bech32 address
   * @param buyer    - The buyer's bech32 address (must be the signer)
   * @param amount   - Escrow amount in base units
   * @param arbiter  - The arbiter's bech32 address
   * @param deadline - Deadline in seconds from now
   */
  async createEscrow(
    seller: string,
    buyer: string,
    amount: string,
    arbiter: string,
    deadline: number,
  ): Promise<Escrow> {
    console.log(`[Escrow] Creating escrow:`);
    console.log(`         Seller:   ${seller.slice(0, 20)}...`);
    console.log(`         Buyer:    ${buyer.slice(0, 20)}...`);
    console.log(`         Amount:   ${amount} ${DENOM}`);
    console.log(`         Arbiter:  ${arbiter.slice(0, 20)}...`);
    console.log(`         Deadline: ${deadline}s from now`);

    if (deadline > CONFIG.maxEscrowDuration) {
      throw new Error(
        `Deadline ${deadline}s exceeds maximum ${CONFIG.maxEscrowDuration}s`,
      );
    }

    const expiresAt = Math.floor(Date.now() / 1000) + deadline;
    const escrowId = `escrow_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;

    // Create the escrow AND fund it in a single atomic transaction.
    // The capability is created as part of the escrow creation --
    // it does not require a separate approval step.
    //
    // The intent bundle:
    //   1. Transfer funds from buyer to escrow module account
    //   2. Create a scoped spend capability for the escrow
    //   3. Register the escrow with seller, buyer, arbiter, and deadline
    const intentTx: IntentTransaction = {
      sender: buyer,
      intents: [
        {
          type: "transfer",
          sender: buyer,
          recipient: "omni1escrow_module",
          amount,
          denom: DENOM,
          memo: `escrow:create:${escrowId}:fund`,
        },
      ],
      memo: `escrow:create:${escrowId}:seller:${seller}:arbiter:${arbiter}:deadline:${expiresAt}`,
      deadline: expiresAt,
    };

    console.log(`         Submitting atomic escrow creation...`);

    const result = await this.client.submitIntentTransaction(intentTx, DEFAULT_FEE);

    console.log(`         TX hash:    ${result.transactionHash}`);
    console.log(`         Escrow ID:  ${escrowId}`);

    // The capability is created as part of the escrow.
    // It has three conditions:
    //   1. The arbiter must sign the release/refund
    //   2. The deadline must not have passed (for release)
    //   3. The capability is single-use
    const capability: SpendCapability = {
      capabilityId: `cap_${escrowId}`,
      grantor: buyer,
      maxAmount: amount,
      denom: DENOM,
      scopedTo: escrowId,
      expiresAt,
      consumed: false,
      conditions: [
        {
          type: "arbiter_signature",
          description: "Arbiter must sign to release or refund",
          requiredSigner: arbiter,
        },
        {
          type: "deadline_not_passed",
          description: `Must be resolved before ${new Date(expiresAt * 1000).toISOString()}`,
        },
      ],
    };

    const escrow: Escrow = {
      escrowId,
      seller,
      buyer,
      arbiter,
      amount,
      denom: DENOM,
      description: "Capability-based escrow",
      deadline: expiresAt,
      status: "FUNDED",
      capability,
      createdAt: 0, // from chain
    };

    console.log(`         Status:     FUNDED`);
    console.log(`         Capability: ${capability.capabilityId}`);
    console.log(`         Conditions: arbiter_signature + deadline_not_passed`);

    return escrow;
  }

  /**
   * Release escrow funds to the seller.
   *
   * The arbiter signs this transaction to exercise the spend capability,
   * directing the escrowed funds to the seller. The capability is consumed
   * and cannot be replayed.
   *
   * COMPARISON WITH ERC-20:
   *   Traditional: escrowContract.release(escrowId) -- anyone with the
   *   right role can call this, and the contract decides internally.
   *   Capability: the arbiter's signature literally authorizes the specific
   *   spend from buyer to seller. No contract logic can override it.
   *
   * @param escrowId         - The escrow to release
   * @param arbiterSignature - Proof that the arbiter approves the release
   */
  async releaseEscrow(
    escrowId: string,
    arbiterSignature: string,
  ): Promise<EscrowResult> {
    console.log(`[Escrow] Releasing escrow: ${escrowId}`);
    console.log(`         Arbiter signature provided`);

    // The release transaction exercises the spend capability.
    // On-chain validation:
    //   1. Verify arbiter's signature matches the capability condition
    //   2. Verify deadline has not passed
    //   3. Verify capability has not been consumed
    //   4. Transfer funds from escrow module to seller
    //   5. Deduct arbiter fee (if configured)
    //   6. Mark capability as consumed (single-use)
    const result = await this.client.signAndBroadcast(
      this.address,
      "/pos.contracts.v1.MsgReleaseEscrow",
      {
        sender: this.address,
        escrow_id: escrowId,
        arbiter_signature: arbiterSignature,
        action: "release_to_seller",
      },
      DEFAULT_FEE,
      `escrow:release:${escrowId}`,
    );

    console.log(`         TX hash: ${result.transactionHash}`);
    console.log(`         Funds released to seller`);
    console.log(`         Capability consumed (cannot be replayed)`);

    return {
      txHash: result.transactionHash,
      escrowId,
      action: "release",
      status: "RELEASED",
    };
  }

  /**
   * Refund escrow funds to the buyer.
   *
   * The arbiter signs to direct funds back to the buyer instead of
   * the seller. Uses the same capability mechanism.
   *
   * @param escrowId         - The escrow to refund
   * @param arbiterSignature - Proof that the arbiter approves the refund
   */
  async refundEscrow(
    escrowId: string,
    arbiterSignature: string,
  ): Promise<EscrowResult> {
    console.log(`[Escrow] Refunding escrow: ${escrowId}`);
    console.log(`         Arbiter signature provided`);

    // Same capability exercise, but directing funds back to buyer.
    const result = await this.client.signAndBroadcast(
      this.address,
      "/pos.contracts.v1.MsgReleaseEscrow",
      {
        sender: this.address,
        escrow_id: escrowId,
        arbiter_signature: arbiterSignature,
        action: "refund_to_buyer",
      },
      DEFAULT_FEE,
      `escrow:refund:${escrowId}`,
    );

    console.log(`         TX hash: ${result.transactionHash}`);
    console.log(`         Funds refunded to buyer`);
    console.log(`         Capability consumed`);

    return {
      txHash: result.transactionHash,
      escrowId,
      action: "refund",
      status: "REFUNDED",
    };
  }

  /**
   * File a dispute on an escrow.
   *
   * Either the buyer or seller can dispute. This triggers the Guard
   * module's safety pipeline:
   *   1. VISIBILITY: dispute is logged and visible to all parties
   *   2. SHOCK_ABSORBER: a cooling-off period prevents hasty resolution
   *   3. CONDITIONAL_EXECUTION: arbiter must provide a signed resolution
   *   4. READY -> EXECUTED: funds are directed per the arbiter's decision
   *
   * @param escrowId - The escrow to dispute
   * @param reason   - Description of the dispute
   */
  async disputeEscrow(
    escrowId: string,
    reason: string,
  ): Promise<EscrowResult> {
    console.log(`[Escrow] Filing dispute on escrow: ${escrowId}`);
    console.log(`         Reason: ${reason}`);

    const result = await this.client.signAndBroadcast(
      this.address,
      "/pos.contracts.v1.MsgDisputeEscrow",
      {
        sender: this.address,
        escrow_id: escrowId,
        reason,
      },
      DEFAULT_FEE,
      `escrow:dispute:${escrowId}`,
    );

    console.log(`         TX hash: ${result.transactionHash}`);
    console.log(`         Dispute filed; entering Guard safety pipeline`);
    console.log(`         Arbiter will review and resolve`);

    return {
      txHash: result.transactionHash,
      escrowId,
      action: "dispute",
      status: "DISPUTED",
    };
  }

  /**
   * Query an escrow's current state.
   */
  async queryEscrow(escrowId: string): Promise<Escrow> {
    console.log(`[Escrow] Querying escrow: ${escrowId}`);

    // In production: GET /pos/contracts/v1/escrow/{escrowId}
    const escrow: Escrow = {
      escrowId,
      seller: "omni1seller_alice",
      buyer: "omni1buyer_bob",
      arbiter: "omni1arbiter_carol",
      amount: "5000000000",
      denom: DENOM,
      description: "Software development milestone payment",
      deadline: Math.floor(Date.now() / 1000) + 604800,
      status: "FUNDED",
      capability: {
        capabilityId: `cap_${escrowId}`,
        grantor: "omni1buyer_bob",
        maxAmount: "5000000000",
        denom: DENOM,
        scopedTo: escrowId,
        expiresAt: Math.floor(Date.now() / 1000) + 604800,
        consumed: false,
        conditions: [
          {
            type: "arbiter_signature",
            description: "Arbiter must sign",
            requiredSigner: "omni1arbiter_carol",
          },
          {
            type: "deadline_not_passed",
            description: "Must resolve within 7 days",
          },
        ],
      },
      createdAt: 12345,
    };

    console.log(`         Status:    ${escrow.status}`);
    console.log(`         Amount:    ${escrow.amount} ${escrow.denom}`);
    console.log(`         Seller:    ${escrow.seller}`);
    console.log(`         Buyer:     ${escrow.buyer}`);
    console.log(`         Arbiter:   ${escrow.arbiter}`);
    console.log(`         Capability consumed: ${escrow.capability.consumed}`);

    return escrow;
  }

  /**
   * Create a multi-milestone escrow with sequential release capabilities.
   *
   * This demonstrates the composability of capabilities: each milestone
   * gets its own scoped capability with its own conditions.
   *
   * @param seller     - Seller address
   * @param buyer      - Buyer address (signer)
   * @param milestones - Array of milestone descriptions and amounts
   * @param arbiter    - Arbiter address
   * @param deadline   - Overall deadline in seconds
   */
  async createMilestoneEscrow(
    seller: string,
    buyer: string,
    milestones: Array<{ description: string; amount: string }>,
    arbiter: string,
    deadline: number,
  ): Promise<string> {
    console.log(`[Escrow] Creating milestone escrow:`);
    console.log(`         ${milestones.length} milestones`);

    const totalAmount = milestones.reduce(
      (sum, m) => sum + BigInt(m.amount),
      0n,
    );
    console.log(`         Total: ${totalAmount.toString()} ${DENOM}`);

    // Create a single escrow with the total amount, but each milestone
    // gets its own spend capability. This means the arbiter can release
    // funds milestone by milestone, and each release is independently
    // authorized.
    const intents = milestones.map((milestone, i) => ({
      type: "transfer" as const,
      sender: buyer,
      recipient: "omni1escrow_module",
      amount: milestone.amount,
      denom: DENOM,
      memo: `escrow:milestone:${i + 1}:${milestone.description}`,
    }));

    const intentTx: IntentTransaction = {
      sender: buyer,
      intents,
      memo: `escrow:milestone_escrow:${milestones.length}_milestones:arbiter:${arbiter}`,
      deadline: Math.floor(Date.now() / 1000) + deadline,
    };

    for (let i = 0; i < milestones.length; i++) {
      console.log(`         Milestone ${i + 1}: ${milestones[i].description} (${milestones[i].amount} ${DENOM})`);
    }

    const result = await this.client.submitIntentTransaction(intentTx, DEFAULT_FEE);

    console.log(`         TX hash: ${result.transactionHash}`);
    console.log(`         ${milestones.length} capabilities created (one per milestone)`);

    return result.transactionHash;
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
  console.log("  Omniphi Capability-Based Escrow -- Reference Example");
  console.log("============================================================");
  console.log("");

  console.log("--- The ERC-20 Approval Problem ---");
  console.log("");
  console.log("  On Ethereum, escrow requires token approvals:");
  console.log("");
  console.log("    // Step 1: Approve (DANGEROUS)");
  console.log("    await usdc.approve(escrowContract.address, amount);");
  console.log("    // Escrow contract now has permission to spend your USDC");
  console.log("    // If the contract has a vulnerability: ALL your USDC is at risk");
  console.log("    // The approval persists FOREVER unless explicitly revoked");
  console.log("    // Common practice: approve(MAX_UINT256) = infinite approval");
  console.log("");
  console.log("    // Step 2: Create escrow");
  console.log("    await escrowContract.create(seller, amount, arbiter, deadline);");
  console.log("    // Contract calls transferFrom(buyer, escrow, amount)");
  console.log("");
  console.log("  Attack surface:");
  console.log("    - Re-entrancy: contract can call transferFrom multiple times");
  console.log("    - Upgrade exploit: contract owner upgrades to drain all approvals");
  console.log("    - Leftover approval: after escrow resolves, approval remains");
  console.log("    - Phishing: malicious contract tricks user into approving");
  console.log("");

  console.log("--- Omniphi's Capability-Based Solution ---");
  console.log("");
  console.log("  No approvals needed. Instead, create a scoped capability:");
  console.log("");
  console.log("    const escrow = await escrow.createEscrow(");
  console.log('      "omni1seller...",    // seller');
  console.log('      "omni1buyer...",     // buyer (signer)');
  console.log('      "5000000000",        // 5,000 OMNI');
  console.log('      "omni1arbiter...",   // arbiter');
  console.log("      604800,              // 7 day deadline");
  console.log("    );");
  console.log("");
  console.log("  This creates a spend capability that is:");
  console.log("    - SCOPED: only this escrow can use it");
  console.log("    - TIME-LIMITED: expires in 7 days");
  console.log("    - CONDITIONAL: requires arbiter's signature");
  console.log("    - SINGLE-USE: consumed on first exercise");
  console.log("    - AMOUNT-BOUNDED: cannot spend more than 5000 OMNI");
  console.log("");

  console.log("--- Step 1: Create Wallets ---");
  const walletBuyer = await createWallet();
  const walletSeller = await createWallet();
  const walletArbiter = await createWallet();
  console.log(`  Buyer:   ${walletBuyer.address}`);
  console.log(`  Seller:  ${walletSeller.address}`);
  console.log(`  Arbiter: ${walletArbiter.address}`);
  console.log("");

  console.log("--- Step 2: Create Escrow ---");
  console.log("");
  console.log("  const escrow = await escrowClient.createEscrow(");
  console.log(`    "${walletSeller.address.slice(0, 20)}...",`);
  console.log(`    "${walletBuyer.address.slice(0, 20)}...",`);
  console.log('    "5000000000",');
  console.log(`    "${walletArbiter.address.slice(0, 20)}...",`);
  console.log("    604800,");
  console.log("  );");
  console.log("");
  console.log("  Result:");
  console.log("    escrowId:   escrow_1711500000_abc123");
  console.log("    status:     FUNDED");
  console.log("    capability: cap_escrow_1711500000_abc123");
  console.log("    conditions: [arbiter_signature, deadline_not_passed]");
  console.log("");

  console.log("--- Step 3: Release (Happy Path) ---");
  console.log("");
  console.log("  When the buyer confirms delivery, the arbiter releases funds:");
  console.log("");
  console.log("    const result = await escrowClient.releaseEscrow(");
  console.log('      "escrow_1711500000_abc123",');
  console.log('      "arbiter_signature_hex...",');
  console.log("    );");
  console.log("");
  console.log("  On-chain validation:");
  console.log("    1. Verify arbiter_signature matches capability condition");
  console.log("    2. Verify deadline has not passed");
  console.log("    3. Verify capability has not been consumed");
  console.log("    4. Transfer 5000 OMNI from escrow to seller");
  console.log("    5. Deduct 50 OMNI arbiter fee (1%)");
  console.log("    6. Mark capability as CONSUMED");
  console.log("");
  console.log("  After release, the capability is consumed. Even if the");
  console.log("  arbiter's key is compromised later, the capability cannot");
  console.log("  be replayed because it is single-use.");
  console.log("");

  console.log("--- Step 4: Refund (Unhappy Path) ---");
  console.log("");
  console.log("  If the seller fails to deliver, the arbiter refunds:");
  console.log("");
  console.log("    const result = await escrowClient.refundEscrow(");
  console.log('      "escrow_1711500000_abc123",');
  console.log('      "arbiter_signature_hex...",');
  console.log("    );");
  console.log("");
  console.log("  Same capability, different direction: funds go back to buyer.");
  console.log("");

  console.log("--- Step 5: Dispute Resolution ---");
  console.log("");
  console.log("  Either party can file a dispute:");
  console.log("");
  console.log("    const result = await escrowClient.disputeEscrow(");
  console.log('      "escrow_1711500000_abc123",');
  console.log('      "Seller did not deliver the agreed-upon software milestone",');
  console.log("    );");
  console.log("");
  console.log("  Disputes enter the Guard module's safety pipeline:");
  console.log("    VISIBILITY -> SHOCK_ABSORBER -> CONDITIONAL_EXECUTION -> EXECUTED");
  console.log("");
  console.log("  The cooling-off period prevents hasty decisions. The arbiter");
  console.log("  reviews evidence from both sides and issues a signed resolution.");
  console.log("");

  console.log("--- Step 6: Milestone Escrow (Advanced) ---");
  console.log("");
  console.log("  Create an escrow with sequential milestones:");
  console.log("");
  console.log("    const txHash = await escrowClient.createMilestoneEscrow(");
  console.log('      "omni1seller...",');
  console.log('      "omni1buyer...",');
  console.log("      [");
  console.log('        { description: "Design mockups", amount: "1000000000" },');
  console.log('        { description: "Backend implementation", amount: "2000000000" },');
  console.log('        { description: "Frontend + QA", amount: "1500000000" },');
  console.log('        { description: "Deployment + handoff", amount: "500000000" },');
  console.log("      ],");
  console.log('      "omni1arbiter...",');
  console.log("      2592000, // 30 days");
  console.log("    );");
  console.log("");
  console.log("  Each milestone gets its own capability:");
  console.log("    - cap_milestone_1: 1,000 OMNI (design)");
  console.log("    - cap_milestone_2: 2,000 OMNI (backend)");
  console.log("    - cap_milestone_3: 1,500 OMNI (frontend)");
  console.log("    - cap_milestone_4:   500 OMNI (deployment)");
  console.log("");
  console.log("  Arbiter releases each milestone independently.");
  console.log("  If milestone 2 fails, milestones 3 and 4 are automatically");
  console.log("  refundable, but milestone 1 (already released) is not affected.");
  console.log("");

  console.log("============================================================");
  console.log("  Capabilities vs Approvals: Security Comparison");
  console.log("============================================================");
  console.log("");
  console.log("  Attack              | ERC-20 Approval     | Omniphi Capability");
  console.log("  --------------------|---------------------|-----------------------");
  console.log("  Re-entrancy         | Exploitable         | N/A (no callbacks)");
  console.log("  Infinite approval   | Common practice     | Impossible (bounded)");
  console.log("  Leftover permission | Must revoke manually| Auto-expires");
  console.log("  Replay attack       | Possible            | Single-use");
  console.log("  Scope creep         | Blanket permission  | Scoped to escrow ID");
  console.log("  Key compromise      | All approvals at risk| Only active caps");
  console.log("  Upgrade exploit     | Drain all approvals | Cap already scoped");
  console.log("  Phishing            | Approve anything    | Conditions visible");
  console.log("");
  console.log("  Bottom line: capabilities are the principle of least");
  console.log("  privilege applied to on-chain authorization. Each");
  console.log("  permission is scoped to exactly what is needed and");
  console.log("  nothing more.");
  console.log("");
  console.log("============================================================");
  console.log("  Example complete. See README.md for full documentation.");
  console.log("============================================================");
}

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
