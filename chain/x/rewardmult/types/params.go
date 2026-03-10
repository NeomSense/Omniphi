package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// Default parameter values
var (
	DefaultMinMultiplier = math.LegacyNewDecWithPrec(85, 2)  // 0.85
	DefaultMaxMultiplier = math.LegacyNewDecWithPrec(115, 2) // 1.15

	DefaultUptimeThresholdHigh = math.LegacyNewDecWithPrec(999, 3) // 0.999 = 99.9%
	DefaultUptimeThresholdMed  = math.LegacyNewDecWithPrec(995, 3) // 0.995 = 99.5%
	DefaultUptimeBonusHigh     = math.LegacyNewDecWithPrec(3, 2)   // +0.03
	DefaultUptimeBonusMed      = math.LegacyNewDecWithPrec(15, 3)  // +0.015

	DefaultMaxParticipationBonus = math.LegacyNewDecWithPrec(1, 2) // +0.01

	DefaultSlashPenalty      = math.LegacyNewDecWithPrec(5, 2)  // -0.05 (downtime)
	DefaultDoubleSignPenalty = math.LegacyNewDecWithPrec(10, 2) // -0.10 (double-sign, heavier)
	DefaultFraudPenalty      = math.LegacyNewDecWithPrec(10, 2) // -0.10
	DefaultMaxQualityBonus   = math.LegacyNewDecWithPrec(2, 2)  // +0.02 max quality bonus from PoC metrics
)

const (
	DefaultEMAWindow                int64 = 8
	DefaultSlashLookbackEpochs      int64 = 30
	DefaultDoubleSignLookbackEpochs int64 = 60 // longer decay for double-sign
	DefaultFraudLookbackEpochs      int64 = 30
)

// Params defines the module parameters for x/rewardmult
type Params struct {
	// Multiplier bounds
	MinMultiplier math.LegacyDec `json:"min_multiplier"`
	MaxMultiplier math.LegacyDec `json:"max_multiplier"`

	// EMA smoothing window (number of epochs)
	EMAWindow int64 `json:"ema_window"`

	// Uptime thresholds and bonuses
	UptimeThresholdHigh math.LegacyDec `json:"uptime_threshold_high"` // 99.9%
	UptimeThresholdMed  math.LegacyDec `json:"uptime_threshold_med"`  // 99.5%
	UptimeBonusHigh     math.LegacyDec `json:"uptime_bonus_high"`     // +0.03
	UptimeBonusMed      math.LegacyDec `json:"uptime_bonus_med"`      // +0.015

	// Participation bonus (PoV endorsement activity)
	MaxParticipationBonus math.LegacyDec `json:"max_participation_bonus"` // +0.01 max

	// Slash penalty and lookback (downtime)
	SlashPenalty         math.LegacyDec `json:"slash_penalty"`          // -0.05
	SlashLookbackEpochs int64          `json:"slash_lookback_epochs"`

	// Double-sign penalty and lookback (heavier, longer decay)
	DoubleSignPenalty         math.LegacyDec `json:"double_sign_penalty"`          // -0.10
	DoubleSignLookbackEpochs int64          `json:"double_sign_lookback_epochs"`

	// Fraud penalty and lookback (future PoR integration)
	FraudPenalty         math.LegacyDec `json:"fraud_penalty"`          // -0.10
	FraudLookbackEpochs int64          `json:"fraud_lookback_epochs"`

	// Quality bonus from PoC originality/quality metrics (Layer 4 integration)
	MaxQualityBonus math.LegacyDec `json:"max_quality_bonus"` // +0.02 max
}

// DefaultParams returns the default module parameters
func DefaultParams() Params {
	return Params{
		MinMultiplier:         DefaultMinMultiplier,
		MaxMultiplier:         DefaultMaxMultiplier,
		EMAWindow:             DefaultEMAWindow,
		UptimeThresholdHigh:   DefaultUptimeThresholdHigh,
		UptimeThresholdMed:    DefaultUptimeThresholdMed,
		UptimeBonusHigh:       DefaultUptimeBonusHigh,
		UptimeBonusMed:        DefaultUptimeBonusMed,
		MaxParticipationBonus: DefaultMaxParticipationBonus,
		SlashPenalty:              DefaultSlashPenalty,
		SlashLookbackEpochs:      DefaultSlashLookbackEpochs,
		DoubleSignPenalty:         DefaultDoubleSignPenalty,
		DoubleSignLookbackEpochs: DefaultDoubleSignLookbackEpochs,
		FraudPenalty:              DefaultFraudPenalty,
		FraudLookbackEpochs:   DefaultFraudLookbackEpochs,
		MaxQualityBonus:       DefaultMaxQualityBonus,
	}
}

// Validate performs parameter validation
func (p Params) Validate() error {
	// Multiplier bounds
	if p.MinMultiplier.IsNil() || p.MaxMultiplier.IsNil() {
		return fmt.Errorf("%w: min/max multiplier cannot be nil", ErrInvalidMultiplierRange)
	}
	if p.MinMultiplier.IsNegative() {
		return fmt.Errorf("%w: min multiplier cannot be negative: %s", ErrInvalidMultiplierRange, p.MinMultiplier)
	}
	if p.MaxMultiplier.LTE(p.MinMultiplier) {
		return fmt.Errorf("%w: max multiplier (%s) must be > min multiplier (%s)", ErrInvalidMultiplierRange, p.MaxMultiplier, p.MinMultiplier)
	}
	// Sanity: multipliers should be reasonable (0.5 to 2.0 range)
	if p.MinMultiplier.LT(math.LegacyNewDecWithPrec(50, 2)) {
		return fmt.Errorf("%w: min multiplier too low: %s (floor 0.50)", ErrInvalidMultiplierRange, p.MinMultiplier)
	}
	if p.MaxMultiplier.GT(math.LegacyNewDec(2)) {
		return fmt.Errorf("%w: max multiplier too high: %s (ceiling 2.00)", ErrInvalidMultiplierRange, p.MaxMultiplier)
	}

	// EMA window
	if p.EMAWindow < 1 {
		return fmt.Errorf("%w: must be >= 1, got %d", ErrInvalidEMAWindow, p.EMAWindow)
	}
	if p.EMAWindow > 100 {
		return fmt.Errorf("%w: must be <= 100, got %d", ErrInvalidEMAWindow, p.EMAWindow)
	}

	// Uptime thresholds
	if p.UptimeThresholdHigh.IsNil() || p.UptimeThresholdMed.IsNil() {
		return fmt.Errorf("%w: uptime thresholds cannot be nil", ErrInvalidUptimeThreshold)
	}
	if p.UptimeThresholdHigh.LTE(p.UptimeThresholdMed) {
		return fmt.Errorf("%w: high threshold (%s) must be > med threshold (%s)", ErrInvalidUptimeThreshold, p.UptimeThresholdHigh, p.UptimeThresholdMed)
	}
	if p.UptimeThresholdHigh.GT(math.LegacyOneDec()) || p.UptimeThresholdMed.GT(math.LegacyOneDec()) {
		return fmt.Errorf("%w: thresholds cannot exceed 1.0", ErrInvalidUptimeThreshold)
	}
	if p.UptimeThresholdMed.IsNegative() {
		return fmt.Errorf("%w: thresholds cannot be negative", ErrInvalidUptimeThreshold)
	}

	// Bonuses must be non-negative
	if p.UptimeBonusHigh.IsNil() || p.UptimeBonusHigh.IsNegative() {
		return fmt.Errorf("%w: uptime_bonus_high must be non-negative", ErrInvalidBonusValue)
	}
	if p.UptimeBonusMed.IsNil() || p.UptimeBonusMed.IsNegative() {
		return fmt.Errorf("%w: uptime_bonus_med must be non-negative", ErrInvalidBonusValue)
	}
	if p.MaxParticipationBonus.IsNil() || p.MaxParticipationBonus.IsNegative() {
		return fmt.Errorf("%w: max_participation_bonus must be non-negative", ErrMaxParticipationBonus)
	}

	// Penalties must be non-negative
	if p.SlashPenalty.IsNil() || p.SlashPenalty.IsNegative() {
		return fmt.Errorf("%w: slash_penalty must be non-negative", ErrInvalidPenaltyValue)
	}
	if p.DoubleSignPenalty.IsNil() || p.DoubleSignPenalty.IsNegative() {
		return fmt.Errorf("%w: double_sign_penalty must be non-negative", ErrInvalidPenaltyValue)
	}
	if p.DoubleSignPenalty.GT(math.LegacyNewDecWithPrec(30, 2)) {
		return fmt.Errorf("%w: double_sign_penalty must be <= 0.30, got %s", ErrInvalidPenaltyValue, p.DoubleSignPenalty)
	}
	if p.FraudPenalty.IsNil() || p.FraudPenalty.IsNegative() {
		return fmt.Errorf("%w: fraud_penalty must be non-negative", ErrInvalidPenaltyValue)
	}

	// Quality bonus
	if !p.MaxQualityBonus.IsNil() && p.MaxQualityBonus.IsNegative() {
		return fmt.Errorf("%w: max_quality_bonus must be non-negative", ErrInvalidBonusValue)
	}

	// Lookback epochs
	if p.SlashLookbackEpochs < 1 {
		return fmt.Errorf("%w: slash_lookback_epochs must be >= 1, got %d", ErrInvalidLookbackEpochs, p.SlashLookbackEpochs)
	}
	if p.DoubleSignLookbackEpochs < 1 {
		return fmt.Errorf("%w: double_sign_lookback_epochs must be >= 1, got %d", ErrInvalidLookbackEpochs, p.DoubleSignLookbackEpochs)
	}
	if p.DoubleSignLookbackEpochs > 365 {
		return fmt.Errorf("%w: double_sign_lookback_epochs must be <= 365, got %d", ErrInvalidLookbackEpochs, p.DoubleSignLookbackEpochs)
	}
	if p.FraudLookbackEpochs < 1 {
		return fmt.Errorf("%w: fraud_lookback_epochs must be >= 1, got %d", ErrInvalidLookbackEpochs, p.FraudLookbackEpochs)
	}

	return nil
}
