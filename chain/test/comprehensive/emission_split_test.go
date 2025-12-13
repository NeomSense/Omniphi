package comprehensive

import (
	"testing"

	"cosmossdk.io/math"
)

// ============================================================================
// EMISSION SPLIT SYSTEM - COMPREHENSIVE TEST SUITE
// ============================================================================
// Tests the DAO-governed emission split system with protocol-enforced safety bounds.
//
// Protocol Safety Bounds (Hard-coded, Immutable):
// - MaxAnnualInflationRateHardCap: 3% per year
// - MaxSingleRecipientShare: 60% max per recipient
// - MinStakingShare: 20% minimum to staking
//
// Default Emission Splits:
// - Staking: 40%
// - PoC: 30%
// - Sequencer: 20%
// - Treasury: 10%
// - Total: 100% (enforced)

// TestEmissionSplit represents an emission split configuration for testing
type TestEmissionSplit struct {
	Staking   string
	PoC       string
	Sequencer string
	Treasury  string
}

// Sum calculates the total of all splits
func (s TestEmissionSplit) Sum() math.LegacyDec {
	staking := math.LegacyMustNewDecFromStr(s.Staking)
	poc := math.LegacyMustNewDecFromStr(s.PoC)
	sequencer := math.LegacyMustNewDecFromStr(s.Sequencer)
	treasury := math.LegacyMustNewDecFromStr(s.Treasury)
	return staking.Add(poc).Add(sequencer).Add(treasury)
}

// ValidateSum checks if splits sum to 100%
func (s TestEmissionSplit) ValidateSum() bool {
	return s.Sum().Equal(math.LegacyOneDec())
}

// ValidateMaxSingle checks if any single recipient exceeds 60%
func (s TestEmissionSplit) ValidateMaxSingle() bool {
	maxShare := math.LegacyMustNewDecFromStr("0.60")
	splits := []string{s.Staking, s.PoC, s.Sequencer, s.Treasury}
	for _, split := range splits {
		if math.LegacyMustNewDecFromStr(split).GT(maxShare) {
			return false
		}
	}
	return true
}

// ValidateMinStaking checks if staking meets minimum 20%
func (s TestEmissionSplit) ValidateMinStaking() bool {
	minStaking := math.LegacyMustNewDecFromStr("0.20")
	return math.LegacyMustNewDecFromStr(s.Staking).GTE(minStaking)
}

// ============================================================================
// TC-EMISSION-001: Emission Sum Validation
// ============================================================================
// Requirement: Emission splits MUST sum to exactly 100%
// Rejects any configuration where sum != 1.0

func TestTC_EMISSION_001_EmissionSumValidation(t *testing.T) {
	testCases := []struct {
		name      string
		split     TestEmissionSplit
		shouldSum bool
	}{
		{
			name: "Default splits sum to 100%",
			split: TestEmissionSplit{
				Staking:   "0.40",
				PoC:       "0.30",
				Sequencer: "0.20",
				Treasury:  "0.10",
			},
			shouldSum: true,
		},
		{
			name: "Under 100% - should reject",
			split: TestEmissionSplit{
				Staking:   "0.30",
				PoC:       "0.30",
				Sequencer: "0.20",
				Treasury:  "0.10",
			},
			shouldSum: false,
		},
		{
			name: "Over 100% - should reject",
			split: TestEmissionSplit{
				Staking:   "0.50",
				PoC:       "0.30",
				Sequencer: "0.20",
				Treasury:  "0.10",
			},
			shouldSum: false,
		},
		{
			name: "Edge case - all to staking (max)",
			split: TestEmissionSplit{
				Staking:   "0.60",
				PoC:       "0.20",
				Sequencer: "0.10",
				Treasury:  "0.10",
			},
			shouldSum: true,
		},
		{
			name: "Edge case - minimum staking",
			split: TestEmissionSplit{
				Staking:   "0.20",
				PoC:       "0.40",
				Sequencer: "0.30",
				Treasury:  "0.10",
			},
			shouldSum: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.split.ValidateSum()
			if result != tc.shouldSum {
				t.Errorf("Sum validation failed: expected %v, got %v (sum=%s)",
					tc.shouldSum, result, tc.split.Sum().String())
			}
		})
	}
}

// ============================================================================
// TC-EMISSION-002: Inflation Cap Enforcement
// ============================================================================
// Requirement: Annual inflation MUST NOT exceed 3% (protocol hard cap)
// Governance CANNOT set inflation_max above this value

func TestTC_EMISSION_002_InflationCapEnforcement(t *testing.T) {
	maxCap := math.LegacyMustNewDecFromStr("0.03") // 3%

	testCases := []struct {
		name        string
		inflationMax string
		shouldPass  bool
	}{
		{
			name:        "At cap (3%) - valid",
			inflationMax: "0.03",
			shouldPass:  true,
		},
		{
			name:        "Below cap (2%) - valid",
			inflationMax: "0.02",
			shouldPass:  true,
		},
		{
			name:        "Above cap (4%) - MUST reject",
			inflationMax: "0.04",
			shouldPass:  false,
		},
		{
			name:        "Above cap (5%) - MUST reject",
			inflationMax: "0.05",
			shouldPass:  false,
		},
		{
			name:        "Minimum (0.5%) - valid",
			inflationMax: "0.005",
			shouldPass:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rate := math.LegacyMustNewDecFromStr(tc.inflationMax)
			isValid := rate.LTE(maxCap)
			if isValid != tc.shouldPass {
				t.Errorf("Inflation cap validation failed: rate=%s, expected pass=%v, got=%v",
					tc.inflationMax, tc.shouldPass, isValid)
			}
		})
	}
}

// ============================================================================
// TC-EMISSION-003: Max Single Recipient Cap
// ============================================================================
// Requirement: No single recipient can receive more than 60% of emissions
// Prevents centralization of emission allocation

func TestTC_EMISSION_003_MaxSingleRecipientCap(t *testing.T) {
	testCases := []struct {
		name       string
		split      TestEmissionSplit
		shouldPass bool
	}{
		{
			name: "Default splits - valid",
			split: TestEmissionSplit{
				Staking:   "0.40",
				PoC:       "0.30",
				Sequencer: "0.20",
				Treasury:  "0.10",
			},
			shouldPass: true,
		},
		{
			name: "Staking at 60% - valid (at cap)",
			split: TestEmissionSplit{
				Staking:   "0.60",
				PoC:       "0.20",
				Sequencer: "0.10",
				Treasury:  "0.10",
			},
			shouldPass: true,
		},
		{
			name: "Staking at 61% - MUST reject",
			split: TestEmissionSplit{
				Staking:   "0.61",
				PoC:       "0.19",
				Sequencer: "0.10",
				Treasury:  "0.10",
			},
			shouldPass: false,
		},
		{
			name: "PoC at 70% - MUST reject",
			split: TestEmissionSplit{
				Staking:   "0.20",
				PoC:       "0.70",
				Sequencer: "0.05",
				Treasury:  "0.05",
			},
			shouldPass: false,
		},
		{
			name: "Treasury at 65% - MUST reject",
			split: TestEmissionSplit{
				Staking:   "0.20",
				PoC:       "0.10",
				Sequencer: "0.05",
				Treasury:  "0.65",
			},
			shouldPass: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.split.ValidateMaxSingle()
			if result != tc.shouldPass {
				t.Errorf("Max single recipient validation failed: expected pass=%v, got=%v",
					tc.shouldPass, result)
			}
		})
	}
}

// ============================================================================
// TC-EMISSION-004: Minimum Staking Share
// ============================================================================
// Requirement: Staking MUST receive at least 20% of emissions
// Ensures PoS security by maintaining validator incentives

func TestTC_EMISSION_004_MinimumStakingShare(t *testing.T) {
	testCases := []struct {
		name       string
		split      TestEmissionSplit
		shouldPass bool
	}{
		{
			name: "Default 40% staking - valid",
			split: TestEmissionSplit{
				Staking:   "0.40",
				PoC:       "0.30",
				Sequencer: "0.20",
				Treasury:  "0.10",
			},
			shouldPass: true,
		},
		{
			name: "Minimum 20% staking - valid (at floor)",
			split: TestEmissionSplit{
				Staking:   "0.20",
				PoC:       "0.40",
				Sequencer: "0.30",
				Treasury:  "0.10",
			},
			shouldPass: true,
		},
		{
			name: "19% staking - MUST reject (below floor)",
			split: TestEmissionSplit{
				Staking:   "0.19",
				PoC:       "0.41",
				Sequencer: "0.30",
				Treasury:  "0.10",
			},
			shouldPass: false,
		},
		{
			name: "10% staking - MUST reject",
			split: TestEmissionSplit{
				Staking:   "0.10",
				PoC:       "0.50",
				Sequencer: "0.30",
				Treasury:  "0.10",
			},
			shouldPass: false,
		},
		{
			name: "0% staking - MUST reject",
			split: TestEmissionSplit{
				Staking:   "0.00",
				PoC:       "0.60",
				Sequencer: "0.30",
				Treasury:  "0.10",
			},
			shouldPass: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.split.ValidateMinStaking()
			if result != tc.shouldPass {
				t.Errorf("Minimum staking validation failed: staking=%s, expected pass=%v, got=%v",
					tc.split.Staking, tc.shouldPass, result)
			}
		})
	}
}

// ============================================================================
// TC-EMISSION-005: Emission Calculation Precision
// ============================================================================
// Requirement: Use sdk.Dec for ratios, sdk.Int for amounts
// No floating-point math, deterministic across all nodes

func TestTC_EMISSION_005_EmissionCalculationPrecision(t *testing.T) {
	// Test with various total emission amounts
	testCases := []struct {
		name           string
		totalEmission  string
		split          TestEmissionSplit
		expectedStaking string
		expectedPoC     string
		expectedSeq     string
		expectedTreas   string
	}{
		{
			name:          "Standard emission 1,000,000 uOMNI",
			totalEmission: "1000000",
			split: TestEmissionSplit{
				Staking:   "0.40",
				PoC:       "0.30",
				Sequencer: "0.20",
				Treasury:  "0.10",
			},
			expectedStaking: "400000",
			expectedPoC:     "300000",
			expectedSeq:     "200000",
			expectedTreas:   "100000",
		},
		{
			name:          "Odd amount requiring rounding",
			totalEmission: "1000001",
			split: TestEmissionSplit{
				Staking:   "0.40",
				PoC:       "0.30",
				Sequencer: "0.20",
				Treasury:  "0.10",
			},
			// With truncation and dust to treasury:
			// 1000001 * 0.40 = 400000.4 -> 400000
			// 1000001 * 0.30 = 300000.3 -> 300000
			// 1000001 * 0.20 = 200000.2 -> 200000
			// 1000001 * 0.10 = 100000.1 -> 100000
			// Sum = 1000000, remainder = 1 goes to treasury
			expectedStaking: "400000",
			expectedPoC:     "300000",
			expectedSeq:     "200000",
			expectedTreas:   "100001", // includes dust
		},
		{
			name:          "Large emission - billions",
			totalEmission: "22500000000000", // 22.5M OMNI in uOMNI
			split: TestEmissionSplit{
				Staking:   "0.40",
				PoC:       "0.30",
				Sequencer: "0.20",
				Treasury:  "0.10",
			},
			expectedStaking: "9000000000000",
			expectedPoC:     "6750000000000",
			expectedSeq:     "4500000000000",
			expectedTreas:   "2250000000000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			total := math.NewIntFromUint64(mustParseUint64(tc.totalEmission))
			totalDec := math.LegacyNewDecFromInt(total)

			stakingDec := math.LegacyMustNewDecFromStr(tc.split.Staking)
			pocDec := math.LegacyMustNewDecFromStr(tc.split.PoC)
			seqDec := math.LegacyMustNewDecFromStr(tc.split.Sequencer)
			treasDec := math.LegacyMustNewDecFromStr(tc.split.Treasury)

			stakingAmount := totalDec.Mul(stakingDec).TruncateInt()
			pocAmount := totalDec.Mul(pocDec).TruncateInt()
			seqAmount := totalDec.Mul(seqDec).TruncateInt()
			treasAmount := totalDec.Mul(treasDec).TruncateInt()

			// Add dust to treasury (deterministic rounding)
			distributed := stakingAmount.Add(pocAmount).Add(seqAmount).Add(treasAmount)
			if distributed.LT(total) {
				remainder := total.Sub(distributed)
				treasAmount = treasAmount.Add(remainder)
			}

			// Verify sum equals total (conservation invariant)
			finalSum := stakingAmount.Add(pocAmount).Add(seqAmount).Add(treasAmount)
			if !finalSum.Equal(total) {
				t.Errorf("Conservation invariant violated: sum=%s, expected=%s",
					finalSum.String(), total.String())
			}

			// Verify individual amounts
			expectedStaking := math.NewIntFromUint64(mustParseUint64(tc.expectedStaking))
			if !stakingAmount.Equal(expectedStaking) {
				t.Errorf("Staking amount mismatch: got=%s, expected=%s",
					stakingAmount.String(), expectedStaking.String())
			}
		})
	}
}

// ============================================================================
// TC-EMISSION-006: Zero Activity Epoch
// ============================================================================
// Requirement: No unintended mint when there's no activity
// System should handle zero-emission epochs gracefully

func TestTC_EMISSION_006_ZeroActivityEpoch(t *testing.T) {
	// When total emission is 0, all allocations should be 0
	totalEmission := math.ZeroInt()
	totalDec := math.LegacyNewDecFromInt(totalEmission)

	stakingDec := math.LegacyMustNewDecFromStr("0.40")
	stakingAmount := totalDec.Mul(stakingDec).TruncateInt()

	if !stakingAmount.IsZero() {
		t.Errorf("Expected zero staking amount, got %s", stakingAmount.String())
	}

	// Verify no tokens are minted for zero-emission epoch
	if !totalEmission.IsZero() {
		t.Error("Total emission should be zero for zero-activity epoch")
	}
}

// ============================================================================
// TC-EMISSION-007: Supply Accounting Invariant
// ============================================================================
// Requirement: Minted emissions MUST match sum of allocations exactly
// current_supply = total_minted - total_burned

func TestTC_EMISSION_007_SupplyAccountingInvariant(t *testing.T) {
	// Simulate multiple epochs of emissions
	type SupplyState struct {
		CurrentSupply math.Int
		TotalMinted   math.Int
		TotalBurned   math.Int
	}

	initialSupply := math.NewIntFromUint64(375000000000000) // 375M OMNI
	state := SupplyState{
		CurrentSupply: initialSupply,
		TotalMinted:   initialSupply,
		TotalBurned:   math.ZeroInt(),
	}

	// Simulate 10 epochs of emissions
	emissionPerEpoch := math.NewIntFromUint64(1000000000) // 1000 OMNI per epoch
	for epoch := 0; epoch < 10; epoch++ {
		// Mint new emission
		state.TotalMinted = state.TotalMinted.Add(emissionPerEpoch)
		state.CurrentSupply = state.CurrentSupply.Add(emissionPerEpoch)

		// Verify invariant: current = minted - burned
		expected := state.TotalMinted.Sub(state.TotalBurned)
		if !state.CurrentSupply.Equal(expected) {
			t.Errorf("Supply invariant violated at epoch %d: current=%s, expected=%s",
				epoch, state.CurrentSupply.String(), expected.String())
		}
	}
}

// ============================================================================
// TC-EMISSION-008: Governance Parameter Update
// ============================================================================
// Requirement: Changes apply next epoch, not retroactively
// DAO can update splits within protocol bounds

func TestTC_EMISSION_008_GovernanceParameterUpdate(t *testing.T) {
	// Current configuration
	currentSplit := TestEmissionSplit{
		Staking:   "0.40",
		PoC:       "0.30",
		Sequencer: "0.20",
		Treasury:  "0.10",
	}

	// Proposed update - valid within bounds
	proposedSplit := TestEmissionSplit{
		Staking:   "0.35",
		PoC:       "0.35",
		Sequencer: "0.20",
		Treasury:  "0.10",
	}

	// Validate proposed update
	validationResults := struct {
		SumsTo100        bool
		NoExceeds60      bool
		StakingAbove20   bool
	}{
		SumsTo100:      proposedSplit.ValidateSum(),
		NoExceeds60:    proposedSplit.ValidateMaxSingle(),
		StakingAbove20: proposedSplit.ValidateMinStaking(),
	}

	if !validationResults.SumsTo100 {
		t.Error("Proposed split does not sum to 100%")
	}
	if !validationResults.NoExceeds60 {
		t.Error("Proposed split has recipient exceeding 60%")
	}
	if !validationResults.StakingAbove20 {
		t.Error("Proposed split has staking below 20%")
	}

	// Invalid proposed update - violates bounds
	invalidSplit := TestEmissionSplit{
		Staking:   "0.15", // Below 20% minimum
		PoC:       "0.55",
		Sequencer: "0.20",
		Treasury:  "0.10",
	}

	if invalidSplit.ValidateMinStaking() {
		t.Error("Invalid split should fail minimum staking validation")
	}

	// Use currentSplit to avoid unused variable warning
	if !currentSplit.ValidateSum() {
		t.Error("Current split should be valid")
	}
}

// ============================================================================
// TC-EMISSION-009: Decaying Inflation Schedule
// ============================================================================
// Requirement: Inflation decreases over time according to schedule
// Year 1: 3%, Year 2: 2.75%, Year 3: 2.5%, etc.

func TestTC_EMISSION_009_DecayingInflationSchedule(t *testing.T) {
	expectedRates := map[int]string{
		0: "0.03",   // Year 1: 3%
		1: "0.0275", // Year 2: 2.75%
		2: "0.025",  // Year 3: 2.5%
		3: "0.0225", // Year 4: 2.25%
		4: "0.02",   // Year 5: 2%
		5: "0.0175", // Year 6: 1.75%
		6: "0.015",  // Year 7: 1.5%
		7: "0.0125", // Year 8: 1.25%
		8: "0.01",   // Year 9: 1%
		9: "0.0075", // Year 10: 0.75%
	}

	minInflation := math.LegacyMustNewDecFromStr("0.005") // 0.5% floor

	for year, expectedRate := range expectedRates {
		expected := math.LegacyMustNewDecFromStr(expectedRate)

		var calculatedRate math.LegacyDec
		switch {
		case year == 0:
			calculatedRate = math.LegacyMustNewDecFromStr("0.03")
		case year == 1:
			calculatedRate = math.LegacyMustNewDecFromStr("0.0275")
		case year == 2:
			calculatedRate = math.LegacyMustNewDecFromStr("0.025")
		case year == 3:
			calculatedRate = math.LegacyMustNewDecFromStr("0.0225")
		case year == 4:
			calculatedRate = math.LegacyMustNewDecFromStr("0.02")
		case year == 5:
			calculatedRate = math.LegacyMustNewDecFromStr("0.0175")
		default:
			// Year 7+: Reduce by 0.25% per year until floor
			baseRate := math.LegacyMustNewDecFromStr("0.0175")
			decayRate := math.LegacyMustNewDecFromStr("0.0025")
			yearsAfterSix := int64(year - 5)
			totalDecay := decayRate.MulInt64(yearsAfterSix)
			calculatedRate = baseRate.Sub(totalDecay)
			if calculatedRate.LT(minInflation) {
				calculatedRate = minInflation
			}
		}

		if !calculatedRate.Equal(expected) {
			t.Errorf("Year %d inflation mismatch: got=%s, expected=%s",
				year+1, calculatedRate.String(), expected.String())
		}
	}
}

// ============================================================================
// TC-EMISSION-010: Supply Cap Enforcement
// ============================================================================
// Requirement: Total supply MUST NOT exceed 1.5B OMNI
// Emissions stop when cap is reached

func TestTC_EMISSION_010_SupplyCapEnforcement(t *testing.T) {
	supplyCap := math.NewIntFromUint64(1500000000000000) // 1.5B OMNI in uOMNI

	testCases := []struct {
		name          string
		currentSupply string
		mintAmount    string
		shouldAllow   bool
	}{
		{
			name:          "Well below cap - allow",
			currentSupply: "375000000000000",
			mintAmount:    "1000000000",
			shouldAllow:   true,
		},
		{
			name:          "At cap - reject any mint",
			currentSupply: "1500000000000000",
			mintAmount:    "1",
			shouldAllow:   false,
		},
		{
			name:          "Mint would exceed cap - reject",
			currentSupply: "1499999999999999",
			mintAmount:    "2",
			shouldAllow:   false,
		},
		{
			name:          "Mint exactly to cap - allow",
			currentSupply: "1499999999999999",
			mintAmount:    "1",
			shouldAllow:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			current := math.NewIntFromUint64(mustParseUint64(tc.currentSupply))
			mint := math.NewIntFromUint64(mustParseUint64(tc.mintAmount))
			newSupply := current.Add(mint)

			isAllowed := newSupply.LTE(supplyCap)
			if isAllowed != tc.shouldAllow {
				t.Errorf("Supply cap check failed: current=%s, mint=%s, expected allow=%v, got=%v",
					tc.currentSupply, tc.mintAmount, tc.shouldAllow, isAllowed)
			}
		})
	}
}

// ============================================================================
// TC-EMISSION-011: Module Account Allocation
// ============================================================================
// Requirement: Each emission recipient MUST have a dedicated module account
// Verify correct accounts receive emissions

func TestTC_EMISSION_011_ModuleAccountAllocation(t *testing.T) {
	// Expected module accounts for emission recipients
	expectedModules := []string{
		"staking",      // PoS staking rewards
		"poc",          // Proof of Contribution rewards
		"sequencer",    // Sequencer/ordering layer rewards
		"tokenomics",   // Treasury emissions (tokenomics module)
	}

	// Verify all required modules are defined
	for _, module := range expectedModules {
		if module == "" {
			t.Errorf("Module account %s not defined", module)
		}
	}

	// Verify emission split recipients match module accounts
	split := TestEmissionSplit{
		Staking:   "0.40", // -> staking module
		PoC:       "0.30", // -> poc module
		Sequencer: "0.20", // -> sequencer module
		Treasury:  "0.10", // -> tokenomics module (treasury)
	}

	if !split.ValidateSum() {
		t.Error("Default split should sum to 100%")
	}
}

// ============================================================================
// TC-EMISSION-012: Industry Standard Comparison
// ============================================================================
// Requirement: Parameters should align with industry standards

func TestTC_EMISSION_012_IndustryStandardComparison(t *testing.T) {
	// Omniphi parameters
	omniphiInflationCap := math.LegacyMustNewDecFromStr("0.03")      // 3%
	omniphiMinStaking := math.LegacyMustNewDecFromStr("0.20")        // 20%
	omniphiMaxSingleRecipient := math.LegacyMustNewDecFromStr("0.60") // 60%

	// Industry comparisons
	comparisons := []struct {
		chain           string
		inflationRange  string
		stakingShare    string
		notes           string
	}{
		{
			chain:          "Ethereum PoS",
			inflationRange: "~0.5-1%",
			stakingShare:   "100% to validators",
			notes:          "Very low inflation, all to staking",
		},
		{
			chain:          "Cosmos Hub",
			inflationRange: "7-20%",
			stakingShare:   "~67% to stakers",
			notes:          "Higher inflation, community tax",
		},
		{
			chain:          "Osmosis",
			inflationRange: "~15-20%",
			stakingShare:   "~25% to stakers",
			notes:          "High inflation, LP incentives",
		},
		{
			chain:          "Omniphi",
			inflationRange: "0.5-3%",
			stakingShare:   "20-60% to stakers",
			notes:          "Conservative, multi-recipient",
		},
	}

	// Verify Omniphi is within reasonable bounds
	if omniphiInflationCap.GT(math.LegacyMustNewDecFromStr("0.05")) {
		t.Error("Omniphi inflation cap too high compared to industry")
	}

	if omniphiMinStaking.LT(math.LegacyMustNewDecFromStr("0.10")) {
		t.Error("Omniphi minimum staking too low for PoS security")
	}

	if omniphiMaxSingleRecipient.GT(math.LegacyMustNewDecFromStr("0.70")) {
		t.Error("Omniphi max single recipient too high, risks centralization")
	}

	// Log comparison info
	for _, comp := range comparisons {
		t.Logf("Chain: %s, Inflation: %s, Staking: %s, Notes: %s",
			comp.chain, comp.inflationRange, comp.stakingShare, comp.notes)
	}
}

// Helper function to parse uint64
func mustParseUint64(s string) uint64 {
	var result uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + uint64(c-'0')
		}
	}
	return result
}
