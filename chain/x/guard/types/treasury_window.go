package types

// TreasuryOutflowWindow tracks cumulative treasury outflow in a rolling window.
// Stored under TreasuryOutflowWindowKey as a single entry.
type TreasuryOutflowWindow struct {
	WindowStartUnix  int64  `json:"window_start_unix"`
	TotalOutflowBps  uint64 `json:"total_outflow_bps"`
	LastUpdateHeight int64  `json:"last_update_height"`
}

// TreasuryOutflowWindowSeconds is the rolling window size for treasury caps.
// Default: 24 hours.
const TreasuryOutflowWindowSeconds int64 = 86400
