package keeper

import (
	"context"
	"encoding/json"

	storetypes "cosmossdk.io/store/types"

	"pos/x/poseq/types"
)

// ─── SequencerRankingProfile store ────────────────────────────────────────────

// StoreRankingProfile stores a sequencer ranking profile keyed by nodeIDHex.
// Overwrites any prior record.
func (k Keeper) StoreRankingProfile(ctx context.Context, profile types.SequencerRankingProfile) error {
	if len(profile.NodeID) != 64 {
		return types.ErrInvalidNodeID.Wrap("ranking: node_id must be 64 hex chars")
	}
	bz, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.GetRankingProfileKey(profile.NodeID), bz)
}

// GetRankingProfile retrieves the ranking profile for nodeIDHex.
// Returns (nil, nil) if not found.
func (k Keeper) GetRankingProfile(ctx context.Context, nodeIDHex string) (*types.SequencerRankingProfile, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetRankingProfileKey(nodeIDHex))
	if err != nil || bz == nil {
		return nil, err
	}
	var profile types.SequencerRankingProfile
	if err := json.Unmarshal(bz, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// AllRankingProfiles returns all stored sequencer ranking profiles.
// Uses a prefix scan over [0x15 | ...].
func (k Keeper) AllRankingProfiles(ctx context.Context) ([]types.SequencerRankingProfile, error) {
	prefix := []byte{types.KeyPrefixRankingProfile}
	end := storetypes.PrefixEndBytes(prefix)

	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(prefix, end)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var profiles []types.SequencerRankingProfile
	for ; iter.Valid(); iter.Next() {
		var p types.SequencerRankingProfile
		if err := json.Unmarshal(iter.Value(), &p); err != nil {
			k.logger.Error("failed to unmarshal ranking profile", "error", err)
			continue
		}
		profiles = append(profiles, p)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return profiles, nil
}

// ─── Ranking computation ──────────────────────────────────────────────────────

// ComputeSequencerRankingProfile builds a SequencerRankingProfile for a node
// from the current epoch's reward score and bond state.
//
// RankScore formula (all integer):
//
//	rank_score = participation_rate_bps * poc_multiplier_bps / 10000
//
// Bond gates committee inclusion but is not part of the continuous rank factor,
// avoiding concentration of ranking power in large-bond operators.
func (k Keeper) ComputeSequencerRankingProfile(
	ctx context.Context,
	nodeIDHex string,
	epoch uint64,
) (*types.SequencerRankingProfile, error) {
	// Fetch reward score for this epoch
	rs, err := k.GetRewardScore(ctx, epoch, nodeIDHex)
	if err != nil {
		return nil, err
	}
	if rs == nil {
		// No reward score yet — build minimal profile
		rs = &types.EpochRewardScore{
			NodeID: nodeIDHex,
			Epoch:  epoch,
		}
	}

	// Fetch bond
	var availableBond uint64
	var operatorAddr string
	isBonded := false
	bond, _ := k.GetActiveBondForNode(ctx, nodeIDHex)
	if bond != nil {
		isBonded = true
		availableBond = bond.AvailableBond
		if availableBond == 0 {
			availableBond = bond.BondAmount
		}
		operatorAddr = bond.OperatorAddress
	}

	// PoC multiplier (default 10000)
	pocMult := rs.PoCMultiplierBps
	if pocMult == 0 {
		pocMult = 10_000
	}

	// Participation rate
	participationBps := rs.BaseScoreBps

	// Fault history: count faults across trailing MaxFaultHistoryEpochs
	params := k.GetParams(ctx)
	maxHistory := uint64(params.MaxFaultHistoryEpochs)
	if maxHistory == 0 {
		maxHistory = 5
	}
	var faultEventsRecent uint64
	for e := epoch; e > 0 && epoch-e < maxHistory; e-- {
		priorRS, rErr := k.GetRewardScore(ctx, e, nodeIDHex)
		if rErr != nil || priorRS == nil {
			continue
		}
		// fault_penalty_bps / 500 ≈ fault_events (inverse of compute_fault_penalty)
		// We don't store raw fault count in reward score, use penalty as proxy
		faultEventsRecent += uint64(priorRS.FaultPenaltyBps / 500)
	}

	// Compute rank score
	rankScore := uint32(uint64(participationBps) * uint64(pocMult) / 10_000)

	// Determine activation epoch for tier classification
	var epochsSinceActivation uint64
	seq, _ := k.GetSequencer(ctx, nodeIDHex)
	if seq != nil {
		if seq.StatusSince <= epoch {
			epochsSinceActivation = epoch - seq.StatusSince
		}
	}

	tier := types.ClassifyTier(isBonded, pocMult, participationBps, faultEventsRecent, epochsSinceActivation)

	profile := &types.SequencerRankingProfile{
		NodeID:               nodeIDHex,
		OperatorAddress:      operatorAddr,
		Epoch:                epoch,
		AvailableBond:        availableBond,
		PoCMultiplierBps:     pocMult,
		ParticipationRateBps: participationBps,
		FaultEventsRecent:    faultEventsRecent,
		RankScore:            rankScore,
		Tier:                 tier,
	}

	return profile, nil
}

// IsAdmissibleForCommittee returns true if the node meets all economic admission
// criteria set in params:
//   - If MinBondForCommittee > 0: AvailableBond >= MinBondForCommittee
//   - If MinParticipationBps > 0: participation_rate_bps >= MinParticipationBps
//
// Does not check MaxFaultHistoryEpochs — that is enforced in BuildCommitteeSnapshot.
func (k Keeper) IsAdmissibleForCommittee(ctx context.Context, nodeIDHex string, epoch uint64) (bool, string) {
	params := k.GetParams(ctx)

	if params.MinBondForCommittee > 0 {
		bond, err := k.GetActiveBondForNode(ctx, nodeIDHex)
		if err != nil || bond == nil {
			return false, "no active bond"
		}
		available := bond.AvailableBond
		if available == 0 {
			available = bond.BondAmount
		}
		if available < params.MinBondForCommittee {
			return false, "available_bond below min_bond_for_committee"
		}
	}

	if params.MinParticipationBps > 0 {
		rs, err := k.GetRewardScore(ctx, epoch, nodeIDHex)
		if err == nil && rs != nil {
			if rs.BaseScoreBps < params.MinParticipationBps {
				return false, "participation_rate below min_participation_bps"
			}
		}
	}

	return true, ""
}
