package keeper

import (
	"context"
	"encoding/json"

	cosmossdk_io_math "cosmossdk.io/math"
	"pos/x/poc/types"
)

// GetParams returns the current module parameters
func (k Keeper) GetParams(ctx context.Context) types.Params {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get([]byte("params"))
	if err != nil || bz == nil {
		return types.DefaultParams()
	}

	var params types.Params
	k.cdc.MustUnmarshal(bz, &params)

	// Load access control params from separate JSON storage (workaround for proto map serialization)
	acBz, err := store.Get([]byte("params_access_control"))
	if err == nil && acBz != nil {
		var ac struct {
			EnableCscoreGating      bool              `json:"enable_cscore_gating"`
			MinCscoreForCtype       map[string]string `json:"min_cscore_for_ctype"`
			EnableIdentityGating    bool              `json:"enable_identity_gating"`
			RequireIdentityForCtype map[string]bool   `json:"require_identity_for_ctype"`
			ExemptAddresses         []string          `json:"exempt_addresses"`
		}
		if err := json.Unmarshal(acBz, &ac); err == nil {
			params.EnableCscoreGating = ac.EnableCscoreGating
			params.EnableIdentityGating = ac.EnableIdentityGating
			params.RequireIdentityForCtype = ac.RequireIdentityForCtype
			params.ExemptAddresses = ac.ExemptAddresses

			// Convert string map to math.Int map
			params.MinCscoreForCtype = make(map[string]cosmossdk_io_math.Int)
			for ctype, scoreStr := range ac.MinCscoreForCtype {
				if score, ok := cosmossdk_io_math.NewIntFromString(scoreStr); ok {
					params.MinCscoreForCtype[ctype] = score
				}
			}
		}
	}

	// Load similarity engine params from separate JSON storage
	simBz, err := store.Get([]byte("params_similarity"))
	if err == nil && simBz != nil {
		var simParams struct {
			SimilarityOracleAllowlist []string `json:"similarity_oracle_allowlist"`
			DerivativeThreshold       uint32   `json:"derivative_threshold"`
			SimilarityEpochBlocks     int64    `json:"similarity_epoch_blocks"`
			EnableSimilarityCheck     bool     `json:"enable_similarity_check"`
		}
		if err := json.Unmarshal(simBz, &simParams); err == nil {
			params.SimilarityOracleAllowlist = simParams.SimilarityOracleAllowlist
			params.DerivativeThreshold = simParams.DerivativeThreshold
			params.SimilarityEpochBlocks = simParams.SimilarityEpochBlocks
			params.EnableSimilarityCheck = simParams.EnableSimilarityCheck
		}
	}

	// Load canonical hash layer params from separate JSON storage
	chBz, err := store.Get([]byte("params_canonical_hash"))
	if err == nil && chBz != nil {
		var chParams struct {
			DuplicateBond              string `json:"duplicate_bond"`
			EnableCanonicalHashCheck   bool   `json:"enable_canonical_hash_check"`
			MaxDuplicatesPerEpoch      uint32 `json:"max_duplicates_per_epoch"`
			DuplicateBondEscalationBps uint32 `json:"duplicate_bond_escalation_bps"`
		}
		if err := json.Unmarshal(chBz, &chParams); err == nil {
			// Parse DuplicateBond
			if coin, err := types.ParseCoinFromString(chParams.DuplicateBond); err == nil {
				params.DuplicateBond = coin
			}
			params.EnableCanonicalHashCheck = chParams.EnableCanonicalHashCheck
			params.MaxDuplicatesPerEpoch = chParams.MaxDuplicatesPerEpoch
			params.DuplicateBondEscalationBps = chParams.DuplicateBondEscalationBps
		}
	}

	// Load human review layer params from separate JSON storage
	hrBz, err := store.Get([]byte("params_human_review"))
	if err == nil && hrBz != nil {
		var hrParams struct {
			VerifiersPerClaim     uint32 `json:"verifiers_per_claim"`
			ReviewVotePeriod      int64  `json:"review_vote_period"`
			EnableHumanReview     bool   `json:"enable_human_review"`
			ReviewQuorumPct       uint32 `json:"review_quorum_pct"`
			MinReviewerBond       string `json:"min_reviewer_bond"`
			MinReviewerReputation uint64 `json:"min_reviewer_reputation"`
			AppealBond            string `json:"appeal_bond"`
			AppealVotePeriod      int64  `json:"appeal_vote_period"`
			CollusionThresholdBps uint32 `json:"collusion_threshold_bps"`
		}
		if err := json.Unmarshal(hrBz, &hrParams); err == nil {
			params.VerifiersPerClaim = hrParams.VerifiersPerClaim
			params.ReviewVotePeriod = hrParams.ReviewVotePeriod
			params.EnableHumanReview = hrParams.EnableHumanReview
			params.ReviewQuorumPct = hrParams.ReviewQuorumPct
			params.MinReviewerBond = hrParams.MinReviewerBond
			params.MinReviewerReputation = hrParams.MinReviewerReputation
			params.AppealBond = hrParams.AppealBond
			params.AppealVotePeriod = hrParams.AppealVotePeriod
			params.CollusionThresholdBps = hrParams.CollusionThresholdBps
		}
	}

	// Load economic adjustment params from separate JSON storage
	econBz, err := store.Get([]byte("params_economic"))
	if err == nil && econBz != nil {
		var econParams struct {
			RoyaltyShare                    string                   `json:"royalty_share"`
			ImmediateRewardRatio            string                   `json:"immediate_reward_ratio"`
			VestingEpochs                   int64                    `json:"vesting_epochs"`
			TreasuryAddress                 string                   `json:"treasury_address"`
			TreasuryShareRatio              string                   `json:"treasury_share_ratio"`
			EnableConfigurableBands         bool                     `json:"enable_configurable_bands"`
			OriginalityBands                []types.OriginalityBand  `json:"originality_bands"`
			GrandparentRoyaltyShare         string                   `json:"grandparent_royalty_share"`
			MaxRoyaltyDepth                 uint32                   `json:"max_royalty_depth"`
			MaxTotalRoyaltyShare            string                   `json:"max_total_royalty_share"`
			RepeatOffenderThreshold         uint64                   `json:"repeat_offender_threshold"`
			RepeatOffenderBondEscalationBps uint32                   `json:"repeat_offender_bond_escalation_bps"`
			RepeatOffenderRewardCap         string                   `json:"repeat_offender_reward_cap"`
			RepeatOffenderVestingMultiplier string                   `json:"repeat_offender_vesting_multiplier"`
			EnableAutoClawback              bool                     `json:"enable_auto_clawback"`
		}
		if err := json.Unmarshal(econBz, &econParams); err == nil {
			if dec, err := cosmossdk_io_math.LegacyNewDecFromStr(econParams.RoyaltyShare); err == nil {
				params.RoyaltyShare = dec
			}
			if dec, err := cosmossdk_io_math.LegacyNewDecFromStr(econParams.ImmediateRewardRatio); err == nil {
				params.ImmediateRewardRatio = dec
			}
			params.VestingEpochs = econParams.VestingEpochs
			params.TreasuryAddress = econParams.TreasuryAddress
			if dec, err := cosmossdk_io_math.LegacyNewDecFromStr(econParams.TreasuryShareRatio); err == nil {
				params.TreasuryShareRatio = dec
			}
			// Layer 4 fields
			params.EnableConfigurableBands = econParams.EnableConfigurableBands
			if len(econParams.OriginalityBands) > 0 {
				params.OriginalityBands = econParams.OriginalityBands
			}
			if dec, err := cosmossdk_io_math.LegacyNewDecFromStr(econParams.GrandparentRoyaltyShare); err == nil {
				params.GrandparentRoyaltyShare = dec
			}
			if econParams.MaxRoyaltyDepth > 0 {
				params.MaxRoyaltyDepth = econParams.MaxRoyaltyDepth
			}
			if dec, err := cosmossdk_io_math.LegacyNewDecFromStr(econParams.MaxTotalRoyaltyShare); err == nil {
				params.MaxTotalRoyaltyShare = dec
			}
			params.RepeatOffenderThreshold = econParams.RepeatOffenderThreshold
			params.RepeatOffenderBondEscalationBps = econParams.RepeatOffenderBondEscalationBps
			if dec, err := cosmossdk_io_math.LegacyNewDecFromStr(econParams.RepeatOffenderRewardCap); err == nil {
				params.RepeatOffenderRewardCap = dec
			}
			if dec, err := cosmossdk_io_math.LegacyNewDecFromStr(econParams.RepeatOffenderVestingMultiplier); err == nil {
				params.RepeatOffenderVestingMultiplier = dec
			}
			params.EnableAutoClawback = econParams.EnableAutoClawback
		}
	}

	// Load provenance registry params from separate JSON storage
	provBz, err := store.Get([]byte("params_provenance"))
	if err == nil && provBz != nil {
		var provParams struct {
			MaxProvenanceDepth       uint32 `json:"max_provenance_depth"`
			EnableProvenanceRegistry bool   `json:"enable_provenance_registry"`
			ProvenanceSchemaVersion  uint32 `json:"provenance_schema_version"`
		}
		if err := json.Unmarshal(provBz, &provParams); err == nil {
			params.MaxProvenanceDepth = provParams.MaxProvenanceDepth
			params.EnableProvenanceRegistry = provParams.EnableProvenanceRegistry
			params.ProvenanceSchemaVersion = provParams.ProvenanceSchemaVersion
		}
	}

	// Load ARVS params from separate JSON storage
	arvsBz, err := store.Get([]byte("params_arvs"))
	if err == nil && arvsBz != nil {
		var arvsParams struct {
			EnableARVS                 bool                    `json:"enable_arvs"`
			ARVSWeights                types.ARVSWeights       `json:"arvs_weights"`
			ARVSVestingProfiles        []types.VestingProfile  `json:"arvs_vesting_profiles"`
			ARVSCategoryRiskMapJSON    string                  `json:"arvs_category_risk_map_json"`
			ARVSBountyDistribution     types.BountyDistribution `json:"arvs_bounty_distribution"`
			ARVSEnableBounty           bool                    `json:"arvs_enable_bounty"`
			ARVSTreasuryAddress        string                  `json:"arvs_treasury_address"`
			ARVSRiskScoreLowThreshold  uint32                  `json:"arvs_risk_score_low_threshold"`
			ARVSRiskScoreHighThreshold uint32                  `json:"arvs_risk_score_high_threshold"`
		}
		if err := json.Unmarshal(arvsBz, &arvsParams); err == nil {
			params.EnableARVS = arvsParams.EnableARVS
			params.ARVSWeights = arvsParams.ARVSWeights
			params.ARVSVestingProfiles = arvsParams.ARVSVestingProfiles
			params.ARVSCategoryRiskMapJSON = arvsParams.ARVSCategoryRiskMapJSON
			params.ARVSBountyDistribution = arvsParams.ARVSBountyDistribution
			params.ARVSEnableBounty = arvsParams.ARVSEnableBounty
			params.ARVSTreasuryAddress = arvsParams.ARVSTreasuryAddress
			params.ARVSRiskScoreLowThreshold = arvsParams.ARVSRiskScoreLowThreshold
			params.ARVSRiskScoreHighThreshold = arvsParams.ARVSRiskScoreHighThreshold
		}
	}

	// Load 3-layer fee params from separate JSON storage (workaround for proto field serialization)
	feeBz, err := store.Get([]byte("params_3layer_fee"))
	if err == nil && feeBz != nil {
		var feeParams struct {
			BaseSubmissionFee         string `json:"base_submission_fee"`          // Coin as "amount+denom"
			TargetSubmissionsPerBlock uint32 `json:"target_submissions_per_block"`
			MaxCscoreDiscount         string `json:"max_cscore_discount"`          // LegacyDec as string
			MinimumSubmissionFee      string `json:"minimum_submission_fee"`       // Coin as "amount+denom"
		}
		if err := json.Unmarshal(feeBz, &feeParams); err == nil {
			// Parse BaseSubmissionFee
			if coin, err := types.ParseCoinFromString(feeParams.BaseSubmissionFee); err == nil {
				params.BaseSubmissionFee = coin
			}
			// Parse TargetSubmissionsPerBlock
			params.TargetSubmissionsPerBlock = feeParams.TargetSubmissionsPerBlock
			// Parse MaxCscoreDiscount
			if discount, err := cosmossdk_io_math.LegacyNewDecFromStr(feeParams.MaxCscoreDiscount); err == nil {
				params.MaxCscoreDiscount = discount
			}
			// Parse MinimumSubmissionFee
			if coin, err := types.ParseCoinFromString(feeParams.MinimumSubmissionFee); err == nil {
				params.MinimumSubmissionFee = coin
			}
		}
	}

	return params
}

// SetParams sets the module parameters
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&params)
	if err := store.Set([]byte("params"), bz); err != nil {
		return err
	}

	// Store access control params separately as JSON (workaround for proto map serialization)
	ac := struct {
		EnableCscoreGating      bool              `json:"enable_cscore_gating"`
		MinCscoreForCtype       map[string]string `json:"min_cscore_for_ctype"`
		EnableIdentityGating    bool              `json:"enable_identity_gating"`
		RequireIdentityForCtype map[string]bool   `json:"require_identity_for_ctype"`
		ExemptAddresses         []string          `json:"exempt_addresses"`
	}{
		EnableCscoreGating:      params.EnableCscoreGating,
		EnableIdentityGating:    params.EnableIdentityGating,
		RequireIdentityForCtype: params.RequireIdentityForCtype,
		ExemptAddresses:         params.ExemptAddresses,
		MinCscoreForCtype:       make(map[string]string),
	}

	// Convert math.Int map to string map
	for ctype, score := range params.MinCscoreForCtype {
		ac.MinCscoreForCtype[ctype] = score.String()
	}

	acBz, err := json.Marshal(ac)
	if err != nil {
		return err
	}

	if err := store.Set([]byte("params_access_control"), acBz); err != nil {
		return err
	}

	// Store 3-layer fee params separately as JSON (workaround for proto field serialization)
	feeParams := struct {
		BaseSubmissionFee         string `json:"base_submission_fee"`
		TargetSubmissionsPerBlock uint32 `json:"target_submissions_per_block"`
		MaxCscoreDiscount         string `json:"max_cscore_discount"`
		MinimumSubmissionFee      string `json:"minimum_submission_fee"`
	}{
		BaseSubmissionFee:         params.BaseSubmissionFee.String(),
		TargetSubmissionsPerBlock: params.TargetSubmissionsPerBlock,
		MaxCscoreDiscount:         params.MaxCscoreDiscount.String(),
		MinimumSubmissionFee:      params.MinimumSubmissionFee.String(),
	}

	feeBz, err := json.Marshal(feeParams)
	if err != nil {
		return err
	}

	if err := store.Set([]byte("params_3layer_fee"), feeBz); err != nil {
		return err
	}

	// Store canonical hash layer params separately as JSON
	canonicalParams := struct {
		DuplicateBond              string `json:"duplicate_bond"`
		EnableCanonicalHashCheck   bool   `json:"enable_canonical_hash_check"`
		MaxDuplicatesPerEpoch      uint32 `json:"max_duplicates_per_epoch"`
		DuplicateBondEscalationBps uint32 `json:"duplicate_bond_escalation_bps"`
	}{
		DuplicateBond:              params.DuplicateBond.String(),
		EnableCanonicalHashCheck:   params.EnableCanonicalHashCheck,
		MaxDuplicatesPerEpoch:      params.MaxDuplicatesPerEpoch,
		DuplicateBondEscalationBps: params.DuplicateBondEscalationBps,
	}

	canonicalBz, err := json.Marshal(canonicalParams)
	if err != nil {
		return err
	}

	if err := store.Set([]byte("params_canonical_hash"), canonicalBz); err != nil {
		return err
	}

	// Store human review layer params separately as JSON
	hrParams := struct {
		VerifiersPerClaim     uint32 `json:"verifiers_per_claim"`
		ReviewVotePeriod      int64  `json:"review_vote_period"`
		EnableHumanReview     bool   `json:"enable_human_review"`
		ReviewQuorumPct       uint32 `json:"review_quorum_pct"`
		MinReviewerBond       string `json:"min_reviewer_bond"`
		MinReviewerReputation uint64 `json:"min_reviewer_reputation"`
		AppealBond            string `json:"appeal_bond"`
		AppealVotePeriod      int64  `json:"appeal_vote_period"`
		CollusionThresholdBps uint32 `json:"collusion_threshold_bps"`
	}{
		VerifiersPerClaim:     params.VerifiersPerClaim,
		ReviewVotePeriod:      params.ReviewVotePeriod,
		EnableHumanReview:     params.EnableHumanReview,
		ReviewQuorumPct:       params.ReviewQuorumPct,
		MinReviewerBond:       params.MinReviewerBond,
		MinReviewerReputation: params.MinReviewerReputation,
		AppealBond:            params.AppealBond,
		AppealVotePeriod:      params.AppealVotePeriod,
		CollusionThresholdBps: params.CollusionThresholdBps,
	}

	hrBz, err := json.Marshal(hrParams)
	if err != nil {
		return err
	}

	if err := store.Set([]byte("params_human_review"), hrBz); err != nil {
		return err
	}

	// Store economic adjustment params separately as JSON
	econParams := struct {
		RoyaltyShare                    string                   `json:"royalty_share"`
		ImmediateRewardRatio            string                   `json:"immediate_reward_ratio"`
		VestingEpochs                   int64                    `json:"vesting_epochs"`
		TreasuryAddress                 string                   `json:"treasury_address"`
		TreasuryShareRatio              string                   `json:"treasury_share_ratio"`
		EnableConfigurableBands         bool                     `json:"enable_configurable_bands"`
		OriginalityBands                []types.OriginalityBand  `json:"originality_bands"`
		GrandparentRoyaltyShare         string                   `json:"grandparent_royalty_share"`
		MaxRoyaltyDepth                 uint32                   `json:"max_royalty_depth"`
		MaxTotalRoyaltyShare            string                   `json:"max_total_royalty_share"`
		RepeatOffenderThreshold         uint64                   `json:"repeat_offender_threshold"`
		RepeatOffenderBondEscalationBps uint32                   `json:"repeat_offender_bond_escalation_bps"`
		RepeatOffenderRewardCap         string                   `json:"repeat_offender_reward_cap"`
		RepeatOffenderVestingMultiplier string                   `json:"repeat_offender_vesting_multiplier"`
		EnableAutoClawback              bool                     `json:"enable_auto_clawback"`
	}{
		RoyaltyShare:                    params.RoyaltyShare.String(),
		ImmediateRewardRatio:            params.ImmediateRewardRatio.String(),
		VestingEpochs:                   params.VestingEpochs,
		TreasuryAddress:                 params.TreasuryAddress,
		TreasuryShareRatio:              params.TreasuryShareRatio.String(),
		EnableConfigurableBands:         params.EnableConfigurableBands,
		OriginalityBands:                params.OriginalityBands,
		GrandparentRoyaltyShare:         params.GrandparentRoyaltyShare.String(),
		MaxRoyaltyDepth:                 params.MaxRoyaltyDepth,
		MaxTotalRoyaltyShare:            params.MaxTotalRoyaltyShare.String(),
		RepeatOffenderThreshold:         params.RepeatOffenderThreshold,
		RepeatOffenderBondEscalationBps: params.RepeatOffenderBondEscalationBps,
		RepeatOffenderRewardCap:         params.RepeatOffenderRewardCap.String(),
		RepeatOffenderVestingMultiplier: params.RepeatOffenderVestingMultiplier.String(),
		EnableAutoClawback:              params.EnableAutoClawback,
	}

	econBz, err := json.Marshal(econParams)
	if err != nil {
		return err
	}

	if err := store.Set([]byte("params_economic"), econBz); err != nil {
		return err
	}

	// Store similarity engine params separately as JSON
	simParams := struct {
		SimilarityOracleAllowlist []string `json:"similarity_oracle_allowlist"`
		DerivativeThreshold       uint32   `json:"derivative_threshold"`
		SimilarityEpochBlocks     int64    `json:"similarity_epoch_blocks"`
		EnableSimilarityCheck     bool     `json:"enable_similarity_check"`
	}{
		SimilarityOracleAllowlist: params.SimilarityOracleAllowlist,
		DerivativeThreshold:       params.DerivativeThreshold,
		SimilarityEpochBlocks:     params.SimilarityEpochBlocks,
		EnableSimilarityCheck:     params.EnableSimilarityCheck,
	}

	simBz, err := json.Marshal(simParams)
	if err != nil {
		return err
	}

	if err := store.Set([]byte("params_similarity"), simBz); err != nil {
		return err
	}

	// Store provenance registry params separately as JSON
	provParams := struct {
		MaxProvenanceDepth       uint32 `json:"max_provenance_depth"`
		EnableProvenanceRegistry bool   `json:"enable_provenance_registry"`
		ProvenanceSchemaVersion  uint32 `json:"provenance_schema_version"`
	}{
		MaxProvenanceDepth:       params.MaxProvenanceDepth,
		EnableProvenanceRegistry: params.EnableProvenanceRegistry,
		ProvenanceSchemaVersion:  params.ProvenanceSchemaVersion,
	}

	provBz, err := json.Marshal(provParams)
	if err != nil {
		return err
	}

	if err := store.Set([]byte("params_provenance"), provBz); err != nil {
		return err
	}

	// Store ARVS params separately as JSON (complex nested types not supported by proto marshal)
	arvsParams := struct {
		EnableARVS                 bool                    `json:"enable_arvs"`
		ARVSWeights                types.ARVSWeights       `json:"arvs_weights"`
		ARVSVestingProfiles        []types.VestingProfile  `json:"arvs_vesting_profiles"`
		ARVSCategoryRiskMapJSON    string                  `json:"arvs_category_risk_map_json"`
		ARVSBountyDistribution     types.BountyDistribution `json:"arvs_bounty_distribution"`
		ARVSEnableBounty           bool                    `json:"arvs_enable_bounty"`
		ARVSTreasuryAddress        string                  `json:"arvs_treasury_address"`
		ARVSRiskScoreLowThreshold  uint32                  `json:"arvs_risk_score_low_threshold"`
		ARVSRiskScoreHighThreshold uint32                  `json:"arvs_risk_score_high_threshold"`
	}{
		EnableARVS:                 params.EnableARVS,
		ARVSWeights:                params.ARVSWeights,
		ARVSVestingProfiles:        params.ARVSVestingProfiles,
		ARVSCategoryRiskMapJSON:    params.ARVSCategoryRiskMapJSON,
		ARVSBountyDistribution:     params.ARVSBountyDistribution,
		ARVSEnableBounty:           params.ARVSEnableBounty,
		ARVSTreasuryAddress:        params.ARVSTreasuryAddress,
		ARVSRiskScoreLowThreshold:  params.ARVSRiskScoreLowThreshold,
		ARVSRiskScoreHighThreshold: params.ARVSRiskScoreHighThreshold,
	}

	arvsBz, err := json.Marshal(arvsParams)
	if err != nil {
		return err
	}
	return store.Set([]byte("params_arvs"), arvsBz)
}
