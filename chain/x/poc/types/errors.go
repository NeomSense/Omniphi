package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/poc module sentinel errors
var (
	ErrInvalidCType          = errorsmod.Register(ModuleName, 1, "invalid contribution type")
	ErrInvalidURI            = errorsmod.Register(ModuleName, 2, "invalid URI")
	ErrInvalidHash           = errorsmod.Register(ModuleName, 3, "invalid hash")
	ErrContributionNotFound  = errorsmod.Register(ModuleName, 4, "contribution not found")
	ErrNotValidator          = errorsmod.Register(ModuleName, 5, "signer is not a validator")
	ErrZeroPower             = errorsmod.Register(ModuleName, 6, "validator has zero power")
	ErrAlreadyEndorsed       = errorsmod.Register(ModuleName, 7, "validator already endorsed this contribution")
	ErrNoCredits             = errorsmod.Register(ModuleName, 8, "no credits available to withdraw")
	ErrRateLimitExceeded     = errorsmod.Register(ModuleName, 9, "submission rate limit exceeded")
	ErrInvalidQuorumPct      = errorsmod.Register(ModuleName, 10, "invalid quorum percentage")
	ErrInvalidRewardUnit     = errorsmod.Register(ModuleName, 11, "invalid reward unit")
	ErrInvalidInflationShare = errorsmod.Register(ModuleName, 12, "invalid inflation share")
	ErrInvalidContributor    = errorsmod.Register(ModuleName, 13, "invalid contributor address")
	ErrInvalidSubmissionFee  = errorsmod.Register(ModuleName, 14, "invalid submission fee")
	ErrInvalidBurnRatio      = errorsmod.Register(ModuleName, 15, "invalid burn ratio")
	ErrFeeBelowMinimum       = errorsmod.Register(ModuleName, 16, "submission fee below minimum")
	ErrFeeAboveMaximum       = errorsmod.Register(ModuleName, 17, "submission fee above maximum")
	ErrBurnRatioBelowMinimum = errorsmod.Register(ModuleName, 18, "burn ratio below minimum")
	ErrBurnRatioAboveMaximum = errorsmod.Register(ModuleName, 19, "burn ratio above maximum")
	ErrInsufficientFee       = errorsmod.Register(ModuleName, 20, "insufficient balance to pay submission fee")
	ErrInsufficientCScore    = errorsmod.Register(ModuleName, 21, "insufficient C-Score for contribution type")
	ErrIdentityNotVerified   = errorsmod.Register(ModuleName, 22, "identity verification required for contribution type")
	ErrIdentityCheckFailed   = errorsmod.Register(ModuleName, 23, "identity verification check failed")
	ErrCTypeNotAllowed       = errorsmod.Register(ModuleName, 24, "contribution type not allowed for this contributor")

	// ============================================================================
	// PoC Hardening Upgrade Errors (v2)
	// ============================================================================

	// Finality and PoR integration errors
	ErrContributionNotFinalized = errorsmod.Register(ModuleName, 25, "contribution has not reached finality")
	ErrContributionChallenged   = errorsmod.Register(ModuleName, 26, "contribution is under challenge")
	ErrContributionInvalidated  = errorsmod.Register(ModuleName, 27, "contribution was invalidated by fraud proof")
	ErrFinalityProviderError    = errorsmod.Register(ModuleName, 28, "finality provider returned error")

	// Credit hardening errors
	ErrEpochCreditCapExceeded = errorsmod.Register(ModuleName, 29, "epoch credit cap exceeded for address")
	ErrTypeCreditCapExceeded  = errorsmod.Register(ModuleName, 30, "type credit cap exceeded for address")
	ErrCreditCapExceeded      = errorsmod.Register(ModuleName, 31, "total credit cap exceeded")
	ErrCreditsFrozen          = errorsmod.Register(ModuleName, 32, "credits are frozen pending challenge resolution")

	// Endorsement quality errors
	ErrValidatorFreeriding      = errorsmod.Register(ModuleName, 33, "validator endorsement rate too low")
	ErrValidatorQuorumThreshold = errorsmod.Register(ModuleName, 34, "validator only endorses near quorum threshold")
	ErrEndorsementWindowExpired = errorsmod.Register(ModuleName, 35, "endorsement window has expired")

	// Withdrawal safety errors
	ErrClaimNonceInvalid = errorsmod.Register(ModuleName, 36, "invalid claim nonce")
	ErrClaimNonceReplay  = errorsmod.Register(ModuleName, 37, "claim nonce already used")
	ErrWithdrawalLocked  = errorsmod.Register(ModuleName, 38, "withdrawal locked during challenge period")

	// ============================================================================
	// V2.1 Mainnet Hardening Errors
	// ============================================================================

	// Deterministic finality errors
	ErrChallengeWindowOpen    = errorsmod.Register(ModuleName, 39, "challenge window still open")
	ErrChallengeWindowClosed  = errorsmod.Register(ModuleName, 40, "challenge window has closed")
	ErrInvalidStateTransition = errorsmod.Register(ModuleName, 41, "invalid finality state transition")
	ErrPoRBatchNotFinalized   = errorsmod.Register(ModuleName, 42, "referenced PoR batch not yet finalized")

	// Fraud proof errors
	ErrInvalidFraudProofType   = errorsmod.Register(ModuleName, 43, "unsupported fraud proof type")
	ErrFraudProofFailed        = errorsmod.Register(ModuleName, 44, "fraud proof verification failed")
	ErrFraudProofAlreadyExists = errorsmod.Register(ModuleName, 45, "fraud proof already submitted for this contribution")

	// Governance safety errors
	ErrParamChangeRateExceeded = errorsmod.Register(ModuleName, 46, "parameter change rate limit exceeded")
	ErrParamTimelocked         = errorsmod.Register(ModuleName, 47, "parameter change is timelocked")
	ErrPayoutsPaused           = errorsmod.Register(ModuleName, 48, "PoC payouts are currently paused")

	// V2.2 Fraud endorsement slashing errors
	ErrEndorsementSlashFailed = errorsmod.Register(ModuleName, 49, "failed to slash validator for fraudulent endorsement")

	// ============================================================================
	// Canonical Hash Layer Errors (codes 50-59)
	// ============================================================================

	// ErrDuplicateCanonicalHash is returned when a submission's canonical hash matches an existing claim
	ErrDuplicateCanonicalHash = errorsmod.Register(ModuleName, 50, "canonical hash already registered")

	// ErrInvalidCanonicalHash is returned when the canonical hash has invalid format
	ErrInvalidCanonicalHash = errorsmod.Register(ModuleName, 51, "invalid canonical hash")

	// ErrInvalidSpecVersion is returned when an unsupported canonical spec version is used
	ErrInvalidSpecVersion = errorsmod.Register(ModuleName, 52, "unsupported canonical spec version")

	// ErrBondEscrowFailed is returned when the duplicate bond escrow fails
	ErrBondEscrowFailed = errorsmod.Register(ModuleName, 53, "duplicate bond escrow failed")

	// ErrBondRefundFailed is returned when the duplicate bond refund fails
	ErrBondRefundFailed = errorsmod.Register(ModuleName, 54, "duplicate bond refund failed")

	// ErrDuplicateRateLimitExceeded is returned when too many duplicates are submitted per epoch
	ErrDuplicateRateLimitExceeded = errorsmod.Register(ModuleName, 55, "duplicate submission rate limit exceeded")

	// ErrInsufficientBond is returned when the submitter lacks funds for the duplicate bond
	ErrInsufficientBond = errorsmod.Register(ModuleName, 56, "insufficient balance for duplicate bond")

	// ============================================================================
	// Similarity Engine Errors (codes 60-69)
	// ============================================================================

	// ErrSimilarityCommitmentExists is returned when a similarity commitment already exists for a contribution
	ErrSimilarityCommitmentExists = errorsmod.Register(ModuleName, 60, "similarity commitment already exists for this contribution")

	// ErrInvalidOracleSignature is returned when the oracle signature cannot be verified
	ErrInvalidOracleSignature = errorsmod.Register(ModuleName, 61, "invalid oracle signature")

	// ErrOracleNotAllowlisted is returned when the signer is not in the oracle allowlist
	ErrOracleNotAllowlisted = errorsmod.Register(ModuleName, 62, "oracle address not in allowlist")

	// ErrSimilarityEpochMismatch is returned when the compact data epoch doesn't match the current epoch
	ErrSimilarityEpochMismatch = errorsmod.Register(ModuleName, 63, "similarity epoch mismatch (anti-replay)")

	// ErrInvalidCompactData is returned when the compact data fails validation
	ErrInvalidCompactData = errorsmod.Register(ModuleName, 64, "invalid similarity compact data")

	// ErrInvalidCommitmentHash is returned when the full vector commitment hash is invalid
	ErrInvalidCommitmentHash = errorsmod.Register(ModuleName, 65, "invalid commitment hash")

	// ErrSimilarityDisabled is returned when similarity checking is not enabled
	ErrSimilarityDisabled = errorsmod.Register(ModuleName, 66, "similarity engine is not enabled")

	// Human Review Layer Errors (codes 70-89)
	ErrNotAssignedReviewer = errorsmod.Register(ModuleName, 70, "signer is not an assigned reviewer for this claim")
	ErrReviewPeriodExpired = errorsmod.Register(ModuleName, 71, "review voting period has expired")
	ErrReviewAlreadyVoted  = errorsmod.Register(ModuleName, 72, "reviewer has already voted")
	ErrReviewNotActive     = errorsmod.Register(ModuleName, 73, "review is not active for this contribution")
	ErrInvalidOverride     = errorsmod.Register(ModuleName, 74, "invalid originality override option")

	ErrReviewDisabled                = errorsmod.Register(ModuleName, 75, "human review layer is not enabled")
	ErrIneligibleReviewer            = errorsmod.Register(ModuleName, 76, "address does not meet reviewer eligibility requirements")
	ErrInsufficientReviewerBond      = errorsmod.Register(ModuleName, 77, "insufficient balance for reviewer bond")
	ErrReviewBondEscrowFailed        = errorsmod.Register(ModuleName, 78, "failed to escrow reviewer bond")
	ErrReviewBondRefundFailed        = errorsmod.Register(ModuleName, 79, "failed to refund reviewer bond")
	ErrInvalidQualityScore           = errorsmod.Register(ModuleName, 80, "quality score must be 0-100")
	ErrInvalidNotesPointer           = errorsmod.Register(ModuleName, 81, "invalid notes pointer URI")
	ErrSelfReview                    = errorsmod.Register(ModuleName, 82, "contributor cannot review own contribution")
	ErrAppealNotFound                = errorsmod.Register(ModuleName, 83, "appeal not found")
	ErrAppealAlreadyFiled            = errorsmod.Register(ModuleName, 84, "appeal already filed for this contribution")
	ErrAppealAlreadyResolved         = errorsmod.Register(ModuleName, 85, "appeal has already been resolved")
	ErrInvalidAppealBond             = errorsmod.Register(ModuleName, 86, "invalid appeal bond amount")
	ErrInsufficientEligibleReviewers = errorsmod.Register(ModuleName, 87, "not enough eligible reviewers available")
	ErrReviewerSuspended             = errorsmod.Register(ModuleName, 88, "reviewer is suspended")
	ErrCollusionDetected             = errorsmod.Register(ModuleName, 89, "potential collusion detected, extra review required")
	ErrReviewAlreadyActive           = errorsmod.Register(ModuleName, 90, "review is already active for this contribution")
	ErrReviewNotFinalized            = errorsmod.Register(ModuleName, 91, "review has not been finalized")

	// Economic Adjustment Errors (Layer 4, codes 92-96)
	ErrClawbackFailed         = errorsmod.Register(ModuleName, 92, "clawback failed")
	ErrClawbackAlreadyApplied = errorsmod.Register(ModuleName, 93, "clawback already applied for this claim")
	ErrInsufficientBalance    = errorsmod.Register(ModuleName, 94, "insufficient balance for clawback")
	ErrVestingNotFound        = errorsmod.Register(ModuleName, 95, "vesting schedule not found")
	ErrInvalidClawbackReason  = errorsmod.Register(ModuleName, 96, "invalid clawback reason")

	// Global Provenance Registry Errors (Layer 5, codes 100-106)
	ErrProvenanceAlreadyRegistered = errorsmod.Register(ModuleName, 100, "provenance entry already exists for this claim")
	ErrProvenanceParentNotFound    = errorsmod.Register(ModuleName, 101, "parent claim not found in provenance registry")
	ErrProvenanceParentNotAccepted = errorsmod.Register(ModuleName, 102, "parent claim has not been accepted")
	ErrProvenanceCycleDetected     = errorsmod.Register(ModuleName, 103, "lineage cycle detected")
	ErrProvenanceMaxDepthExceeded  = errorsmod.Register(ModuleName, 104, "maximum provenance depth exceeded")
	ErrProvenanceNotFound          = errorsmod.Register(ModuleName, 105, "provenance entry not found")
	ErrInvalidProvenanceQuery      = errorsmod.Register(ModuleName, 106, "invalid provenance query parameters")
)
