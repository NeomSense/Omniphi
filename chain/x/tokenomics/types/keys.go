package types

const (
	// ModuleName defines the module name
	ModuleName = "tokenomics"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_" + ModuleName

	// BondDenom defines the native token denomination
	BondDenom = "omniphi"

	// DisplayDenom defines the display denomination
	DisplayDenom = "OMNI"

	// Decimals defines the token decimals (6 decimals: 1 OMNI = 1,000,000 omniphi)
	Decimals = 6
)

// Store key prefixes
var (
	// ParamsKey is the key for module parameters
	ParamsKey = []byte{0x01}

	// Supply tracking keys
	KeyCurrentSupply = []byte{0x10}
	KeyTotalMinted   = []byte{0x11}
	KeyTotalBurned   = []byte{0x12}

	// Counter keys
	KeyNextBurnID     = []byte{0x20}
	KeyNextEmissionID = []byte{0x21}

	// Treasury keys
	KeyTreasuryAddress = []byte{0x30}
	KeyTreasuryInflows = []byte{0x31}

	// Fee burn tracking keys
	KeyTotalFeesBurned     = []byte{0x32}
	KeyTotalFeesToTreasury = []byte{0x33}

	// Treasury redirect keys
	KeyAccumulatedRedirectInflows = []byte{0x34}
	KeyLastRedirectHeight         = []byte{0x35}
	KeyTotalRedirected            = []byte{0x36}

	// Redirect target addresses (whitelisted accounts)
	KeyEcosystemGrantsAddress = []byte{0x37}
	KeyBuyAndBurnAddress      = []byte{0x38}
	KeyInsuranceFundAddress   = []byte{0x39}
	KeyResearchFundAddress    = []byte{0x3A}

	// Burn record prefix
	BurnRecordPrefix = []byte{0x40}

	// Burn by source prefix (for aggregation)
	BurnBySourcePrefix = []byte{0x41}

	// Burn by chain prefix (for aggregation)
	BurnByChainPrefix = []byte{0x42}

	// Chain state prefix
	ChainStatePrefix = []byte{0x50}

	// Emission record prefix
	EmissionRecordPrefix = []byte{0x60}

	// Transaction tracking for 7-day rolling average
	// Stores daily tx counts: key = DailyTxCountPrefix + day_index (0-6)
	DailyTxCountPrefix = []byte{0x70}

	// Current day index in the rolling window (0-6)
	KeyCurrentDayIndex = []byte{0x71}

	// Last recorded block height for day rotation
	KeyLastDayRotationHeight = []byte{0x72}

	// Current day's accumulated transaction count
	KeyCurrentDayTxCount = []byte{0x73}
)

// Event types
const (
	EventTypeMint               = "mint_inflation"
	EventTypeBurn               = "burn_tokens"
	EventTypeTreasuryRedirect   = "treasury_redirect"
	EventTypeTreasuryAllocation = "treasury_allocation"
	EventTypeEmissionAllocated  = "emission_allocated"

	AttributeKeyInflationRate    = "inflation_rate"
	AttributeKeyAnnualProvisions = "annual_provisions"
	AttributeKeyBlockProvision   = "block_provision"
	AttributeKeyYear             = "year"
	AttributeKeyBurnAmount       = "burn_amount"
	AttributeKeyBurnSource       = "burn_source"

	// Treasury redirect event attributes
	AttributeKeyTotalInflows        = "total_inflows"
	AttributeKeyRedirectAmount      = "redirect_amount"
	AttributeKeyRetainedAmount      = "retained_amount"
	AttributeKeyRedirectRatio       = "redirect_ratio"
	AttributeKeyAllocationTarget    = "allocation_target"
	AttributeKeyAllocationAmount    = "allocation_amount"
	AttributeKeyAllocationRatio     = "allocation_ratio"
	AttributeKeyRedirectBlockHeight = "redirect_block_height"

	// Emission record event attributes
	AttributeKeyEmissionID  = "emission_id"
	AttributeKeyTotalEmitted = "total_emitted"
	AttributeKeyToStaking    = "to_staking"
	AttributeKeyToPoc        = "to_poc"
	AttributeKeyToSequencer  = "to_sequencer"
	AttributeKeyToTreasury   = "to_treasury"
	AttributeKeyBlockHeight  = "block_height"
)

// GetBurnRecordKey returns the store key for a burn record
func GetBurnRecordKey(burnID uint64) []byte {
	b := make([]byte, 8)
	// Use big-endian encoding for lexicographic ordering
	b[0] = byte(burnID >> 56)
	b[1] = byte(burnID >> 48)
	b[2] = byte(burnID >> 40)
	b[3] = byte(burnID >> 32)
	b[4] = byte(burnID >> 24)
	b[5] = byte(burnID >> 16)
	b[6] = byte(burnID >> 8)
	b[7] = byte(burnID)

	return append(BurnRecordPrefix, b...)
}

// GetBurnBySourceKey returns the store key for source-specific burn tracking
func GetBurnBySourceKey(source BurnSource) []byte {
	return append(BurnBySourcePrefix, byte(source))
}

// GetBurnByChainKey returns the store key for chain-specific burn tracking
func GetBurnByChainKey(chainID string) []byte {
	return append(BurnByChainPrefix, []byte(chainID)...)
}

// GetChainStateKey returns the store key for chain state
func GetChainStateKey(chainID string) []byte {
	return append(ChainStatePrefix, []byte(chainID)...)
}

// GetEmissionRecordKey returns the store key for an emission record
func GetEmissionRecordKey(emissionID uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(emissionID >> 56)
	b[1] = byte(emissionID >> 48)
	b[2] = byte(emissionID >> 40)
	b[3] = byte(emissionID >> 32)
	b[4] = byte(emissionID >> 24)
	b[5] = byte(emissionID >> 16)
	b[6] = byte(emissionID >> 8)
	b[7] = byte(emissionID)

	return append(EmissionRecordPrefix, b...)
}

// GetDailyTxCountKey returns the store key for a specific day's transaction count
// dayIndex should be 0-6 representing the 7 days in the rolling window
func GetDailyTxCountKey(dayIndex uint8) []byte {
	return append(DailyTxCountPrefix, dayIndex)
}
