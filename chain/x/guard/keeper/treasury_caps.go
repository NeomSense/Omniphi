package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/guard/types"
)

// getTreasuryOutflowWindow retrieves the rolling treasury outflow window.
func (k Keeper) getTreasuryOutflowWindow(ctx context.Context) (types.TreasuryOutflowWindow, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TreasuryOutflowWindowKey)
	if err != nil {
		return types.TreasuryOutflowWindow{}, err
	}
	if bz == nil {
		return types.TreasuryOutflowWindow{}, nil
	}
	var w types.TreasuryOutflowWindow
	if err := json.Unmarshal(bz, &w); err != nil {
		return types.TreasuryOutflowWindow{}, err
	}
	return w, nil
}

func (k Keeper) setTreasuryOutflowWindow(ctx context.Context, w types.TreasuryOutflowWindow) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(w)
	if err != nil {
		return err
	}
	return store.Set(types.TreasuryOutflowWindowKey, bz)
}

// RecordTreasuryOutflow records a treasury outflow (in bps) in a rolling window.
// Returns cumulative bps and whether the cap is exceeded.
func (k Keeper) RecordTreasuryOutflow(
	ctx context.Context,
	outflowBps uint64,
	maxOutflowBps uint64,
) (uint64, bool, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	now := sdkCtx.BlockTime().Unix()
	height := sdkCtx.BlockHeight()

	window, err := k.getTreasuryOutflowWindow(ctx)
	if err != nil {
		return 0, false, err
	}

	// Reset if window is stale
	if window.WindowStartUnix == 0 || now-window.WindowStartUnix > types.TreasuryOutflowWindowSeconds {
		window = types.TreasuryOutflowWindow{
			WindowStartUnix:  now,
			TotalOutflowBps:  0,
			LastUpdateHeight: height,
		}
	}

	// Add outflow and cap at 10000 bps
	window.TotalOutflowBps += outflowBps
	if window.TotalOutflowBps > 10000 {
		window.TotalOutflowBps = 10000
	}
	window.LastUpdateHeight = height

	if err := k.setTreasuryOutflowWindow(ctx, window); err != nil {
		return 0, false, err
	}

	if maxOutflowBps == 0 {
		return window.TotalOutflowBps, false, nil
	}

	if window.TotalOutflowBps > maxOutflowBps {
		return window.TotalOutflowBps, true, nil
	}

	return window.TotalOutflowBps, false, nil
}

// FormatTreasuryCapNote builds a deterministic status note for cap aborts.
func FormatTreasuryCapNote(cumulative, max uint64) string {
	return fmt.Sprintf("Aborted: treasury outflow %d bps exceeds daily cap %d bps", cumulative, max)
}
