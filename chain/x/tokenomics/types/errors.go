package types

import (
	errorsmod "cosmossdk.io/errors"
)

// Tokenomics module errors
var (
	// Supply cap errors
	ErrSupplyCapExceeded = errorsmod.Register(ModuleName, 1, "total supply would exceed cap")
	ErrSupplyCapReached  = errorsmod.Register(ModuleName, 2, "total supply has reached cap")

	// Inflation errors
	ErrInflationBelowMin      = errorsmod.Register(ModuleName, 10, "inflation rate below minimum")
	ErrInflationAboveMax      = errorsmod.Register(ModuleName, 11, "inflation rate above maximum")
	ErrProtocolCapViolation   = errorsmod.Register(ModuleName, 12, "parameter exceeds protocol cap")

	// Burn errors
	ErrInsufficientBalance = errorsmod.Register(ModuleName, 20, "insufficient balance to burn")
	ErrInsufficientSupply  = errorsmod.Register(ModuleName, 21, "insufficient supply to burn")
	ErrInvalidBurnSource   = errorsmod.Register(ModuleName, 22, "invalid burn source")
	ErrInvalidHash         = errorsmod.Register(ModuleName, 23, "invalid hash")

	// Permission errors
	ErrUnauthorized            = errorsmod.Register(ModuleName, 30, "unauthorized")
	ErrInsufficientPermissions = errorsmod.Register(ModuleName, 31, "insufficient permissions")
	ErrInsufficientSignatures  = errorsmod.Register(ModuleName, 32, "insufficient multisig signatures")

	// Validation errors
	ErrInvalidAmount     = errorsmod.Register(ModuleName, 40, "invalid amount")
	ErrInvalidAddress    = errorsmod.Register(ModuleName, 41, "invalid address")
	ErrInvalidPercentage = errorsmod.Register(ModuleName, 42, "invalid percentage")
	ErrInvalidParams     = errorsmod.Register(ModuleName, 43, "invalid parameters")

	// Emission errors
	ErrEmissionSplitInvalid       = errorsmod.Register(ModuleName, 50, "emission splits do not sum to 100%")
	ErrEmissionBudgetExceeded     = errorsmod.Register(ModuleName, 51, "emission distribution exceeds budget")
	ErrEmissionRecipientExceedsCap = errorsmod.Register(ModuleName, 52, "single emission recipient exceeds 60% cap")
	ErrStakingShareBelowMinimum   = errorsmod.Register(ModuleName, 53, "staking share below 20% security minimum")
	ErrInflationExceedsHardCap    = errorsmod.Register(ModuleName, 54, "inflation rate exceeds 3% protocol hard cap")

	// IBC errors
	ErrInvalidProof      = errorsmod.Register(ModuleName, 60, "invalid IBC proof")
	ErrDuplicateReport   = errorsmod.Register(ModuleName, 61, "duplicate burn report")
	ErrInvalidChannel    = errorsmod.Register(ModuleName, 62, "invalid IBC channel")
	ErrPacketTimeout     = errorsmod.Register(ModuleName, 63, "IBC packet timeout")

	// Governance errors
	ErrInsufficientDeposit = errorsmod.Register(ModuleName, 70, "insufficient proposal deposit")
	ErrVotingPeriodNotEnded = errorsmod.Register(ModuleName, 71, "voting period not ended")
	ErrQuorumNotReached    = errorsmod.Register(ModuleName, 72, "quorum not reached")
	ErrInsufficientYesVotes = errorsmod.Register(ModuleName, 73, "insufficient yes votes")
	ErrProposalExpired     = errorsmod.Register(ModuleName, 74, "proposal expired")
	ErrObsoleteParams      = errorsmod.Register(ModuleName, 75, "obsolete parameter version")

	// PoC errors
	ErrAlreadyEndorsed  = errorsmod.Register(ModuleName, 80, "already endorsed")
	ErrAlphaExceedsMax  = errorsmod.Register(ModuleName, 81, "amplification factor exceeds maximum")

	// Fee errors
	ErrInsufficientFee   = errorsmod.Register(ModuleName, 90, "insufficient fee")
	ErrInvalidGasPrice   = errorsmod.Register(ModuleName, 91, "invalid gas price")
	ErrInsufficientFunds = errorsmod.Register(ModuleName, 92, "insufficient funds")

	// General errors
	ErrNotFound = errorsmod.Register(ModuleName, 100, "not found")
)
