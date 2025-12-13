# Treasury Redirect Mechanism

## Overview

The Treasury Redirect Mechanism is a **post-collection allocation system** that automatically routes a configurable portion of NEW treasury inflows to designated ecosystem funds. This is NOT a fee or tax—it operates exclusively on funds already collected by the treasury.

## Design Principles

### What Treasury Redirect IS:
- A **post-collection allocation rule** on treasury inflows
- An **internal redistribution** within the protocol's own funds
- A **DAO-governed** mechanism with protocol-enforced safety caps
- A **deterministic, atomic** operation executed at regular intervals

### What Treasury Redirect is NOT:
- NOT a fee on users
- NOT a tax on transactions
- NOT a validator revenue deduction
- NOT double taxation

## Flow of Funds

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           TRANSACTION FEES                                   │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        FEE COLLECTION (fee_collector.go)                     │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Total Fees = Base Fee × Gas Used × Type Multiplier                 │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│           ┌────────────────────────┼────────────────────────┐               │
│           │                        │                        │               │
│           ▼                        ▼                        ▼               │
│   ┌───────────────┐       ┌───────────────┐       ┌───────────────┐        │
│   │  BURN AMOUNT  │       │  VALIDATORS   │       │   TREASURY    │        │
│   │  (Dynamic %)  │       │    (70%)      │       │    (30%)      │        │
│   │               │       │               │       │               │        │
│   │  Permanently  │       │  Distributed  │       │  Inflow       │        │
│   │  Destroyed    │       │  to Stakers   │       │  Tracked      │        │
│   └───────────────┘       └───────────────┘       └───────────────┘        │
└─────────────────────────────────────────────────────────────────────────────┘
                                                            │
                                                            │ Accumulated
                                                            │ Inflows
                                                            ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    TREASURY REDIRECT (treasury_redirect.go)                  │
│                    Executes every N blocks (interval=100 mainnet)            │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Redirect Amount = Accumulated Inflows × Redirect Ratio (≤10%)      │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│    ┌───────────────┬───────────────┼───────────────┬───────────────┐        │
│    │               │               │               │               │        │
│    ▼               ▼               ▼               ▼               ▼        │
│ ┌────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐     │
│ │Treasury│   │Ecosystem │   │Buy & Burn│   │Insurance │   │Research  │     │
│ │Retained│   │ Grants   │   │  Fund    │   │  Fund    │   │  Fund    │     │
│ │  90%   │   │   40%    │   │   30%    │   │   20%    │   │   10%    │     │
│ │        │   │of redirect   │of redirect   │of redirect   │of redirect     │
│ └────────┘   └──────────┘   └──────────┘   └──────────┘   └──────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Economic Guarantees

### 1. Protocol-Enforced 10% Cap

The maximum redirect ratio is **hardcoded at 10%** in the protocol:

```go
// MaxRedirectRatio is the protocol-enforced maximum redirect ratio
// This CANNOT be changed by governance - it's a safety invariant
const MaxRedirectRatio = "0.10"
```

Even if governance attempts to set a higher ratio, the protocol enforces the cap:

```go
maxRatio := math.LegacyMustNewDecFromStr(MaxRedirectRatio)
if redirectRatio.GT(maxRatio) {
    redirectRatio = maxRatio  // Enforce protocol cap
}
```

### 2. Inflow-Only Operation

The redirect mechanism ONLY operates on NEW treasury inflows, never on the existing treasury balance:

- `accumulated_redirect_inflows` tracks only new deposits since last redirect
- After each redirect execution, the accumulator resets to zero
- Historical treasury funds are never touched

### 3. No Validator Impact

Validator rewards are calculated and distributed BEFORE treasury allocation:
- Validators receive their 70% share of post-burn fees
- Treasury redirect operates on the remaining 30% that went to treasury
- Validator income is completely isolated from redirect mechanism

### 4. No Double Taxation

The fee flow is strictly sequential with no overlap:
1. User pays transaction fee
2. Dynamic burn is applied (removed from circulation)
3. Remaining split: 70% validators, 30% treasury
4. Treasury redirect operates ONLY on the 30% already allocated to treasury
5. No additional fees are extracted at any point

### 5. Atomic Execution

All allocations execute atomically within a single transaction:
- Either ALL target allocations succeed, or NONE do
- No partial state on failure
- Dust (rounding remainder) stays in treasury

## Governance Controls

### DAO-Adjustable Parameters

| Parameter | Description | Default | Constraints |
|-----------|-------------|---------|-------------|
| `treasury_redirect_enabled` | Master on/off switch | `true` | Boolean |
| `treasury_redirect_ratio` | % of inflows to redirect | `0.10` (10%) | 0-10%, protocol capped |
| `redirect_to_ecosystem_grants` | Grants allocation | `0.40` (40%) | Must sum to 100% |
| `redirect_to_buy_and_burn` | Buy-back allocation | `0.30` (30%) | Must sum to 100% |
| `redirect_to_insurance_fund` | Insurance allocation | `0.20` (20%) | Must sum to 100% |
| `redirect_to_research_fund` | R&D allocation | `0.10` (10%) | Must sum to 100% |
| `redirect_execution_interval` | Blocks between executions | `100` | Must be > 0 |

### Parameter Validation

```go
func ValidateTreasuryRedirectParams(params Params) error {
    // 1. Redirect ratio must be 0-10%
    if ratio < 0 || ratio > 0.10 {
        return ErrInvalidRedirectRatio
    }

    // 2. Target allocations must sum to exactly 100%
    total := ecosystemGrants + buyAndBurn + insuranceFund + researchFund
    if total != 1.0 {
        return ErrInvalidAllocationSum
    }

    // 3. Execution interval must be positive
    if interval <= 0 {
        return ErrInvalidInterval
    }
}
```

### Governance Proposal Example

To modify treasury redirect parameters via governance:

```json
{
  "title": "Adjust Treasury Redirect Allocations",
  "description": "Increase ecosystem grants allocation from 40% to 50%",
  "changes": [
    {
      "subspace": "tokenomics",
      "key": "redirect_to_ecosystem_grants",
      "value": "0.50"
    },
    {
      "subspace": "tokenomics",
      "key": "redirect_to_buy_and_burn",
      "value": "0.25"
    },
    {
      "subspace": "tokenomics",
      "key": "redirect_to_insurance_fund",
      "value": "0.15"
    },
    {
      "subspace": "tokenomics",
      "key": "redirect_to_research_fund",
      "value": "0.10"
    }
  ]
}
```

## Target Fund Purposes

### Ecosystem Grants (40%)
- Developer grants and bounties
- Protocol integrations
- Ecosystem growth initiatives
- Hackathon prizes

### Buy and Burn (30%)
- Market buybacks of OMNI tokens
- Purchased tokens are permanently burned
- Deflationary pressure mechanism

### Insurance Fund (20%)
- Protocol risk coverage
- Validator slashing compensation
- Black swan event reserves
- Bridge security fund

### Research Fund (10%)
- Protocol R&D
- Security audits
- Academic partnerships
- Innovation grants

## State Management

### Storage Keys

```go
KeyAccumulatedRedirectInflows = []byte{0x34}  // Current period inflows
KeyLastRedirectHeight         = []byte{0x35}  // Last execution block
KeyTotalRedirected            = []byte{0x36}  // All-time redirected amount
KeyEcosystemGrantsAddress     = []byte{0x37}  // Target addresses
KeyBuyAndBurnAddress          = []byte{0x38}
KeyInsuranceFundAddress       = []byte{0x39}
KeyResearchFundAddress        = []byte{0x3A}
```

### Events Emitted

```go
EventTypeTreasuryRedirect = "treasury_redirect"
// Attributes:
//   - total_inflows: Amount accumulated since last redirect
//   - redirect_amount: Total amount being redirected
//   - ecosystem_grants_amount: Amount to grants fund
//   - buy_and_burn_amount: Amount to buy-back fund
//   - insurance_fund_amount: Amount to insurance
//   - research_fund_amount: Amount to R&D
//   - execution_height: Block height of execution

EventTypeTreasuryAllocation = "treasury_allocation"
// Emitted for each individual allocation
```

## Execution Flow

### Block EndBlocker Integration

```go
func (k Keeper) EndBlocker(ctx context.Context) error {
    // ... other end block logic ...

    // Process treasury redirect if interval reached
    result, err := k.ProcessTreasuryRedirect(ctx)
    if err != nil {
        return err
    }

    if result != nil {
        // Log redirect execution
        k.Logger(ctx).Info("treasury redirect executed",
            "total_redirected", result.TotalRedirected,
            "height", ctx.BlockHeight(),
        )
    }

    return nil
}
```

### Redirect Execution Logic

```go
func (k Keeper) ProcessTreasuryRedirect(ctx context.Context) (*RedirectResult, error) {
    // 1. Check if enabled
    if !params.TreasuryRedirectEnabled {
        return nil, nil
    }

    // 2. Check execution interval
    if currentHeight - lastHeight < interval {
        return nil, nil
    }

    // 3. Get accumulated inflows
    inflows := k.GetAccumulatedRedirectInflows(ctx)
    if inflows.IsZero() {
        return nil, nil
    }

    // 4. Calculate redirect with protocol cap enforcement
    ratio := min(params.RedirectRatio, 0.10)
    redirectAmount := inflows * ratio

    // 5. Calculate individual allocations
    ecosystemAmount := redirectAmount * params.EcosystemGrantsRatio
    buyBurnAmount := redirectAmount * params.BuyAndBurnRatio
    insuranceAmount := redirectAmount * params.InsuranceFundRatio
    researchAmount := redirectAmount * params.ResearchFundRatio

    // 6. Handle dust (rounding remainder stays in treasury)
    dust := redirectAmount - (ecosystem + buyBurn + insurance + research)

    // 7. Execute transfers atomically
    // ... bank sends to each target address ...

    // 8. Update state
    k.ResetAccumulatedRedirectInflows(ctx)
    k.SetLastRedirectHeight(ctx, currentHeight)
    k.IncrementTotalRedirected(ctx, redirectAmount)

    // 9. Emit events
    // ...

    return result, nil
}
```

## Network Configuration

### Mainnet (omniphi-mainnet-1)

```json
{
  "treasury_redirect_enabled": true,
  "treasury_redirect_ratio": "0.10",
  "redirect_to_ecosystem_grants": "0.40",
  "redirect_to_buy_and_burn": "0.30",
  "redirect_to_insurance_fund": "0.20",
  "redirect_to_research_fund": "0.10",
  "redirect_execution_interval": 100
}
```

Execution: Every 100 blocks (~10 minutes at 6s blocks)

### Testnet (omniphi-testnet-1)

```json
{
  "treasury_redirect_enabled": true,
  "treasury_redirect_ratio": "0.10",
  "redirect_to_ecosystem_grants": "0.40",
  "redirect_to_buy_and_burn": "0.30",
  "redirect_to_insurance_fund": "0.20",
  "redirect_to_research_fund": "0.10",
  "redirect_execution_interval": 50
}
```

Execution: Every 50 blocks (~5 minutes) for faster testing

## Audit Notes

### Confirmed: No Double Taxation

```
User Fee → [BURN] → [VALIDATORS 70%] → [TREASURY 30%] → [REDIRECT ≤10% of 30%]
                           ↑                                      ↑
                    Untouched by redirect              Only affects treasury share
```

The maximum effective redirect from total fees is: 30% × 10% = **3% of total fees**

### Confirmed: No Validator Impact

Validators receive exactly `(TotalFees - BurnAmount) × 0.70` regardless of redirect settings.

### Confirmed: Protocol-Enforced Caps

- Redirect ratio: Hardcoded 10% maximum
- Target allocations: Must sum to exactly 100%
- Execution interval: Must be positive integer

### Confirmed: Atomic Execution

All allocations succeed or fail together. No partial state possible.

## Query Endpoints

### Query Current Redirect State

```bash
posd query tokenomics treasury-redirect-state
```

Response:
```json
{
  "enabled": true,
  "redirect_ratio": "0.10",
  "accumulated_inflows": "1000000000",
  "last_redirect_height": "12500",
  "total_redirected_all_time": "50000000000",
  "next_execution_height": "12600"
}
```

### Query Target Addresses

```bash
posd query tokenomics redirect-targets
```

Response:
```json
{
  "ecosystem_grants_address": "omni1...",
  "buy_and_burn_address": "omni1...",
  "insurance_fund_address": "omni1...",
  "research_fund_address": "omni1..."
}
```

## Security Considerations

1. **Target Address Validation**: All target addresses must be valid bech32 addresses
2. **Overflow Protection**: All arithmetic uses SDK math types with overflow checks
3. **Zero Amount Handling**: Zero-amount transfers are skipped (no empty operations)
4. **Module Account Security**: Target addresses should be module accounts or multisigs
5. **Governance Timelock**: Parameter changes go through standard governance voting period

## Comparison with Industry Standards

| Feature | Omniphi | Cosmos Hub | Osmosis |
|---------|---------|------------|---------|
| Treasury redirect | 10% max | N/A | N/A |
| Community tax | 2% | 2% | 0% |
| Protocol-owned liquidity | Via buy-back | N/A | POL |
| Insurance fund | 20% of redirect | N/A | N/A |
| Research funding | 10% of redirect | Grants | Grants |

Omniphi's treasury redirect is a novel mechanism providing sustainable protocol funding while maintaining strong economic guarantees and governance controls.
