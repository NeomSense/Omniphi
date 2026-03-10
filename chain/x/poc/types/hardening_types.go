package types

import (
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
)

// ============================================================================
// PoC Hardening Upgrade Types (v2)
// ============================================================================

// FinalityStatus represents the finality state of a contribution
type FinalityStatus uint32

const (
	// FinalityStatusPending - contribution awaiting finality confirmation
	FinalityStatusPending FinalityStatus = 0
	// FinalityStatusFinal - contribution finalized and eligible for rewards
	FinalityStatusFinal FinalityStatus = 1
	// FinalityStatusChallenged - contribution under active challenge
	FinalityStatusChallenged FinalityStatus = 2
	// FinalityStatusInvalidated - contribution invalidated by fraud proof
	FinalityStatusInvalidated FinalityStatus = 3
)

func (s FinalityStatus) String() string {
	switch s {
	case FinalityStatusPending:
		return "PENDING"
	case FinalityStatusFinal:
		return "FINAL"
	case FinalityStatusChallenged:
		return "CHALLENGED"
	case FinalityStatusInvalidated:
		return "INVALIDATED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// ContributionFinality tracks finality state for a contribution
type ContributionFinality struct {
	ContributionID uint64         `json:"contribution_id"`
	Status         FinalityStatus `json:"status"`
	FinalizedAt    int64          `json:"finalized_at,omitempty"`  // Block height when finalized
	ChallengeID    uint64         `json:"challenge_id,omitempty"` // PoR challenge ID if challenged
	BatchID        uint64         `json:"batch_id,omitempty"`     // PoR batch ID if linked
}

func (cf *ContributionFinality) Marshal() ([]byte, error) {
	return json.Marshal(cf)
}

func (cf *ContributionFinality) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, cf)
}

// IsFinal returns true if the contribution has reached finality
func (cf *ContributionFinality) IsFinal() bool {
	return cf.Status == FinalityStatusFinal
}

// IsBlocked returns true if the contribution is challenged or invalidated
func (cf *ContributionFinality) IsBlocked() bool {
	return cf.Status == FinalityStatusChallenged || cf.Status == FinalityStatusInvalidated
}

// ============================================================================
// ReputationScore - Slow-moving EMA for governance boost
// ============================================================================

// ReputationScore stores the slow-moving EMA reputation score for governance
// This is separate from RewardScore (raw credits) to prevent governance gaming
type ReputationScore struct {
	Address      string         `json:"address"`
	Score        math.LegacyDec `json:"score"`         // Current EMA score
	LastUpdated  int64          `json:"last_updated"`  // Epoch when last updated
	TotalEarned  math.Int       `json:"total_earned"`  // Lifetime credits earned (for audit)
	TotalDecayed math.Int       `json:"total_decayed"` // Lifetime credits decayed (for audit)
}

// NewReputationScore creates a new ReputationScore with zero values
func NewReputationScore(addr string) ReputationScore {
	return ReputationScore{
		Address:      addr,
		Score:        math.LegacyZeroDec(),
		LastUpdated:  0,
		TotalEarned:  math.ZeroInt(),
		TotalDecayed: math.ZeroInt(),
	}
}

func (rs *ReputationScore) Marshal() ([]byte, error) {
	return json.Marshal(rs)
}

func (rs *ReputationScore) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, rs)
}

// UpdateWithEMA updates the reputation score using exponential moving average
// alpha controls how fast new values affect the score (lower = slower)
// Default alpha = 0.1 (10% new, 90% historical)
func (rs *ReputationScore) UpdateWithEMA(newCredits math.Int, alpha math.LegacyDec, currentEpoch int64) {
	newValue := math.LegacyNewDecFromInt(newCredits)
	// EMA = alpha * newValue + (1 - alpha) * oldValue
	oneMinusAlpha := math.LegacyOneDec().Sub(alpha)
	rs.Score = alpha.Mul(newValue).Add(oneMinusAlpha.Mul(rs.Score))
	rs.LastUpdated = currentEpoch
	rs.TotalEarned = rs.TotalEarned.Add(newCredits)
}

// ApplyDecay applies a decay rate to the reputation score
// Returns the amount decayed
func (rs *ReputationScore) ApplyDecay(decayRate math.LegacyDec) math.LegacyDec {
	if rs.Score.IsZero() || decayRate.IsZero() {
		return math.LegacyZeroDec()
	}
	decayAmount := rs.Score.Mul(decayRate)
	rs.Score = rs.Score.Sub(decayAmount)
	if rs.Score.IsNegative() {
		rs.Score = math.LegacyZeroDec()
	}
	rs.TotalDecayed = rs.TotalDecayed.Add(decayAmount.TruncateInt())
	return decayAmount
}

// ============================================================================
// EpochCredits - Per-epoch credit tracking for caps
// ============================================================================

// EpochCredits tracks credits earned in a specific epoch for an address
type EpochCredits struct {
	Address string   `json:"address"`
	Epoch   uint64   `json:"epoch"`
	Credits math.Int `json:"credits"`
}

func NewEpochCredits(addr string, epoch uint64) EpochCredits {
	return EpochCredits{
		Address: addr,
		Epoch:   epoch,
		Credits: math.ZeroInt(),
	}
}

func (ec *EpochCredits) Marshal() ([]byte, error) {
	return json.Marshal(ec)
}

func (ec *EpochCredits) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, ec)
}

// ============================================================================
// TypeCredits - Per-type credit tracking for caps
// ============================================================================

// TypeCredits tracks credits earned per contribution type for an address
type TypeCredits struct {
	Address string   `json:"address"`
	Ctype   string   `json:"ctype"`
	Credits math.Int `json:"credits"`
	Count   uint64   `json:"count"` // Number of contributions of this type
}

func NewTypeCredits(addr string, ctype string) TypeCredits {
	return TypeCredits{
		Address: addr,
		Ctype:   ctype,
		Credits: math.ZeroInt(),
		Count:   0,
	}
}

func (tc *TypeCredits) Marshal() ([]byte, error) {
	return json.Marshal(tc)
}

func (tc *TypeCredits) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, tc)
}

// ============================================================================
// ValidatorEndorsementStats - Endorsement quality tracking
// ============================================================================

// ValidatorEndorsementStats tracks endorsement participation metrics for a validator
type ValidatorEndorsementStats struct {
	ValidatorAddress string         `json:"validator_address"`
	TotalEndorsed    uint64         `json:"total_endorsed"`      // Total contributions endorsed
	TotalOpportunity uint64         `json:"total_opportunity"`   // Total contributions available to endorse
	QuorumEndorsed   uint64         `json:"quorum_endorsed"`     // Endorsements made after quorum reached
	EarlyEndorsed    uint64         `json:"early_endorsed"`      // Endorsements made before 50% quorum
	ParticipationEMA math.LegacyDec `json:"participation_ema"`   // EMA of participation rate
	LastUpdated      int64          `json:"last_updated"`        // Block height
}

func NewValidatorEndorsementStats(valAddr string) ValidatorEndorsementStats {
	return ValidatorEndorsementStats{
		ValidatorAddress: valAddr,
		TotalEndorsed:    0,
		TotalOpportunity: 0,
		QuorumEndorsed:   0,
		EarlyEndorsed:    0,
		ParticipationEMA: math.LegacyZeroDec(),
		LastUpdated:      0,
	}
}

func (ves *ValidatorEndorsementStats) Marshal() ([]byte, error) {
	return json.Marshal(ves)
}

func (ves *ValidatorEndorsementStats) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, ves)
}

// GetParticipationRate returns the current participation rate
func (ves *ValidatorEndorsementStats) GetParticipationRate() math.LegacyDec {
	if ves.TotalOpportunity == 0 {
		return math.LegacyZeroDec()
	}
	return math.LegacyNewDec(int64(ves.TotalEndorsed)).Quo(math.LegacyNewDec(int64(ves.TotalOpportunity)))
}

// GetEarlyParticipationRate returns the rate of early (pre-quorum) endorsements
func (ves *ValidatorEndorsementStats) GetEarlyParticipationRate() math.LegacyDec {
	if ves.TotalEndorsed == 0 {
		return math.LegacyZeroDec()
	}
	return math.LegacyNewDec(int64(ves.EarlyEndorsed)).Quo(math.LegacyNewDec(int64(ves.TotalEndorsed)))
}

// IsFreeriding returns true if the validator is below minimum participation threshold
func (ves *ValidatorEndorsementStats) IsFreeriding(minRate math.LegacyDec) bool {
	// Require minimum 10 opportunities before flagging
	if ves.TotalOpportunity < 10 {
		return false
	}
	return ves.GetParticipationRate().LT(minRate)
}

// IsQuorumGaming returns true if the validator mostly endorses after quorum is nearly reached
func (ves *ValidatorEndorsementStats) IsQuorumGaming(maxQuorumRate math.LegacyDec) bool {
	if ves.TotalEndorsed < 10 {
		return false
	}
	quorumRate := math.LegacyNewDec(int64(ves.QuorumEndorsed)).Quo(math.LegacyNewDec(int64(ves.TotalEndorsed)))
	return quorumRate.GT(maxQuorumRate)
}

// UpdateParticipationEMA updates the EMA with a new participation value
func (ves *ValidatorEndorsementStats) UpdateParticipationEMA(alpha math.LegacyDec) {
	rate := ves.GetParticipationRate()
	oneMinusAlpha := math.LegacyOneDec().Sub(alpha)
	ves.ParticipationEMA = alpha.Mul(rate).Add(oneMinusAlpha.Mul(ves.ParticipationEMA))
}

// ============================================================================
// FrozenCredits - Credits frozen pending challenge resolution
// ============================================================================

// FrozenCredits tracks credits that are frozen due to pending challenges
type FrozenCredits struct {
	Address        string   `json:"address"`
	Amount         math.Int `json:"amount"`
	ContributionID uint64   `json:"contribution_id"` // The contribution causing the freeze
	FrozenAt       int64    `json:"frozen_at"`       // Block height when frozen
	Reason         string   `json:"reason"`          // Reason for freeze (challenge, fraud, etc.)
}

func NewFrozenCredits(addr string, amount math.Int, contributionID uint64, blockHeight int64, reason string) FrozenCredits {
	return FrozenCredits{
		Address:        addr,
		Amount:         amount,
		ContributionID: contributionID,
		FrozenAt:       blockHeight,
		Reason:         reason,
	}
}

func (fc *FrozenCredits) Marshal() ([]byte, error) {
	return json.Marshal(fc)
}

func (fc *FrozenCredits) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, fc)
}

// ============================================================================
// Finality Provider Interface (PoR-Ready Abstraction)
// ============================================================================

// RecordFinalityProvider defines the interface for checking contribution finality
// This abstraction allows PoC to work with direct PoV finality initially,
// then switch to PoR-based finality when PoR is live.
type RecordFinalityProvider interface {
	// IsFinal returns true if the contribution has reached finality
	// For direct PoV: verified=true implies final
	// For PoR: requires batch finalization + challenge window
	IsFinal(contributionID uint64) bool

	// IsChallenged returns true if the contribution is under active challenge
	IsChallenged(contributionID uint64) bool

	// IsInvalidated returns true if the contribution was invalidated by fraud proof
	IsInvalidated(contributionID uint64) bool

	// GetBatchID returns the PoR batch ID if the contribution is linked to a batch
	// Returns 0 if not linked (direct PoV mode)
	GetBatchID(contributionID uint64) uint64
}

// ContributionEvidenceProvider defines the interface for contribution evidence
// Used by PoR to validate contribution data against batch records
type ContributionEvidenceProvider interface {
	// GetContributionHash returns the hash of a contribution for PoR validation
	GetContributionHash(contributionID uint64) ([]byte, error)

	// GetContributionData returns the full contribution data for fraud proof verification
	GetContributionData(contributionID uint64) (Contribution, error)
}

// ============================================================================
// Default Parameter Values for Hardening
// ============================================================================

// Default hardening parameters
const (
	// DefaultCreditCap is the hard cap on total credits (100,000)
	DefaultCreditCap = 100000

	// DefaultEpochCreditCap is the max credits per epoch per address (10,000)
	DefaultEpochCreditCap = 10000

	// DefaultTypeCreditCap is the max credits per type per address (50,000)
	DefaultTypeCreditCap = 50000

	// DefaultCreditDecayRate is the decay rate per epoch (0.5% = 0.005)
	DefaultCreditDecayRateBps = 50 // basis points

	// DefaultReputationEMAAlpha is the EMA smoothing factor for reputation (0.1)
	DefaultReputationEMAAlphaBps = 1000 // basis points (10%)

	// DefaultMinValidatorParticipation is the minimum endorsement participation rate (20%)
	DefaultMinValidatorParticipationBps = 2000 // basis points

	// DefaultMaxQuorumEndorsementRate is the max rate of post-quorum endorsements (70%)
	DefaultMaxQuorumEndorsementRateBps = 7000 // basis points

	// V2.1 Deterministic Finality Parameters
	// DefaultChallengeWindowBlocks is the number of blocks after verification before finality (100 blocks ≈ ~10 min)
	DefaultChallengeWindowBlocks int64 = 100

	// V2.1 Endorsement Penalty Parameters
	// DefaultEndorsementPenaltyEpochs is how many epochs a soft penalty lasts
	DefaultEndorsementPenaltyEpochs int64 = 10
	// DefaultMaxEndorsementWeightReduction is the max endorsement weight reduction (10% = 1000 bps)
	DefaultMaxEndorsementWeightReductionBps = 1000

	// V2.1 Governance Safety Parameters
	// DefaultMaxParamChangePctPerProposal is the max % any critical param can change in one proposal (20%)
	DefaultMaxParamChangePctPerProposalBps = 2000
	// DefaultParamTimelockBlocks is the number of blocks a critical param change is delayed
	DefaultParamTimelockBlocks int64 = 1000

	// V2.2 Fraud Endorsement Slashing Parameters
	// DefaultSlashFractionFraudEndorsementBps is the slash fraction for validators who endorsed
	// a fraudulent contribution (1% = 100 bps). This is actual stake slashing, not a soft penalty.
	DefaultSlashFractionFraudEndorsementBps = 100 // 1%
)

// DefaultSlashFractionFraudEndorsement returns the default slash fraction as LegacyDec
func DefaultSlashFractionFraudEndorsement() math.LegacyDec {
	return math.LegacyNewDecWithPrec(int64(DefaultSlashFractionFraudEndorsementBps), 4) // 0.0100
}

// DiminishingReturnsCurve applies sqrt-based diminishing returns
// This prevents linear gaming where 2x contributions = 2x credits
func DiminishingReturnsCurve(rawCredits math.Int, cap math.Int) math.Int {
	if rawCredits.IsZero() || cap.IsZero() {
		return math.ZeroInt()
	}

	// Curve: effective = cap * sqrt(raw / cap)
	// This gives diminishing returns as raw approaches cap
	rawDec := math.LegacyNewDecFromInt(rawCredits)
	capDec := math.LegacyNewDecFromInt(cap)

	// Ratio = raw / cap
	ratio := rawDec.Quo(capDec)

	// Clamp ratio to [0, 1] to prevent over-cap
	if ratio.GT(math.LegacyOneDec()) {
		ratio = math.LegacyOneDec()
	}

	// sqrt approximation using Newton-Raphson (3 iterations)
	// For production, use proper BigInt sqrt
	sqrtRatio := approxSqrt(ratio)

	// effective = cap * sqrt(ratio)
	effective := capDec.Mul(sqrtRatio)

	return effective.TruncateInt()
}

// ============================================================================
// V2.1 Mainnet Hardening Types
// ============================================================================

// FraudProofType enumerates the supported deterministic fraud proof categories.
// Only these types are accepted — anything else is rejected explicitly.
type FraudProofType uint32

const (
	// FraudProofInvalidQuorum - endorsement power < 2/3 at time of verification
	FraudProofInvalidQuorum FraudProofType = 0
	// FraudProofHashMismatch - contribution hash/CID does not match declared data
	FraudProofHashMismatch FraudProofType = 1
	// FraudProofNonceReplay - claim nonce was already used (replay attack)
	FraudProofNonceReplay FraudProofType = 2
	// FraudProofMerkleInclusion - PoR merkle inclusion proof mismatch
	FraudProofMerkleInclusion FraudProofType = 3
)

func (fp FraudProofType) String() string {
	switch fp {
	case FraudProofInvalidQuorum:
		return "INVALID_QUORUM"
	case FraudProofHashMismatch:
		return "HASH_MISMATCH"
	case FraudProofNonceReplay:
		return "NONCE_REPLAY"
	case FraudProofMerkleInclusion:
		return "MERKLE_INCLUSION"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", fp)
	}
}

// IsValid returns true if this is a supported fraud proof type
func (fp FraudProofType) IsValid() bool {
	return fp <= FraudProofMerkleInclusion
}

// FraudProof stores a validated fraud proof for a contribution
type FraudProof struct {
	ContributionID uint64         `json:"contribution_id"`
	ProofType      FraudProofType `json:"proof_type"`
	Challenger     string         `json:"challenger"`   // bech32 address
	ProofData      []byte         `json:"proof_data"`   // encoded deterministic proof
	SubmittedAt    int64          `json:"submitted_at"` // block height
	Validated      bool           `json:"validated"`     // whether the proof passed verification
}

func (fp *FraudProof) Marshal() ([]byte, error) {
	return json.Marshal(fp)
}

func (fp *FraudProof) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, fp)
}

// ChallengeWindow tracks the challenge window for a contribution's finality
type ChallengeWindow struct {
	ContributionID uint64 `json:"contribution_id"`
	StartHeight    int64  `json:"start_height"` // block height when verification completed
	EndHeight      int64  `json:"end_height"`   // block height when window closes
}

func (cw *ChallengeWindow) Marshal() ([]byte, error) {
	return json.Marshal(cw)
}

func (cw *ChallengeWindow) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, cw)
}

// IsExpired returns true if the challenge window has closed at the given height
func (cw *ChallengeWindow) IsExpired(currentHeight int64) bool {
	return currentHeight > cw.EndHeight
}

// EndorsementPenalty tracks a soft penalty applied to a validator for poor endorsement quality
type EndorsementPenalty struct {
	ValidatorAddress string `json:"validator_address"`
	// PenaltyStartEpoch is the epoch when the penalty was applied
	PenaltyStartEpoch int64 `json:"penalty_start_epoch"`
	// PenaltyEndEpoch is the epoch when the penalty expires
	PenaltyEndEpoch int64 `json:"penalty_end_epoch"`
	// ParticipationBonusBlocked prevents ParticipationBonus eligibility
	ParticipationBonusBlocked bool `json:"participation_bonus_blocked"`
	// EndorsementWeightReductionBps is the endorsement weight reduction in basis points (max 1000 = 10%)
	EndorsementWeightReductionBps uint32 `json:"endorsement_weight_reduction_bps"`
	// Reason describes why the penalty was applied
	Reason string `json:"reason"`
}

func (ep *EndorsementPenalty) Marshal() ([]byte, error) {
	return json.Marshal(ep)
}

func (ep *EndorsementPenalty) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, ep)
}

// IsActive returns true if the penalty is still active at the given epoch
func (ep *EndorsementPenalty) IsActive(currentEpoch int64) bool {
	return currentEpoch >= ep.PenaltyStartEpoch && currentEpoch < ep.PenaltyEndEpoch
}

// GetWeightMultiplier returns the endorsement weight multiplier (e.g., 0.90 for 10% reduction)
func (ep *EndorsementPenalty) GetWeightMultiplier() math.LegacyDec {
	if ep.EndorsementWeightReductionBps == 0 {
		return math.LegacyOneDec()
	}
	reduction := math.LegacyNewDecWithPrec(int64(ep.EndorsementWeightReductionBps), 4)
	return math.LegacyOneDec().Sub(reduction)
}

// ParamChangeRecord tracks a governance param change for rate limiting
type ParamChangeRecord struct {
	BlockHeight int64  `json:"block_height"`
	ParamName   string `json:"param_name"`
	OldValue    string `json:"old_value"`
	NewValue    string `json:"new_value"`
}

func (pcr *ParamChangeRecord) Marshal() ([]byte, error) {
	return json.Marshal(pcr)
}

func (pcr *ParamChangeRecord) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, pcr)
}

// IsCriticalParam returns true if the param name is in the timelocked set
func IsCriticalParam(paramName string) bool {
	criticalParams := map[string]bool{
		"credit_cap":       true,
		"decay_rate":       true,
		"ema_alpha":        true,
		"quorum_pct":       true,
		"min_multiplier":   true,
		"max_multiplier":   true,
		"epoch_credit_cap": true,
		"type_credit_cap":  true,
	}
	return criticalParams[paramName]
}

// approxSqrt approximates square root using Newton-Raphson method
func approxSqrt(x math.LegacyDec) math.LegacyDec {
	if x.IsZero() || x.IsNegative() {
		return math.LegacyZeroDec()
	}
	if x.Equal(math.LegacyOneDec()) {
		return math.LegacyOneDec()
	}

	// Initial guess
	guess := x.Quo(math.LegacyNewDec(2))

	// Newton-Raphson: x_new = (x_old + S/x_old) / 2
	two := math.LegacyNewDec(2)
	for i := 0; i < 10; i++ {
		if guess.IsZero() {
			break
		}
		newGuess := guess.Add(x.Quo(guess)).Quo(two)
		// Check convergence
		diff := newGuess.Sub(guess).Abs()
		if diff.LT(math.LegacyNewDecWithPrec(1, 18)) { // 1e-18 precision
			return newGuess
		}
		guess = newGuess
	}

	return guess
}
