/**
 * Omniphi Intent-Based DEX (Decentralized Exchange)
 *
 * This example demonstrates how Omniphi's intent-based execution model
 * transforms decentralized trading. Instead of constructing exact swap
 * instructions against a specific AMM pool (as you would on Uniswap or
 * Osmosis), users submit *swap intents* that declare what they want:
 *
 *   "I want to trade 1000 OMNI for as much USDC as possible,
 *    tolerating at most 0.5% slippage."
 *
 * The chain's solver network then competes to fill these intents by
 * finding the best execution route -- across multiple pools, order books,
 * or even cross-chain bridges. The user gets price improvement without
 * needing to know the routing details.
 *
 * Key advantages over traditional AMM-only DEXes:
 *   1. MEV protection: intents are filled by solvers, not by front-runners
 *   2. Best execution: solvers route across all available liquidity
 *   3. Simplicity: users declare outcomes, not execution paths
 *   4. Composability: intents can be bundled atomically
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
  SwapIntent,
  IntentTransaction,
  StdFee,
} from "@omniphi/sdk";

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

const CONFIG = {
  rpcEndpoint: process.env.OMNIPHI_RPC || DEFAULT_RPC_ENDPOINT,
  restEndpoint: process.env.OMNIPHI_REST || DEFAULT_REST_ENDPOINT,
  /** Basis points: 50 = 0.5% */
  defaultSlippageBps: 50,
  /** Known denominations on the Omniphi chain */
  denoms: {
    OMNI: "omniphi",
    USDC: "ibc/usdc",
    ETH: "ibc/eth",
    BTC: "ibc/btc",
  },
};

// ---------------------------------------------------------------------------
// Pool types -- models the on-chain DEX module state
// ---------------------------------------------------------------------------

/** Represents a liquidity pool on the Omniphi DEX. */
interface Pool {
  poolId: number;
  denomA: string;
  denomB: string;
  reserveA: string;
  reserveB: string;
  swapFeeRate: string;
  totalShares: string;
}

/** A price quote from the DEX. */
interface PriceQuote {
  inputDenom: string;
  outputDenom: string;
  price: string;
  /** Estimated output for 1 unit of input */
  spotPrice: number;
  /** Depth of liquidity available at this price */
  liquidityDepth: string;
  /** Route taken: direct pool, multi-hop, or cross-chain */
  route: string[];
}

/** Result of a swap intent submission. */
interface SwapResult {
  txHash: string;
  inputAmount: string;
  inputDenom: string;
  outputAmount: string;
  outputDenom: string;
  effectivePrice: number;
  solverAddress: string;
  route: string[];
}

/** Result of adding liquidity to a pool. */
interface AddLiquidityResult {
  txHash: string;
  poolId: number;
  sharesReceived: string;
  amountADeposited: string;
  amountBDeposited: string;
}

// ---------------------------------------------------------------------------
// OmniphiDex -- high-level DEX client wrapping the SDK
// ---------------------------------------------------------------------------

/**
 * High-level DEX client for intent-based trading on Omniphi.
 *
 * Usage:
 *   const dex = await OmniphiDex.connect(mnemonic);
 *   const result = await dex.createSwapIntent("omniphi", "ibc/usdc", "1000000", 50);
 */
class OmniphiDex {
  private client: OmniphiClient;
  private address: string;

  private constructor(client: OmniphiClient, address: string) {
    this.client = client;
    this.address = address;
  }

  /**
   * Connect to the Omniphi chain with a signing wallet.
   */
  static async connect(mnemonic: string): Promise<OmniphiDex> {
    const wallet = await fromMnemonic(mnemonic);
    const client = await OmniphiClient.connectWithSigner(
      CONFIG.rpcEndpoint,
      wallet.signer,
      { restEndpoint: CONFIG.restEndpoint },
    );
    console.log(`[DEX] Connected as ${wallet.address}`);
    return new OmniphiDex(client, wallet.address);
  }

  /**
   * Connect with a read-only client (no signing capability).
   */
  static async connectReadOnly(): Promise<OmniphiDex> {
    const client = await OmniphiClient.connect(
      CONFIG.rpcEndpoint,
      { restEndpoint: CONFIG.restEndpoint },
    );
    return new OmniphiDex(client, "");
  }

  get walletAddress(): string {
    return this.address;
  }

  // -------------------------------------------------------------------------
  // Core DEX operations
  // -------------------------------------------------------------------------

  /**
   * Submit a swap intent to the Omniphi chain.
   *
   * TRADITIONAL DEX (e.g., Uniswap):
   *   - You must specify the exact pool to swap through
   *   - You must calculate the expected output amount yourself
   *   - You must handle multi-hop routing yourself
   *   - Front-runners can sandwich your transaction
   *
   * OMNIPHI INTENT-BASED DEX:
   *   - You declare WHAT you want: "swap X of A for as much B as possible"
   *   - Solvers compete to find the best route and price
   *   - The chain enforces your slippage constraint
   *   - MEV protection is built into the intent settlement layer
   *
   * @param inputDenom     - The token you are selling (e.g., "omniphi")
   * @param outputDenom    - The token you want to buy (e.g., "ibc/usdc")
   * @param inputAmount    - Amount of input token (in base units)
   * @param maxSlippageBps - Maximum slippage in basis points (50 = 0.5%)
   * @returns The swap result including the actual execution details
   */
  async createSwapIntent(
    inputDenom: string,
    outputDenom: string,
    inputAmount: string,
    maxSlippageBps: number = CONFIG.defaultSlippageBps,
  ): Promise<SwapResult> {
    console.log(`[DEX] Creating swap intent:`);
    console.log(`      Input:    ${inputAmount} ${inputDenom}`);
    console.log(`      Output:   ${outputDenom}`);
    console.log(`      Slippage: ${maxSlippageBps / 100}%`);

    // First, query the current price to compute the minimum acceptable output.
    // In a real deployment the solver network handles price discovery, but we
    // set a floor so the chain can reject bad fills.
    const quote = await this.getPrice(inputDenom, outputDenom);
    const expectedOutput = BigInt(inputAmount) * BigInt(Math.floor(quote.spotPrice * 1e6)) / BigInt(1e6);
    const minOutput = expectedOutput * BigInt(10000 - maxSlippageBps) / BigInt(10000);

    // Build the swap intent.
    //
    // Unlike a traditional MsgSwapExactAmountIn, this intent does not specify
    // a pool or route. The solver network picks the optimal path.
    const swapIntent: SwapIntent = {
      type: "swap",
      sender: this.address,
      inputDenom,
      inputAmount,
      outputDenom,
      minOutputAmount: minOutput.toString(),
      maxSlippageBps,
    };

    console.log(`      Min output: ${minOutput.toString()} ${outputDenom}`);
    console.log(`      Submitting intent to chain...`);

    // Submit the intent through the SDK. This calls MsgSubmitSwapIntent under
    // the hood, which enters the intent mempool. Solvers pick it up, compete
    // on fill price, and the best fill is executed atomically.
    const result = await this.client.submitIntent(
      this.address,
      swapIntent,
      DEFAULT_FEE,
      "omniphi-dex:swap-intent",
    );

    console.log(`      TX hash: ${result.transactionHash}`);
    console.log(`      Status:  SUCCESS`);

    // Parse the execution events to extract the actual fill details.
    // In production, you would parse result.events for the solver fill data.
    return {
      txHash: result.transactionHash,
      inputAmount,
      inputDenom,
      outputAmount: minOutput.toString(), // actual fill >= this
      outputDenom,
      effectivePrice: quote.spotPrice,
      solverAddress: "omni1solver...", // extracted from events
      route: quote.route,
    };
  }

  /**
   * Query a liquidity pool by its ID.
   *
   * Pools on Omniphi are similar to Uniswap V2 constant-product pools,
   * but they serve as one of many liquidity sources that solvers can tap.
   * Users never interact with pools directly -- intents abstract that away.
   *
   * @param poolId - The numeric pool ID
   * @returns Pool details including reserves and fee rate
   */
  async queryPool(poolId: number): Promise<Pool> {
    console.log(`[DEX] Querying pool ${poolId}...`);

    // Query the DEX module state via REST.
    // Endpoint: /pos/contracts/v1/dex/pool/{poolId}
    try {
      const balance = await this.client.getBalance(this.address, DENOM);
      // In a live environment, this would call:
      //   restQuery(`/pos/contracts/v1/dex/pool/${poolId}`)
      // For this reference example, we construct a representative response
      // showing the data shape that the on-chain module returns.

      const pool: Pool = {
        poolId,
        denomA: CONFIG.denoms.OMNI,
        denomB: CONFIG.denoms.USDC,
        reserveA: "50000000000",   // 50,000 OMNI
        reserveB: "125000000000",  // 125,000 USDC
        swapFeeRate: "0.003",      // 0.3%
        totalShares: "78000000000",
      };

      console.log(`      Pool ${poolId}: ${pool.denomA}/${pool.denomB}`);
      console.log(`      Reserves: ${pool.reserveA} / ${pool.reserveB}`);
      console.log(`      Fee rate: ${pool.swapFeeRate}`);

      return pool;
    } catch (error) {
      throw new Error(`Failed to query pool ${poolId}: ${error}`);
    }
  }

  /**
   * Get the current price between two assets.
   *
   * This queries the solver network's aggregated price feed, which takes
   * into account all available liquidity: AMM pools, order books, and
   * cross-chain sources.
   *
   * @param denomA - The base asset denomination
   * @param denomB - The quote asset denomination
   * @returns A price quote with spot price and available routes
   */
  async getPrice(denomA: string, denomB: string): Promise<PriceQuote> {
    console.log(`[DEX] Querying price: ${denomA} -> ${denomB}`);

    // In production, the price endpoint aggregates across all liquidity:
    //   GET /pos/contracts/v1/dex/price?input={denomA}&output={denomB}
    //
    // Solvers maintain off-chain indices of pool states, order book depth,
    // and cross-chain bridge liquidity. The on-chain query returns the
    // consensus-agreed best available price.

    // Determine the route. The solver network finds the optimal path:
    // - Direct pool: OMNI/USDC if a pool exists
    // - Multi-hop: OMNI -> ETH -> USDC if that gives better pricing
    // - Cross-chain: OMNI -> bridge -> remote DEX -> bridge back
    const route = this.findOptimalRoute(denomA, denomB);
    const spotPrice = this.calculateSpotPrice(denomA, denomB);

    const quote: PriceQuote = {
      inputDenom: denomA,
      outputDenom: denomB,
      price: spotPrice.toFixed(6),
      spotPrice,
      liquidityDepth: "10000000000", // available without significant slippage
      route,
    };

    console.log(`      Spot price: 1 ${denomA} = ${quote.price} ${denomB}`);
    console.log(`      Route: ${route.join(" -> ")}`);

    return quote;
  }

  /**
   * Add liquidity to a pool.
   *
   * Even liquidity provision benefits from intents. Instead of specifying
   * exact token amounts that must match the pool ratio precisely, you submit
   * a liquidity intent. The solver layer handles:
   *   - Optimal ratio calculation
   *   - Rebalancing if the pool moved since your last check
   *   - Single-sided deposits (if the pool supports them)
   *
   * @param poolId  - The pool to add liquidity to
   * @param amountA - Amount of token A
   * @param amountB - Amount of token B
   * @returns Details of the liquidity addition
   */
  async addLiquidity(
    poolId: number,
    amountA: string,
    amountB: string,
  ): Promise<AddLiquidityResult> {
    console.log(`[DEX] Adding liquidity to pool ${poolId}:`);
    console.log(`      Amount A: ${amountA}`);
    console.log(`      Amount B: ${amountB}`);

    // Fetch the pool to know which denoms we are working with.
    const pool = await this.queryPool(poolId);

    // Submit an atomic intent transaction that:
    //   1. Transfers amountA of denomA into the pool
    //   2. Transfers amountB of denomB into the pool
    //   3. Mints LP shares to the sender
    //
    // The key insight: we submit this as an *intent bundle*, not as three
    // separate transactions. The chain ensures all three succeed atomically,
    // and the solver layer optimizes the deposit ratio.
    const intentTx: IntentTransaction = {
      sender: this.address,
      intents: [
        {
          type: "transfer",
          sender: this.address,
          recipient: `omni1pool_${poolId}`, // pool module account
          amount: amountA,
          denom: pool.denomA,
          memo: `dex:add_liquidity:${poolId}:tokenA`,
        },
        {
          type: "transfer",
          sender: this.address,
          recipient: `omni1pool_${poolId}`,
          amount: amountB,
          denom: pool.denomB,
          memo: `dex:add_liquidity:${poolId}:tokenB`,
        },
      ],
      memo: `dex:add_liquidity:pool_${poolId}`,
      deadline: Math.floor(Date.now() / 1000) + 300, // 5 minute deadline
    };

    console.log(`      Submitting atomic liquidity intent...`);

    const result = await this.client.submitIntentTransaction(intentTx, DEFAULT_FEE);

    console.log(`      TX hash: ${result.transactionHash}`);

    return {
      txHash: result.transactionHash,
      poolId,
      sharesReceived: "0", // parsed from events
      amountADeposited: amountA,
      amountBDeposited: amountB,
    };
  }

  /**
   * Execute a limit order as an intent.
   *
   * In traditional DEXes, limit orders require a centralized order book
   * or a specialized on-chain module. With Omniphi intents, a limit order
   * is just a swap intent with a specific minimum output and a deadline.
   * Solvers fill it when market conditions match.
   *
   * @param inputDenom   - Token to sell
   * @param outputDenom  - Token to buy
   * @param inputAmount  - Amount to sell
   * @param limitPrice   - Minimum price per unit of output
   * @param expirySeconds - How long the order remains valid
   */
  async submitLimitOrder(
    inputDenom: string,
    outputDenom: string,
    inputAmount: string,
    limitPrice: number,
    expirySeconds: number = 3600,
  ): Promise<SwapResult> {
    console.log(`[DEX] Submitting limit order:`);
    console.log(`      Sell: ${inputAmount} ${inputDenom}`);
    console.log(`      Buy:  ${outputDenom} at ${limitPrice} or better`);
    console.log(`      Expiry: ${expirySeconds}s`);

    // The minimum output is derived from the limit price.
    // This is enforced on-chain: if no solver can fill at this price,
    // the intent expires and the user's tokens are returned.
    const minOutput = BigInt(Math.floor(parseFloat(inputAmount) * limitPrice));

    const swapIntent: SwapIntent = {
      type: "swap",
      sender: this.address,
      inputDenom,
      inputAmount,
      outputDenom,
      minOutputAmount: minOutput.toString(),
      // No slippage: we want exactly this price or better
      maxSlippageBps: 0,
    };

    const result = await this.client.submitIntent(
      this.address,
      swapIntent,
      DEFAULT_FEE,
      `dex:limit_order:${limitPrice}`,
    );

    return {
      txHash: result.transactionHash,
      inputAmount,
      inputDenom,
      outputAmount: minOutput.toString(),
      outputDenom,
      effectivePrice: limitPrice,
      solverAddress: "omni1solver...",
      route: [inputDenom, outputDenom],
    };
  }

  /**
   * Execute a multi-leg swap atomically.
   *
   * TRADITIONAL DEX:
   *   - Submit separate transactions for each leg
   *   - Risk partial execution (first leg succeeds, second fails)
   *   - Pay gas for each transaction separately
   *   - Must handle complex error recovery
   *
   * OMNIPHI INTENT-BASED:
   *   - Submit all legs as a single intent bundle
   *   - Atomic execution: all or nothing
   *   - Solvers optimize the overall route
   *   - Single gas payment
   *
   * @param legs - Array of swap legs to execute atomically
   */
  async multiLegSwap(
    legs: Array<{ inputDenom: string; outputDenom: string; inputAmount: string }>,
  ): Promise<string> {
    console.log(`[DEX] Executing ${legs.length}-leg atomic swap:`);

    const intents: SwapIntent[] = legs.map((leg, i) => {
      console.log(`      Leg ${i + 1}: ${leg.inputAmount} ${leg.inputDenom} -> ${leg.outputDenom}`);
      return {
        type: "swap" as const,
        sender: this.address,
        inputDenom: leg.inputDenom,
        inputAmount: leg.inputAmount,
        outputDenom: leg.outputDenom,
        minOutputAmount: "1", // solver optimizes
        maxSlippageBps: CONFIG.defaultSlippageBps,
      };
    });

    const intentTx: IntentTransaction = {
      sender: this.address,
      intents,
      memo: `dex:multi_leg_swap:${legs.length}_legs`,
      deadline: Math.floor(Date.now() / 1000) + 120,
    };

    const result = await this.client.submitIntentTransaction(intentTx, DEFAULT_FEE);
    console.log(`      TX hash: ${result.transactionHash}`);
    return result.transactionHash;
  }

  // -------------------------------------------------------------------------
  // Private helpers
  // -------------------------------------------------------------------------

  /**
   * Determines the optimal route between two assets.
   * In production, this is handled by the solver network.
   */
  private findOptimalRoute(denomA: string, denomB: string): string[] {
    // Direct route if a pool exists
    if (this.hasDirectPool(denomA, denomB)) {
      return [denomA, denomB];
    }
    // Otherwise route through OMNI as the hub asset
    if (denomA !== CONFIG.denoms.OMNI && denomB !== CONFIG.denoms.OMNI) {
      return [denomA, CONFIG.denoms.OMNI, denomB];
    }
    return [denomA, denomB];
  }

  private hasDirectPool(denomA: string, denomB: string): boolean {
    const directPairs = [
      [CONFIG.denoms.OMNI, CONFIG.denoms.USDC],
      [CONFIG.denoms.OMNI, CONFIG.denoms.ETH],
      [CONFIG.denoms.OMNI, CONFIG.denoms.BTC],
      [CONFIG.denoms.ETH, CONFIG.denoms.USDC],
    ];
    return directPairs.some(
      ([a, b]) => (a === denomA && b === denomB) || (a === denomB && b === denomA),
    );
  }

  private calculateSpotPrice(denomA: string, denomB: string): number {
    // Reference prices for demonstration (in production, derived from pool reserves)
    const prices: Record<string, number> = {
      "omniphi": 2.50,
      "ibc/usdc": 1.00,
      "ibc/eth": 3500.00,
      "ibc/btc": 65000.00,
    };
    const priceA = prices[denomA] || 1;
    const priceB = prices[denomB] || 1;
    return priceA / priceB;
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
  console.log("  Omniphi Intent-Based DEX -- Reference Example");
  console.log("============================================================");
  console.log("");

  // In a real deployment, load the mnemonic from a secure store.
  // Here we generate a fresh wallet for demonstration.
  console.log("--- Step 1: Create Wallet ---");
  const wallet = await createWallet();
  console.log(`  Address:  ${wallet.address}`);
  console.log(`  Mnemonic: ${wallet.mnemonic?.split(" ").slice(0, 3).join(" ")}... (truncated)`);
  console.log("");

  // For this reference example, we demonstrate the API patterns without
  // requiring a live chain. Each method below shows the exact SDK calls
  // you would make against a running Omniphi node.

  console.log("--- Step 2: Connect to Chain ---");
  console.log("  In production, connect with:");
  console.log('    const dex = await OmniphiDex.connect(mnemonic);');
  console.log("");

  // Demonstrate the intent-based swap flow
  console.log("--- Step 3: Intent-Based Swap (vs. Traditional) ---");
  console.log("");
  console.log("  TRADITIONAL DEX (Uniswap-style):");
  console.log("    1. Query pool reserves for OMNI/USDC");
  console.log("    2. Calculate expected output with constant-product formula");
  console.log("    3. Build MsgSwapExactAmountIn with specific pool ID and route");
  console.log("    4. Set minAmountOut based on your own slippage calculation");
  console.log("    5. Submit transaction -- front-runners can sandwich it");
  console.log("    6. If pool moved, transaction fails; retry from step 1");
  console.log("");
  console.log("  OMNIPHI INTENT-BASED DEX:");
  console.log("    1. Submit swap intent: 'I want to sell 1000 OMNI for USDC'");
  console.log("    2. Chain's solver network finds the best route automatically");
  console.log("    3. MEV protection: solvers compete, no sandwich attacks");
  console.log("    4. Done. You get the best available price.");
  console.log("");

  console.log("  Code for intent-based swap:");
  console.log("  ```");
  console.log("  const result = await dex.createSwapIntent(");
  console.log('    "omniphi",    // selling OMNI');
  console.log('    "ibc/usdc",   // buying USDC');
  console.log('    "1000000000", // 1000 OMNI (in base units)');
  console.log("    50,           // max 0.5% slippage");
  console.log("  );");
  console.log("  ```");
  console.log("");

  // Show the underlying SDK call that createSwapIntent uses
  console.log("--- Step 4: What Happens Under the Hood ---");
  console.log("");
  console.log("  The SDK translates the swap intent into:");
  console.log("  ```");
  console.log('  client.submitIntent(address, {');
  console.log('    type: "swap",');
  console.log('    sender: "omni1...",');
  console.log('    inputDenom: "omniphi",');
  console.log('    inputAmount: "1000000000",');
  console.log('    outputDenom: "ibc/usdc",');
  console.log('    minOutputAmount: "2487500000",  // 0.5% below spot');
  console.log('    maxSlippageBps: 50,');
  console.log("  });");
  console.log("  ```");
  console.log("");
  console.log("  This becomes a MsgSubmitSwapIntent on-chain, which:");
  console.log("  1. Enters the intent mempool (separate from standard tx mempool)");
  console.log("  2. Solvers query pending intents and submit fill proposals");
  console.log("  3. The chain selects the best fill (highest output for user)");
  console.log("  4. Atomic settlement: user sends OMNI, receives USDC");
  console.log("");

  // Demonstrate pool queries
  console.log("--- Step 5: Querying Pools and Prices ---");
  console.log("");
  console.log("  Query a specific pool:");
  console.log("  ```");
  console.log("  const pool = await dex.queryPool(1);");
  console.log("  // { poolId: 1, denomA: 'omniphi', denomB: 'ibc/usdc',");
  console.log("  //   reserveA: '50000000000', reserveB: '125000000000',");
  console.log("  //   swapFeeRate: '0.003', totalShares: '78000000000' }");
  console.log("  ```");
  console.log("");
  console.log("  Get aggregated price (across all liquidity sources):");
  console.log("  ```");
  console.log('  const price = await dex.getPrice("omniphi", "ibc/usdc");');
  console.log("  // { spotPrice: 2.5, route: ['omniphi', 'ibc/usdc'],");
  console.log("  //   liquidityDepth: '10000000000' }");
  console.log("  ```");
  console.log("");

  // Demonstrate liquidity provision
  console.log("--- Step 6: Adding Liquidity with Intents ---");
  console.log("");
  console.log("  Traditional LP: must calculate exact ratio, two approvals, then deposit.");
  console.log("  Omniphi LP: submit a liquidity intent, solver handles the rest.");
  console.log("");
  console.log("  ```");
  console.log("  const lp = await dex.addLiquidity(");
  console.log("    1,              // pool ID");
  console.log('    "5000000000",   // 5000 OMNI');
  console.log('    "12500000000",  // 12500 USDC');
  console.log("  );");
  console.log("  ```");
  console.log("");

  // Demonstrate limit orders
  console.log("--- Step 7: Limit Orders as Intents ---");
  console.log("");
  console.log("  A limit order is simply a swap intent with a deadline:");
  console.log("  ```");
  console.log("  const order = await dex.submitLimitOrder(");
  console.log('    "omniphi",      // sell OMNI');
  console.log('    "ibc/usdc",     // buy USDC');
  console.log('    "10000000000",  // 10000 OMNI');
  console.log("    3.00,           // limit price: 3.00 USDC per OMNI");
  console.log("    86400,          // valid for 24 hours");
  console.log("  );");
  console.log("  ```");
  console.log("");

  // Demonstrate multi-leg atomic swaps
  console.log("--- Step 8: Multi-Leg Atomic Swaps ---");
  console.log("");
  console.log("  Swap through multiple assets in one atomic transaction:");
  console.log("  ```");
  console.log("  const txHash = await dex.multiLegSwap([");
  console.log('    { inputDenom: "omniphi", outputDenom: "ibc/eth", inputAmount: "5000000000" },');
  console.log('    { inputDenom: "ibc/eth", outputDenom: "ibc/btc", inputAmount: "0" },');
  console.log("  ]);");
  console.log("  // All legs execute atomically: if any fails, all revert.");
  console.log("  ```");
  console.log("");

  // Architecture summary
  console.log("============================================================");
  console.log("  Architecture Summary");
  console.log("============================================================");
  console.log("");
  console.log("  User          -> SwapIntent         -> Omniphi Chain");
  console.log("  (what I want)    (declarative)         (intent mempool)");
  console.log("                                              |");
  console.log("                                         Solver Network");
  console.log("                                     (competitive filling)");
  console.log("                                              |");
  console.log("                                         Best Fill");
  console.log("                                     (atomic settlement)");
  console.log("                                              |");
  console.log("  User          <- Result             <- Omniphi Chain");
  console.log("  (what I got)     (tokens received)     (state updated)");
  console.log("");
  console.log("  Key difference from traditional DEXes:");
  console.log("  - Users never specify HOW to execute, only WHAT they want");
  console.log("  - Solvers handle all routing, price optimization, and MEV protection");
  console.log("  - The chain enforces constraints (min output, deadline, slippage)");
  console.log("  - Atomic execution prevents partial fills and stuck transactions");
  console.log("");
  console.log("============================================================");
  console.log("  Example complete. See README.md for full documentation.");
  console.log("============================================================");
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
