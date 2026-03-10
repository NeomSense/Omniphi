package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/uci/types"
)

// Keeper manages the state and business logic for x/uci
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger
	authority    string

	bankKeeper    types.BankKeeper
	accountKeeper types.AccountKeeper

	// Optional keepers (nil-safe, set post-init)
	pocKeeper types.PocKeeper
}

// NewKeeper creates a new Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,
	bankKeeper types.BankKeeper,
	accountKeeper types.AccountKeeper,
) Keeper {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address: %s", authority))
	}

	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		logger:        logger,
		authority:     authority,
		bankKeeper:    bankKeeper,
		accountKeeper: accountKeeper,
		pocKeeper:     nil,
	}
}

// SetPocKeeper sets the optional PoC keeper (called post-init)
func (k *Keeper) SetPocKeeper(pocKeeper types.PocKeeper) {
	k.pocKeeper = pocKeeper
}

func (k Keeper) GetAuthority() string { return k.authority }
func (k Keeper) Logger() log.Logger   { return k.logger }

// ========== Adapter CRUD ==========

func (k Keeper) GetNextAdapterID(ctx context.Context) uint64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyNextAdapterID)
	if err != nil || bz == nil {
		return 1
	}
	return sdk.BigEndianToUint64(bz)
}

func (k Keeper) SetNextAdapterID(ctx context.Context, id uint64) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.KeyNextAdapterID, sdk.Uint64ToBigEndian(id))
}

func (k Keeper) SetAdapter(ctx context.Context, adapter types.Adapter) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(adapter)
	if err != nil {
		return fmt.Errorf("failed to marshal adapter: %w", err)
	}
	key := types.GetAdapterKey(adapter.AdapterID)
	if err := kvStore.Set(key, bz); err != nil {
		return err
	}

	// Update owner index
	ownerKey := types.GetAdapterByOwnerKey(adapter.Owner, adapter.AdapterID)
	return kvStore.Set(ownerKey, []byte{})
}

func (k Keeper) GetAdapter(ctx context.Context, adapterID uint64) (types.Adapter, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetAdapterKey(adapterID))
	if err != nil || bz == nil {
		return types.Adapter{}, false
	}
	var adapter types.Adapter
	if err := json.Unmarshal(bz, &adapter); err != nil {
		k.logger.Error("failed to unmarshal adapter", "error", err)
		return types.Adapter{}, false
	}
	return adapter, true
}

func (k Keeper) GetAllAdapters(ctx context.Context) []types.Adapter {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(
		types.KeyPrefixAdapter,
		storetypes.PrefixEndBytes(types.KeyPrefixAdapter),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	var adapters []types.Adapter
	for ; iter.Valid(); iter.Next() {
		var adapter types.Adapter
		if err := json.Unmarshal(iter.Value(), &adapter); err != nil {
			continue
		}
		adapters = append(adapters, adapter)
	}
	return adapters
}

func (k Keeper) GetAdaptersByOwner(ctx context.Context, owner string) []types.Adapter {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetAdapterByOwnerPrefixKey(owner)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var adapters []types.Adapter
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) < 8 {
			continue
		}
		adapterID := sdk.BigEndianToUint64(key[len(key)-8:])
		adapter, found := k.GetAdapter(ctx, adapterID)
		if found {
			adapters = append(adapters, adapter)
		}
	}
	return adapters
}

// ========== ContributionMapping CRUD ==========

func (k Keeper) SetContributionMapping(ctx context.Context, cm types.ContributionMapping) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(cm)
	if err != nil {
		return fmt.Errorf("failed to marshal contribution mapping: %w", err)
	}
	key := types.GetContributionMappingKey(cm.AdapterID, cm.ExternalID)
	return kvStore.Set(key, bz)
}

func (k Keeper) GetContributionMapping(ctx context.Context, adapterID uint64, externalID string) (types.ContributionMapping, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetContributionMappingKey(adapterID, externalID)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.ContributionMapping{}, false
	}
	var cm types.ContributionMapping
	if err := json.Unmarshal(bz, &cm); err != nil {
		return types.ContributionMapping{}, false
	}
	return cm, true
}

// ========== AdapterStats CRUD ==========

func (k Keeper) GetAdapterStats(ctx context.Context, adapterID uint64) types.AdapterStats {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetAdapterStatsKey(adapterID))
	if err != nil || bz == nil {
		return types.NewAdapterStats(adapterID)
	}
	var stats types.AdapterStats
	if err := json.Unmarshal(bz, &stats); err != nil {
		return types.NewAdapterStats(adapterID)
	}
	return stats
}

func (k Keeper) SetAdapterStats(ctx context.Context, stats types.AdapterStats) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	return kvStore.Set(types.GetAdapterStatsKey(stats.AdapterID), bz)
}

// ========== Oracle Attestation CRUD ==========

func (k Keeper) SetOracleAttestation(ctx context.Context, att types.OracleAttestation) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(att)
	if err != nil {
		return err
	}
	key := types.GetOracleAttestationKey(att.AdapterID, att.BatchID)
	return kvStore.Set(key, bz)
}

func (k Keeper) GetOracleAttestation(ctx context.Context, adapterID uint64, batchID string) (types.OracleAttestation, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetOracleAttestationKey(adapterID, batchID)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.OracleAttestation{}, false
	}
	var att types.OracleAttestation
	if err := json.Unmarshal(bz, &att); err != nil {
		return types.OracleAttestation{}, false
	}
	return att, true
}

// ========== Core Business Logic ==========

// ProcessDePINContribution takes an external DePIN contribution and routes it through PoC
func (k Keeper) ProcessDePINContribution(
	ctx context.Context,
	adapterID uint64,
	externalID string,
	contributor string,
	hash string,
	uri string,
) (uint64, error) {
	params := k.GetParams(ctx)
	if !params.Enabled {
		return 0, types.ErrModuleDisabled
	}

	adapter, found := k.GetAdapter(ctx, adapterID)
	if !found {
		return 0, types.ErrAdapterNotFound
	}
	if adapter.Status != types.AdapterStatusActive {
		return 0, types.ErrAdapterSuspended
	}

	// Check if already mapped
	_, exists := k.GetContributionMapping(ctx, adapterID, externalID)
	if exists {
		return 0, types.ErrExternalIDAlreadyMapped
	}

	// Submit to PoC via keeper interface
	var pocContributionID uint64
	if k.pocKeeper != nil {
		ctype := fmt.Sprintf("depin/%s", adapter.NetworkType)
		id, err := k.pocKeeper.SubmitContribution(ctx, contributor, hash, uri, ctype)
		if err != nil {
			return 0, fmt.Errorf("failed to submit to PoC: %w", err)
		}
		pocContributionID = id
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Store mapping
	mapping := types.ContributionMapping{
		AdapterID:        adapterID,
		ExternalID:       externalID,
		PocContributionID: pocContributionID,
		Contributor:      contributor,
		MappedAtHeight:   sdkCtx.BlockHeight(),
		RewardAmount:     math.ZeroInt(),
		OracleVerified:   false,
	}
	if err := k.SetContributionMapping(ctx, mapping); err != nil {
		return 0, err
	}

	// Update adapter stats
	stats := k.GetAdapterStats(ctx, adapterID)
	stats.TotalSubmissions++
	stats.Successful++
	stats.LastSubmissionHeight = sdkCtx.BlockHeight()
	if err := k.SetAdapterStats(ctx, stats); err != nil {
		k.logger.Error("failed to update adapter stats", "adapter_id", adapterID, "error", err)
	}

	// Update adapter total contributions
	adapter.TotalContributions++
	if err := k.SetAdapter(ctx, adapter); err != nil {
		k.logger.Error("failed to update adapter", "adapter_id", adapterID, "error", err)
	}

	return pocContributionID, nil
}

// IsOracleAuthorized checks if an oracle address is in an adapter's allowlist
func (k Keeper) IsOracleAuthorized(adapter types.Adapter, oracleAddr string) bool {
	for _, allowed := range adapter.OracleAllowlist {
		if allowed == oracleAddr {
			return true
		}
	}
	return false
}
