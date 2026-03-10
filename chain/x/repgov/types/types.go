package types

import (
	"context"
	"encoding/json"

	"cosmossdk.io/math"
)

// VoterWeight stores the computed governance weight for an address
type VoterWeight struct {
	Address string         `json:"address"`
	Epoch   int64          `json:"epoch"`

	// Raw reputation score from PoC credits [0, 1]
	ReputationScore math.LegacyDec `json:"reputation_score"`

	// Individual source scores [0, 1]
	CScore          math.LegacyDec `json:"c_score"`
	EndorsementRate math.LegacyDec `json:"endorsement_rate"`
	OriginalityAvg  math.LegacyDec `json:"originality_avg"`
	UptimeScore     math.LegacyDec `json:"uptime_score"`
	LongevityScore  math.LegacyDec `json:"longevity_score"`

	// Computed composite weight = 1 + weighted_sum * (max_cap - 1)
	// Clamped to [1.0, MaxVotingWeightCap]
	CompositeWeight math.LegacyDec `json:"composite_weight"`

	// Final weight after delegation adjustments
	EffectiveWeight math.LegacyDec `json:"effective_weight"`

	// Delegation received (from others delegating to this address)
	DelegatedWeight math.LegacyDec `json:"delegated_weight"`

	// Last block where this voter participated in governance
	LastVoteHeight int64 `json:"last_vote_height"`
}

// NewVoterWeight creates a neutral (1.0x) voter weight
func NewVoterWeight(addr string, epoch int64) VoterWeight {
	return VoterWeight{
		Address:         addr,
		Epoch:           epoch,
		ReputationScore: math.LegacyZeroDec(),
		CScore:          math.LegacyZeroDec(),
		EndorsementRate: math.LegacyZeroDec(),
		OriginalityAvg:  math.LegacyZeroDec(),
		UptimeScore:     math.LegacyZeroDec(),
		LongevityScore:  math.LegacyZeroDec(),
		CompositeWeight: math.LegacyOneDec(),
		EffectiveWeight: math.LegacyOneDec(),
		DelegatedWeight: math.LegacyZeroDec(),
		LastVoteHeight:  0,
	}
}

// Marshal serializes the VoterWeight to JSON bytes
func (vw VoterWeight) Marshal() ([]byte, error) {
	return json.Marshal(vw)
}

// Unmarshal deserializes the VoterWeight from JSON bytes
func (vw *VoterWeight) Unmarshal(bz []byte) error {
	return json.Unmarshal(bz, vw)
}

// DelegatedReputation represents a reputation delegation from one address to another
type DelegatedReputation struct {
	Delegator string         `json:"delegator"`
	Delegatee string         `json:"delegatee"`
	Amount    math.LegacyDec `json:"amount"` // fraction of delegator's reputation [0, MaxDelegableRatio]
	Epoch     int64          `json:"epoch"`  // epoch when delegation was created
}

// ReputationSnapshot records the reputation state at an epoch boundary for audit
type ReputationSnapshot struct {
	Address         string         `json:"address"`
	Epoch           int64          `json:"epoch"`
	ReputationScore math.LegacyDec `json:"reputation_score"`
	CompositeWeight math.LegacyDec `json:"composite_weight"`
	EffectiveWeight math.LegacyDec `json:"effective_weight"`
	StakeAmount     math.Int       `json:"stake_amount"`
}

// TallyOverride records how reputation weighting modified a governance tally
type TallyOverride struct {
	ProposalID        uint64         `json:"proposal_id"`
	OriginalYes       math.LegacyDec `json:"original_yes"`
	OriginalNo        math.LegacyDec `json:"original_no"`
	OriginalAbstain   math.LegacyDec `json:"original_abstain"`
	OriginalVeto      math.LegacyDec `json:"original_veto"`
	WeightedYes       math.LegacyDec `json:"weighted_yes"`
	WeightedNo        math.LegacyDec `json:"weighted_no"`
	WeightedAbstain   math.LegacyDec `json:"weighted_abstain"`
	WeightedVeto      math.LegacyDec `json:"weighted_veto"`
	TotalVoters       int64          `json:"total_voters"`
	WeightedVoters    int64          `json:"weighted_voters"` // voters with weight > 1.0
	BlockHeight       int64          `json:"block_height"`
}

// SybilScore tracks sybil resistance indicators for an address
type SybilScore struct {
	Address           string         `json:"address"`
	UniqueInteractions int64         `json:"unique_interactions"` // unique addresses interacted with
	AccountAge        int64          `json:"account_age"`         // blocks since first tx
	ContributionCount int64          `json:"contribution_count"`  // total PoC contributions
	Score             math.LegacyDec `json:"score"`               // composite sybil resistance [0, 1]
}

// ============================================================================
// Message types
// ============================================================================

// MsgServer defines the message handlers for x/repgov
type MsgServer interface {
	UpdateParams(ctx context.Context, msg *MsgUpdateParams) (*MsgUpdateParamsResponse, error)
	DelegateReputation(ctx context.Context, msg *MsgDelegateReputation) (*MsgDelegateReputationResponse, error)
	UndelegateReputation(ctx context.Context, msg *MsgUndelegateReputation) (*MsgUndelegateReputationResponse, error)
}

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

func (m *MsgUpdateParams) ProtoMessage()  {}
func (m *MsgUpdateParams) Reset()         { *m = MsgUpdateParams{} }
func (m *MsgUpdateParams) String() string { return "MsgUpdateParams" }

type MsgUpdateParamsResponse struct{}

func (m *MsgUpdateParamsResponse) ProtoMessage()  {}
func (m *MsgUpdateParamsResponse) Reset()         { *m = MsgUpdateParamsResponse{} }
func (m *MsgUpdateParamsResponse) String() string { return "MsgUpdateParamsResponse" }

// MsgDelegateReputation delegates a portion of governance reputation to another address
type MsgDelegateReputation struct {
	Delegator string         `json:"delegator"`
	Delegatee string         `json:"delegatee"`
	Amount    math.LegacyDec `json:"amount"` // fraction to delegate [0, MaxDelegableRatio]
}

func (m *MsgDelegateReputation) ValidateBasic() error {
	if m.Delegator == "" || m.Delegatee == "" {
		return ErrInvalidAddress
	}
	if m.Delegator == m.Delegatee {
		return ErrSelfDelegation
	}
	if m.Amount.IsNil() || m.Amount.IsNegative() || m.Amount.IsZero() {
		return ErrInvalidWeight
	}
	return nil
}

func (m *MsgDelegateReputation) ProtoMessage()  {}
func (m *MsgDelegateReputation) Reset()         { *m = MsgDelegateReputation{} }
func (m *MsgDelegateReputation) String() string { return "MsgDelegateReputation" }

type MsgDelegateReputationResponse struct{}

func (m *MsgDelegateReputationResponse) ProtoMessage()  {}
func (m *MsgDelegateReputationResponse) Reset()         { *m = MsgDelegateReputationResponse{} }
func (m *MsgDelegateReputationResponse) String() string { return "MsgDelegateReputationResponse" }

// MsgUndelegateReputation removes a reputation delegation
type MsgUndelegateReputation struct {
	Delegator string `json:"delegator"`
	Delegatee string `json:"delegatee"`
}

func (m *MsgUndelegateReputation) ValidateBasic() error {
	if m.Delegator == "" || m.Delegatee == "" {
		return ErrInvalidAddress
	}
	return nil
}

func (m *MsgUndelegateReputation) ProtoMessage()  {}
func (m *MsgUndelegateReputation) Reset()         { *m = MsgUndelegateReputation{} }
func (m *MsgUndelegateReputation) String() string { return "MsgUndelegateReputation" }

type MsgUndelegateReputationResponse struct{}

func (m *MsgUndelegateReputationResponse) ProtoMessage()  {}
func (m *MsgUndelegateReputationResponse) Reset()         { *m = MsgUndelegateReputationResponse{} }
func (m *MsgUndelegateReputationResponse) String() string { return "MsgUndelegateReputationResponse" }

// ============================================================================
// Query types
// ============================================================================

// QueryServer defines the query handlers for x/repgov
type QueryServer interface {
	Params(ctx context.Context, req *QueryParamsRequest) (*QueryParamsResponse, error)
	VoterWeight(ctx context.Context, req *QueryVoterWeightRequest) (*QueryVoterWeightResponse, error)
	AllVoterWeights(ctx context.Context, req *QueryAllVoterWeightsRequest) (*QueryAllVoterWeightsResponse, error)
	Delegations(ctx context.Context, req *QueryDelegationsRequest) (*QueryDelegationsResponse, error)
	TallyOverride(ctx context.Context, req *QueryTallyOverrideRequest) (*QueryTallyOverrideResponse, error)
}

type QueryParamsRequest struct{}
type QueryParamsResponse struct {
	Params Params `json:"params"`
}

type QueryVoterWeightRequest struct {
	Address string `json:"address"`
}
type QueryVoterWeightResponse struct {
	Weight VoterWeight `json:"weight"`
}

type QueryAllVoterWeightsRequest struct{}
type QueryAllVoterWeightsResponse struct {
	Weights []VoterWeight `json:"weights"`
}

type QueryDelegationsRequest struct {
	Address string `json:"address"`
}
type QueryDelegationsResponse struct {
	Delegations []DelegatedReputation `json:"delegations"`
}

type QueryTallyOverrideRequest struct {
	ProposalID uint64 `json:"proposal_id"`
}
type QueryTallyOverrideResponse struct {
	Override TallyOverride `json:"override"`
}

// ============================================================================
// Genesis types
// ============================================================================

type GenesisState struct {
	Params      Params          `json:"params"`
	VoterWeights []VoterWeight `json:"voter_weights"`
	Delegations []DelegatedReputation `json:"delegations"`
}

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:       DefaultParams(),
		VoterWeights: []VoterWeight{},
		Delegations:  []DelegatedReputation{},
	}
}

func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}

// ============================================================================
// Service Registration Stubs
// ============================================================================

func RegisterMsgServer(s interface{}, srv MsgServer)     {}
func RegisterQueryServer(s interface{}, srv QueryServer)  {}
