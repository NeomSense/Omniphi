package types

import (
	"context"
	"encoding/json"

	"cosmossdk.io/math"
)

// ValidatorMultiplier stores the current multiplier state for a validator
type ValidatorMultiplier struct {
	// ValidatorAddress is the operator address (bech32 valoper)
	ValidatorAddress string         `json:"validator_address"`
	Epoch            int64          `json:"epoch"`
	MRaw             math.LegacyDec `json:"m_raw"`
	MEMA             math.LegacyDec `json:"m_ema"`
	MEffective       math.LegacyDec `json:"m_effective"`

	// Component breakdown for transparency
	UptimeBonus        math.LegacyDec `json:"uptime_bonus"`
	ParticipationBonus math.LegacyDec `json:"participation_bonus"`
	QualityBonus       math.LegacyDec `json:"quality_bonus"`
	SlashPenalty       math.LegacyDec `json:"slash_penalty"`
	FraudPenalty       math.LegacyDec `json:"fraud_penalty"`
}

// NewValidatorMultiplier creates a new ValidatorMultiplier with neutral defaults
func NewValidatorMultiplier(valAddr string, epoch int64) ValidatorMultiplier {
	return ValidatorMultiplier{
		ValidatorAddress:   valAddr,
		Epoch:              epoch,
		MRaw:               math.LegacyOneDec(),
		MEMA:               math.LegacyOneDec(),
		MEffective:         math.LegacyOneDec(),
		UptimeBonus:        math.LegacyZeroDec(),
		ParticipationBonus: math.LegacyZeroDec(),
		QualityBonus:       math.LegacyZeroDec(),
		SlashPenalty:       math.LegacyZeroDec(),
		FraudPenalty:       math.LegacyZeroDec(),
	}
}

// Marshal serializes the ValidatorMultiplier to JSON bytes
func (vm ValidatorMultiplier) Marshal() ([]byte, error) {
	return json.Marshal(vm)
}

// Unmarshal deserializes the ValidatorMultiplier from JSON bytes
func (vm *ValidatorMultiplier) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, vm)
}

// EMAHistory stores the ring buffer of raw multiplier values for EMA computation
type EMAHistory struct {
	ValidatorAddress string           `json:"validator_address"`
	Values           []math.LegacyDec `json:"values"`
}

// NewEMAHistory creates a new empty EMA history for a validator
func NewEMAHistory(valAddr string) EMAHistory {
	return EMAHistory{
		ValidatorAddress: valAddr,
		Values:           []math.LegacyDec{},
	}
}

// AddValue appends a new value to the ring buffer, trimming to maxSize
func (h *EMAHistory) AddValue(value math.LegacyDec, maxSize int64) {
	h.Values = append(h.Values, value)
	if int64(len(h.Values)) > maxSize {
		h.Values = h.Values[int64(len(h.Values))-maxSize:]
	}
}

// ComputeEMA computes the exponential moving average over the stored values.
// Uses smoothing factor alpha = 2 / (N + 1) where N = window size.
// Returns 1.0 (neutral) if no values exist.
func (h *EMAHistory) ComputeEMA(window int64) math.LegacyDec {
	if len(h.Values) == 0 {
		return math.LegacyOneDec() // neutral default
	}

	// Alpha = 2 / (N + 1)
	alpha := math.LegacyNewDec(2).Quo(math.LegacyNewDec(window + 1))
	oneMinusAlpha := math.LegacyOneDec().Sub(alpha)

	// Start with first value
	ema := h.Values[0]
	for i := 1; i < len(h.Values); i++ {
		// EMA = alpha * value + (1 - alpha) * prev_ema
		ema = alpha.Mul(h.Values[i]).Add(oneMinusAlpha.Mul(ema))
	}

	return ema
}

// Marshal serializes the EMAHistory to JSON bytes
func (h EMAHistory) Marshal() ([]byte, error) {
	return json.Marshal(h)
}

// Unmarshal deserializes the EMAHistory from JSON bytes
func (h *EMAHistory) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, h)
}

// Infraction type constants for slash event classification
const (
	InfractionDowntime   = "downtime"
	InfractionDoubleSign = "double_sign"
	InfractionUnknown    = "unknown"
)

// SlashEvent records a slashing event for penalty decay tracking
type SlashEvent struct {
	ValidatorAddress string         `json:"validator_address"`
	Epoch            int64          `json:"epoch"`
	InfractionType   string         `json:"infraction_type"`
	SlashFraction    math.LegacyDec `json:"slash_fraction"`
}

// FraudEvent records a fraud attestation event (stub for PoR integration)
type FraudEvent struct {
	ValidatorAddress string `json:"validator_address"`
	Epoch            int64  `json:"epoch"`
	BatchId          uint64 `json:"batch_id"`
}

// MsgServer defines the message handlers for x/rewardmult
type MsgServer interface {
	UpdateParams(ctx context.Context, msg *MsgUpdateParams) (*MsgUpdateParamsResponse, error)
}

// QueryServer defines the query handlers for x/rewardmult
type QueryServer interface {
	Params(ctx context.Context, req *QueryParamsRequest) (*QueryParamsResponse, error)
	ValidatorMultiplierQuery(ctx context.Context, req *QueryValidatorMultiplierRequest) (*QueryValidatorMultiplierResponse, error)
	AllMultipliers(ctx context.Context, req *QueryAllMultipliersRequest) (*QueryAllMultipliersResponse, error)
	ClampPressure(ctx context.Context, req *QueryClampPressureRequest) (*QueryClampPressureResponse, error)
	StakeSnapshot(ctx context.Context, req *QueryStakeSnapshotRequest) (*QueryStakeSnapshotResponse, error)
}

// Message types

// MsgUpdateParams is the message to update module parameters (governance-gated)
type MsgUpdateParams struct {
	Authority string `json:"authority"`
	Params    Params `json:"params"`
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if m.Authority == "" {
		return ErrInvalidAuthority
	}
	return m.Params.Validate()
}

// ProtoMessage implements proto.Message interface
func (m *MsgUpdateParams) ProtoMessage() {}

// Reset implements proto.Message interface
func (m *MsgUpdateParams) Reset() { *m = MsgUpdateParams{} }

// String implements proto.Message interface
func (m *MsgUpdateParams) String() string { return "MsgUpdateParams" }

// MsgUpdateParamsResponse is the response type for MsgUpdateParams
type MsgUpdateParamsResponse struct{}

// ProtoMessage implements proto.Message interface
func (m *MsgUpdateParamsResponse) ProtoMessage() {}

// Reset implements proto.Message interface
func (m *MsgUpdateParamsResponse) Reset() { *m = MsgUpdateParamsResponse{} }

// String implements proto.Message interface
func (m *MsgUpdateParamsResponse) String() string { return "MsgUpdateParamsResponse" }

// Query types

type QueryParamsRequest struct{}
type QueryParamsResponse struct {
	Params Params `json:"params"`
}

type QueryValidatorMultiplierRequest struct {
	ValidatorAddress string `json:"validator_address"`
}
type QueryValidatorMultiplierResponse struct {
	Multiplier ValidatorMultiplier `json:"multiplier"`
}

type QueryAllMultipliersRequest struct{}
type QueryAllMultipliersResponse struct {
	Multipliers []ValidatorMultiplier `json:"multipliers"`
}

// GenesisState defines the genesis state for x/rewardmult
type GenesisState struct {
	Params      Params                `json:"params"`
	Multipliers []ValidatorMultiplier `json:"multipliers"`
	EmaHistory  []EMAHistory          `json:"ema_history"`
}

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:      DefaultParams(),
		Multipliers: []ValidatorMultiplier{},
		EmaHistory:  []EMAHistory{},
	}
}

// Validate performs genesis state validation
func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}

// ============================================================================
// V2.2 Types — Stake Snapshots & Normalization Audit Data
// ============================================================================

// EpochStakeSnapshot records a validator's bonded tokens at an epoch boundary.
// This ensures normalization and distribution use identical stake weights,
// preventing mid-epoch stake changes from creating accounting inconsistencies.
type EpochStakeSnapshot struct {
	Epoch            int64    `json:"epoch"`
	ValidatorAddress string   `json:"validator_address"`
	BondedTokens     math.Int `json:"bonded_tokens"`
}

// EpochNormalizationStats holds audit-grade telemetry for one epoch's normalization.
// Emitted as an event so third-party auditors can reconstruct reward math,
// verify budget neutrality, and detect clamp pressure.
type EpochNormalizationStats struct {
	Epoch                 int64          `json:"epoch"`
	NormFactor            math.LegacyDec `json:"norm_factor"`
	TotalStake            math.LegacyDec `json:"total_stake"`
	WeightedSumBeforeNorm math.LegacyDec `json:"weighted_sum_before_norm"`
	WeightedSumAfterNorm  math.LegacyDec `json:"weighted_sum_after_norm"`
	CountClampedMin       int            `json:"count_clamped_min"`
	CountClampedMax       int            `json:"count_clamped_max"`
	IterativeRounds       int            `json:"iterative_rounds"`
	BudgetError           math.LegacyDec `json:"budget_error"`
}

// V2.2 Query types

// QueryClampPressureRequest is the request for the clamp pressure query
type QueryClampPressureRequest struct{}

// QueryClampPressureResponse returns clamp pressure telemetry for the latest epoch
type QueryClampPressureResponse struct {
	Epoch           int64 `json:"epoch"`
	CountClampedMin int   `json:"count_clamped_min"`
	CountClampedMax int   `json:"count_clamped_max"`
	TotalValidators int   `json:"total_validators"`
}

// QueryStakeSnapshotRequest queries stake snapshots for an epoch
type QueryStakeSnapshotRequest struct {
	Epoch int64 `json:"epoch"`
}

// QueryStakeSnapshotResponse returns stake snapshots for the requested epoch
type QueryStakeSnapshotResponse struct {
	Snapshots []EpochStakeSnapshot `json:"snapshots"`
}

// ============================================================================
// Service Registration Stubs
// ============================================================================

// RegisterMsgServer registers the MsgServer implementation with the gRPC server.
// For manual types (non-protoc-generated), this is a no-op placeholder.
func RegisterMsgServer(s interface{}, srv MsgServer) {
	// In manual types mode, registration is handled by the module's RegisterServices method.
}

// RegisterQueryServer registers the QueryServer implementation with the gRPC server.
func RegisterQueryServer(s interface{}, srv QueryServer) {
	// Same as above - placeholder for manual types mode.
}
