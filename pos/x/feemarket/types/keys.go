package types

const (
	// ModuleName defines the module name
	ModuleName = "feemarket"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_feemarket"

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)

// KVStore keys
var (
	// ParamsKey is the key for module parameters
	ParamsKey = []byte{0x01}

	// CurrentBaseFeeKey is the key for current base fee
	CurrentBaseFeeKey = []byte{0x02}

	// PreviousBlockUtilizationKey is the key for previous block utilization
	PreviousBlockUtilizationKey = []byte{0x03}

	// TreasuryAddressKey is the key for treasury address
	TreasuryAddressKey = []byte{0x04}

	// CumulativeBurnedKey is the key for cumulative burned amount
	CumulativeBurnedKey = []byte{0x05}

	// CumulativeToTreasuryKey is the key for cumulative treasury amount
	CumulativeToTreasuryKey = []byte{0x06}

	// CumulativeToValidatorsKey is the key for cumulative validator amount
	CumulativeToValidatorsKey = []byte{0x07}
)
