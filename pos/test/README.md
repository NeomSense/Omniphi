# Omniphi Comprehensive Test Suite

This directory contains the complete test implementation for the Omniphi blockchain comprehensive test program, covering all 122 test cases across P0/P1/P2 priorities.

## Directory Structure

```
test/
├── comprehensive/          # P0 unit tests for critical functionality
│   ├── test_base.go       # Common test infrastructure and mocks
│   ├── supply_tests.go    # TC-001 to TC-013: Supply & monetary policy
│   └── governance_tests.go # TC-022 to TC-033: Governance & time-lock
├── reconciliation/         # Reconciliation and audit tools
│   └── reconciliation.go  # Supply, burn, reward, and fee auditors
└── README.md              # This file
```

## Test Coverage

### Phase 1 (Implemented)
✅ **Supply & Monetary Policy Tests** (TC-001 to TC-013)
- Hard cap enforcement
- Inflation bounds (1-5%)
- Genesis integrity
- Burn correctness
- Supply conservation

✅ **Governance Tests** (TC-022 to TC-033)
- Time-lock enforcement
- Permission scope
- Quorum/threshold logic
- Atomic execution

✅ **Reconciliation Tools**
- Supply reconciliation checker
- Per-module burn auditor
- Epoch reward validator
- Fee split auditor

### Coming Soon
🔄 **PoC Tests** (TC-034 to TC-049)
🔄 **Fee & Gas Tests** (TC-050 to TC-060)
🔄 **Integration Test Harness** (Phase 2)
🔄 **Soak Tests** (Phase 3)

## Quick Start

### Run All Tests
```bash
# Run all comprehensive tests
./scripts/run_comprehensive_tests.sh

# Run with verbose output
./scripts/run_comprehensive_tests.sh --verbose

# Run with coverage report
./scripts/run_comprehensive_tests.sh --coverage
```

### Run Specific Test Categories
```bash
# Run only P0 tests
./scripts/run_comprehensive_tests.sh --priority p0

# Run specific test
./scripts/run_comprehensive_tests.sh --test TestTC001_HardCapEnforcementAtBoundary

# Run supply tests only
go test -v ./test/comprehensive -run TestTC00
```

### Using Make (if available)
```bash
# Run all unit tests
make test-unit

# Run with race detector
make test-race

# Generate coverage report
make test-cover
```

## Test Categories

### P0: Critical Tests (Must Pass 100%)

#### Supply & Monetary Policy (13 tests)
- **TC-001**: Hard cap enforcement at boundary
- **TC-002**: Hard cap under concurrent mints
- **TC-003**: Inflation below minimum (1%) rejected
- **TC-004**: Inflation above maximum (5%) rejected
- **TC-005**: Valid inflation update
- **TC-006**: Genesis integrity
- **TC-007**: Base fee burn correctness
- **TC-008**: Contract fee burn correctness
- **TC-009**: Module fee burn correctness
- **TC-010**: Burn underflow protection
- **TC-011**: Supply reconciliation (mint vs burn)
- **TC-012**: Module transfer reconciliation
- **TC-013**: Epoch rollover precision

#### Governance (12 tests)
- **TC-022**: Time-lock early execution prevention
- **TC-023**: Time-lock exact expiry
- **TC-024**: Param change causality
- **TC-025**: Unauthorized param mutation prevention
- **TC-026**: Treasury spending authority
- **TC-027**: Minter cap enforcement
- **TC-028**: Burner cap enforcement
- **TC-029**: Below quorum rejection
- **TC-030**: Exact quorum threshold
- **TC-031**: Tie vote rejection
- **TC-032**: Super-majority pass
- **TC-033**: Atomic execution (no partial state)

## Reconciliation Tools

### Supply Reconciliation
```go
import "pos/test/reconciliation"

// Create checker
checker := reconciliation.NewSupplyChecker(
    genesisSupply,
    hardCap,
    driftTolerance,
)

// Perform check
result := checker.Check(ctx, observedSupply, totalMinted, totalBurned)

// Generate report
fmt.Println(result.Report())
```

**Output:**
```
═══════════════════════════════════════════════════════════
                Supply Reconciliation Report
═══════════════════════════════════════════════════════════
Timestamp:          2025-10-21T15:30:45Z
Block Height:       12345

Genesis Supply:     1000000000000000 omniphi
Total Minted:       50000000000000 omniphi
Total Burned:       20000000000000 omniphi
Expected Supply:    1030000000000000 omniphi
Observed Supply:    1030000000000003 omniphi

Drift:              3 omniphi (0.000000%)
Status:             PASS
═══════════════════════════════════════════════════════════
```

### Burn Auditor
```go
// Create auditor
auditor := reconciliation.NewBurnAuditor()

// Record burns
auditor.RecordBurn("base_fee", amount1)
auditor.RecordBurn("contract", amount2)
auditor.RecordBurn("module", amount3)

// Get breakdown
breakdown := auditor.GetBreakdown(ctx, startBlock, endBlock)
fmt.Println(breakdown.Report())
```

**Output:**
```
═══════════════════════════════════════════════════════════
              Per-Module Burn Breakdown
═══════════════════════════════════════════════════════════
Module              Amount Burned      % of Total   Tx Count   Avg/Tx
──────────────────────────────────────────────────────────────
base_fee            12000000000000       60.00%    2400000    5000000
contract             6000000000000       30.00%     600000   10000000
module               2000000000000       10.00%     200000   10000000
──────────────────────────────────────────────────────────────
TOTAL               20000000000000      100.00%
═══════════════════════════════════════════════════════════
```

### Reward Validator
```go
// Create validator
validator := reconciliation.NewRewardValidator(dustTolerance)

// Validate epoch rewards
result := validator.ValidateEpochRewards(
    ctx,
    epochNumber,
    mintBudget,
    validatorRewards,
    pocRewards,
)

fmt.Println(result.Report())
```

### Fee Auditor
```go
// Define expected ratios
expectedRatios := reconciliation.FeeSplitRatios{
    Validator: 50.0,  // 50%
    Treasury:  30.0,  // 30%
    Burn:      20.0,  // 20%
}

// Create auditor
auditor := reconciliation.NewFeeAuditor(expectedRatios, tolerance)

// Perform audit
result := auditor.Audit(
    ctx,
    startBlock, endBlock,
    totalFees, validatorShare, treasuryShare, burnShare,
)

fmt.Println(result.Report())
```

## Writing New Tests

### Test Structure
All tests follow this pattern:

```go
package comprehensive

import (
    "testing"
    "github.com/stretchr/testify/require"
)

// TC-XXX: Test Name
// Priority: P0/P1/P2
// Purpose: Brief description of what is being tested
func TestTCXXX_TestName(t *testing.T) {
    // Setup test context
    tc := SetupTestContext(t)

    // Perform test actions
    // ...

    // Verify expected results
    require.Equal(t, expected, actual, "Failure message")

    // Use helper assertions
    AssertSupplyWithinCap(t, tc.BankKeeper)
    AssertSupplyConservation(t, tc.BankKeeper, initialSupply)
    AssertNoNegativeBalances(t, tc.BankKeeper)
}
```

### Available Mocks

**MockAccountKeeper**
- `GetModuleAddress(name string) sdk.AccAddress`
- `GetModuleAccount(ctx, name string) sdk.ModuleAccountI`

**MockBankKeeper**
- `GetBalance(ctx, addr, denom) sdk.Coin`
- `GetAllBalances(ctx, addr) sdk.Coins`
- `SendCoinsFromModuleToAccount(ctx, module, addr, amt) error`
- `SendCoinsFromAccountToModule(ctx, addr, module, amt) error`
- `MintCoins(ctx, module, amt) error`
- `BurnCoins(ctx, module, amt) error`
- `GetSupply(ctx, denom) sdk.Coin`
- `GetMinted() sdk.Coins` (test helper)
- `GetBurned() sdk.Coins` (test helper)

**MockStakingKeeper**
- `TotalBondedTokens(ctx) math.Int`
- `SetTotalBonded(amount math.Int)` (test helper)

### Helper Functions

**AssertSupplyWithinCap(t, bankKeeper)**
- Verifies total supply ≤ hard cap (1.5B OMNI)

**AssertSupplyConservation(t, bankKeeper, initialSupply)**
- Verifies: `current_supply = initial + minted - burned`

**AssertNoNegativeBalances(t, bankKeeper)**
- Verifies no account or module has negative balance

## Constants

```go
const (
    TestDenom        = "omniphi"
    TestHardCap      = 1_500_000_000_000_000  // 1.5B OMNI (6 decimals)
    TestMinInflation = 0.01                    // 1%
    TestMaxInflation = 0.05                    // 5%
)
```

## Running Tests in CI/CD

### GitHub Actions Example
```yaml
name: Comprehensive Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.22'

      - name: Run P0 Tests
        run: ./scripts/run_comprehensive_tests.sh --priority p0

      - name: Generate Coverage
        run: ./scripts/run_comprehensive_tests.sh --coverage

      - name: Upload Coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
```

## Troubleshooting

### Tests Failing
1. **Check module initialization**
   - Ensure all mocks are properly set up
   - Verify params are initialized with `DefaultParams()`

2. **Supply drift errors**
   - Check that all mints/burns are tracked
   - Verify no operations bypass the bank keeper

3. **Governance tests failing**
   - Ensure context block time/height are set correctly
   - Verify time-lock duration matches test expectations

### Common Errors

**"Params not initialized"**
```go
// Fix: Initialize params in setup
params := types.DefaultParams()
tc.TokenomicsKeeper.Params.Set(tc.Ctx, params)
```

**"Supply exceeds cap"**
```go
// Fix: Set initial supply within cap
initialSupply := math.NewInt(1_000_000_000_000_000) // 1B OMNI
tc.BankKeeper.SetSupply(sdk.NewCoins(sdk.NewCoin(TestDenom, initialSupply)))
```

**"Conservation violated"**
```go
// Fix: Track all mints and burns
tc.BankKeeper.MintCoins(...)  // Tracked automatically
tc.BankKeeper.BurnCoins(...)  // Tracked automatically
```

## Exit Criteria

### P0 Tests
- **Pass Rate**: 100% required
- **Invariant Violations**: 0 allowed
- **Determinism**: All tests must pass consistently with same seed

### P1 Tests
- **Pass Rate**: ≥95% required
- **Critical Flows**: All must complete successfully

### P2 Tests
- **Pass Rate**: ≥90% required
- **Observability**: Reports must match raw data

## Next Steps

1. **Add PoC Tests** (TC-034 to TC-049)
   - Contribution lifecycle
   - Effective power formula
   - Collusion detection
   - Rate limiting

2. **Add Fee Tests** (TC-050 to TC-060)
   - Min gas price enforcement
   - Fee split accuracy
   - Dual-VM gas parity (if enabled)

3. **Integration Test Harness**
   - Dev chain startup/teardown
   - Transaction generators
   - Scenario runners

4. **Soak Tests**
   - 10k+ block tests
   - Drift detection
   - Performance monitoring

## Support

For questions or issues:
1. Check this README
2. Review test code comments
3. Check the main test program document
4. Open an issue in the repository

## License

Same as Omniphi blockchain project.
