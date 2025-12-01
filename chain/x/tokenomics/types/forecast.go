package types

import (
	"cosmossdk.io/math"
)

// InflationForecast represents projected inflation for a future year
type InflationForecast struct {
	Year          int64
	InflationRate math.LegacyDec
	AnnualMint    math.Int
	Supply        math.Int
}
