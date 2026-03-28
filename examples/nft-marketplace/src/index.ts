/**
 * Omniphi Programmable Ownership Marketplace
 *
 * Omniphi replaces static NFTs with Programmable Ownership Objects -- digital
 * assets whose ownership rules are defined by executable logic, not just a
 * mapping from token ID to address.
 *
 * Traditional NFT marketplaces (OpenSea, Blur) work with ERC-721 tokens that
 * have a single owner field and no built-in rules. All marketplace logic lives
 * in separate smart contracts (Seaport, Wyvern), creating a fragmented system
 * where listing, bidding, and settlement are disconnected operations.
 *
 * Omniphi's approach:
 *   - Ownership objects carry their own transfer rules (time-locks, conditions,
 *     royalty enforcement, split ownership)
 *   - Buying and selling happen through intents, not direct transfers
 *   - The marketplace is a protocol layer, not a monolithic contract
 *   - Solvers match buyers and sellers, optimizing for price and conditions
 *
 * This example demonstrates:
 *   1. Listing an ownership object for sale with configurable conditions
 *   2. Buying through purchase intents (solver-matched)
 *   3. Making and accepting offers
 *   4. Time-locked and conditional transfers
 *   5. Querying listings and offer books
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
};

// ---------------------------------------------------------------------------
// Ownership and marketplace types
// ---------------------------------------------------------------------------

/** A programmable ownership object on the Omniphi chain. */
interface OwnershipObject {
  objectId: string;
  /** Current owner address */
  owner: string;
  /** The creator who originally minted this object */
  creator: string;
  /** Human-readable name */
  name: string;
  /** Content-addressed metadata (IPFS CID or similar) */
  metadataUri: string;
  /** SHA-256 hash of the metadata for integrity */
  metadataHash: string;
  /** Royalty percentage paid to creator on each transfer (basis points) */
  royaltyBps: number;
  /** Transfer rules encoded in the object itself */
  transferRules: TransferRules;
  /** Creation block height */
  createdAt: number;
  /** Last transfer block height */
  lastTransferAt: number;
}

/** Rules governing how this object can be transferred. */
interface TransferRules {
  /** If true, transfers require a minimum holding period */
  timelockEnabled: boolean;
  /** Minimum seconds the owner must hold before transferring */
  timelockDuration: number;
  /** If true, the creator must approve each transfer */
  creatorApprovalRequired: boolean;
  /** If set, transfers are restricted to addresses in this list */
  allowedRecipients: string[];
  /** Maximum number of transfers allowed (0 = unlimited) */
  maxTransfers: number;
  /** Current transfer count */
  transferCount: number;
  /** If true, the object can be split into fractional shares */
  fractionalizable: boolean;
}

/** A listing on the marketplace. */
interface Listing {
  listingId: string;
  objectId: string;
  seller: string;
  /** Asking price in base units */
  price: string;
  priceDenom: string;
  /** Listing expiry as UNIX timestamp */
  expiresAt: number;
  /** Whether the seller accepts offers below the asking price */
  acceptsOffers: boolean;
  /** Minimum offer the seller will consider (0 = any) */
  minOffer: string;
  /** Listing creation timestamp */
  createdAt: number;
  active: boolean;
}

/** An offer on a listed (or unlisted) object. */
interface Offer {
  offerId: string;
  objectId: string;
  buyer: string;
  /** Offered price in base units */
  price: string;
  priceDenom: string;
  /** Offer expiry as UNIX timestamp */
  expiresAt: number;
  /** Optional conditions attached to the offer */
  conditions: OfferCondition[];
  createdAt: number;
  active: boolean;
}

/** A condition that must be met for an offer to execute. */
interface OfferCondition {
  type: "min_holding_period" | "metadata_verified" | "royalty_paid" | "custom";
  /** Human-readable description */
  description: string;
  /** Condition parameter (interpretation depends on type) */
  value: string;
}

/** Result of a marketplace transaction. */
interface MarketplaceResult {
  txHash: string;
  action: string;
  objectId: string;
  from: string;
  to: string;
  price: string;
  royaltyPaid: string;
}

// ---------------------------------------------------------------------------
// OmniphiMarketplace -- programmable ownership marketplace client
// ---------------------------------------------------------------------------

class OmniphiMarketplace {
  private client: OmniphiClient;
  private address: string;

  private constructor(client: OmniphiClient, address: string) {
    this.client = client;
    this.address = address;
  }

  static async connect(mnemonic: string): Promise<OmniphiMarketplace> {
    const wallet = await fromMnemonic(mnemonic);
    const client = await OmniphiClient.connectWithSigner(
      CONFIG.rpcEndpoint,
      wallet.signer,
      { restEndpoint: CONFIG.restEndpoint },
    );
    console.log(`[Marketplace] Connected as ${wallet.address}`);
    return new OmniphiMarketplace(client, wallet.address);
  }

  get walletAddress(): string {
    return this.address;
  }

  // -------------------------------------------------------------------------
  // Listing operations
  // -------------------------------------------------------------------------

  /**
   * List an ownership object for sale on the marketplace.
   *
   * TRADITIONAL NFT MARKETPLACE (OpenSea):
   *   1. Approve the marketplace contract to transfer your NFT (setApprovalForAll)
   *   2. Sign an off-chain order (EIP-712 signature)
   *   3. Order is stored on the marketplace's centralized server
   *   4. Buyer calls fulfillOrder() -- marketplace transfers NFT + payment
   *   5. Royalties are optional and easily bypassed
   *
   * OMNIPHI PROGRAMMABLE OWNERSHIP:
   *   1. Submit a listing intent directly to the chain
   *   2. No approval needed -- the object's transfer rules enforce conditions
   *   3. Listing is on-chain, decentralized, and censorship-resistant
   *   4. Royalties are enforced by the object itself, not the marketplace
   *   5. Solver network matches buyers automatically
   *
   * @param objectId  - The ownership object to list
   * @param price     - Asking price in base units
   * @param duration  - Listing duration in seconds
   */
  async listForSale(
    objectId: string,
    price: string,
    duration: number,
  ): Promise<Listing> {
    console.log(`[Marketplace] Listing object for sale:`);
    console.log(`              Object: ${objectId}`);
    console.log(`              Price:  ${price} ${DENOM}`);
    console.log(`              Duration: ${duration}s`);

    const expiresAt = Math.floor(Date.now() / 1000) + duration;

    // Submit the listing as a chain message. The object remains in the
    // seller's possession until a buyer's intent is matched and settled.
    // No approval is needed because the object's transfer rules are
    // evaluated at settlement time, not listing time.
    const result = await this.client.signAndBroadcast(
      this.address,
      "/pos.contracts.v1.MsgListOwnershipObject",
      {
        seller: this.address,
        object_id: objectId,
        price: { denom: DENOM, amount: price },
        expires_at: expiresAt,
        accepts_offers: true,
        min_offer: "0",
      },
      DEFAULT_FEE,
      `marketplace:list:${objectId}`,
    );

    console.log(`              TX hash: ${result.transactionHash}`);

    const listing: Listing = {
      listingId: `listing_${objectId}_${Date.now()}`,
      objectId,
      seller: this.address,
      price,
      priceDenom: DENOM,
      expiresAt,
      acceptsOffers: true,
      minOffer: "0",
      createdAt: Math.floor(Date.now() / 1000),
      active: true,
    };

    console.log(`              Listing ID: ${listing.listingId}`);
    return listing;
  }

  /**
   * Submit a buy intent for an ownership object.
   *
   * TRADITIONAL: Call fulfillOrder() with exact payment, hope the order
   * is still valid and nobody front-ran you.
   *
   * OMNIPHI: Submit a buy intent with your maximum price. The solver
   * network matches you with the best available listing. If the asking
   * price is below your max, you pay the asking price (not your max).
   * MEV protection prevents front-running.
   *
   * @param objectId - The object to buy
   * @param maxPrice - Maximum price you are willing to pay
   */
  async buyIntent(
    objectId: string,
    maxPrice: string,
  ): Promise<MarketplaceResult> {
    console.log(`[Marketplace] Submitting buy intent:`);
    console.log(`              Object:    ${objectId}`);
    console.log(`              Max price: ${maxPrice} ${DENOM}`);

    // The buy intent is an atomic bundle:
    //   1. Lock the buyer's payment (up to maxPrice)
    //   2. Verify the object's transfer rules are satisfied
    //   3. Transfer the object to the buyer
    //   4. Pay the seller (minus royalties)
    //   5. Pay royalties to the creator
    //
    // All five steps happen atomically. The object's embedded transfer
    // rules (time-locks, creator approval, etc.) are checked at step 2.
    // If any rule fails, the entire transaction reverts.
    const intentTx: IntentTransaction = {
      sender: this.address,
      intents: [
        {
          type: "transfer",
          sender: this.address,
          recipient: "omni1marketplace_escrow",
          amount: maxPrice,
          denom: DENOM,
          memo: `marketplace:buy:${objectId}`,
        },
      ],
      memo: `marketplace:buy_intent:${objectId}:max_${maxPrice}`,
      deadline: Math.floor(Date.now() / 1000) + 300,
    };

    const result = await this.client.submitIntentTransaction(intentTx, DEFAULT_FEE);

    console.log(`              TX hash: ${result.transactionHash}`);
    console.log(`              Object transferred to buyer`);

    // In production, parse events for actual settlement details
    const royaltyAmount = String(BigInt(maxPrice) * 500n / 10000n); // 5% royalty

    return {
      txHash: result.transactionHash,
      action: "buy",
      objectId,
      from: "omni1seller...", // from events
      to: this.address,
      price: maxPrice,
      royaltyPaid: royaltyAmount,
    };
  }

  /**
   * Make an offer on an ownership object.
   *
   * Offers can include conditions that must be met before the trade
   * settles. This is a capability unique to programmable ownership --
   * traditional NFTs cannot carry conditional transfer logic.
   *
   * @param objectId - The object to make an offer on
   * @param price    - Offered price in base units
   * @param expiry   - Offer expiry in seconds from now
   */
  async makeOffer(
    objectId: string,
    price: string,
    expiry: number,
  ): Promise<Offer> {
    console.log(`[Marketplace] Making offer:`);
    console.log(`              Object: ${objectId}`);
    console.log(`              Price:  ${price} ${DENOM}`);
    console.log(`              Expiry: ${expiry}s`);

    const expiresAt = Math.floor(Date.now() / 1000) + expiry;

    // The offer locks the buyer's funds in a capability-based escrow.
    // Unlike ERC-20 approvals, this escrow is scoped to this specific
    // object purchase and expires automatically. See the escrow example
    // for details on capability-based fund locking.
    const result = await this.client.signAndBroadcast(
      this.address,
      "/pos.contracts.v1.MsgSubmitOffer",
      {
        buyer: this.address,
        object_id: objectId,
        price: { denom: DENOM, amount: price },
        expires_at: expiresAt,
        conditions: [
          {
            type: "royalty_paid",
            description: "Creator royalty must be paid on transfer",
            value: "enforced",
          },
        ],
      },
      DEFAULT_FEE,
      `marketplace:offer:${objectId}`,
    );

    console.log(`              TX hash: ${result.transactionHash}`);

    const offer: Offer = {
      offerId: `offer_${objectId}_${Date.now()}`,
      objectId,
      buyer: this.address,
      price,
      priceDenom: DENOM,
      expiresAt,
      conditions: [
        {
          type: "royalty_paid",
          description: "Creator royalty must be paid on transfer",
          value: "enforced",
        },
      ],
      createdAt: Math.floor(Date.now() / 1000),
      active: true,
    };

    console.log(`              Offer ID: ${offer.offerId}`);
    return offer;
  }

  // -------------------------------------------------------------------------
  // Query operations
  // -------------------------------------------------------------------------

  /**
   * Query all active listings on the marketplace.
   */
  async queryListings(): Promise<Listing[]> {
    console.log(`[Marketplace] Querying active listings...`);

    // In production: GET /pos/contracts/v1/marketplace/listings
    const listings: Listing[] = [
      {
        listingId: "listing_obj_001_1711500000",
        objectId: "obj_001",
        seller: "omni1creator_alice",
        price: "50000000000",     // 50,000 OMNI
        priceDenom: DENOM,
        expiresAt: Math.floor(Date.now() / 1000) + 86400 * 7,
        acceptsOffers: true,
        minOffer: "40000000000",
        createdAt: Math.floor(Date.now() / 1000) - 3600,
        active: true,
      },
      {
        listingId: "listing_obj_002_1711500100",
        objectId: "obj_002",
        seller: "omni1artist_bob",
        price: "15000000000",     // 15,000 OMNI
        priceDenom: DENOM,
        expiresAt: Math.floor(Date.now() / 1000) + 86400 * 30,
        acceptsOffers: true,
        minOffer: "0",
        createdAt: Math.floor(Date.now() / 1000) - 7200,
        active: true,
      },
      {
        listingId: "listing_obj_003_1711500200",
        objectId: "obj_003",
        seller: "omni1collector_carol",
        price: "200000000000",    // 200,000 OMNI
        priceDenom: DENOM,
        expiresAt: Math.floor(Date.now() / 1000) + 86400 * 14,
        acceptsOffers: false,
        minOffer: "0",
        createdAt: Math.floor(Date.now() / 1000) - 1800,
        active: true,
      },
    ];

    console.log(`              Found ${listings.length} active listings`);
    for (const l of listings) {
      console.log(`              - ${l.objectId}: ${l.price} ${l.priceDenom} (seller: ${l.seller.slice(0, 20)}...)`);
    }

    return listings;
  }

  /**
   * Query all offers on a specific object.
   */
  async queryOffers(objectId: string): Promise<Offer[]> {
    console.log(`[Marketplace] Querying offers for ${objectId}...`);

    // In production: GET /pos/contracts/v1/marketplace/offers/{objectId}
    const offers: Offer[] = [
      {
        offerId: `offer_${objectId}_1`,
        objectId,
        buyer: "omni1bidder_dave",
        price: "45000000000",     // 45,000 OMNI
        priceDenom: DENOM,
        expiresAt: Math.floor(Date.now() / 1000) + 86400,
        conditions: [],
        createdAt: Math.floor(Date.now() / 1000) - 600,
        active: true,
      },
      {
        offerId: `offer_${objectId}_2`,
        objectId,
        buyer: "omni1bidder_eve",
        price: "47500000000",     // 47,500 OMNI
        priceDenom: DENOM,
        expiresAt: Math.floor(Date.now() / 1000) + 43200,
        conditions: [
          {
            type: "metadata_verified",
            description: "Metadata must be verified on IPFS",
            value: "required",
          },
        ],
        createdAt: Math.floor(Date.now() / 1000) - 300,
        active: true,
      },
    ];

    console.log(`              Found ${offers.length} active offers`);
    for (const o of offers) {
      console.log(`              - ${o.offerId}: ${o.price} ${o.priceDenom} from ${o.buyer.slice(0, 20)}...`);
    }

    return offers;
  }

  /**
   * Query a specific ownership object.
   */
  async queryObject(objectId: string): Promise<OwnershipObject> {
    console.log(`[Marketplace] Querying object: ${objectId}`);

    // In production: GET /pos/contracts/v1/ownership/{objectId}
    const obj: OwnershipObject = {
      objectId,
      owner: "omni1creator_alice",
      creator: "omni1creator_alice",
      name: "Genesis Contribution #1",
      metadataUri: "ipfs://QmExampleHash123456789",
      metadataHash: "a1b2c3d4e5f6789012345678901234567890123456789012345678901234abcd",
      royaltyBps: 500, // 5%
      transferRules: {
        timelockEnabled: true,
        timelockDuration: 86400, // 24 hours
        creatorApprovalRequired: false,
        allowedRecipients: [],   // anyone
        maxTransfers: 0,         // unlimited
        transferCount: 3,
        fractionalizable: true,
      },
      createdAt: 1000,
      lastTransferAt: 5000,
    };

    console.log(`              Name:     ${obj.name}`);
    console.log(`              Owner:    ${obj.owner}`);
    console.log(`              Creator:  ${obj.creator}`);
    console.log(`              Royalty:  ${obj.royaltyBps / 100}%`);
    console.log(`              Timelock: ${obj.transferRules.timelockEnabled ? `${obj.transferRules.timelockDuration}s` : "none"}`);

    return obj;
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
  console.log("  Omniphi Programmable Ownership Marketplace");
  console.log("============================================================");
  console.log("");

  console.log("--- Step 1: Create Wallet ---");
  const wallet = await createWallet();
  console.log(`  Address: ${wallet.address}`);
  console.log("");

  // ----- Programmable ownership vs static NFTs -----
  console.log("--- Programmable Ownership vs Static NFTs ---");
  console.log("");
  console.log("  ERC-721 NFT:");
  console.log("    - One field: ownerOf[tokenId] = address");
  console.log("    - No built-in transfer rules");
  console.log("    - Royalties are optional, easily bypassed (Blur, Sudoswap)");
  console.log("    - Marketplace logic lives in separate contracts");
  console.log("    - Requires setApprovalForAll (security risk)");
  console.log("");
  console.log("  Omniphi Ownership Object:");
  console.log("    - Owner field + embedded transfer rules");
  console.log("    - Time-locks: must hold for N seconds before transferring");
  console.log("    - Creator approval: creator can gate transfers");
  console.log("    - Enforced royalties: royalty payment is a transfer condition");
  console.log("    - Recipient restrictions: allowlist for controlled distribution");
  console.log("    - Fractionalization: built-in split ownership");
  console.log("    - No approvals needed: intent-based transfer");
  console.log("");

  // ----- Listing -----
  console.log("--- Step 2: List an Object for Sale ---");
  console.log("");
  console.log("  const listing = await marketplace.listForSale(");
  console.log('    "obj_001",         // object ID');
  console.log('    "50000000000",     // 50,000 OMNI');
  console.log("    604800,            // 7 days");
  console.log("  );");
  console.log("");
  console.log("  No approval needed! The object stays in your wallet until");
  console.log("  a buyer's intent is matched and settled atomically.");
  console.log("");

  // ----- Buying with intents -----
  console.log("--- Step 3: Buy with a Purchase Intent ---");
  console.log("");
  console.log("  TRADITIONAL (OpenSea):");
  console.log("    1. Approve WETH spending: WETH.approve(Seaport, amount)");
  console.log("    2. Call fulfillOrder() with exact order parameters");
  console.log("    3. If order was already filled: transaction reverts, gas wasted");
  console.log("    4. Royalty payment depends on marketplace policy (optional)");
  console.log("");
  console.log("  OMNIPHI:");
  console.log("    const result = await marketplace.buyIntent(");
  console.log('      "obj_001",         // object ID');
  console.log('      "52000000000",     // max 52,000 OMNI (willing to pay up to)');
  console.log("    );");
  console.log("");
  console.log("  Atomic settlement:");
  console.log("    1. Buyer's payment locked in escrow");
  console.log("    2. Object's transfer rules verified (timelock, royalty, etc.)");
  console.log("    3. Object transferred to buyer");
  console.log("    4. Seller receives payment minus royalty");
  console.log("    5. Creator receives royalty (enforced, not optional)");
  console.log("    -- All 5 steps atomic: all succeed or all revert --");
  console.log("");

  // ----- Making offers -----
  console.log("--- Step 4: Make a Conditional Offer ---");
  console.log("");
  console.log("  const offer = await marketplace.makeOffer(");
  console.log('    "obj_001",         // object ID');
  console.log('    "47500000000",     // offer 47,500 OMNI');
  console.log("    86400,             // expires in 24 hours");
  console.log("  );");
  console.log("");
  console.log("  Offers can include conditions:");
  console.log("  - metadata_verified: IPFS metadata must be accessible");
  console.log("  - min_holding_period: seller must have held for N days");
  console.log("  - royalty_paid: creator royalty must be honored");
  console.log("  - custom: arbitrary on-chain verifiable conditions");
  console.log("");

  // ----- Time-locked transfers -----
  console.log("--- Step 5: Time-Locked Transfers ---");
  console.log("");
  console.log("  Ownership objects can enforce holding periods:");
  console.log("");
  console.log("    Object obj_001 transfer rules:");
  console.log("      timelockEnabled: true");
  console.log("      timelockDuration: 86400 (24 hours)");
  console.log("");
  console.log("  If the owner tries to sell before 24 hours, the intent");
  console.log("  settlement will revert with 'transfer_rule_violation:timelock'.");
  console.log("  This is enforced at the protocol level, not by the marketplace.");
  console.log("");
  console.log("  Use cases:");
  console.log("  - Prevent pump-and-dump (force holding period)");
  console.log("  - Vesting schedules (gradual unlock)");
  console.log("  - Loyalty rewards (longer hold = better terms)");
  console.log("");

  // ----- Conditional ownership -----
  console.log("--- Step 6: Conditional Ownership Transfers ---");
  console.log("");
  console.log("  Beyond time-locks, objects can require:");
  console.log("");
  console.log("  1. Creator Approval:");
  console.log("     Artist retains approval rights over transfers.");
  console.log("     Prevents sales to sanctioned entities or");
  console.log("     competitors without the artist's consent.");
  console.log("");
  console.log("  2. Recipient Restrictions:");
  console.log("     Object can only be transferred to addresses");
  console.log("     on an allowlist. Useful for gated communities,");
  console.log("     membership tokens, or compliance-restricted assets.");
  console.log("");
  console.log("  3. Max Transfer Count:");
  console.log("     Object can only change hands N times total.");
  console.log("     Creates provable scarcity beyond just quantity.");
  console.log("");
  console.log("  4. Enforced Royalties:");
  console.log("     Unlike ERC-721 where royalties are suggestions,");
  console.log("     Omniphi objects enforce royalty payment as a");
  console.log("     transfer precondition. The transfer physically");
  console.log("     cannot happen without the royalty being paid.");
  console.log("");

  // ----- Query listings and offers -----
  console.log("--- Step 7: Querying the Marketplace ---");
  console.log("");
  console.log("  List all active listings:");
  console.log("    const listings = await marketplace.queryListings();");
  console.log("");
  console.log("  Sample response:");
  console.log("    [");
  console.log("      { objectId: 'obj_001', price: '50000000000', seller: 'omni1creator_alice' },");
  console.log("      { objectId: 'obj_002', price: '15000000000', seller: 'omni1artist_bob' },");
  console.log("    ]");
  console.log("");
  console.log("  Query offers on a specific object:");
  console.log("    const offers = await marketplace.queryOffers('obj_001');");
  console.log("");
  console.log("  Sample response:");
  console.log("    [");
  console.log("      { buyer: 'omni1bidder_dave', price: '45000000000' },");
  console.log("      { buyer: 'omni1bidder_eve',  price: '47500000000', conditions: [...] },");
  console.log("    ]");
  console.log("");

  // ----- Architecture -----
  console.log("============================================================");
  console.log("  Architecture: Intent-Based vs Contract-Based Marketplace");
  console.log("============================================================");
  console.log("");
  console.log("  Traditional (OpenSea/Seaport):");
  console.log("    User -> approve() -> Seaport contract -> fulfillOrder() -> transfer");
  console.log("    - Centralized order book (off-chain)");
  console.log("    - Approval is a blank check on your tokens");
  console.log("    - Royalties enforced only by marketplace policy");
  console.log("    - Different marketplaces = different order formats");
  console.log("");
  console.log("  Omniphi (Intent-Based):");
  console.log("    Seller -> listIntent --------+");
  console.log("                                 |---> Solver Network ---> Settlement");
  console.log("    Buyer  -> buyIntent  --------+");
  console.log("    - On-chain listing (decentralized)");
  console.log("    - No approvals (capability-based)");
  console.log("    - Royalties enforced by the object itself");
  console.log("    - Any marketplace UI can read the same on-chain state");
  console.log("    - Transfer rules are object properties, not marketplace policies");
  console.log("");
  console.log("============================================================");
  console.log("  Example complete. See README.md for full documentation.");
  console.log("============================================================");
}

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
