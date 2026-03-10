package keeper

import (
	"context"
	"crypto/sha256"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// RegisterPoSeqCommitment registers a PoSeq state root commitment on-chain (F8).
// Only authorized sequencers (from the PoSeq sequencer set) may register commitments.
func (ms msgServer) RegisterPoSeqCommitment(goCtx context.Context, msg *types.MsgRegisterPoSeqCommitment) (*types.MsgRegisterPoSeqCommitmentResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate commitment hash
	if len(msg.CommitmentHash) != 32 {
		return nil, fmt.Errorf("commitment hash must be exactly 32 bytes, got %d", len(msg.CommitmentHash))
	}
	if len(msg.StateRoot) != 32 {
		return nil, fmt.Errorf("state root must be exactly 32 bytes, got %d", len(msg.StateRoot))
	}

	// Verify the commitment hash matches SHA256(state_root)
	expectedHash := sha256.Sum256(msg.StateRoot)
	if !bytesEqual(expectedHash[:], msg.CommitmentHash) {
		return nil, fmt.Errorf("commitment hash does not match SHA256(state_root)")
	}

	// Check authorization: must be governance authority OR an authorized sequencer
	isAuthority := msg.Authority == ms.GetAuthority()
	isSequencer := false

	if !isAuthority {
		seqSet, found := ms.GetPoSeqSequencerSet(goCtx)
		if found {
			isSequencer = seqSet.IsAuthorizedSequencer(msg.Authority)
		}
	}

	if !isAuthority && !isSequencer {
		return nil, types.ErrNotAuthorizedSequencer.Wrapf(
			"address %s is not governance authority or authorized sequencer", msg.Authority,
		)
	}

	// Check for duplicate commitment
	_, exists := ms.GetPoSeqCommitment(goCtx, msg.CommitmentHash)
	if exists {
		return nil, fmt.Errorf("PoSeq commitment already registered")
	}

	now := ctx.BlockTime().Unix()
	commitment := types.PoSeqCommitment{
		CommitmentHash: msg.CommitmentHash,
		StateRoot:      msg.StateRoot,
		SequencerAddr:  msg.Authority,
		BlockHeight:    msg.BlockHeight,
		Timestamp:      now,
	}

	if err := ms.SetPoSeqCommitment(goCtx, commitment); err != nil {
		return nil, fmt.Errorf("failed to store PoSeq commitment: %w", err)
	}

	ms.Logger().Info("PoSeq commitment registered",
		"commitment_hash_len", len(msg.CommitmentHash),
		"block_height", msg.BlockHeight,
		"sequencer", msg.Authority,
	)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"por_poseq_commitment_registered",
		sdk.NewAttribute("block_height", fmt.Sprintf("%d", msg.BlockHeight)),
		sdk.NewAttribute("sequencer", msg.Authority),
	))

	return &types.MsgRegisterPoSeqCommitmentResponse{}, nil
}

// UpdatePoSeqSequencerSet updates the authorized sequencer set (F8, governance only).
func (ms msgServer) UpdatePoSeqSequencerSet(goCtx context.Context, msg *types.MsgUpdatePoSeqSequencerSet) (*types.MsgUpdatePoSeqSequencerSetResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Only governance authority can update the sequencer set
	if msg.Authority != ms.GetAuthority() {
		return nil, types.ErrInvalidAuthority.Wrapf(
			"expected %s, got %s", ms.GetAuthority(), msg.Authority,
		)
	}

	// Validate sequencer addresses
	for _, addr := range msg.Sequencers {
		if _, err := sdk.AccAddressFromBech32(addr); err != nil {
			return nil, fmt.Errorf("invalid sequencer address %s: %w", addr, err)
		}
	}

	if msg.Threshold == 0 {
		return nil, fmt.Errorf("threshold must be greater than 0")
	}

	set := types.PoSeqSequencerSet{
		Sequencers: msg.Sequencers,
		Threshold:  msg.Threshold,
	}

	if err := ms.SetPoSeqSequencerSet(goCtx, set); err != nil {
		return nil, fmt.Errorf("failed to store PoSeq sequencer set: %w", err)
	}

	ms.Logger().Info("PoSeq sequencer set updated",
		"sequencer_count", len(msg.Sequencers),
		"threshold", msg.Threshold,
	)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"por_poseq_sequencer_set_updated",
		sdk.NewAttribute("sequencer_count", fmt.Sprintf("%d", len(msg.Sequencers))),
		sdk.NewAttribute("threshold", fmt.Sprintf("%d", msg.Threshold)),
	))

	return &types.MsgUpdatePoSeqSequencerSetResponse{}, nil
}
