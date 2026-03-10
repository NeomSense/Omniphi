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

	"pos/x/repgov/types"
)

// Keeper manages the state and business logic for x/repgov
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	logger       log.Logger
	authority    string // governance module account

	// Required keepers
	stakingKeeper types.StakingKeeper

	// Optional keepers (nil-safe, set post-init)
	pocKeeper types.PocKeeper
}

// NewKeeper creates a new Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	logger log.Logger,
	authority string,
	stakingKeeper types.StakingKeeper,
) Keeper {
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address: %s", authority))
	}

	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		logger:        logger,
		authority:     authority,
		stakingKeeper: stakingKeeper,
		pocKeeper:     nil,
	}
}

// SetPocKeeper sets the optional PoC keeper (called post-init)
func (k *Keeper) SetPocKeeper(pocKeeper types.PocKeeper) {
	k.pocKeeper = pocKeeper
}

// GetAuthority returns the module's authority address
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger
func (k Keeper) Logger() log.Logger {
	return k.logger
}

// ========== VoterWeight CRUD ==========

// SetVoterWeight stores a voter's governance weight
func (k Keeper) SetVoterWeight(ctx context.Context, vw types.VoterWeight) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(vw)
	if err != nil {
		return fmt.Errorf("failed to marshal voter weight: %w", err)
	}
	key := types.GetVoterWeightKey(vw.Address)
	return kvStore.Set(key, bz)
}

// GetVoterWeight retrieves a voter's governance weight
func (k Keeper) GetVoterWeight(ctx context.Context, addr string) (types.VoterWeight, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetVoterWeightKey(addr)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.VoterWeight{}, false
	}
	var vw types.VoterWeight
	if err := json.Unmarshal(bz, &vw); err != nil {
		k.logger.Error("failed to unmarshal voter weight", "error", err)
		return types.VoterWeight{}, false
	}
	return vw, true
}

// GetAllVoterWeights returns all stored voter weights
func (k Keeper) GetAllVoterWeights(ctx context.Context) []types.VoterWeight {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(
		types.KeyPrefixVoterWeight,
		storetypes.PrefixEndBytes(types.KeyPrefixVoterWeight),
	)
	if err != nil {
		k.logger.Error("failed to create voter weight iterator", "error", err)
		return nil
	}
	defer iter.Close()

	var weights []types.VoterWeight
	for ; iter.Valid(); iter.Next() {
		var vw types.VoterWeight
		if err := json.Unmarshal(iter.Value(), &vw); err != nil {
			k.logger.Error("failed to unmarshal voter weight during iteration", "error", err)
			continue
		}
		weights = append(weights, vw)
	}
	return weights
}

// GetEffectiveVotingWeight returns the effective governance weight for an address.
// Returns 1.0 (neutral) if module is disabled or no weight is stored.
func (k Keeper) GetEffectiveVotingWeight(ctx context.Context, addr string) math.LegacyDec {
	params := k.GetParams(ctx)
	if !params.Enabled {
		return math.LegacyOneDec()
	}

	vw, found := k.GetVoterWeight(ctx, addr)
	if !found {
		return math.LegacyOneDec()
	}
	return vw.EffectiveWeight
}

// ========== Delegation CRUD ==========

// SetDelegation stores a reputation delegation
func (k Keeper) SetDelegation(ctx context.Context, d types.DelegatedReputation) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("failed to marshal delegation: %w", err)
	}
	key := types.GetDelegatedReputationKey(d.Delegator, d.Delegatee)
	return kvStore.Set(key, bz)
}

// GetDelegation retrieves a specific delegation
func (k Keeper) GetDelegation(ctx context.Context, delegator, delegatee string) (types.DelegatedReputation, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetDelegatedReputationKey(delegator, delegatee)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.DelegatedReputation{}, false
	}
	var d types.DelegatedReputation
	if err := json.Unmarshal(bz, &d); err != nil {
		k.logger.Error("failed to unmarshal delegation", "error", err)
		return types.DelegatedReputation{}, false
	}
	return d, true
}

// DeleteDelegation removes a delegation
func (k Keeper) DeleteDelegation(ctx context.Context, delegator, delegatee string) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetDelegatedReputationKey(delegator, delegatee)
	return kvStore.Delete(key)
}

// GetDelegationsFrom returns all delegations from a delegator
func (k Keeper) GetDelegationsFrom(ctx context.Context, delegator string) []types.DelegatedReputation {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetDelegatedReputationPrefixKey(delegator)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var delegations []types.DelegatedReputation
	for ; iter.Valid(); iter.Next() {
		var d types.DelegatedReputation
		if err := json.Unmarshal(iter.Value(), &d); err != nil {
			continue
		}
		delegations = append(delegations, d)
	}
	return delegations
}

// ========== Tally Override CRUD ==========

// SetTallyOverride stores a tally override record
func (k Keeper) SetTallyOverride(ctx context.Context, to types.TallyOverride) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(to)
	if err != nil {
		return fmt.Errorf("failed to marshal tally override: %w", err)
	}
	key := types.GetTallyOverrideKey(to.ProposalID)
	return kvStore.Set(key, bz)
}

// GetTallyOverride retrieves a tally override record
func (k Keeper) GetTallyOverride(ctx context.Context, proposalID uint64) (types.TallyOverride, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetTallyOverrideKey(proposalID)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.TallyOverride{}, false
	}
	var to types.TallyOverride
	if err := json.Unmarshal(bz, &to); err != nil {
		k.logger.Error("failed to unmarshal tally override", "error", err)
		return types.TallyOverride{}, false
	}
	return to, true
}

// ========== Last Computed Epoch ==========

// SetLastComputedEpoch stores the last epoch when weights were computed
func (k Keeper) SetLastComputedEpoch(ctx context.Context, epoch int64) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.KeyLastComputedEpoch, sdk.Uint64ToBigEndian(uint64(epoch)))
}

// GetLastComputedEpoch returns the last epoch when weights were computed
func (k Keeper) GetLastComputedEpoch(ctx context.Context) int64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyLastComputedEpoch)
	if err != nil || bz == nil {
		return 0
	}
	return int64(sdk.BigEndianToUint64(bz))
}

// ========== Core Computation Engine ==========

// ComputeVoterWeight calculates the governance weight for a single address.
// Formula: weight = 1 + normalized_reputation * (max_cap - 1)
// Where normalized_reputation = sum(source_score * source_weight) / sum(source_weights)
func (k Keeper) ComputeVoterWeight(ctx context.Context, addr string, epoch int64) types.VoterWeight {
	params := k.GetParams(ctx)
	vw := types.NewVoterWeight(addr, epoch)

	if !params.Enabled || k.pocKeeper == nil {
		return vw
	}

	// Gather reputation signals from PoC
	repScore := k.pocKeeper.GetReputationScoreValue(ctx, addr)
	vw.ReputationScore = repScore

	// If below minimum threshold, no bonus
	if repScore.LT(params.MinReputationThreshold) {
		return vw
	}

	// C-Score: normalize credits to [0, 1] range
	// min(credits / 10000, 1.0)
	creditAmt := k.pocKeeper.GetCreditAmount(ctx, addr)
	cScore := math.LegacyZeroDec()
	if creditAmt.IsPositive() {
		cScore = math.LegacyNewDecFromInt(creditAmt).Quo(math.LegacyNewDec(10000))
		if cScore.GT(math.LegacyOneDec()) {
			cScore = math.LegacyOneDec()
		}
	}
	vw.CScore = cScore

	// Endorsement participation rate (for validators)
	valAddr, err := sdk.ValAddressFromBech32(addr)
	endorsementRate := math.LegacyZeroDec()
	if err == nil {
		// Address is a valid validator address
		rate, err := k.pocKeeper.GetEndorsementParticipationRate(ctx, valAddr)
		if err == nil {
			endorsementRate = rate
		}
	}
	vw.EndorsementRate = endorsementRate

	// Originality metrics (for validators)
	originalityAvg := math.LegacyZeroDec()
	if err == nil {
		avgOrig, _, origErr := k.pocKeeper.GetValidatorOriginalityMetrics(ctx, valAddr)
		if origErr == nil {
			originalityAvg = avgOrig
		}
	}
	vw.OriginalityAvg = originalityAvg

	// Longevity: use reputation score as proxy (higher rep = longer active participation)
	vw.LongevityScore = repScore

	// Compute weighted sum
	weightedSum := cScore.Mul(params.CScoreWeight).
		Add(endorsementRate.Mul(params.EndorsementWeight)).
		Add(originalityAvg.Mul(params.OriginalityWeight)).
		Add(vw.UptimeScore.Mul(params.UptimeWeight)).
		Add(vw.LongevityScore.Mul(params.LongevityWeight))

	// Normalize by total weight
	totalSourceWeight := params.CScoreWeight.Add(params.EndorsementWeight).
		Add(params.OriginalityWeight).Add(params.UptimeWeight).Add(params.LongevityWeight)

	normalizedRep := math.LegacyZeroDec()
	if totalSourceWeight.IsPositive() {
		normalizedRep = weightedSum.Quo(totalSourceWeight)
	}

	// Composite weight = 1 + normalizedRep * (maxCap - 1)
	// This maps [0, 1] reputation to [1.0, maxCap] weight
	maxCapMinusOne := params.MaxVotingWeightCap.Sub(math.LegacyOneDec())
	compositeWeight := math.LegacyOneDec().Add(normalizedRep.Mul(maxCapMinusOne))

	// Clamp to [1.0, maxCap]
	if compositeWeight.LT(math.LegacyOneDec()) {
		compositeWeight = math.LegacyOneDec()
	}
	if compositeWeight.GT(params.MaxVotingWeightCap) {
		compositeWeight = params.MaxVotingWeightCap
	}

	vw.CompositeWeight = compositeWeight
	vw.EffectiveWeight = compositeWeight // before delegation adjustments

	return vw
}

// RecomputeAllWeights recalculates governance weights for all known contributors.
// Called at epoch boundaries from the EndBlocker.
func (k Keeper) RecomputeAllWeights(ctx context.Context, epoch int64) error {
	params := k.GetParams(ctx)
	if !params.Enabled {
		return nil
	}

	// Get all existing voter weights and recompute
	existingWeights := k.GetAllVoterWeights(ctx)
	for _, existing := range existingWeights {
		vw := k.ComputeVoterWeight(ctx, existing.Address, epoch)

		// Preserve last vote height from existing record
		vw.LastVoteHeight = existing.LastVoteHeight

		// Apply decay if voter hasn't participated recently
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		blocksSinceVote := sdkCtx.BlockHeight() - existing.LastVoteHeight
		if existing.LastVoteHeight > 0 && blocksSinceVote > params.RecomputeInterval*3 {
			// Decay the bonus portion (keep base 1.0)
			bonus := vw.EffectiveWeight.Sub(math.LegacyOneDec())
			if bonus.IsPositive() {
				decayedBonus := bonus.Mul(math.LegacyOneDec().Sub(params.DecayRate))
				vw.EffectiveWeight = math.LegacyOneDec().Add(decayedBonus)
			}
		}

		// Apply delegation adjustments
		vw = k.applyDelegations(ctx, vw, params)

		if err := k.SetVoterWeight(ctx, vw); err != nil {
			k.logger.Error("failed to set voter weight", "addr", existing.Address, "error", err)
			continue
		}
	}

	if err := k.SetLastComputedEpoch(ctx, epoch); err != nil {
		return err
	}

	k.logger.Info("recomputed governance weights",
		"epoch", epoch,
		"voters", len(existingWeights),
	)

	return nil
}

// applyDelegations adjusts a voter's weight based on reputation delegations received
func (k Keeper) applyDelegations(ctx context.Context, vw types.VoterWeight, params types.Params) types.VoterWeight {
	if !params.DelegationEnabled {
		return vw
	}

	// Sum delegated reputation received by this address
	// We iterate all delegations and find ones where delegatee == vw.Address
	// This is O(n) per voter — acceptable for small sets, should use index for large chains
	allWeights := k.GetAllVoterWeights(ctx)
	delegatedTotal := math.LegacyZeroDec()

	for _, other := range allWeights {
		if other.Address == vw.Address {
			continue
		}
		d, found := k.GetDelegation(ctx, other.Address, vw.Address)
		if found {
			// Add the delegated portion of the other voter's composite weight bonus
			bonus := other.CompositeWeight.Sub(math.LegacyOneDec())
			if bonus.IsPositive() {
				delegatedPortion := bonus.Mul(d.Amount)
				delegatedTotal = delegatedTotal.Add(delegatedPortion)
			}
		}
	}

	vw.DelegatedWeight = delegatedTotal
	vw.EffectiveWeight = vw.EffectiveWeight.Add(delegatedTotal)

	// Re-clamp to max cap
	if vw.EffectiveWeight.GT(params.MaxVotingWeightCap) {
		vw.EffectiveWeight = params.MaxVotingWeightCap
	}

	return vw
}

// RecordVoteParticipation updates the last vote height for an address.
// Called by gov hooks when a vote is cast.
func (k Keeper) RecordVoteParticipation(ctx context.Context, addr string, blockHeight int64) error {
	vw, found := k.GetVoterWeight(ctx, addr)
	if !found {
		// First-time voter — create weight entry
		vw = types.NewVoterWeight(addr, 0)
	}
	vw.LastVoteHeight = blockHeight
	return k.SetVoterWeight(ctx, vw)
}

// EnsureVoterRegistered creates a voter weight entry if one doesn't exist.
// Called when a contributor first submits to PoC, so they're tracked for governance.
func (k Keeper) EnsureVoterRegistered(ctx context.Context, addr string) error {
	_, found := k.GetVoterWeight(ctx, addr)
	if found {
		return nil // already registered
	}
	vw := types.NewVoterWeight(addr, 0)
	return k.SetVoterWeight(ctx, vw)
}

// RecordContributionOutcome updates a contributor's originality and reputation signals
// based on a PoC review outcome. Called by the PoC keeper's finalizeReviewSession.
//
// For accepted contributions: OriginalityAvg is nudged upward by quality and penalised
// by similarity (higher similarity = more derivative = lower originality signal).
// For rejected contributions: OriginalityAvg is nudged downward.
//
// The adjustment uses an EMA-style blend to avoid wild swings from single outcomes.
// alpha = 0.1 (new sample weight); signal = qualityNorm * (1 - similarityScore).
func (k Keeper) RecordContributionOutcome(ctx context.Context, contributor string, accepted bool, qualityScore math.LegacyDec, similarityScore math.LegacyDec) error {
	params := k.GetParams(ctx)
	if !params.Enabled {
		return nil
	}

	// Ensure voter is registered
	vw, found := k.GetVoterWeight(ctx, contributor)
	if !found {
		vw = types.NewVoterWeight(contributor, 0)
	}

	// originality signal: quality normalized to [0,1] penalised by similarity
	// qualityScore is in [0, 10]; normalize to [0, 1]
	qualNorm := qualityScore.Quo(math.LegacyNewDec(10))
	if qualNorm.GT(math.LegacyOneDec()) {
		qualNorm = math.LegacyOneDec()
	}

	one := math.LegacyOneDec()
	originalitySignal := qualNorm.Mul(one.Sub(similarityScore))

	if !accepted {
		// Rejection: drive signal toward zero
		originalitySignal = math.LegacyZeroDec()
	}

	// EMA blend: new = 0.9 * old + 0.1 * signal
	alpha := math.LegacyNewDecWithPrec(1, 1) // 0.1
	oneMinusAlpha := one.Sub(alpha)
	vw.OriginalityAvg = oneMinusAlpha.Mul(vw.OriginalityAvg).Add(alpha.Mul(originalitySignal))

	// Clamp to [0, 1]
	if vw.OriginalityAvg.IsNegative() {
		vw.OriginalityAvg = math.LegacyZeroDec()
	}
	if vw.OriginalityAvg.GT(one) {
		vw.OriginalityAvg = one
	}

	return k.SetVoterWeight(ctx, vw)
}
