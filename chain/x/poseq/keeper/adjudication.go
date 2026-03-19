package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"pos/x/poseq/types"
)

// ─── AdjudicationRecord store ─────────────────────────────────────────────────

// StoreAdjudicationRecord stores an adjudication record keyed by packet_hash.
//
// Write semantics:
//   - Pending records can be updated (Pending → Penalized / Dismissed / Escalated).
//   - Finalized records (non-Pending) are immutable: returns ErrAdjudicationConflict
//     if a second finalized record is written for the same packet hash.
func (k Keeper) StoreAdjudicationRecord(ctx context.Context, rec types.AdjudicationRecord) error {
	pktHashBytes, err := hex.DecodeString(rec.PacketHash)
	if err != nil || len(pktHashBytes) != 32 {
		return types.ErrInvalidPacketHash.Wrap("adjudication: packet_hash must be 64 hex chars")
	}

	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.GetAdjudicationKey(pktHashBytes)

	existing, err := kvStore.Get(key)
	if err != nil {
		return err
	}
	if existing != nil {
		var existingRec types.AdjudicationRecord
		if jsonErr := json.Unmarshal(existing, &existingRec); jsonErr == nil {
			if existingRec.Decision != types.AdjudicationDecisionPending {
				// Finalized records are immutable
				return types.ErrAdjudicationConflict.Wrapf(
					"adjudication already finalized as %s for packet %s",
					existingRec.Decision, rec.PacketHash,
				)
			}
		}
	}

	bz, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return kvStore.Set(key, bz)
}

// GetAdjudicationRecord retrieves an adjudication record by packet hash (hex).
// Returns (nil, nil) if not found.
func (k Keeper) GetAdjudicationRecord(ctx context.Context, packetHashHex string) (*types.AdjudicationRecord, error) {
	pktHashBytes, err := hex.DecodeString(packetHashHex)
	if err != nil || len(pktHashBytes) != 32 {
		return nil, types.ErrInvalidPacketHash
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetAdjudicationKey(pktHashBytes))
	if err != nil || bz == nil {
		return nil, err
	}
	var rec types.AdjudicationRecord
	if err := json.Unmarshal(bz, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// ─── Bond slash execution ─────────────────────────────────────────────────────

// ApplyBondSlash reduces the AvailableBond for the node identified by nodeIDHex
// by slash_bps percent of the current BondAmount.
//
// Returns the absolute amount slashed (may be 0 if bond already exhausted).
//
// Error cases:
//   - ErrNodeNotBonded: no active bond found.
//   - ErrBondExhausted: AvailableBond is already 0.
//
// BondState transitions:
//
//	Active           → PartiallySlashed (partial slash, available > 0)
//	Active           → Jailed (full slash or jailed by caller)
//	PartiallySlashed → Jailed (further slash or jailed by caller)
//	Jailed           → Exhausted (available reaches 0)
//	PartiallySlashed → Exhausted (available reaches 0)
func (k Keeper) ApplyBondSlash(ctx context.Context, nodeIDHex string, slashBps uint32, epoch uint64) (uint64, error) {
	bond, err := k.GetActiveBondForNode(ctx, nodeIDHex)
	if err != nil {
		return 0, err
	}
	if bond == nil {
		return 0, types.ErrNodeNotBonded
	}

	// Guard: if AvailableBond was never initialized (old bond), set from BondAmount
	if bond.AvailableBond == 0 && bond.SlashedAmount == 0 {
		bond.AvailableBond = bond.BondAmount
	}

	if bond.AvailableBond == 0 {
		return 0, types.ErrBondExhausted
	}

	// Compute slash amount: slashBps of the original BondAmount, capped at AvailableBond.
	// Using BondAmount as the base ensures consistent slash fractions across multiple slashes.
	slashAmount := uint64(slashBps) * bond.BondAmount / 10_000
	if slashAmount == 0 {
		slashAmount = 1 // minimum 1 unit to avoid no-op slashes
	}
	if slashAmount > bond.AvailableBond {
		slashAmount = bond.AvailableBond
	}

	bond.AvailableBond -= slashAmount
	bond.SlashedAmount += slashAmount
	bond.LastSlashEpoch = epoch
	bond.SlashCount++

	// Update bond state.
	// Treat empty string as Active (zero-value for bonds declared before Phase 6).
	if bond.State == "" {
		bond.State = types.BondStateActive
	}
	if bond.AvailableBond == 0 {
		bond.State = types.BondStateExhausted
	} else if bond.State == types.BondStateActive {
		bond.State = types.BondStatePartiallySlashed
	}
	// PartiallySlashed or Jailed: keep state (further slashes don't downgrade to PartiallySlashed)

	// Persist updated bond via primary key
	bz, err := json.Marshal(bond)
	if err != nil {
		return 0, err
	}
	primaryKey := types.GetOperatorBondKey(bond.OperatorAddress, bond.NodeID)
	kvStore := k.storeService.OpenKVStore(ctx)
	if err := kvStore.Set(primaryKey, bz); err != nil {
		return 0, err
	}

	k.Logger().Info("bond slashed",
		"node_id", nodeIDHex,
		"slash_bps", slashBps,
		"slash_amount", slashAmount,
		"available_bond", bond.AvailableBond,
		"bond_state", bond.State,
		"epoch", epoch,
	)
	return slashAmount, nil
}

// ─── Automatic adjudication ───────────────────────────────────────────────────

// AdjudicateEvidence automatically adjudicates an evidence packet if it qualifies
// for automatic processing. Called during IngestExportBatch for Critical evidence.
//
// Auto-adjudication rules:
//   - Minor    severity → Automatic path, Dismissed (Informational, no slash)
//   - Moderate severity → Automatic path, Penalized (slash_bps applied)
//   - Severe / Critical → GovernanceReview path, Escalated (no immediate slash)
//
// Returns the AdjudicationRecord written. Returns nil if the packet was already adjudicated.
func (k Keeper) AdjudicateEvidence(ctx context.Context, pkt types.EvidencePacket, epoch uint64) (*types.AdjudicationRecord, error) {
	packetHashHex := fmt.Sprintf("%x", pkt.PacketHash)

	// Idempotency: skip if already adjudicated
	existing, err := k.GetAdjudicationRecord(ctx, packetHashHex)
	if err != nil {
		return nil, fmt.Errorf("checking existing adjudication: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	severity := string(pkt.Severity)
	slashBps := types.SlashBpsForSeverity(severity)

	var path types.AdjudicationPath
	var decision types.AdjudicationDecision

	switch severity {
	case "Minor":
		// Informational — no slash, no governance escalation.
		path = types.AdjudicationPathAutomatic
		decision = types.AdjudicationDecisionDismissed
	case "Moderate":
		path = types.AdjudicationPathAutomatic
		decision = types.AdjudicationDecisionPenalized
	default:
		// Severe, Critical → governance review
		path = types.AdjudicationPathGovernanceReview
		decision = types.AdjudicationDecisionEscalated
	}

	nodeIDHex := fmt.Sprintf("%x", pkt.OffenderNodeID)

	rec := types.AdjudicationRecord{
		PacketHash:      packetHashHex,
		NodeID:          nodeIDHex,
		MisbehaviorType: string(pkt.Kind),
		Epoch:           pkt.Epoch,
		Path:            path,
		Decision:        decision,
		SlashBps:        slashBps,
		DecidedAtEpoch:  epoch,
		Reason:          fmt.Sprintf("auto-adjudicated: %s severity %s", pkt.Kind, severity),
		AutoApplied:     path == types.AdjudicationPathAutomatic,
	}

	if err := k.StoreAdjudicationRecord(ctx, rec); err != nil {
		return nil, fmt.Errorf("storing adjudication record: %w", err)
	}

	// For auto-adjudicated penalizable events, enqueue a slash entry
	if path == types.AdjudicationPathAutomatic && slashBps > 0 {
		// Look up operator bond
		bond, bondErr := k.GetActiveBondForNode(ctx, nodeIDHex)
		operatorAddr := ""
		if bondErr == nil && bond != nil {
			operatorAddr = bond.OperatorAddress
		}
		entryID := computeSlashEntryID(operatorAddr, nodeIDHex, epoch)
		entry := types.SlashQueueEntry{
			EntryID:         entryID,
			OperatorAddress: operatorAddr,
			NodeID:          nodeIDHex,
			EvidenceRef:     pkt.PacketHash,
			Severity:        severity,
			SlashBps:        slashBps,
			Epoch:           epoch,
			Reason:          rec.Reason,
			Executed:        false,
		}
		if qErr := k.EnqueueSlashEntry(ctx, entry); qErr != nil {
			k.Logger().Warn("failed to enqueue auto-adjudicated slash entry",
				"node_id", nodeIDHex,
				"error", qErr.Error(),
			)
		}
	}

	return &rec, nil
}
