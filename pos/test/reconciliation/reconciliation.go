package reconciliation

import (
	"fmt"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SupplyReconciliation represents a supply reconciliation check result
type SupplyReconciliation struct {
	Timestamp       time.Time
	BlockHeight     int64
	GenesisSupply   math.Int
	TotalMinted     math.Int
	TotalBurned     math.Int
	ExpectedSupply  math.Int
	ObservedSupply  math.Int
	Drift           math.Int
	DriftPercentage float64
	Status          string
	Errors          []string
}

// SupplyChecker performs supply reconciliation checks
type SupplyChecker struct {
	genesisSupply math.Int
	hardCap       math.Int
	driftTolerance math.Int
}

// NewSupplyChecker creates a new supply checker
func NewSupplyChecker(genesisSupply, hardCap, driftTolerance math.Int) *SupplyChecker {
	return &SupplyChecker{
		genesisSupply:  genesisSupply,
		hardCap:        hardCap,
		driftTolerance: driftTolerance,
	}
}

// Check performs a full supply reconciliation
func (sc *SupplyChecker) Check(
	ctx sdk.Context,
	observedSupply, totalMinted, totalBurned math.Int,
) *SupplyReconciliation {
	result := &SupplyReconciliation{
		Timestamp:      ctx.BlockTime(),
		BlockHeight:    ctx.BlockHeight(),
		GenesisSupply:  sc.genesisSupply,
		TotalMinted:    totalMinted,
		TotalBurned:    totalBurned,
		ObservedSupply: observedSupply,
		Errors:         []string{},
	}

	// Calculate expected supply
	result.ExpectedSupply = sc.genesisSupply.Add(totalMinted).Sub(totalBurned)

	// Calculate drift
	result.Drift = observedSupply.Sub(result.ExpectedSupply).Abs()

	// Calculate drift percentage
	if result.ExpectedSupply.GT(math.ZeroInt()) {
		driftFloat := float64(result.Drift.Int64())
		expectedFloat := float64(result.ExpectedSupply.Int64())
		result.DriftPercentage = (driftFloat / expectedFloat) * 100
	}

	// Check invariants
	result.Status = "PASS"

	// Invariant 1: Supply must not exceed hard cap
	if observedSupply.GT(sc.hardCap) {
		result.Status = "FAIL"
		result.Errors = append(result.Errors,
			fmt.Sprintf("Supply %s exceeds hard cap %s", observedSupply, sc.hardCap))
	}

	// Invariant 2: Drift must be within tolerance
	if result.Drift.GT(sc.driftTolerance) {
		result.Status = "FAIL"
		result.Errors = append(result.Errors,
			fmt.Sprintf("Drift %s exceeds tolerance %s", result.Drift, sc.driftTolerance))
	}

	// Invariant 3: Supply conservation (minted - burned = net change)
	netChange := observedSupply.Sub(sc.genesisSupply)
	expectedNetChange := totalMinted.Sub(totalBurned)
	if !netChange.Equal(expectedNetChange) {
		delta := netChange.Sub(expectedNetChange).Abs()
		if delta.GT(sc.driftTolerance) {
			result.Status = "FAIL"
			result.Errors = append(result.Errors,
				fmt.Sprintf("Conservation violated: net change %s != minted-burned %s (delta: %s)",
					netChange, expectedNetChange, delta))
		}
	}

	return result
}

// Report generates a human-readable report
func (sr *SupplyReconciliation) Report() string {
	report := fmt.Sprintf(`
═══════════════════════════════════════════════════════════
                Supply Reconciliation Report
═══════════════════════════════════════════════════════════
Timestamp:          %s
Block Height:       %d

Genesis Supply:     %s omniphi
Total Minted:       %s omniphi
Total Burned:       %s omniphi
Expected Supply:    %s omniphi
Observed Supply:    %s omniphi

Drift:              %s omniphi (%.6f%%)
Status:             %s
`,
		sr.Timestamp.Format(time.RFC3339),
		sr.BlockHeight,
		sr.GenesisSupply.String(),
		sr.TotalMinted.String(),
		sr.TotalBurned.String(),
		sr.ExpectedSupply.String(),
		sr.ObservedSupply.String(),
		sr.Drift.String(),
		sr.DriftPercentage,
		sr.Status,
	)

	if len(sr.Errors) > 0 {
		report += "\nERRORS:\n"
		for i, err := range sr.Errors {
			report += fmt.Sprintf("  %d. %s\n", i+1, err)
		}
	}

	report += "═══════════════════════════════════════════════════════════\n"

	return report
}

// BurnBreakdown represents per-module burn statistics
type BurnBreakdown struct {
	Timestamp   time.Time
	BlockHeight int64
	BlockRange  [2]int64 // [start, end]
	Modules     map[string]BurnStats
	TotalBurned math.Int
}

// BurnStats contains burn statistics for a module
type BurnStats struct {
	ModuleName    string
	AmountBurned  math.Int
	TxCount       int64
	Percentage    float64
	AveragePerTx  math.Int
}

// BurnAuditor tracks and validates burn operations
type BurnAuditor struct {
	burnsByModule map[string]math.Int
	burnTxCount   map[string]int64
	totalBurned   math.Int
}

// NewBurnAuditor creates a new burn auditor
func NewBurnAuditor() *BurnAuditor {
	return &BurnAuditor{
		burnsByModule: make(map[string]math.Int),
		burnTxCount:   make(map[string]int64),
		totalBurned:   math.ZeroInt(),
	}
}

// RecordBurn records a burn operation
func (ba *BurnAuditor) RecordBurn(module string, amount math.Int) {
	if current, exists := ba.burnsByModule[module]; exists {
		ba.burnsByModule[module] = current.Add(amount)
	} else {
		ba.burnsByModule[module] = amount
	}

	ba.burnTxCount[module]++
	ba.totalBurned = ba.totalBurned.Add(amount)
}

// GetBreakdown returns burn breakdown report
func (ba *BurnAuditor) GetBreakdown(ctx sdk.Context, startBlock, endBlock int64) *BurnBreakdown {
	breakdown := &BurnBreakdown{
		Timestamp:   ctx.BlockTime(),
		BlockHeight: ctx.BlockHeight(),
		BlockRange:  [2]int64{startBlock, endBlock},
		Modules:     make(map[string]BurnStats),
		TotalBurned: ba.totalBurned,
	}

	totalFloat := float64(ba.totalBurned.Int64())

	for module, amount := range ba.burnsByModule {
		txCount := ba.burnTxCount[module]
		avgPerTx := math.ZeroInt()
		if txCount > 0 {
			avgPerTx = amount.QuoRaw(txCount)
		}

		percentage := 0.0
		if ba.totalBurned.GT(math.ZeroInt()) {
			percentage = (float64(amount.Int64()) / totalFloat) * 100
		}

		breakdown.Modules[module] = BurnStats{
			ModuleName:   module,
			AmountBurned: amount,
			TxCount:      txCount,
			Percentage:   percentage,
			AveragePerTx: avgPerTx,
		}
	}

	return breakdown
}

// Report generates burn breakdown report
func (bb *BurnBreakdown) Report() string {
	report := fmt.Sprintf(`
═══════════════════════════════════════════════════════════
              Per-Module Burn Breakdown
═══════════════════════════════════════════════════════════
Timestamp:     %s
Block Range:   %d - %d

Module              Amount Burned      %% of Total   Tx Count   Avg/Tx
──────────────────────────────────────────────────────────────
`,
		bb.Timestamp.Format(time.RFC3339),
		bb.BlockRange[0],
		bb.BlockRange[1],
	)

	for _, stats := range bb.Modules {
		report += fmt.Sprintf("%-18s  %15s  %8.2f%%  %8d  %10s\n",
			stats.ModuleName,
			stats.AmountBurned.String(),
			stats.Percentage,
			stats.TxCount,
			stats.AveragePerTx.String(),
		)
	}

	report += "──────────────────────────────────────────────────────────────\n"
	report += fmt.Sprintf("%-18s  %15s  %8.2f%%\n",
		"TOTAL",
		bb.TotalBurned.String(),
		100.0,
	)
	report += "═══════════════════════════════════════════════════════════\n"

	return report
}

// RewardReconciliation represents epoch reward validation
type RewardReconciliation struct {
	Timestamp        time.Time
	Epoch            int64
	MintBudget       math.Int
	ValidatorRewards math.Int
	PoCRewards       math.Int
	TotalDistributed math.Int
	Dust             math.Int
	Leakage          math.Int
	Status           string
	Errors           []string
}

// RewardValidator validates reward distribution
type RewardValidator struct {
	dustTolerance math.Int
}

// NewRewardValidator creates a new reward validator
func NewRewardValidator(dustTolerance math.Int) *RewardValidator {
	return &RewardValidator{
		dustTolerance: dustTolerance,
	}
}

// ValidateEpochRewards validates reward distribution for an epoch
func (rv *RewardValidator) ValidateEpochRewards(
	ctx sdk.Context,
	epoch int64,
	mintBudget, validatorRewards, pocRewards math.Int,
) *RewardReconciliation {
	result := &RewardReconciliation{
		Timestamp:        ctx.BlockTime(),
		Epoch:            epoch,
		MintBudget:       mintBudget,
		ValidatorRewards: validatorRewards,
		PoCRewards:       pocRewards,
		Errors:           []string{},
	}

	// Calculate total distributed
	result.TotalDistributed = validatorRewards.Add(pocRewards)

	// Calculate dust (rounding leftover)
	result.Dust = mintBudget.Sub(result.TotalDistributed)

	// Check if leakage occurred (over-distribution)
	if result.TotalDistributed.GT(mintBudget) {
		result.Leakage = result.TotalDistributed.Sub(mintBudget)
		result.Status = "FAIL"
		result.Errors = append(result.Errors,
			fmt.Sprintf("Over-distribution detected: distributed %s > budget %s (leakage: %s)",
				result.TotalDistributed, mintBudget, result.Leakage))
	} else if result.Dust.GT(rv.dustTolerance) {
		// Dust exceeds tolerance (too much undistributed)
		result.Status = "WARN"
		result.Errors = append(result.Errors,
			fmt.Sprintf("Dust %s exceeds tolerance %s", result.Dust, rv.dustTolerance))
	} else {
		result.Status = "PASS"
	}

	return result
}

// Report generates reward reconciliation report
func (rr *RewardReconciliation) Report() string {
	report := fmt.Sprintf(`
═══════════════════════════════════════════════════════════
            Epoch Reward Reconciliation
═══════════════════════════════════════════════════════════
Timestamp:           %s
Epoch:               %d

Mint Budget:         %s omniphi
Validator Rewards:   %s omniphi
PoC Rewards:         %s omniphi
Total Distributed:   %s omniphi
Dust/Rounding:       %s omniphi
Leakage:             %s omniphi

Status:              %s
`,
		rr.Timestamp.Format(time.RFC3339),
		rr.Epoch,
		rr.MintBudget.String(),
		rr.ValidatorRewards.String(),
		rr.PoCRewards.String(),
		rr.TotalDistributed.String(),
		rr.Dust.String(),
		rr.Leakage.String(),
		rr.Status,
	)

	if len(rr.Errors) > 0 {
		report += "\nISSUES:\n"
		for i, err := range rr.Errors {
			report += fmt.Sprintf("  %d. %s\n", i+1, err)
		}
	}

	report += "═══════════════════════════════════════════════════════════\n"

	return report
}

// FeeAudit represents fee split validation
type FeeAudit struct {
	Timestamp       time.Time
	BlockRange      [2]int64
	TotalFees       math.Int
	ValidatorShare  math.Int
	TreasuryShare   math.Int
	BurnShare       math.Int
	ExpectedSplit   FeeSplitRatios
	ActualSplit     FeeSplitRatios
	Discrepancy     math.Int
	Status          string
	Errors          []string
}

// FeeSplitRatios defines fee split percentages
type FeeSplitRatios struct {
	Validator float64
	Treasury  float64
	Burn      float64
}

// FeeAuditor validates fee split accuracy
type FeeAuditor struct {
	expectedRatios FeeSplitRatios
	tolerance      math.Int
}

// NewFeeAuditor creates a new fee auditor
func NewFeeAuditor(expectedRatios FeeSplitRatios, tolerance math.Int) *FeeAuditor {
	return &FeeAuditor{
		expectedRatios: expectedRatios,
		tolerance:      tolerance,
	}
}

// Audit performs fee split audit
func (fa *FeeAuditor) Audit(
	ctx sdk.Context,
	startBlock, endBlock int64,
	totalFees, validatorShare, treasuryShare, burnShare math.Int,
) *FeeAudit {
	result := &FeeAudit{
		Timestamp:      ctx.BlockTime(),
		BlockRange:     [2]int64{startBlock, endBlock},
		TotalFees:      totalFees,
		ValidatorShare: validatorShare,
		TreasuryShare:  treasuryShare,
		BurnShare:      burnShare,
		ExpectedSplit:  fa.expectedRatios,
		Errors:         []string{},
	}

	// Calculate actual split percentages
	if totalFees.GT(math.ZeroInt()) {
		totalFloat := float64(totalFees.Int64())
		result.ActualSplit = FeeSplitRatios{
			Validator: (float64(validatorShare.Int64()) / totalFloat) * 100,
			Treasury:  (float64(treasuryShare.Int64()) / totalFloat) * 100,
			Burn:      (float64(burnShare.Int64()) / totalFloat) * 100,
		}
	}

	// Verify split sums to 100%
	sumShares := validatorShare.Add(treasuryShare).Add(burnShare)
	result.Discrepancy = totalFees.Sub(sumShares).Abs()

	result.Status = "PASS"

	// Check discrepancy within tolerance
	if result.Discrepancy.GT(fa.tolerance) {
		result.Status = "FAIL"
		result.Errors = append(result.Errors,
			fmt.Sprintf("Fee split discrepancy %s exceeds tolerance %s",
				result.Discrepancy, fa.tolerance))
	}

	// Check individual ratios
	if !fa.ratioWithinTolerance(result.ActualSplit.Validator, fa.expectedRatios.Validator) {
		result.Errors = append(result.Errors,
			fmt.Sprintf("Validator ratio %.2f%% != expected %.2f%%",
				result.ActualSplit.Validator, fa.expectedRatios.Validator))
	}

	if !fa.ratioWithinTolerance(result.ActualSplit.Treasury, fa.expectedRatios.Treasury) {
		result.Errors = append(result.Errors,
			fmt.Sprintf("Treasury ratio %.2f%% != expected %.2f%%",
				result.ActualSplit.Treasury, fa.expectedRatios.Treasury))
	}

	if !fa.ratioWithinTolerance(result.ActualSplit.Burn, fa.expectedRatios.Burn) {
		result.Errors = append(result.Errors,
			fmt.Sprintf("Burn ratio %.2f%% != expected %.2f%%",
				result.ActualSplit.Burn, fa.expectedRatios.Burn))
	}

	return result
}

func (fa *FeeAuditor) ratioWithinTolerance(actual, expected float64) bool {
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}
	return diff <= 0.1 // 0.1% tolerance
}

// Report generates fee audit report
func (fa *FeeAudit) Report() string {
	report := fmt.Sprintf(`
═══════════════════════════════════════════════════════════
                    Fee Split Audit
═══════════════════════════════════════════════════════════
Timestamp:     %s
Block Range:   %d - %d

Total Fees:    %s omniphi

Share          Amount              Expected    Actual
──────────────────────────────────────────────────────────
Validator      %15s    %6.2f%%    %6.2f%%
Treasury       %15s    %6.2f%%    %6.2f%%
Burn           %15s    %6.2f%%    %6.2f%%
──────────────────────────────────────────────────────────
TOTAL          %15s     100.00%%   %6.2f%%

Discrepancy:   %s omniphi
Status:        %s
`,
		fa.Timestamp.Format(time.RFC3339),
		fa.BlockRange[0],
		fa.BlockRange[1],
		fa.TotalFees.String(),
		fa.ValidatorShare.String(), fa.ExpectedSplit.Validator, fa.ActualSplit.Validator,
		fa.TreasuryShare.String(), fa.ExpectedSplit.Treasury, fa.ActualSplit.Treasury,
		fa.BurnShare.String(), fa.ExpectedSplit.Burn, fa.ActualSplit.Burn,
		fa.TotalFees.String(),
		fa.ActualSplit.Validator+fa.ActualSplit.Treasury+fa.ActualSplit.Burn,
		fa.Discrepancy.String(),
		fa.Status,
	)

	if len(fa.Errors) > 0 {
		report += "\nISSUES:\n"
		for i, err := range fa.Errors {
			report += fmt.Sprintf("  %d. %s\n", i+1, err)
		}
	}

	report += "═══════════════════════════════════════════════════════════\n"

	return report
}
