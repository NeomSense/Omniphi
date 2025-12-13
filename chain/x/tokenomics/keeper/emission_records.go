package keeper

import (
	"context"
	"encoding/binary"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/tokenomics/types"
)

// ============================================================================
// EMISSION RECORD TRACKING
// ============================================================================
// Tracks historical emission events for auditing and transparency.
// Each emission event records the total amount and per-recipient breakdown.

// GetNextEmissionID returns the next emission record ID
func (k Keeper) GetNextEmissionID(ctx context.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.KeyNextEmissionID)
	if err != nil || bz == nil {
		return 1
	}

	return binary.BigEndian.Uint64(bz)
}

// SetNextEmissionID sets the next emission record ID
func (k Keeper) SetNextEmissionID(ctx context.Context, id uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, id)
	return store.Set(types.KeyNextEmissionID, bz)
}

// IncrementEmissionID increments and returns the current emission ID
func (k Keeper) IncrementEmissionID(ctx context.Context) uint64 {
	id := k.GetNextEmissionID(ctx)
	_ = k.SetNextEmissionID(ctx, id+1)
	return id
}

// RecordEmission creates a new emission record for auditing
func (k Keeper) RecordEmission(
	ctx context.Context,
	totalEmitted math.Int,
	toStaking math.Int,
	toPoc math.Int,
	toSequencer math.Int,
	toTreasury math.Int,
) (*types.EmissionRecord, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	emissionID := k.IncrementEmissionID(ctx)

	record := &types.EmissionRecord{
		EmissionId:   emissionID,
		BlockHeight:  sdkCtx.BlockHeight(),
		TotalEmitted: totalEmitted,
		ToStaking:    toStaking,
		ToPoc:        toPoc,
		ToSequencer:  toSequencer,
		ToTreasury:   toTreasury,
		Timestamp:    sdkCtx.BlockTime().Unix(),
	}

	// Store the record
	if err := k.SetEmissionRecord(ctx, record); err != nil {
		return nil, err
	}

	// Emit event for transparency
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeEmissionAllocated,
			sdk.NewAttribute(types.AttributeKeyEmissionID, string(rune(emissionID))),
			sdk.NewAttribute(types.AttributeKeyTotalEmitted, totalEmitted.String()),
			sdk.NewAttribute(types.AttributeKeyToStaking, toStaking.String()),
			sdk.NewAttribute(types.AttributeKeyToPoc, toPoc.String()),
			sdk.NewAttribute(types.AttributeKeyToSequencer, toSequencer.String()),
			sdk.NewAttribute(types.AttributeKeyToTreasury, toTreasury.String()),
			sdk.NewAttribute(types.AttributeKeyBlockHeight, string(rune(sdkCtx.BlockHeight()))),
		),
	)

	return record, nil
}

// SetEmissionRecord stores an emission record
func (k Keeper) SetEmissionRecord(ctx context.Context, record *types.EmissionRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetEmissionRecordKey(record.EmissionId)
	bz := k.cdc.MustMarshal(record)
	return store.Set(key, bz)
}

// GetEmissionRecord retrieves an emission record by ID
func (k Keeper) GetEmissionRecord(ctx context.Context, id uint64) (*types.EmissionRecord, error) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetEmissionRecordKey(id)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, types.ErrNotFound
	}

	var record types.EmissionRecord
	k.cdc.MustUnmarshal(bz, &record)
	return &record, nil
}

// GetEmissionRecordsByRange retrieves emission records within a block height range
func (k Keeper) GetEmissionRecordsByRange(ctx context.Context, startHeight, endHeight int64) ([]*types.EmissionRecord, error) {
	var records []*types.EmissionRecord

	// Iterate through all emission records and filter by height
	// Note: In production, consider using secondary indexes for efficiency
	nextID := k.GetNextEmissionID(ctx)
	for i := uint64(1); i < nextID; i++ {
		record, err := k.GetEmissionRecord(ctx, i)
		if err != nil {
			continue
		}

		if record.BlockHeight >= startHeight && record.BlockHeight <= endHeight {
			records = append(records, record)
		}
	}

	return records, nil
}

// GetLatestEmissionRecords retrieves the N most recent emission records
func (k Keeper) GetLatestEmissionRecords(ctx context.Context, limit uint64) ([]*types.EmissionRecord, error) {
	var records []*types.EmissionRecord

	nextID := k.GetNextEmissionID(ctx)
	if nextID <= 1 {
		return records, nil
	}

	// Start from the most recent and work backwards
	count := uint64(0)
	for i := nextID - 1; i >= 1 && count < limit; i-- {
		record, err := k.GetEmissionRecord(ctx, i)
		if err != nil {
			continue
		}
		records = append(records, record)
		count++
	}

	return records, nil
}

// GetEmissionStats returns aggregate emission statistics
func (k Keeper) GetEmissionStats(ctx context.Context) (*EmissionStats, error) {
	stats := &EmissionStats{
		TotalRecords:     0,
		TotalEmitted:     math.ZeroInt(),
		TotalToStaking:   math.ZeroInt(),
		TotalToPoc:       math.ZeroInt(),
		TotalToSequencer: math.ZeroInt(),
		TotalToTreasury:  math.ZeroInt(),
	}

	nextID := k.GetNextEmissionID(ctx)
	for i := uint64(1); i < nextID; i++ {
		record, err := k.GetEmissionRecord(ctx, i)
		if err != nil {
			continue
		}

		stats.TotalRecords++
		stats.TotalEmitted = stats.TotalEmitted.Add(record.TotalEmitted)
		stats.TotalToStaking = stats.TotalToStaking.Add(record.ToStaking)
		stats.TotalToPoc = stats.TotalToPoc.Add(record.ToPoc)
		stats.TotalToSequencer = stats.TotalToSequencer.Add(record.ToSequencer)
		stats.TotalToTreasury = stats.TotalToTreasury.Add(record.ToTreasury)
	}

	// Calculate percentages
	if stats.TotalEmitted.IsPositive() {
		totalDec := math.LegacyNewDecFromInt(stats.TotalEmitted)
		stats.StakingPercentage = math.LegacyNewDecFromInt(stats.TotalToStaking).Quo(totalDec)
		stats.PocPercentage = math.LegacyNewDecFromInt(stats.TotalToPoc).Quo(totalDec)
		stats.SequencerPercentage = math.LegacyNewDecFromInt(stats.TotalToSequencer).Quo(totalDec)
		stats.TreasuryPercentage = math.LegacyNewDecFromInt(stats.TotalToTreasury).Quo(totalDec)
	}

	return stats, nil
}

// EmissionStats holds aggregate emission statistics
type EmissionStats struct {
	TotalRecords        uint64
	TotalEmitted        math.Int
	TotalToStaking      math.Int
	TotalToPoc          math.Int
	TotalToSequencer    math.Int
	TotalToTreasury     math.Int
	StakingPercentage   math.LegacyDec
	PocPercentage       math.LegacyDec
	SequencerPercentage math.LegacyDec
	TreasuryPercentage  math.LegacyDec
}

// ValidateEmissionInvariant checks that emission allocations match total minted
// This is called periodically to ensure supply accounting integrity
func (k Keeper) ValidateEmissionInvariant(ctx context.Context) error {
	stats, err := k.GetEmissionStats(ctx)
	if err != nil {
		return err
	}

	// Sum of allocations should equal total emitted
	allocSum := stats.TotalToStaking.
		Add(stats.TotalToPoc).
		Add(stats.TotalToSequencer).
		Add(stats.TotalToTreasury)

	if !allocSum.Equal(stats.TotalEmitted) {
		return types.ErrEmissionBudgetExceeded
	}

	return nil
}
