package types

import errorsmod "cosmossdk.io/errors"

// x/por module sentinel errors
var (
	ErrAppNotFound             = errorsmod.Register(ModuleName, 1, "app not found")
	ErrAppNotActive            = errorsmod.Register(ModuleName, 2, "app is not active")
	ErrBatchNotFound           = errorsmod.Register(ModuleName, 3, "batch not found")
	ErrVerifierSetNotFound     = errorsmod.Register(ModuleName, 4, "verifier set not found")
	ErrNotVerifier             = errorsmod.Register(ModuleName, 5, "signer is not a member of the verifier set")
	ErrAlreadyAttested         = errorsmod.Register(ModuleName, 6, "verifier already attested this batch")
	ErrChallengeWindowClosed   = errorsmod.Register(ModuleName, 7, "challenge window has closed")
	ErrChallengeWindowOpen     = errorsmod.Register(ModuleName, 8, "challenge window is still open")
	ErrBatchAlreadyFinalized   = errorsmod.Register(ModuleName, 9, "batch already finalized")
	ErrBatchRejected           = errorsmod.Register(ModuleName, 10, "batch has been rejected")
	ErrInvalidMerkleRoot       = errorsmod.Register(ModuleName, 11, "invalid merkle root")
	ErrInvalidProofData        = errorsmod.Register(ModuleName, 12, "invalid proof data")
	ErrInvalidChallengePeriod  = errorsmod.Register(ModuleName, 13, "invalid challenge period")
	ErrInvalidMinVerifiers     = errorsmod.Register(ModuleName, 14, "invalid minimum verifiers")
	ErrInvalidQuorumPct        = errorsmod.Register(ModuleName, 15, "invalid quorum percentage")
	ErrNotAppOwner             = errorsmod.Register(ModuleName, 16, "signer is not the app owner")
	ErrDuplicateAppName        = errorsmod.Register(ModuleName, 17, "app with this name already exists")
	ErrBatchNotPending         = errorsmod.Register(ModuleName, 18, "batch is not in pending status")
	ErrInvalidRecordCount      = errorsmod.Register(ModuleName, 19, "invalid record count")
	ErrInvalidSignature        = errorsmod.Register(ModuleName, 20, "invalid attestation signature")
	ErrRateLimitExceeded       = errorsmod.Register(ModuleName, 21, "batch submission rate limit exceeded")
	ErrInvalidAppName          = errorsmod.Register(ModuleName, 22, "invalid app name")
	ErrInvalidSchemaCid        = errorsmod.Register(ModuleName, 23, "invalid schema CID")
	ErrChallengeNotFound       = errorsmod.Register(ModuleName, 24, "challenge not found")
	ErrInvalidConfidenceWeight = errorsmod.Register(ModuleName, 25, "invalid confidence weight")
	ErrEpochMismatch           = errorsmod.Register(ModuleName, 26, "epoch mismatch")
	ErrVerifierSetMismatch     = errorsmod.Register(ModuleName, 27, "verifier set does not belong to app")
	ErrInsufficientStake       = errorsmod.Register(ModuleName, 28, "insufficient stake for verifier")
	ErrOpenChallengesExist     = errorsmod.Register(ModuleName, 29, "cannot finalize batch with open challenges")
	ErrBatchNotSubmitted       = errorsmod.Register(ModuleName, 30, "batch is not in submitted or pending status")
	ErrInvalidAuthority        = errorsmod.Register(ModuleName, 31, "invalid authority")
	ErrDuplicateVerifier       = errorsmod.Register(ModuleName, 32, "duplicate verifier in set")
	ErrTooManyVerifiers        = errorsmod.Register(ModuleName, 33, "verifier set exceeds maximum size")
	ErrInsufficientReputation  = errorsmod.Register(ModuleName, 34, "insufficient reputation to attest")
	ErrProofVerificationFailed = errorsmod.Register(ModuleName, 35, "fraud proof verification failed")

	// F1: Verifier stake binding
	ErrNotBondedValidator = errorsmod.Register(ModuleName, 36, "member is not a bonded validator")

	// F2/F6: Credit caps & DA enforcement
	ErrEpochCreditCapExceeded = errorsmod.Register(ModuleName, 37, "epoch credit cap exceeded")
	ErrDACommitmentRequired   = errorsmod.Register(ModuleName, 38, "DA commitment hash required")

	// F4: Challenge bond & rate limiting
	ErrInsufficientChallengeBond  = errorsmod.Register(ModuleName, 39, "insufficient challenge bond")
	ErrChallengeRateLimitExceeded = errorsmod.Register(ModuleName, 40, "challenge rate limit exceeded for this address")

	// F8: PoSeq commitment verification
	ErrPoSeqCommitmentNotFound = errorsmod.Register(ModuleName, 41, "PoSeq commitment not registered")
	ErrNotAuthorizedSequencer  = errorsmod.Register(ModuleName, 42, "not an authorized PoSeq sequencer")
)
