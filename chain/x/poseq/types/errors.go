package types

import "cosmossdk.io/errors"

var (
	ErrDuplicateEvidencePacket   = errors.Register(ModuleName, 1, "evidence packet already submitted")
	ErrDuplicateEscalation       = errors.Register(ModuleName, 2, "escalation record already submitted")
	ErrDuplicateCheckpointAnchor = errors.Register(ModuleName, 3, "checkpoint anchor already exists for this epoch/slot")
	ErrInvalidPacketHash         = errors.Register(ModuleName, 4, "evidence packet hash is invalid or zero")
	ErrInvalidEscalationID       = errors.Register(ModuleName, 5, "escalation ID is invalid or zero")
	ErrInvalidCheckpointID       = errors.Register(ModuleName, 6, "checkpoint ID is invalid or zero")
	ErrInvalidEpoch              = errors.Register(ModuleName, 7, "epoch must be > 0")
	ErrInvalidExportBatch        = errors.Register(ModuleName, 8, "export batch is malformed or missing required fields")
	ErrUnauthorized              = errors.Register(ModuleName, 9, "sender is not authorized to submit PoSeq accountability records")
	ErrCheckpointAnchorTampered  = errors.Register(ModuleName, 10, "checkpoint anchor hash verification failed")

	// Sequencer registration errors
	ErrInvalidNodeID            = errors.Register(ModuleName, 11, "node_id is invalid (must be 32-byte hex)")
	ErrInvalidPublicKey         = errors.Register(ModuleName, 12, "public_key is invalid (must be 32-byte Ed25519 hex)")
	ErrInvalidMoniker           = errors.Register(ModuleName, 13, "moniker is empty or exceeds 64 characters")
	ErrSequencerAlreadyExists   = errors.Register(ModuleName, 14, "a sequencer with this node_id is already registered")
	ErrSequencerNotFound        = errors.Register(ModuleName, 15, "sequencer not found for the given node_id")
	ErrOperatorMismatch         = errors.Register(ModuleName, 16, "sender is not the operator who registered this sequencer")

	// Settlement errors
	ErrInvalidBatchID          = errors.Register(ModuleName, 17, "batch_id is invalid (must be 32-byte hex)")
	ErrBatchAlreadyCommitted   = errors.Register(ModuleName, 18, "batch has already been committed on-chain")
	ErrInvalidFinalizationHash = errors.Register(ModuleName, 19, "finalization_hash is invalid or does not match batch commitment")

	// Lifecycle FSM errors
	ErrInvalidLifecycleTransition = errors.Register(ModuleName, 20, "lifecycle transition is not permitted for current status")

	// Committee snapshot errors
	ErrSnapshotHashMismatch = errors.Register(ModuleName, 22, "committee snapshot hash verification failed")
	ErrDuplicateSnapshot    = errors.Register(ModuleName, 23, "committee snapshot already exists for this epoch")
	ErrInvalidSnapshotEpoch = errors.Register(ModuleName, 24, "snapshot epoch is zero or invalid")

	// Liveness / performance errors
	ErrLivenessEventInvalid     = errors.Register(ModuleName, 25, "liveness event is malformed")
	ErrPerformanceRecordInvalid = errors.Register(ModuleName, 26, "performance record is malformed or node not found")

	// Bond and slash queue errors (Phase 5)
	ErrBondAlreadyExists    = errors.Register(ModuleName, 27, "operator bond already declared")
	ErrBondNotFound         = errors.Register(ModuleName, 28, "operator bond not found")
	ErrBondAlreadyWithdrawn = errors.Register(ModuleName, 29, "operator bond already withdrawn")
	ErrInvalidBondAmount    = errors.Register(ModuleName, 30, "bond amount must be > 0")
	ErrSlashQueueFull       = errors.Register(ModuleName, 31, "slash queue is at capacity")
	ErrNodeNotBonded        = errors.Register(ModuleName, 32, "no active bond found for node")

	// Phase 6 — Slashing enforcement errors
	ErrDoubleSlash              = errors.Register(ModuleName, 33, "slash already executed for this evidence packet")
	ErrStaleEvidence            = errors.Register(ModuleName, 34, "evidence packet is from an epoch too far in the past")
	ErrInsufficientBondForSlash = errors.Register(ModuleName, 35, "bond is exhausted — no remaining amount to slash")
	ErrAdjudicationConflict     = errors.Register(ModuleName, 36, "conflicting adjudication record already exists")
	ErrInvalidSlashBps          = errors.Register(ModuleName, 37, "slash_bps must be between 1 and 10000")
	ErrBondJailed               = errors.Register(ModuleName, 38, "bond is in Jailed state — cannot withdraw")
	ErrBondExhausted            = errors.Register(ModuleName, 39, "bond is exhausted and cannot be slashed further")

	// Bridge ACK loop errors
	ErrDuplicateExportBatch = errors.Register(ModuleName, 40, "export batch for this epoch has already been ingested")
)
