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

	return store.Set([]byte("params_3layer_fee"), feeBz)
}
