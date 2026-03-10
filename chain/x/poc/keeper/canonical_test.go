package keeper_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/poc/keeper"
	"pos/x/poc/types"
)

// ============================================================================
// Canonicalization Reference Implementation Tests
// ============================================================================

func TestCanonicalizeText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "whitespace normalization",
			input:    "  hello   world  \r\n",
			expected: "hello world\n",
		},
		{
			name:     "case folding",
			input:    "Hello WORLD\n",
			expected: "hello world\n",
		},
		{
			name:     "markdown bold stripping",
			input:    "**bold** text\n",
			expected: "bold text\n",
		},
		{
			name:     "markdown italic stripping",
			input:    "_italic_ text\n",
			expected: "italic text\n",
		},
		{
			name:     "empty lines removed",
			input:    "line1\n\n\nline2\n",
			expected: "line1\nline2\n",
		},
		{
			name:     "CRLF normalized to LF",
			input:    "line1\r\nline2\r\n",
			expected: "line1\nline2\n",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := keeper.CanonicalizeText([]byte(tc.input))
			require.Equal(t, tc.expected, string(result))
		})
	}
}

func TestCanonicalizeCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single-line comment stripped",
			input:    "x = 1 // comment\n",
			expected: "x = 1\n",
		},
		{
			name:     "multi-line comment stripped",
			input:    "x /* block */ = 1\n",
			expected: "x = 1\n",
		},
		{
			name:     "import sorting",
			input:    "import b\nimport a\n",
			expected: "import a\nimport b\n",
		},
		{
			name:     "whitespace collapse",
			input:    "x  =   1\n",
			expected: "x = 1\n",
		},
		{
			name:     "empty lines removed",
			input:    "x = 1\n\ny = 2\n",
			expected: "x = 1\ny = 2\n",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := keeper.CanonicalizeCode([]byte(tc.input))
			require.Equal(t, tc.expected, string(result))
		})
	}
}

func TestCanonicalizeDataset(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "row sorting",
			input:    "{\"b\":2}\n{\"a\":1}\n",
			expected: "{\"a\":1}\n{\"b\":2}\n",
		},
		{
			name:     "whitespace trimmed",
			input:    "  row1  \n  row2  \n",
			expected: "row1\nrow2\n",
		},
		{
			name:     "empty lines removed",
			input:    "row1\n\nrow2\n",
			expected: "row1\nrow2\n",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := keeper.CanonicalizeDataset([]byte(tc.input))
			require.Equal(t, tc.expected, string(result))
		})
	}
}

func TestComputeCanonicalHash(t *testing.T) {
	// Deterministic test vector: canonicalize "hello world\n" as text, then SHA-256
	canonical := keeper.CanonicalizeText([]byte("Hello World\n"))
	expectedHash := sha256.Sum256(canonical)

	hash, err := keeper.ComputeCanonicalHash("text", []byte("Hello World\n"))
	require.NoError(t, err)
	require.Equal(t, expectedHash[:], hash)
	require.Len(t, hash, 32)

	// Unsupported category
	_, err = keeper.ComputeCanonicalHash("unknown", []byte("data"))
	require.Error(t, err)
}

// ============================================================================
// Canonical Registry Keeper Tests
// ============================================================================

func makeTestHash(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func TestRegisterCanonicalClaim_NewHash(t *testing.T) {
	f := SetupKeeperTest(t)

	hash := makeTestHash("unique-content-1")
	claim := types.ClaimRecord{
		ClaimID:        1,
		Submitter:      sdk.AccAddress("contrib1____________").String(),
		Category:       "code",
		StoragePointer: "ipfs://Qm123",
		BlockHeight:    100,
		SpecVersion:    1,
	}

	isDuplicate, existingClaims, err := f.keeper.RegisterCanonicalClaim(f.ctx, hash, claim)
	require.NoError(t, err)
	require.False(t, isDuplicate)
	require.Nil(t, existingClaims)

	// Verify stored
	registry, found := f.keeper.GetCanonicalRegistry(f.ctx, hash)
	require.True(t, found)
	require.Len(t, registry.Claims, 1)
	require.Equal(t, uint64(1), registry.Claims[0].ClaimID)
}

func TestRegisterCanonicalClaim_DuplicateHash(t *testing.T) {
	f := SetupKeeperTest(t)

	hash := makeTestHash("duplicate-content")

	// First claim
	claim1 := types.ClaimRecord{
		ClaimID:   1,
		Submitter: sdk.AccAddress("contrib1____________").String(),
		Category:  "code",
	}
	isDup, _, err := f.keeper.RegisterCanonicalClaim(f.ctx, hash, claim1)
	require.NoError(t, err)
	require.False(t, isDup)

	// Second claim with same hash
	claim2 := types.ClaimRecord{
		ClaimID:   2,
		Submitter: sdk.AccAddress("contrib2____________").String(),
		Category:  "code",
	}
	isDup, existing, err := f.keeper.RegisterCanonicalClaim(f.ctx, hash, claim2)
	require.NoError(t, err)
	require.True(t, isDup)
	require.Len(t, existing, 1)
	require.Equal(t, uint64(1), existing[0].ClaimID)

	// Registry should have both claims
	registry, found := f.keeper.GetCanonicalRegistry(f.ctx, hash)
	require.True(t, found)
	require.Len(t, registry.Claims, 2)
}

func TestHashCollision_MultipleClaimIDs(t *testing.T) {
	f := SetupKeeperTest(t)

	hash := makeTestHash("colliding-content")

	// Register 3 claims for the same hash
	for i := uint64(1); i <= 3; i++ {
		claim := types.ClaimRecord{
			ClaimID:   i,
			Submitter: sdk.AccAddress("contrib_____________").String(),
			Category:  "code",
		}
		_, _, err := f.keeper.RegisterCanonicalClaim(f.ctx, hash, claim)
		require.NoError(t, err)
	}

	registry, found := f.keeper.GetCanonicalRegistry(f.ctx, hash)
	require.True(t, found)
	require.Len(t, registry.Claims, 3)
	require.Equal(t, uint64(1), registry.Claims[0].ClaimID)
	require.Equal(t, uint64(2), registry.Claims[1].ClaimID)
	require.Equal(t, uint64(3), registry.Claims[2].ClaimID)
}

func TestCheckCanonicalHash(t *testing.T) {
	f := SetupKeeperTest(t)

	hash := makeTestHash("check-content")

	// Not found initially
	exists, claim, err := f.keeper.CheckCanonicalHash(f.ctx, hash)
	require.NoError(t, err)
	require.False(t, exists)
	require.Nil(t, claim)

	// Register a claim
	c := types.ClaimRecord{
		ClaimID:   42,
		Submitter: sdk.AccAddress("contrib1____________").String(),
		Category:  "docs",
	}
	_, _, err = f.keeper.RegisterCanonicalClaim(f.ctx, hash, c)
	require.NoError(t, err)

	// Now found
	exists, claim, err = f.keeper.CheckCanonicalHash(f.ctx, hash)
	require.NoError(t, err)
	require.True(t, exists)
	require.NotNil(t, claim)
	require.Equal(t, uint64(42), claim.ClaimID)
}

// ============================================================================
// Bond Escrow Tests
// ============================================================================

// SetupBondKeeperTest creates a test fixture with a funded bank mock
// that returns sufficient balance for bond operations.
func SetupBondKeeperTest(t *testing.T) *KeeperTestFixture {
	f := SetupKeeperTest(t)
	// The default mockBankKeeper returns zero balance.
	// Bond tests verify storage/escrow logic, not bank balances,
	// so we test with zero bonds (which are no-ops) and test the
	// insufficient balance error path explicitly.
	return f
}

func TestDuplicateBond_Escrow(t *testing.T) {
	f := SetupKeeperTest(t)

	contributor := sdk.AccAddress("contrib1____________")
	bond := sdk.NewCoin("omniphi", math.NewInt(10000))

	// Mock bank keeper returns 0 balance, so bond collection should fail with insufficient balance
	err := f.keeper.CollectDuplicateBond(f.ctx, contributor, bond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient balance")
}

func TestDuplicateBond_EscrowZeroBond(t *testing.T) {
	f := SetupKeeperTest(t)

	contributor := sdk.AccAddress("contrib1____________")
	bond := sdk.NewCoin("omniphi", math.NewInt(0))

	// Zero bond should be a no-op
	err := f.keeper.CollectDuplicateBond(f.ctx, contributor, bond)
	require.NoError(t, err)
}

func TestDuplicateBond_SlashOnDuplicate(t *testing.T) {
	f := SetupKeeperTest(t)

	contributor := sdk.AccAddress("contrib1____________")
	bond := sdk.NewCoin("omniphi", math.NewInt(10000))

	// SlashDuplicateBondDirect calls BurnCoins directly (mock allows it)
	err := f.keeper.SlashDuplicateBondDirect(f.ctx, contributor, bond)
	require.NoError(t, err)
}

func TestDuplicateBond_RefundOnOriginal(t *testing.T) {
	f := SetupKeeperTest(t)

	contributor := sdk.AccAddress("contrib1____________")
	bond := sdk.NewCoin("omniphi", math.NewInt(10000))

	// Fund module account so refund transfer succeeds
	f.bankKeeper.setBalance(
		sdk.AccAddress("module_address______").String(),
		"omniphi",
		math.NewInt(100000),
	)

	err := f.keeper.RefundDuplicateBondDirect(f.ctx, contributor, bond)
	require.NoError(t, err)
}

// ============================================================================
// Rate Limiting Tests
// ============================================================================

func TestDuplicateRateLimit(t *testing.T) {
	f := SetupKeeperTest(t)

	addr := sdk.AccAddress("contrib1____________").String()
	epoch := uint64(1)

	// Initially zero
	count, err := f.keeper.GetDuplicateCount(f.ctx, addr, epoch)
	require.NoError(t, err)
	require.Equal(t, uint32(0), count)

	// Increment 3 times
	for i := 0; i < 3; i++ {
		err = f.keeper.IncrementDuplicateCount(f.ctx, addr, epoch)
		require.NoError(t, err)
	}

	count, err = f.keeper.GetDuplicateCount(f.ctx, addr, epoch)
	require.NoError(t, err)
	require.Equal(t, uint32(3), count)

	// Different epoch has separate count
	count2, err := f.keeper.GetDuplicateCount(f.ctx, addr, epoch+1)
	require.NoError(t, err)
	require.Equal(t, uint32(0), count2)
}

// ============================================================================
// Bond Escalation Tests
// ============================================================================

func TestBondEscalation(t *testing.T) {
	f := SetupKeeperTest(t)

	// Enable canonical hash check and set params
	params := types.DefaultParams()
	params.EnableCanonicalHashCheck = true
	params.DuplicateBond = sdk.NewCoin("omniphi", math.NewInt(10000))
	params.DuplicateBondEscalationBps = 5000 // 50% escalation
	params.MaxDuplicatesPerEpoch = 10
	err := f.keeper.SetParams(f.ctx, params)
	require.NoError(t, err)

	contributor := sdk.AccAddress("contrib1____________")

	// First bond: base amount (no prior duplicates)
	bond, err := f.keeper.CalculateEscalatedBond(f.ctx, contributor)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(10000), bond.Amount)

	// Simulate 2 duplicates in current epoch
	epoch := f.keeper.GetCurrentEpoch(f.ctx)
	_ = f.keeper.IncrementDuplicateCount(f.ctx, contributor.String(), epoch)
	_ = f.keeper.IncrementDuplicateCount(f.ctx, contributor.String(), epoch)

	// Escalated bond: 10000 * (10000 + 2*5000) / 10000 = 10000 * 20000 / 10000 = 20000
	bond, err = f.keeper.CalculateEscalatedBond(f.ctx, contributor)
	require.NoError(t, err)
	require.Equal(t, math.NewInt(20000), bond.Amount)
}

// ============================================================================
// Feature Toggle Tests
// ============================================================================

func TestCanonicalHashCheck_DisabledByDefault(t *testing.T) {
	f := SetupKeeperTest(t)

	params := f.keeper.GetParams(f.ctx)
	require.False(t, params.EnableCanonicalHashCheck)
}

// ============================================================================
// Spec Version Validation Tests
// ============================================================================

func TestSpecVersionValidation(t *testing.T) {
	// ValidateBasic should reject unsupported spec versions
	msg := &types.MsgSubmitContribution{
		Contributor:          sdk.AccAddress("contrib1____________").String(),
		Ctype:                "code",
		Uri:                  "ipfs://Qm123",
		Hash:                 makeTestHash("content"),
		CanonicalHash:        makeTestHash("canonical"),
		CanonicalSpecVersion: 99, // unsupported
	}

	err := msg.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "spec version 99 > current")
}

func TestSpecVersionValidation_ZeroWithHash(t *testing.T) {
	msg := &types.MsgSubmitContribution{
		Contributor:          sdk.AccAddress("contrib1____________").String(),
		Ctype:                "code",
		Uri:                  "ipfs://Qm123",
		Hash:                 makeTestHash("content"),
		CanonicalHash:        makeTestHash("canonical"),
		CanonicalSpecVersion: 0, // missing when hash provided
	}

	err := msg.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "spec version required")
}

func TestSpecVersionValidation_ValidMessage(t *testing.T) {
	msg := &types.MsgSubmitContribution{
		Contributor:          sdk.AccAddress("contrib1____________").String(),
		Ctype:                "code",
		Uri:                  "ipfs://Qm123",
		Hash:                 makeTestHash("content"),
		CanonicalHash:        makeTestHash("canonical"),
		CanonicalSpecVersion: 1,
	}

	err := msg.ValidateBasic()
	require.NoError(t, err)
}

func TestSpecVersionValidation_NoCanonicalHash(t *testing.T) {
	// When no canonical hash is provided, spec version is not checked
	msg := &types.MsgSubmitContribution{
		Contributor: sdk.AccAddress("contrib1____________").String(),
		Ctype:       "code",
		Uri:         "ipfs://Qm123",
		Hash:        makeTestHash("content"),
	}

	err := msg.ValidateBasic()
	require.NoError(t, err)
}

// ============================================================================
// Duplicate Record Tests
// ============================================================================

func TestDuplicateRecord_StoreAndRetrieve(t *testing.T) {
	f := SetupKeeperTest(t)

	hash := makeTestHash("dup-content")
	record := types.DuplicateRecord{
		ContributionID:  5,
		CanonicalHash:   hash,
		OriginalClaimID: 1,
		OriginalSubmitter: sdk.AccAddress("orig________________").String(),
	}

	err := f.keeper.SetDuplicateRecord(f.ctx, record)
	require.NoError(t, err)

	retrieved, found := f.keeper.GetDuplicateRecord(f.ctx, 5)
	require.True(t, found)
	require.Equal(t, uint64(5), retrieved.ContributionID)
	require.Equal(t, uint64(1), retrieved.OriginalClaimID)
	require.Equal(t, hex.EncodeToString(hash), hex.EncodeToString(retrieved.CanonicalHash))

	// Non-existent record
	_, notFound := f.keeper.GetDuplicateRecord(f.ctx, 999)
	require.False(t, notFound)
}

// ============================================================================
// Canonical Hash Validation Tests
// ============================================================================

func TestValidateCanonicalHash(t *testing.T) {
	// Valid hash
	err := types.ValidateCanonicalHash(makeTestHash("valid"))
	require.NoError(t, err)

	// Wrong size
	err = types.ValidateCanonicalHash([]byte{1, 2, 3})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be 32 bytes")

	// All zeros
	err = types.ValidateCanonicalHash(make([]byte, 32))
	require.Error(t, err)
	require.Contains(t, err.Error(), "all zeros")

	// All ones
	allOnes := make([]byte, 32)
	for i := range allOnes {
		allOnes[i] = 0xFF
	}
	err = types.ValidateCanonicalHash(allOnes)
	require.Error(t, err)
	require.Contains(t, err.Error(), "all ones")
}
