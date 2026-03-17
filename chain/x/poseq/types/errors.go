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
	ErrInvalidBatchID           = errors.Register(ModuleName, 17, "batch_id is invalid (must be 32-byte hex)")
	ErrBatchAlreadyCommitted    = errors.Register(ModuleName, 18, "batch has already been committed on-chain")
	ErrInvalidFinalizationHash  = errors.Register(ModuleName, 19, "finalization_hash is invalid or does not match batch commitment")
)
