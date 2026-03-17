package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"pos/x/poseq/types"
)

// ─── SequencerRecord store ────────────────────────────────────────────────────

// RegisterSequencer stores a new SequencerRecord. Returns ErrSequencerAlreadyExists
// if a record with the same NodeID is already present.
func (k Keeper) RegisterSequencer(ctx context.Context, rec types.SequencerRecord) error {
	nodeIDBytes, err := hex.DecodeString(rec.NodeID)
	if err != nil || len(nodeIDBytes) != 32 {
		return types.ErrInvalidNodeID
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetSequencerKey(nodeIDBytes)
	existing, err := kvStore.Get(key)
	if err != nil {
		return err
	}
	if existing != nil {
		return types.ErrSequencerAlreadyExists
	}
	bz, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return kvStore.Set(key, bz)
}

// GetSequencer retrieves a SequencerRecord by its hex NodeID.
// Returns (nil, nil) if not found.
func (k Keeper) GetSequencer(ctx context.Context, nodeIDHex string) (*types.SequencerRecord, error) {
	nodeIDBytes, err := hex.DecodeString(nodeIDHex)
	if err != nil || len(nodeIDBytes) != 32 {
		return nil, types.ErrInvalidNodeID
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetSequencerKey(nodeIDBytes))
	if err != nil || bz == nil {
		return nil, err
	}
	var rec types.SequencerRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// SetSequencerActive sets IsActive for a sequencer record.
// Returns ErrSequencerNotFound if the node is not registered.
func (k Keeper) SetSequencerActive(ctx context.Context, nodeIDHex string, active bool) error {
	nodeIDBytes, err := hex.DecodeString(nodeIDHex)
	if err != nil || len(nodeIDBytes) != 32 {
		return types.ErrInvalidNodeID
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetSequencerKey(nodeIDBytes)
	bz, err := kvStore.Get(key)
	if err != nil {
		return err
	}
	if bz == nil {
		return types.ErrSequencerNotFound
	}
	var rec types.SequencerRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return err
	}
	rec.IsActive = active
	updated, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return kvStore.Set(key, updated)
}

// IsSequencerActive returns true if the node_id is registered and IsActive == true.
func (k Keeper) IsSequencerActive(ctx context.Context, nodeIDHex string) (bool, error) {
	rec, err := k.GetSequencer(ctx, nodeIDHex)
	if err != nil {
		return false, err
	}
	if rec == nil {
		return false, nil
	}
	return rec.IsActive, nil
}

// ─── Message handlers ─────────────────────────────────────────────────────────

// HandleRegisterSequencer processes MsgRegisterSequencer.
// Registration is permissionless; activation requires governance.
func (m MsgServer) RegisterSequencer(ctx context.Context, msg *types.MsgRegisterSequencer) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// Derive operator address from sender — sender IS the operator
	rec := types.SequencerRecord{
		NodeID:          msg.NodeID,
		PublicKey:       msg.PublicKey,
		Moniker:         msg.Moniker,
		OperatorAddress: msg.Sender,
		RegisteredEpoch: msg.Epoch,
		IsActive:        false, // activation requires governance
	}

	if err := m.Keeper.RegisterSequencer(ctx, rec); err != nil {
		return fmt.Errorf("registering sequencer: %w", err)
	}

	k := m.Keeper
	k.Logger().Info("sequencer registered",
		"node_id", msg.NodeID,
		"operator", msg.Sender,
		"moniker", msg.Moniker,
		"epoch", msg.Epoch,
	)
	return nil
}

// HandleActivateSequencer processes MsgActivateSequencer.
// Must be called by the governance authority address.
func (m MsgServer) ActivateSequencer(ctx context.Context, msg *types.MsgActivateSequencer) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	if msg.Authority != m.Keeper.Authority() {
		return types.ErrUnauthorized.Wrapf(
			"expected authority %s, got %s",
			m.Keeper.Authority(), msg.Authority,
		)
	}
	if err := m.Keeper.SetSequencerActive(ctx, msg.NodeID, true); err != nil {
		return fmt.Errorf("activating sequencer: %w", err)
	}
	m.Keeper.Logger().Info("sequencer activated", "node_id", msg.NodeID)
	return nil
}

// HandleDeactivateSequencer processes MsgDeactivateSequencer.
// Allowed by the registered operator (self) or governance authority.
func (m MsgServer) DeactivateSequencer(ctx context.Context, msg *types.MsgDeactivateSequencer) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	// Look up record to check operator
	rec, err := m.Keeper.GetSequencer(ctx, msg.NodeID)
	if err != nil {
		return err
	}
	if rec == nil {
		return types.ErrSequencerNotFound
	}

	// Caller must be the registered operator or governance authority
	if msg.Sender != rec.OperatorAddress && msg.Sender != m.Keeper.Authority() {
		return types.ErrOperatorMismatch.Wrapf(
			"sender %s is neither the operator (%s) nor the governance authority",
			msg.Sender, rec.OperatorAddress,
		)
	}

	if err := m.Keeper.SetSequencerActive(ctx, msg.NodeID, false); err != nil {
		return fmt.Errorf("deactivating sequencer: %w", err)
	}
	m.Keeper.Logger().Info("sequencer deactivated",
		"node_id", msg.NodeID,
		"reason", msg.Reason,
	)
	return nil
}
