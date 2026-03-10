package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"pos/x/por/types"
)

// CreateVerifierSet handles MsgCreateVerifierSet - creates a new epoch-scoped verifier set for an application
func (ms msgServer) CreateVerifierSet(goCtx context.Context, msg *types.MsgCreateVerifierSet) (*types.MsgCreateVerifierSetResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate the message
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	// Verify app exists and is active
	app, found := ms.GetApp(goCtx, msg.AppId)
	if !found {
		return nil, types.ErrAppNotFound.Wrapf("app_id: %d", msg.AppId)
	}
	if app.Status != types.AppStatusActive {
		return nil, types.ErrAppNotActive.Wrapf("app_id: %d, status: %s", msg.AppId, app.Status)
	}

	// Only the app owner can create verifier sets
	if app.Owner != msg.Creator {
		return nil, types.ErrNotAppOwner.Wrapf("app owner: %s, signer: %s", app.Owner, msg.Creator)
	}

	// Enforce global min verifiers
	params := ms.GetParams(goCtx)
	if uint32(len(msg.Members)) < params.MinVerifiersGlobal {
		return nil, types.ErrInvalidMinVerifiers.Wrapf(
			"verifier set has %d members, minimum is %d", len(msg.Members), params.MinVerifiersGlobal,
		)
	}
	if uint32(len(msg.Members)) > params.MaxVerifiersPerSet {
		return nil, types.ErrTooManyVerifiers.Wrapf(
			"verifier set has %d members, maximum is %d", len(msg.Members), params.MaxVerifiersPerSet,
		)
	}

	// SECURITY (F1): Validate each member is a bonded validator and bind weight to real stake.
	// Self-declared weights are overridden with actual bonded tokens from the staking module.
	for i, member := range msg.Members {
		accAddr, err := sdk.AccAddressFromBech32(member.Address)
		if err != nil {
			return nil, types.ErrInsufficientStake.Wrapf("invalid member address: %s", member.Address)
		}
		valAddr := sdk.ValAddress(accAddr)

		validator, err := ms.stakingKeeper.GetValidator(goCtx, valAddr)
		if err != nil {
			return nil, types.ErrNotBondedValidator.Wrapf(
				"member %s is not a registered validator", member.Address,
			)
		}

		if validator.GetStatus() != stakingtypes.Bonded {
			return nil, types.ErrNotBondedValidator.Wrapf(
				"member %s validator status is %s, must be Bonded",
				member.Address, validator.GetStatus(),
			)
		}

		bondedTokens := validator.GetBondedTokens()
		if bondedTokens.LT(params.MinStakeForVerifier) {
			return nil, types.ErrInsufficientStake.Wrapf(
				"member %s has bonded tokens %s, minimum is %s",
				member.Address, bondedTokens, params.MinStakeForVerifier,
			)
		}

		// Override self-declared weight with actual bonded tokens
		msg.Members[i].Weight = bondedTokens
	}

	// MinAttestations must not exceed global min verifiers constraint
	if msg.MinAttestations < params.MinVerifiersGlobal {
		return nil, types.ErrInvalidMinVerifiers.Wrapf(
			"min_attestations %d is below global minimum %d", msg.MinAttestations, params.MinVerifiersGlobal,
		)
	}

	// Get next verifier set ID
	vsID, err := ms.GetNextVerifierSetID(goCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next verifier set ID: %w", err)
	}

	// Set JoinedAt timestamp for all members
	now := ctx.BlockTime().Unix()
	members := make([]types.VerifierMember, len(msg.Members))
	for i, m := range msg.Members {
		members[i] = types.NewVerifierMember(m.Address, m.Weight, now)
	}

	// Create the verifier set
	vs := types.NewVerifierSet(
		vsID,
		msg.Epoch,
		members,
		msg.MinAttestations,
		msg.QuorumPct,
		msg.AppId,
	)

	// Store the verifier set
	if err := ms.SetVerifierSet(goCtx, vs); err != nil {
		return nil, fmt.Errorf("failed to store verifier set: %w", err)
	}

	ms.Logger().Info("verifier set created",
		"verifier_set_id", vsID,
		"app_id", msg.AppId,
		"epoch", msg.Epoch,
		"member_count", len(members),
	)

	// Emit events
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			"por_create_verifier_set",
			sdk.NewAttribute("verifier_set_id", fmt.Sprintf("%d", vsID)),
			sdk.NewAttribute("app_id", fmt.Sprintf("%d", msg.AppId)),
			sdk.NewAttribute("epoch", fmt.Sprintf("%d", msg.Epoch)),
			sdk.NewAttribute("member_count", fmt.Sprintf("%d", len(members))),
			sdk.NewAttribute("min_attestations", fmt.Sprintf("%d", msg.MinAttestations)),
			sdk.NewAttribute("quorum_pct", msg.QuorumPct.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute(sdk.AttributeKeySender, msg.Creator),
		),
	})

	return &types.MsgCreateVerifierSetResponse{VerifierSetId: vsID}, nil
}
