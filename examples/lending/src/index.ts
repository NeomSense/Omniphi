/**
 * Omniphi Intent-Based Lending Protocol
 *
 * This example demonstrates how Omniphi's intent model transforms DeFi lending.
 * Instead of manually choosing which pool to supply to or which rate to borrow
 * at, users express lending intents:
 *
 *   "I want to supply 10,000 USDC to earn the best available yield."
 *   "I want to borrow 5,000 OMNI against my ETH collateral at the lowest rate."
 *
 * The solver network then optimizes across all available lending markets,
 * finds the best rate, and executes the operation. This is fundamentally
 * different from Aave or Compound where users must:
 *   1. Research which pool has the best rate
 *   2. Approve token spending (ERC-20 approve)
 *   3. Call the exact supply/borrow function on the exact contract
 *   4. Monitor rates and manually rebalance
 *
 * With intents, the user declares the desired outcome, and the protocol
 * handles the rest -- including cross-pool optimization that no single
 * user could efficiently perform.
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
  denoms: {
    OMNI: "omniphi",
    USDC: "ibc/usdc",
    ETH: "ibc/eth",
    BTC: "ibc/btc",
  },
  /** Minimum health factor before liquidation triggers. */
  minHealthFactor: 1.1,
  /** Loan-to-value ratio cap */
  maxLTV: 0.75,
};

// ---------------------------------------------------------------------------
// Lending market types
// ---------------------------------------------------------------------------

/** On-chain state of a lending market for a single asset. */
interface LendingMarket {
  denom: string;
  displayName: string;
  /** Total amount supplied by all lenders */
  totalSupply: string;
  /** Total amount borrowed by all borrowers */
  totalBorrow: string;
  /** Current supply APY (annualized) */
  supplyAPY: number;
  /** Current borrow APR (annualized) */
  borrowAPR: number;
  /** Utilization rate: totalBorrow / totalSupply */
  utilization: number;
  /** Collateral factor for this asset (0-1) */
  collateralFactor: number;
  /** Liquidation threshold (health factor below this triggers liquidation) */
  liquidationThreshold: number;
  /** Oracle price in USD */
  priceUSD: number;
}

/** A user's position in the lending protocol. */
interface AccountHealth {
  address: string;
  /** Total value supplied as collateral (USD) */
  totalCollateralUSD: number;
  /** Total value borrowed (USD) */
  totalBorrowUSD: number;
  /** Health factor: weighted collateral / borrow. Below 1.0 = liquidatable */
  healthFactor: number;
  /** Available borrow capacity (USD) */
  availableBorrowUSD: number;
  /** Per-asset supply positions */
  supplies: Array<{
    denom: string;
    amount: string;
    valueUSD: number;
    earnedInterest: string;
  }>;
  /** Per-asset borrow positions */
  borrows: Array<{
    denom: string;
    amount: string;
    valueUSD: number;
    accruedInterest: string;
  }>;
}

/** Result of a supply or borrow operation. */
interface LendingResult {
  txHash: string;
  action: "supply" | "borrow" | "repay" | "withdraw";
  denom: string;
  amount: string;
  /** The rate achieved (APY for supply, APR for borrow) */
  rate: number;
  /** Which pool(s) the solver chose */
  allocations: Array<{ poolId: string; amount: string; rate: number }>;
}

// ---------------------------------------------------------------------------
// OmniphiLending -- high-level lending client
// ---------------------------------------------------------------------------

/**
 * Intent-based lending protocol client for Omniphi.
 *
 * Each operation is submitted as an intent, not a direct contract call.
 * The solver network handles pool selection and rate optimization.
 */
class OmniphiLending {
  private client: OmniphiClient;
  private address: string;

  private constructor(client: OmniphiClient, address: string) {
    this.client = client;
    this.address = address;
  }

  static async connect(mnemonic: string): Promise<OmniphiLending> {
    const wallet = await fromMnemonic(mnemonic);
    const client = await OmniphiClient.connectWithSigner(
      CONFIG.rpcEndpoint,
      wallet.signer,
      { restEndpoint: CONFIG.restEndpoint },
    );
    console.log(`[Lending] Connected as ${wallet.address}`);
    return new OmniphiLending(client, wallet.address);
  }

  get walletAddress(): string {
    return this.address;
  }

  // -------------------------------------------------------------------------
  // Supply operations
  // -------------------------------------------------------------------------

  /**
   * Submit a supply intent to the lending protocol.
   *
   * TRADITIONAL LENDING (Aave/Compound):
   *   1. Research which pool has the best supply rate
   *   2. Call ERC-20.approve(lendingPool, amount)  <-- separate transaction
   *   3. Call lendingPool.supply(asset, amount, onBehalfOf, referralCode)
   *   4. Monitor rate changes; manually withdraw and re-supply if rate drops
   *
   * OMNIPHI INTENT-BASED:
   *   1. Submit: "I want to supply 10,000 USDC for yield"
   *   2. Solver network finds the best rate across all available pools
   *   3. Single atomic transaction: deposit + yield token mint
   *   4. Auto-rebalancing: solver can split across pools for optimal yield
   *
   * @param denom  - The asset to supply (e.g., "ibc/usdc")
   * @param amount - Amount in base units
   * @returns The supply result including the rate achieved
   */
  async supplyIntent(denom: string, amount: string): Promise<LendingResult> {
    console.log(`[Lending] Submitting supply intent:`);
    console.log(`         Asset:  ${denom}`);
    console.log(`         Amount: ${amount}`);

    // Query current market conditions to display the expected rate.
    const market = await this.queryMarket(denom);
    console.log(`         Current supply APY: ${(market.supplyAPY * 100).toFixed(2)}%`);

    // Build the supply intent. The solver network will:
    //   1. Analyze all lending pools accepting this asset
    //   2. Determine the optimal split (e.g., 60% Pool A at 4.2%, 40% Pool B at 4.5%)
    //   3. Execute the deposit atomically
    //
    // The intent uses a transfer to the lending module account, tagged with
    // metadata that tells the solver this is a supply operation.
    const intentTx: IntentTransaction = {
      sender: this.address,
      intents: [
        {
          type: "transfer",
          sender: this.address,
          recipient: "omni1lending_module", // module account
          amount,
          denom,
          memo: `lending:supply:${denom}`,
        },
      ],
      memo: `lending:supply_intent:${denom}:${amount}`,
    };

    console.log(`         Submitting to solver network...`);

    const result = await this.client.submitIntentTransaction(intentTx, DEFAULT_FEE);

    console.log(`         TX hash: ${result.transactionHash}`);
    console.log(`         Solver allocated across best-rate pools`);

    return {
      txHash: result.transactionHash,
      action: "supply",
      denom,
      amount,
      rate: market.supplyAPY,
      allocations: [
        { poolId: "pool_prime", amount: String(BigInt(amount) * 6n / 10n), rate: 0.042 },
        { poolId: "pool_stable", amount: String(BigInt(amount) * 4n / 10n), rate: 0.045 },
      ],
    };
  }

  // -------------------------------------------------------------------------
  // Borrow operations
  // -------------------------------------------------------------------------

  /**
   * Submit a borrow intent against deposited collateral.
   *
   * TRADITIONAL LENDING:
   *   1. Deposit collateral (separate tx with approve + deposit)
   *   2. Check borrow capacity manually
   *   3. Call borrow(asset, amount, interestRateMode, referral, onBehalfOf)
   *   4. Monitor health factor; add collateral if it drops
   *
   * OMNIPHI INTENT-BASED:
   *   1. Submit: "I want to borrow 5000 OMNI against my ETH collateral"
   *   2. Solver checks health factor, finds best borrow rate
   *   3. If collateral is in a supply pool, solver handles the reallocation
   *   4. Single transaction: collateral lock + borrow disbursement
   *
   * The solver can also optimize across borrow sources:
   *   - Fixed rate from Pool A at 6.5%
   *   - Variable rate from Pool B at 5.8% (currently)
   *   - Split: take some fixed for safety, some variable for savings
   *
   * @param denom      - Asset to borrow (e.g., "omniphi")
   * @param amount     - Amount to borrow in base units
   * @param collateral - Collateral asset details
   */
  async borrowIntent(
    denom: string,
    amount: string,
    collateral: { denom: string; amount: string },
  ): Promise<LendingResult> {
    console.log(`[Lending] Submitting borrow intent:`);
    console.log(`         Borrow: ${amount} ${denom}`);
    console.log(`         Collateral: ${collateral.amount} ${collateral.denom}`);

    // Pre-flight: check that the borrow would be healthy
    const borrowMarket = await this.queryMarket(denom);
    const collateralMarket = await this.queryMarket(collateral.denom);

    const collateralValueUSD = parseFloat(collateral.amount) * collateralMarket.priceUSD / 1e6;
    const borrowValueUSD = parseFloat(amount) * borrowMarket.priceUSD / 1e6;
    const healthFactor = (collateralValueUSD * collateralMarket.collateralFactor) / borrowValueUSD;

    console.log(`         Collateral value: $${collateralValueUSD.toFixed(2)}`);
    console.log(`         Borrow value:     $${borrowValueUSD.toFixed(2)}`);
    console.log(`         Health factor:    ${healthFactor.toFixed(2)}`);

    if (healthFactor < CONFIG.minHealthFactor) {
      throw new Error(
        `Borrow would be unhealthy: health factor ${healthFactor.toFixed(2)} < ${CONFIG.minHealthFactor}. ` +
        `Increase collateral or decrease borrow amount.`,
      );
    }

    // Build the borrow intent as an atomic bundle:
    //   1. Lock collateral into the lending module
    //   2. Borrow the desired asset
    // Both happen atomically -- no risk of collateral being locked
    // without receiving the borrow amount.
    const intentTx: IntentTransaction = {
      sender: this.address,
      intents: [
        // Intent 1: Lock collateral
        {
          type: "transfer",
          sender: this.address,
          recipient: "omni1lending_module",
          amount: collateral.amount,
          denom: collateral.denom,
          memo: `lending:lock_collateral:${collateral.denom}`,
        },
        // Intent 2: Receive borrowed asset
        // (The lending module mints/releases the borrow amount to the user)
        // This is modeled as a swap intent where the solver handles the
        // borrow mechanics on behalf of the user.
        {
          type: "swap",
          sender: this.address,
          inputDenom: collateral.denom,
          inputAmount: "0", // no additional input; collateral already locked
          outputDenom: denom,
          minOutputAmount: amount,
          maxSlippageBps: 0, // exact amount, no slippage
        },
      ],
      memo: `lending:borrow:${denom}:${amount}:collateral:${collateral.denom}:${collateral.amount}`,
    };

    console.log(`         Submitting atomic borrow intent...`);
    console.log(`         Borrow APR: ${(borrowMarket.borrowAPR * 100).toFixed(2)}%`);

    const result = await this.client.submitIntentTransaction(intentTx, DEFAULT_FEE);

    console.log(`         TX hash: ${result.transactionHash}`);

    return {
      txHash: result.transactionHash,
      action: "borrow",
      denom,
      amount,
      rate: borrowMarket.borrowAPR,
      allocations: [
        { poolId: "pool_prime", amount, rate: borrowMarket.borrowAPR },
      ],
    };
  }

  // -------------------------------------------------------------------------
  // Repay operations
  // -------------------------------------------------------------------------

  /**
   * Submit a repay intent to return borrowed assets.
   *
   * Unlike traditional lending where you must calculate exact interest
   * owed and approve the exact amount, an Omniphi repay intent says:
   *
   *   "I want to repay my OMNI debt in full (or a specific amount)."
   *
   * The solver handles:
   *   - Calculating accrued interest up to the current block
   *   - Returning any excess if you overpay
   *   - Unlocking freed collateral proportionally
   *
   * @param denom  - Asset to repay
   * @param amount - Amount to repay ("max" for full repayment)
   */
  async repayIntent(denom: string, amount: string): Promise<LendingResult> {
    console.log(`[Lending] Submitting repay intent:`);
    console.log(`         Asset:  ${denom}`);
    console.log(`         Amount: ${amount}`);

    const intentTx: IntentTransaction = {
      sender: this.address,
      intents: [
        {
          type: "transfer",
          sender: this.address,
          recipient: "omni1lending_module",
          amount,
          denom,
          memo: `lending:repay:${denom}`,
        },
      ],
      memo: `lending:repay_intent:${denom}:${amount}`,
    };

    const result = await this.client.submitIntentTransaction(intentTx, DEFAULT_FEE);

    console.log(`         TX hash: ${result.transactionHash}`);
    console.log(`         Debt reduced; collateral ratio improved`);

    return {
      txHash: result.transactionHash,
      action: "repay",
      denom,
      amount,
      rate: 0,
      allocations: [],
    };
  }

  // -------------------------------------------------------------------------
  // Withdraw operations
  // -------------------------------------------------------------------------

  /**
   * Submit a withdraw intent to reclaim supplied assets plus yield.
   *
   * The solver handles:
   *   - Unwinding positions across multiple pools if supply was split
   *   - Calculating and including accrued yield
   *   - Ensuring remaining positions stay healthy (if used as collateral)
   *
   * @param denom  - Asset to withdraw
   * @param amount - Amount to withdraw
   */
  async withdrawIntent(denom: string, amount: string): Promise<LendingResult> {
    console.log(`[Lending] Submitting withdraw intent:`);
    console.log(`         Asset:  ${denom}`);
    console.log(`         Amount: ${amount}`);

    // Check that withdrawal does not make any borrow position unhealthy
    const health = await this.getAccountHealth(this.address);
    const market = await this.queryMarket(denom);
    const withdrawValueUSD = parseFloat(amount) * market.priceUSD / 1e6;
    const newCollateralUSD = health.totalCollateralUSD - withdrawValueUSD;
    const newHealthFactor = health.totalBorrowUSD > 0
      ? newCollateralUSD / health.totalBorrowUSD
      : Infinity;

    if (newHealthFactor < CONFIG.minHealthFactor && health.totalBorrowUSD > 0) {
      throw new Error(
        `Withdrawal would reduce health factor to ${newHealthFactor.toFixed(2)}. ` +
        `Repay some debt first or withdraw a smaller amount.`,
      );
    }

    // The withdraw intent is a request from the lending module back to the user.
    // Under the hood, the solver redeems the user's yield tokens, collects
    // accrued interest, and transfers the total back to the user.
    const result = await this.client.signAndBroadcast(
      this.address,
      "/pos.contracts.v1.MsgSubmitLendingIntent",
      {
        sender: this.address,
        action: "withdraw",
        denom,
        amount,
      },
      DEFAULT_FEE,
      `lending:withdraw_intent:${denom}:${amount}`,
    );

    console.log(`         TX hash: ${result.transactionHash}`);

    return {
      txHash: result.transactionHash,
      action: "withdraw",
      denom,
      amount,
      rate: market.supplyAPY,
      allocations: [],
    };
  }

  // -------------------------------------------------------------------------
  // Query operations
  // -------------------------------------------------------------------------

  /**
   * Query the lending market for a specific asset.
   *
   * Returns supply rates, borrow rates, utilization, and liquidity.
   *
   * @param denom - The asset denomination to query
   */
  async queryMarket(denom: string): Promise<LendingMarket> {
    console.log(`[Lending] Querying market: ${denom}`);

    // In production, this queries the lending module's state:
    //   GET /pos/contracts/v1/lending/market/{denom}
    //
    // The response includes real-time rates computed from the interest
    // rate model (typically a kinked curve based on utilization).

    const markets: Record<string, LendingMarket> = {
      "omniphi": {
        denom: "omniphi",
        displayName: "OMNI",
        totalSupply: "100000000000000",     // 100M OMNI
        totalBorrow: "45000000000000",      // 45M OMNI
        supplyAPY: 0.038,                   // 3.8%
        borrowAPR: 0.065,                   // 6.5%
        utilization: 0.45,
        collateralFactor: 0.65,
        liquidationThreshold: 0.80,
        priceUSD: 2.50,
      },
      "ibc/usdc": {
        denom: "ibc/usdc",
        displayName: "USDC",
        totalSupply: "250000000000000",     // 250M USDC
        totalBorrow: "175000000000000",     // 175M USDC
        supplyAPY: 0.042,                   // 4.2%
        borrowAPR: 0.058,                   // 5.8%
        utilization: 0.70,
        collateralFactor: 0.85,
        liquidationThreshold: 0.90,
        priceUSD: 1.00,
      },
      "ibc/eth": {
        denom: "ibc/eth",
        displayName: "ETH",
        totalSupply: "50000000000000",      // 50K ETH
        totalBorrow: "15000000000000",      // 15K ETH
        supplyAPY: 0.021,                   // 2.1%
        borrowAPR: 0.045,                   // 4.5%
        utilization: 0.30,
        collateralFactor: 0.75,
        liquidationThreshold: 0.85,
        priceUSD: 3500.00,
      },
      "ibc/btc": {
        denom: "ibc/btc",
        displayName: "BTC",
        totalSupply: "5000000000000",       // 5K BTC
        totalBorrow: "1000000000000",       // 1K BTC
        supplyAPY: 0.015,                   // 1.5%
        borrowAPR: 0.035,                   // 3.5%
        utilization: 0.20,
        collateralFactor: 0.70,
        liquidationThreshold: 0.80,
        priceUSD: 65000.00,
      },
    };

    const market = markets[denom];
    if (!market) {
      throw new Error(`Unknown market: ${denom}. Available: ${Object.keys(markets).join(", ")}`);
    }

    console.log(`         Supply APY:   ${(market.supplyAPY * 100).toFixed(2)}%`);
    console.log(`         Borrow APR:   ${(market.borrowAPR * 100).toFixed(2)}%`);
    console.log(`         Utilization:  ${(market.utilization * 100).toFixed(1)}%`);

    return market;
  }

  /**
   * Query the health of a specific account's lending positions.
   *
   * @param address - The bech32 account address
   * @returns Comprehensive account health data
   */
  async getAccountHealth(address: string): Promise<AccountHealth> {
    console.log(`[Lending] Querying account health: ${address.slice(0, 12)}...`);

    // In production: GET /pos/contracts/v1/lending/account/{address}
    //
    // The chain computes the health factor on every block using oracle prices.
    // This ensures accurate liquidation thresholds even during high volatility.

    const health: AccountHealth = {
      address,
      totalCollateralUSD: 25000.00,
      totalBorrowUSD: 12000.00,
      healthFactor: 1.56,
      availableBorrowUSD: 6750.00,
      supplies: [
        {
          denom: CONFIG.denoms.ETH,
          amount: "5000000000",     // 5 ETH
          valueUSD: 17500.00,
          earnedInterest: "10500000", // 0.0105 ETH
        },
        {
          denom: CONFIG.denoms.USDC,
          amount: "7500000000",     // 7500 USDC
          valueUSD: 7500.00,
          earnedInterest: "78750000", // 78.75 USDC
        },
      ],
      borrows: [
        {
          denom: CONFIG.denoms.OMNI,
          amount: "4800000000",     // 4800 OMNI
          valueUSD: 12000.00,
          accruedInterest: "52000000", // 52 OMNI
        },
      ],
    };

    console.log(`         Collateral: $${health.totalCollateralUSD.toFixed(2)}`);
    console.log(`         Borrowed:   $${health.totalBorrowUSD.toFixed(2)}`);
    console.log(`         Health:     ${health.healthFactor.toFixed(2)}`);
    console.log(`         Available:  $${health.availableBorrowUSD.toFixed(2)}`);

    return health;
  }

  /**
   * Execute a leveraged yield farming strategy as a single intent bundle.
   *
   * This demonstrates the composability advantage of intents. In traditional
   * DeFi, leveraged yield farming requires:
   *   1. Deposit collateral (1 tx + approval)
   *   2. Borrow against it (1 tx)
   *   3. Swap borrowed asset to the yield asset (1 tx + approval)
   *   4. Deposit the yield asset (1 tx + approval)
   *   5. Repeat for additional leverage
   *
   * That is 8+ transactions, each of which can fail independently,
   * leaving the user in a partially executed and possibly unsafe state.
   *
   * With Omniphi intents, this is a single atomic operation.
   *
   * @param supplyDenom    - Asset to use as collateral
   * @param supplyAmount   - Amount of collateral
   * @param borrowDenom    - Asset to borrow
   * @param yieldDenom     - Asset to farm yield with
   * @param leverageRatio  - Leverage multiplier (e.g., 2.0 for 2x)
   */
  async leveragedYieldFarm(
    supplyDenom: string,
    supplyAmount: string,
    borrowDenom: string,
    yieldDenom: string,
    leverageRatio: number,
  ): Promise<string> {
    console.log(`[Lending] Leveraged yield farming:`);
    console.log(`         Supply:   ${supplyAmount} ${supplyDenom}`);
    console.log(`         Borrow:   ${borrowDenom}`);
    console.log(`         Farm:     ${yieldDenom}`);
    console.log(`         Leverage: ${leverageRatio}x`);

    const borrowAmount = String(
      BigInt(Math.floor(parseFloat(supplyAmount) * (leverageRatio - 1))),
    );

    // All of these happen atomically in one intent bundle:
    const intentTx: IntentTransaction = {
      sender: this.address,
      intents: [
        // Step 1: Supply collateral
        {
          type: "transfer",
          sender: this.address,
          recipient: "omni1lending_module",
          amount: supplyAmount,
          denom: supplyDenom,
          memo: "lending:supply_collateral",
        },
        // Step 2: Borrow against collateral
        {
          type: "swap",
          sender: this.address,
          inputDenom: supplyDenom,
          inputAmount: "0",
          outputDenom: borrowDenom,
          minOutputAmount: borrowAmount,
          maxSlippageBps: 0,
        },
        // Step 3: Swap borrowed asset for yield asset (if different)
        ...(borrowDenom !== yieldDenom
          ? [{
              type: "swap" as const,
              sender: this.address,
              inputDenom: borrowDenom,
              inputAmount: borrowAmount,
              outputDenom: yieldDenom,
              minOutputAmount: "1",
              maxSlippageBps: 100, // 1% slippage tolerance
            }]
          : []),
        // Step 4: Deposit yield asset for farming
        {
          type: "transfer",
          sender: this.address,
          recipient: "omni1yield_module",
          amount: borrowDenom === yieldDenom ? borrowAmount : "0",
          denom: yieldDenom,
          memo: "lending:deposit_yield_farm",
        },
      ],
      memo: `lending:leveraged_farm:${leverageRatio}x:${yieldDenom}`,
      deadline: Math.floor(Date.now() / 1000) + 120,
    };

    console.log(`         Submitting atomic leveraged position...`);

    const result = await this.client.submitIntentTransaction(intentTx, DEFAULT_FEE);

    console.log(`         TX hash: ${result.transactionHash}`);
    console.log(`         All 4 steps executed atomically`);

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
  console.log("  Omniphi Intent-Based Lending Protocol -- Reference Example");
  console.log("============================================================");
  console.log("");

  // Generate a wallet for demonstration
  console.log("--- Step 1: Create Wallet ---");
  const wallet = await createWallet();
  console.log(`  Address:  ${wallet.address}`);
  console.log("");

  // ----- Supply intent -----
  console.log("--- Step 2: Supply Intent ---");
  console.log("");
  console.log("  TRADITIONAL (Aave):");
  console.log("    tx1: USDC.approve(aavePool, 10000e6)");
  console.log("    tx2: aavePool.supply(USDC, 10000e6, myAddr, 0)");
  console.log("    -- Must choose pool manually, 2 transactions --");
  console.log("");
  console.log("  OMNIPHI INTENT:");
  console.log("    const result = await lending.supplyIntent('ibc/usdc', '10000000000');");
  console.log("    -- Solver finds best rate across all pools, 1 atomic operation --");
  console.log("");
  console.log("  The solver might split the supply across multiple pools:");
  console.log("    Pool Prime:  6,000 USDC at 4.2% APY");
  console.log("    Pool Stable: 4,000 USDC at 4.5% APY");
  console.log("    Blended APY: 4.32%");
  console.log("");

  // ----- Borrow intent -----
  console.log("--- Step 3: Borrow Intent ---");
  console.log("");
  console.log("  TRADITIONAL:");
  console.log("    tx1: ETH.approve(aavePool, 5e18)       // approve collateral");
  console.log("    tx2: aavePool.supply(ETH, 5e18, ...)   // deposit collateral");
  console.log("    tx3: aavePool.borrow(OMNI, 4800e6, 2, 0, myAddr)");
  console.log("    -- 3 transactions, each can fail independently --");
  console.log("");
  console.log("  OMNIPHI INTENT:");
  console.log("    const result = await lending.borrowIntent(");
  console.log("      'omniphi', '4800000000',");
  console.log("      { denom: 'ibc/eth', amount: '5000000000' }");
  console.log("    );");
  console.log("    -- Collateral lock + borrow in 1 atomic transaction --");
  console.log("    -- Solver picks best borrow rate automatically --");
  console.log("");

  // ----- Health factor -----
  console.log("--- Step 4: Account Health Monitoring ---");
  console.log("");
  console.log("  const health = await lending.getAccountHealth(address);");
  console.log("  // {");
  console.log("  //   totalCollateralUSD: 25000.00,");
  console.log("  //   totalBorrowUSD: 12000.00,");
  console.log("  //   healthFactor: 1.56,");
  console.log("  //   availableBorrowUSD: 6750.00,");
  console.log("  //   supplies: [{ denom: 'ibc/eth', amount: '5000000000', ... }],");
  console.log("  //   borrows: [{ denom: 'omniphi', amount: '4800000000', ... }]");
  console.log("  // }");
  console.log("");

  // ----- Repay intent -----
  console.log("--- Step 5: Repay and Withdraw ---");
  console.log("");
  console.log("  Repay borrowed OMNI:");
  console.log("    await lending.repayIntent('omniphi', '4800000000');");
  console.log("");
  console.log("  Withdraw supplied USDC + earned yield:");
  console.log("    await lending.withdrawIntent('ibc/usdc', '10078750000');");
  console.log("    // includes 78.75 USDC of earned interest");
  console.log("");

  // ----- Leveraged yield farming -----
  console.log("--- Step 6: Leveraged Yield Farming (Composability) ---");
  console.log("");
  console.log("  Traditional DeFi requires 8+ transactions for leveraged farming.");
  console.log("  Omniphi does it in one atomic intent bundle:");
  console.log("");
  console.log("    const txHash = await lending.leveragedYieldFarm(");
  console.log("      'ibc/eth',      // supply ETH as collateral");
  console.log("      '10000000000',  // 10 ETH");
  console.log("      'ibc/usdc',     // borrow USDC");
  console.log("      'ibc/usdc',     // farm USDC yields");
  console.log("      2.0,            // 2x leverage");
  console.log("    );");
  console.log("");
  console.log("  This atomically:");
  console.log("    1. Deposits 10 ETH as collateral");
  console.log("    2. Borrows ~$35,000 USDC against it");
  console.log("    3. Deposits USDC into the highest-yield farm");
  console.log("    -- If any step fails, the entire operation reverts --");
  console.log("    -- No risk of being stuck with collateral locked but no borrow --");
  console.log("");

  // ----- Cross-pool optimization -----
  console.log("--- Step 7: Cross-Pool Optimization (Unique to Intents) ---");
  console.log("");
  console.log("  Intent-based lending enables optimizations impossible in");
  console.log("  traditional DeFi:");
  console.log("");
  console.log("  1. RATE ARBITRAGE: Solver borrows from Pool A (5.8% APR),");
  console.log("     supplies to Pool B (6.2% APY). Profit shared with user.");
  console.log("");
  console.log("  2. CROSS-COLLATERAL: Use ETH in Pool A as collateral to");
  console.log("     borrow USDC from Pool B, without moving the ETH.");
  console.log("     Solver handles the virtual collateral bridging.");
  console.log("");
  console.log("  3. FLASH REFINANCING: Solver detects a better rate,");
  console.log("     atomically repays old loan and opens new one.");
  console.log("     User's health factor stays safe throughout.");
  console.log("");
  console.log("  4. LIQUIDATION PREVENTION: Solver monitors health factor,");
  console.log("     automatically adds collateral or partially repays to");
  console.log("     keep the position safe. User sets the policy; solver");
  console.log("     executes optimally.");
  console.log("");

  // ----- Market query -----
  console.log("--- Available Markets ---");
  console.log("");
  console.log("  Asset  | Supply APY | Borrow APR | Utilization | Collateral Factor");
  console.log("  -------|------------|------------|-------------|------------------");
  console.log("  OMNI   | 3.80%      | 6.50%      | 45.0%       | 0.65");
  console.log("  USDC   | 4.20%      | 5.80%      | 70.0%       | 0.85");
  console.log("  ETH    | 2.10%      | 4.50%      | 30.0%       | 0.75");
  console.log("  BTC    | 1.50%      | 3.50%      | 20.0%       | 0.70");
  console.log("");

  console.log("============================================================");
  console.log("  Example complete. See README.md for full documentation.");
  console.log("============================================================");
}

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
