package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// Default parameter values
const (
	DefaultMaxBatchesPerBlock     uint32 = 20
	DefaultMinChallengePeriod     int64  = 3600    // 1 hour
	DefaultMaxChallengePeriod     int64  = 604800  // 7 days
	DefaultDefaultChallengePeriod int64  = 86400   // 24 hours
	DefaultMinVerifiersGlobal     uint32 = 3
	DefaultMaxVerifiersPerSet     uint32 = 100
	DefaultMinStakeForVerifier    int64  = 1000000 // 1 OMNI in uomniphi
	DefaultRewardDenom            string = "omniphi"
	DefaultMaxFinalizationsPerBlock    uint32 = 50
	DefaultFraudJailDuration           int64  = 86400  // 24 hours
	DefaultChallengeResolutionTimeout  int64  = 172800 // 48 hours

	// F2/F6: Credit caps
	DefaultMaxCreditsPerEpoch int64 = 50_000_000_000 // 50B credits per epoch
	DefaultMaxCreditsPerBatch int64 = 1_000_000_000  // 1B credits per batch

	// F4: Challenge bonds
	DefaultChallengeBondAmount     int64  = 10_000_000 // 10 OMNI in uomniphi
	DefaultMaxChallengesPerAddress uint32 = 5          // per epoch
)

var (
	DefaultSlashFractionDishonest = math.LegacyNewDecWithPrec(5, 2)  // 5%
	DefaultChallengerRewardRatio  = math.LegacyNewDecWithPrec(50, 2) // 50% of slash to challenger
	DefaultBaseRecordReward       = math.NewInt(100)                 // base credits per record in finalized batch
)

// Params defines the module parameters for the PoR module
type Params struct {
	// MaxBatchesPerBlock is the rate limit on batch submissions per block
	MaxBatchesPerBlock uint32 `json:"max_batches_per_block"`

	// MinChallengePeriod is the minimum challenge window duration in seconds
	MinChallengePeriod int64 `json:"min_challenge_period"`

	// MaxChallengePeriod is the maximum challenge window duration in seconds
	MaxChallengePeriod int64 `json:"max_challenge_period"`

	// DefaultChallengePeriod is the default challenge window duration for new apps
	DefaultChallengePeriod int64 `json:"default_challenge_period"`

	// MinVerifiersGlobal is the minimum number of verifiers required in any set
	MinVerifiersGlobal uint32 `json:"min_verifiers_global"`

	// MaxVerifiersPerSet is the maximum number of verifiers allowed in a single set
	MaxVerifiersPerSet uint32 `json:"max_verifiers_per_set"`

	// SlashFractionDishonest is the fraction of stake to slash for dishonest behavior
	SlashFractionDishonest math.LegacyDec `json:"slash_fraction_dishonest"`

	// ChallengerRewardRatio is the fraction of slashed amount awarded to successful challengers
	ChallengerRewardRatio math.LegacyDec `json:"challenger_reward_ratio"`

	// RewardDenom is the denomination used for rewards
	RewardDenom string `json:"reward_denom"`

	// MinStakeForVerifier is the minimum staked amount required to be a verifier
	MinStakeForVerifier math.Int `json:"min_stake_for_verifier"`

	// BaseRecordReward is the base PoC credit reward per record in a finalized batch
	BaseRecordReward math.Int `json:"base_record_reward"`

	// MaxFinalizationsPerBlock caps EndBlocker processing to prevent gas exhaustion
	MaxFinalizationsPerBlock uint32 `json:"max_finalizations_per_block"`

	// FraudJailDuration is the duration in seconds to jail a validator after fraud slash
	FraudJailDuration int64 `json:"fraud_jail_duration"`

	// MinReputationForAttestation is the minimum reputation score required to submit attestations
	// Default 0 means no enforcement until governance enables it
	MinReputationForAttestation math.Int `json:"min_reputation_for_attestation"`

	// ChallengeResolutionTimeout is the duration in seconds after the challenge window
	// after which inconclusive challenges are auto-rejected
	ChallengeResolutionTimeout int64 `json:"challenge_resolution_timeout"`

	// F2/F6: MaxCreditsPerEpoch caps total PoC credits mintable per epoch across all batches
	MaxCreditsPerEpoch math.Int `json:"max_credits_per_epoch"`

	// F2/F6: MaxCreditsPerBatch caps PoC credits mintable per single batch finalization
	MaxCreditsPerBatch math.Int `json:"max_credits_per_batch"`

	// F2/F6: RequireDACommitment when true requires batches to include a DA layer commitment hash.
	// TRUST MODEL: This is a governance-gated flag. When enabled, batches must include a 32-byte
	// DA commitment hash. The chain validates hash presence and format only — actual DA layer
	// verification (e.g., Celestia blob availability) is performed off-chain by verifiers/challengers.
	// Governance should enable this for mainnet once DA infrastructure is operational.
	RequireDACommitment bool `json:"require_da_commitment"`

	// F3: RequireLeafHashes when true requires batches to include per-record leaf hashes.
	// MAINNET: This MUST be enabled (set to true) in mainnet genesis config.
	// Without leaf hashes, double-inclusion fraud proofs are inconclusive.
	RequireLeafHashes bool `json:"require_leaf_hashes"`

	// F4: ChallengeBondAmount is the bond amount (in uomniphi) required to submit a challenge
	ChallengeBondAmount math.Int `json:"challenge_bond_amount"`

	// F4: MaxChallengesPerAddress caps challenges per address per epoch
	MaxChallengesPerAddress uint32 `json:"max_challenges_per_address"`

	// F8: RequirePoSeqCommitment when true requires batches to reference a registered PoSeq commitment.
	// TRUST MODEL: This is a governance-gated flag. When enabled, batches must reference a PoSeq
	// commitment hash that was previously registered on-chain by an authorized sequencer. The chain
	// validates that the commitment exists — actual PoSeq ordering verification is performed off-chain
	// by verifiers. Governance should enable this once PoSeq sequencer infrastructure is operational.
	RequirePoSeqCommitment bool `json:"require_poseq_commitment"`
}

// DefaultParams returns the default module parameters
func DefaultParams() Params {
	return Params{
		MaxBatchesPerBlock:       DefaultMaxBatchesPerBlock,
		MinChallengePeriod:       DefaultMinChallengePeriod,
		MaxChallengePeriod:       DefaultMaxChallengePeriod,
		DefaultChallengePeriod:   DefaultDefaultChallengePeriod,
		MinVerifiersGlobal:       DefaultMinVerifiersGlobal,
		MaxVerifiersPerSet:       DefaultMaxVerifiersPerSet,
		SlashFractionDishonest:   DefaultSlashFractionDishonest,
		ChallengerRewardRatio:    DefaultChallengerRewardRatio,
		RewardDenom:              DefaultRewardDenom,
		MinStakeForVerifier:      math.NewInt(DefaultMinStakeForVerifier),
		BaseRecordReward:         DefaultBaseRecordReward,
		MaxFinalizationsPerBlock:    DefaultMaxFinalizationsPerBlock,
		FraudJailDuration:           DefaultFraudJailDuration,
		MinReputationForAttestation: math.ZeroInt(),
		ChallengeResolutionTimeout:  DefaultChallengeResolutionTimeout,

		// F2/F6: Credit caps — default on with high limits
		MaxCreditsPerEpoch:  math.NewInt(DefaultMaxCreditsPerEpoch),
		MaxCreditsPerBatch:  math.NewInt(DefaultMaxCreditsPerBatch),
		RequireDACommitment: false, // governance enables when DA infrastructure is ready

		// F3: Leaf hashes — off by default for testnet; MUST be true for mainnet genesis.
		// Without leaf hashes, double-inclusion fraud proofs remain inconclusive.
		RequireLeafHashes: false,

		// F4: Challenge bonds — on by default
		ChallengeBondAmount:     math.NewInt(DefaultChallengeBondAmount),
		MaxChallengesPerAddress: DefaultMaxChallengesPerAddress,

		// F8: PoSeq — off by default; governance enables when sequencer infrastructure is ready
		RequirePoSeqCommitment: false,
	}
}

// Validate performs basic validation of module parameters
func (p Params) Validate() error {
	if p.MaxBatchesPerBlock == 0 {
		return fmt.Errorf("max_batches_per_block must be greater than 0")
	}
	if p.MaxBatchesPerBlock > 1000 {
		return fmt.Errorf("max_batches_per_block cannot exceed 1000 (got %d)", p.MaxBatchesPerBlock)
	}

	if p.MinChallengePeriod <= 0 {
		return fmt.Errorf("min_challenge_period must be positive (got %d)", p.MinChallengePeriod)
	}
	if p.MaxChallengePeriod <= 0 {
		return fmt.Errorf("max_challenge_period must be positive (got %d)", p.MaxChallengePeriod)
	}
	if p.MinChallengePeriod > p.MaxChallengePeriod {
		return fmt.Errorf("min_challenge_period (%d) cannot exceed max_challenge_period (%d)", p.MinChallengePeriod, p.MaxChallengePeriod)
	}
	if p.DefaultChallengePeriod < p.MinChallengePeriod || p.DefaultChallengePeriod > p.MaxChallengePeriod {
		return fmt.Errorf("default_challenge_period (%d) must be between min (%d) and max (%d)", p.DefaultChallengePeriod, p.MinChallengePeriod, p.MaxChallengePeriod)
	}

	if p.MinVerifiersGlobal == 0 {
		return fmt.Errorf("min_verifiers_global must be greater than 0")
	}
	if p.MaxVerifiersPerSet == 0 {
		return fmt.Errorf("max_verifiers_per_set must be greater than 0")
	}
	if p.MinVerifiersGlobal > p.MaxVerifiersPerSet {
		return fmt.Errorf("min_verifiers_global (%d) cannot exceed max_verifiers_per_set (%d)", p.MinVerifiersGlobal, p.MaxVerifiersPerSet)
	}

	if p.SlashFractionDishonest.IsNegative() {
		return fmt.Errorf("slash_fraction_dishonest cannot be negative: %s", p.SlashFractionDishonest)
	}
	// SECURITY: Cap at 33% to prevent governance attacks that could destroy validator
	// economics. Even in the worst fraud case, slashing more than 1/3 of stake risks
	// cascading failures and discourages honest validators from participating.
	maxSlashFraction := math.LegacyNewDecWithPrec(33, 2) // 0.33 = 33%
	if p.SlashFractionDishonest.GT(maxSlashFraction) {
		return fmt.Errorf("slash_fraction_dishonest cannot exceed 33%%: %s", p.SlashFractionDishonest)
	}

	if p.ChallengerRewardRatio.IsNegative() {
		return fmt.Errorf("challenger_reward_ratio cannot be negative: %s", p.ChallengerRewardRatio)
	}
	if p.ChallengerRewardRatio.GT(math.LegacyOneDec()) {
		return fmt.Errorf("challenger_reward_ratio cannot exceed 1.0: %s", p.ChallengerRewardRatio)
	}

	if p.RewardDenom == "" {
		return fmt.Errorf("reward_denom cannot be empty")
	}

	if p.MinStakeForVerifier.IsNegative() {
		return fmt.Errorf("min_stake_for_verifier cannot be negative: %s", p.MinStakeForVerifier)
	}

	if p.BaseRecordReward.IsNegative() {
		return fmt.Errorf("base_record_reward cannot be negative: %s", p.BaseRecordReward)
	}

	if p.MaxFinalizationsPerBlock == 0 {
		return fmt.Errorf("max_finalizations_per_block must be greater than 0")
	}

	if p.FraudJailDuration <= 0 {
		return fmt.Errorf("fraud_jail_duration must be positive (got %d)", p.FraudJailDuration)
	}

	if p.MinReputationForAttestation.IsNegative() {
		return fmt.Errorf("min_reputation_for_attestation cannot be negative: %s", p.MinReputationForAttestation)
	}

	if p.ChallengeResolutionTimeout <= 0 {
		return fmt.Errorf("challenge_resolution_timeout must be positive (got %d)", p.ChallengeResolutionTimeout)
	}

	// F2/F6: Credit cap validation
	if !p.MaxCreditsPerEpoch.IsNil() && p.MaxCreditsPerEpoch.IsNegative() {
		return fmt.Errorf("max_credits_per_epoch cannot be negative: %s", p.MaxCreditsPerEpoch)
	}
	if !p.MaxCreditsPerBatch.IsNil() && p.MaxCreditsPerBatch.IsNegative() {
		return fmt.Errorf("max_credits_per_batch cannot be negative: %s", p.MaxCreditsPerBatch)
	}

	// F4: Challenge bond validation — must be strictly positive to prevent
	// zero-cost challenge spam that could grief honest batch submitters
	if p.ChallengeBondAmount.IsNil() || !p.ChallengeBondAmount.IsPositive() {
		return fmt.Errorf("challenge_bond_amount must be positive (got %s); zero-cost challenges enable griefing", p.ChallengeBondAmount)
	}

	return nil
}

// Reset implements proto.Message (stub for manual types)
func (p *Params) Reset() { *p = Params{} }

// String implements proto.Message (stub for manual types)
func (p *Params) String() string { return fmt.Sprintf("%+v", *p) }

// ProtoMessage implements proto.Message (stub for manual types)
func (*Params) ProtoMessage() {}
