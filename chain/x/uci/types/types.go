package types

import (
	"context"
	"encoding/json"

	"cosmossdk.io/math"
)

// AdapterStatus represents the lifecycle state of a DePIN adapter
type AdapterStatus string

const (
	AdapterStatusActive       AdapterStatus = "ACTIVE"
	AdapterStatusSuspended    AdapterStatus = "SUSPENDED"
	AdapterStatusDeregistered AdapterStatus = "DEREGISTERED"
)

// Adapter represents a registered DePIN network adapter
type Adapter struct {
	// AdapterID is the unique identifier
	AdapterID uint64 `json:"adapter_id"`

	// Name is the human-readable adapter name (max 64 chars)
	Name string `json:"name"`

	// Owner is the adapter operator's bech32 address
	Owner string `json:"owner"`

	// SchemaCID is the IPFS CID for the contribution schema
	SchemaCID string `json:"schema_cid"`

	// OracleAllowlist is the list of authorized oracle addresses
	OracleAllowlist []string `json:"oracle_allowlist"`

	// Status is the adapter lifecycle state
	Status AdapterStatus `json:"status"`

	// NetworkType identifies the DePIN network (e.g., "helium", "hivemapper", "dimo", "custom")
	NetworkType string `json:"network_type"`

	// CreatedAtHeight is the block height when the adapter was registered
	CreatedAtHeight int64 `json:"created_at_height"`

	// TotalContributions is the total number of contributions submitted through this adapter
	TotalContributions uint64 `json:"total_contributions"`

	// TotalRewardsDistributed is the cumulative rewards distributed through this adapter
	TotalRewardsDistributed math.Int `json:"total_rewards_distributed"`

	// RewardShare is the fraction of PoC reward that goes to the DePIN contributor [0, 1]
	RewardShare math.LegacyDec `json:"reward_share"`

	// Description is an optional description of the adapter (max 256 chars)
	Description string `json:"description"`
}

// NewAdapter creates a new active adapter
func NewAdapter(id uint64, name, owner, schemaCID string, oracles []string, networkType string, height int64, rewardShare math.LegacyDec, description string) Adapter {
	return Adapter{
		AdapterID:               id,
		Name:                    name,
		Owner:                   owner,
		SchemaCID:               schemaCID,
		OracleAllowlist:         oracles,
		Status:                  AdapterStatusActive,
		NetworkType:             networkType,
		CreatedAtHeight:         height,
		TotalContributions:      0,
		TotalRewardsDistributed: math.ZeroInt(),
		RewardShare:             rewardShare,
		Description:             description,
	}
}

// Marshal serializes the Adapter to JSON bytes
func (a Adapter) Marshal() ([]byte, error) { return json.Marshal(a) }

// Unmarshal deserializes the Adapter from JSON bytes
func (a *Adapter) Unmarshal(bz []byte) error { return json.Unmarshal(bz, a) }

// ContributionMapping maps an external DePIN contribution to a PoC contribution
type ContributionMapping struct {
	// AdapterID is the adapter that submitted this contribution
	AdapterID uint64 `json:"adapter_id"`

	// ExternalID is the DePIN-native contribution identifier
	ExternalID string `json:"external_id"`

	// PocContributionID is the corresponding PoC module contribution ID
	PocContributionID uint64 `json:"poc_contribution_id"`

	// Contributor is the DePIN contributor's bech32 address
	Contributor string `json:"contributor"`

	// MappedAtHeight is the block height when the mapping was created
	MappedAtHeight int64 `json:"mapped_at_height"`

	// RewardAmount is the reward distributed for this contribution
	RewardAmount math.Int `json:"reward_amount"`

	// OracleVerified indicates whether oracle attestation has been received
	OracleVerified bool `json:"oracle_verified"`
}

// Marshal serializes the ContributionMapping to JSON bytes
func (cm ContributionMapping) Marshal() ([]byte, error) { return json.Marshal(cm) }

// Unmarshal deserializes the ContributionMapping from JSON bytes
func (cm *ContributionMapping) Unmarshal(bz []byte) error { return json.Unmarshal(bz, cm) }

// AdapterStats stores statistics for a DePIN adapter
type AdapterStats struct {
	// AdapterID is the adapter these stats belong to
	AdapterID uint64 `json:"adapter_id"`

	// TotalSubmissions is the total number of contribution submissions
	TotalSubmissions uint64 `json:"total_submissions"`

	// Successful is the number of successfully processed submissions
	Successful uint64 `json:"successful"`

	// Rejected is the number of rejected submissions
	Rejected uint64 `json:"rejected"`

	// TotalRewards is the cumulative rewards distributed
	TotalRewards math.Int `json:"total_rewards"`

	// LastSubmissionHeight is the block height of the most recent submission
	LastSubmissionHeight int64 `json:"last_submission_height"`

	// AvgReward is the average reward per successful submission
	AvgReward math.LegacyDec `json:"avg_reward"`
}

// NewAdapterStats creates a new zeroed AdapterStats
func NewAdapterStats(adapterID uint64) AdapterStats {
	return AdapterStats{
		AdapterID:            adapterID,
		TotalSubmissions:     0,
		Successful:           0,
		Rejected:             0,
		TotalRewards:         math.ZeroInt(),
		LastSubmissionHeight: 0,
		AvgReward:            math.LegacyZeroDec(),
	}
}

// Marshal serializes the AdapterStats to JSON bytes
func (as AdapterStats) Marshal() ([]byte, error) { return json.Marshal(as) }

// Unmarshal deserializes the AdapterStats from JSON bytes
func (as *AdapterStats) Unmarshal(bz []byte) error { return json.Unmarshal(bz, as) }

// OracleAttestation represents an oracle-signed attestation for DePIN data validity
type OracleAttestation struct {
	// AdapterID is the adapter this attestation is for
	AdapterID uint64 `json:"adapter_id"`

	// BatchID is the batch identifier
	BatchID string `json:"batch_id"`

	// OracleAddress is the oracle's bech32 address
	OracleAddress string `json:"oracle_address"`

	// AttestationHash is the SHA256 hex hash of the attested data
	AttestationHash string `json:"attestation_hash"`

	// ContributionCount is the number of contributions in the attested batch
	ContributionCount uint64 `json:"contribution_count"`

	// AttestedAtHeight is the block height when the attestation was submitted
	AttestedAtHeight int64 `json:"attested_at_height"`

	// Signature is the hex-encoded signature
	Signature string `json:"signature"`
}

// Marshal serializes the OracleAttestation to JSON bytes
func (oa OracleAttestation) Marshal() ([]byte, error) { return json.Marshal(oa) }

// Unmarshal deserializes the OracleAttestation from JSON bytes
func (oa *OracleAttestation) Unmarshal(bz []byte) error { return json.Unmarshal(bz, oa) }

// ============================================================================
// Messages
// ============================================================================

// MsgServer defines the message handlers for x/uci
type MsgServer interface {
	UpdateParams(ctx context.Context, msg *MsgUpdateParams) (*MsgUpdateParamsResponse, error)
	RegisterAdapter(ctx context.Context, msg *MsgRegisterAdapter) (*MsgRegisterAdapterResponse, error)
	SuspendAdapter(ctx context.Context, msg *MsgSuspendAdapter) (*MsgSuspendAdapterResponse, error)
	SubmitDePINContribution(ctx context.Context, msg *MsgSubmitDePINContribution) (*MsgSubmitDePINContributionResponse, error)
	SubmitOracleAttestation(ctx context.Context, msg *MsgSubmitOracleAttestation) (*MsgSubmitOracleAttestationResponse, error)
	UpdateAdapterConfig(ctx context.Context, msg *MsgUpdateAdapterConfig) (*MsgUpdateAdapterConfigResponse, error)
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

// MsgRegisterAdapter registers a new DePIN adapter with schema and oracle list
type MsgRegisterAdapter struct {
	Owner           string         `json:"owner"`
	Name            string         `json:"name"`
	SchemaCID       string         `json:"schema_cid"`
	OracleAllowlist []string       `json:"oracle_allowlist"`
	NetworkType     string         `json:"network_type"`
	RewardShare     math.LegacyDec `json:"reward_share"`
	Description     string         `json:"description"`
}

func (m *MsgRegisterAdapter) ValidateBasic() error {
	if m.Owner == "" {
		return ErrInvalidAdapterConfig
	}
	if m.Name == "" || len(m.Name) > 64 {
		return ErrInvalidAdapterConfig
	}
	if m.SchemaCID == "" {
		return ErrInvalidSchema
	}
	if len(m.OracleAllowlist) == 0 {
		return ErrInvalidAdapterConfig
	}
	if m.NetworkType == "" {
		return ErrInvalidAdapterConfig
	}
	if !m.RewardShare.IsNil() && (m.RewardShare.IsNegative() || m.RewardShare.GT(math.LegacyOneDec())) {
		return ErrInvalidAdapterConfig
	}
	if len(m.Description) > 256 {
		return ErrInvalidAdapterConfig
	}
	return nil
}

func (m *MsgRegisterAdapter) ProtoMessage()  {}
func (m *MsgRegisterAdapter) Reset()         { *m = MsgRegisterAdapter{} }
func (m *MsgRegisterAdapter) String() string { return "MsgRegisterAdapter" }

type MsgRegisterAdapterResponse struct {
	AdapterID uint64 `json:"adapter_id"`
}

func (m *MsgRegisterAdapterResponse) ProtoMessage()  {}
func (m *MsgRegisterAdapterResponse) Reset()         { *m = MsgRegisterAdapterResponse{} }
func (m *MsgRegisterAdapterResponse) String() string { return "MsgRegisterAdapterResponse" }

// MsgSuspendAdapter suspends a DePIN adapter (governance or owner)
type MsgSuspendAdapter struct {
	Authority string `json:"authority"` // governance authority or adapter owner
	AdapterID uint64 `json:"adapter_id"`
}

func (m *MsgSuspendAdapter) ValidateBasic() error {
	if m.Authority == "" {
		return ErrInvalidAuthority
	}
	if m.AdapterID == 0 {
		return ErrAdapterNotFound
	}
	return nil
}

func (m *MsgSuspendAdapter) ProtoMessage()  {}
func (m *MsgSuspendAdapter) Reset()         { *m = MsgSuspendAdapter{} }
func (m *MsgSuspendAdapter) String() string { return "MsgSuspendAdapter" }

type MsgSuspendAdapterResponse struct{}

func (m *MsgSuspendAdapterResponse) ProtoMessage()  {}
func (m *MsgSuspendAdapterResponse) Reset()         { *m = MsgSuspendAdapterResponse{} }
func (m *MsgSuspendAdapterResponse) String() string { return "MsgSuspendAdapterResponse" }

// MsgSubmitDePINContribution submits an external DePIN contribution through an adapter
type MsgSubmitDePINContribution struct {
	Submitter   string `json:"submitter"`
	AdapterID   uint64 `json:"adapter_id"`
	ExternalID  string `json:"external_id"`
	Contributor string `json:"contributor"` // DePIN contributor's bech32 address
	DataHash    string `json:"data_hash"`   // SHA256 hash of contribution data
	DataURI     string `json:"data_uri"`    // URI to contribution data (IPFS, etc.)
	BatchID     string `json:"batch_id"`    // optional batch grouping
}

func (m *MsgSubmitDePINContribution) ValidateBasic() error {
	if m.Submitter == "" {
		return ErrInvalidAdapterConfig
	}
	if m.AdapterID == 0 {
		return ErrAdapterNotFound
	}
	if m.ExternalID == "" {
		return ErrInvalidExternalID
	}
	if m.Contributor == "" {
		return ErrInvalidAdapterConfig
	}
	if m.DataHash == "" {
		return ErrInvalidAdapterConfig
	}
	return nil
}

func (m *MsgSubmitDePINContribution) ProtoMessage()  {}
func (m *MsgSubmitDePINContribution) Reset()         { *m = MsgSubmitDePINContribution{} }
func (m *MsgSubmitDePINContribution) String() string { return "MsgSubmitDePINContribution" }

type MsgSubmitDePINContributionResponse struct {
	PocContributionID uint64 `json:"poc_contribution_id"`
}

func (m *MsgSubmitDePINContributionResponse) ProtoMessage()  {}
func (m *MsgSubmitDePINContributionResponse) Reset()         { *m = MsgSubmitDePINContributionResponse{} }
func (m *MsgSubmitDePINContributionResponse) String() string {
	return "MsgSubmitDePINContributionResponse"
}

// MsgSubmitOracleAttestation submits an oracle attestation for DePIN data validity
type MsgSubmitOracleAttestation struct {
	OracleAddress     string `json:"oracle_address"`
	AdapterID         uint64 `json:"adapter_id"`
	BatchID           string `json:"batch_id"`
	AttestationHash   string `json:"attestation_hash"`
	ContributionCount uint64 `json:"contribution_count"`
	Signature         string `json:"signature"`
}

func (m *MsgSubmitOracleAttestation) ValidateBasic() error {
	if m.OracleAddress == "" {
		return ErrOracleNotAuthorized
	}
	if m.AdapterID == 0 {
		return ErrAdapterNotFound
	}
	if m.BatchID == "" {
		return ErrBatchNotFound
	}
	if m.AttestationHash == "" || len(m.AttestationHash) != 64 {
		return ErrInvalidAttestation
	}
	if m.Signature == "" {
		return ErrInvalidOracleSignature
	}
	return nil
}

func (m *MsgSubmitOracleAttestation) ProtoMessage()  {}
func (m *MsgSubmitOracleAttestation) Reset()         { *m = MsgSubmitOracleAttestation{} }
func (m *MsgSubmitOracleAttestation) String() string { return "MsgSubmitOracleAttestation" }

type MsgSubmitOracleAttestationResponse struct{}

func (m *MsgSubmitOracleAttestationResponse) ProtoMessage()  {}
func (m *MsgSubmitOracleAttestationResponse) Reset()         { *m = MsgSubmitOracleAttestationResponse{} }
func (m *MsgSubmitOracleAttestationResponse) String() string {
	return "MsgSubmitOracleAttestationResponse"
}

// MsgUpdateAdapterConfig allows the adapter owner to update oracle list, schema, and reward share
type MsgUpdateAdapterConfig struct {
	Owner           string         `json:"owner"`
	AdapterID       uint64         `json:"adapter_id"`
	SchemaCID       string         `json:"schema_cid"`
	OracleAllowlist []string       `json:"oracle_allowlist"`
	RewardShare     math.LegacyDec `json:"reward_share"`
}

func (m *MsgUpdateAdapterConfig) ValidateBasic() error {
	if m.Owner == "" {
		return ErrInvalidAdapterConfig
	}
	if m.AdapterID == 0 {
		return ErrAdapterNotFound
	}
	if !m.RewardShare.IsNil() && (m.RewardShare.IsNegative() || m.RewardShare.GT(math.LegacyOneDec())) {
		return ErrInvalidAdapterConfig
	}
	return nil
}

func (m *MsgUpdateAdapterConfig) ProtoMessage()  {}
func (m *MsgUpdateAdapterConfig) Reset()         { *m = MsgUpdateAdapterConfig{} }
func (m *MsgUpdateAdapterConfig) String() string { return "MsgUpdateAdapterConfig" }

type MsgUpdateAdapterConfigResponse struct{}

func (m *MsgUpdateAdapterConfigResponse) ProtoMessage()  {}
func (m *MsgUpdateAdapterConfigResponse) Reset()         { *m = MsgUpdateAdapterConfigResponse{} }
func (m *MsgUpdateAdapterConfigResponse) String() string { return "MsgUpdateAdapterConfigResponse" }

// ============================================================================
// Queries
// ============================================================================

// QueryServer defines the query handlers for x/uci
type QueryServer interface {
	Params(ctx context.Context, req *QueryParamsRequest) (*QueryParamsResponse, error)
	Adapter(ctx context.Context, req *QueryAdapterRequest) (*QueryAdapterResponse, error)
	AdaptersByOwner(ctx context.Context, req *QueryAdaptersByOwnerRequest) (*QueryAdaptersByOwnerResponse, error)
	AllAdapters(ctx context.Context, req *QueryAllAdaptersRequest) (*QueryAllAdaptersResponse, error)
	ContributionMapping(ctx context.Context, req *QueryContributionMappingRequest) (*QueryContributionMappingResponse, error)
	AdapterStats(ctx context.Context, req *QueryAdapterStatsRequest) (*QueryAdapterStatsResponse, error)
}

type QueryParamsRequest struct{}
type QueryParamsResponse struct {
	Params Params `json:"params"`
}

type QueryAdapterRequest struct {
	AdapterID uint64 `json:"adapter_id"`
}
type QueryAdapterResponse struct {
	Adapter Adapter `json:"adapter"`
}

type QueryAdaptersByOwnerRequest struct {
	Owner string `json:"owner"`
}
type QueryAdaptersByOwnerResponse struct {
	Adapters []Adapter `json:"adapters"`
}

type QueryAllAdaptersRequest struct{}
type QueryAllAdaptersResponse struct {
	Adapters []Adapter `json:"adapters"`
}

type QueryContributionMappingRequest struct {
	AdapterID  uint64 `json:"adapter_id"`
	ExternalID string `json:"external_id"`
}
type QueryContributionMappingResponse struct {
	Mapping ContributionMapping `json:"mapping"`
}

type QueryAdapterStatsRequest struct {
	AdapterID uint64 `json:"adapter_id"`
}
type QueryAdapterStatsResponse struct {
	Stats AdapterStats `json:"stats"`
}

// ============================================================================
// Genesis
// ============================================================================

// GenesisState defines the genesis state for x/uci
type GenesisState struct {
	Params               Params                `json:"params"`
	Adapters             []Adapter             `json:"adapters"`
	NextAdapterID        uint64                `json:"next_adapter_id"`
	ContributionMappings []ContributionMapping `json:"contribution_mappings"`
}

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:               DefaultParams(),
		Adapters:             []Adapter{},
		NextAdapterID:        1,
		ContributionMappings: []ContributionMapping{},
	}
}

// Validate performs genesis state validation
func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}

// ============================================================================
// Service Registration Stubs
// ============================================================================

// RegisterMsgServer registers the MsgServer implementation with the gRPC server.
// For manual types (non-protoc-generated), this is a no-op placeholder.
func RegisterMsgServer(s interface{}, srv MsgServer) {}

// RegisterQueryServer registers the QueryServer implementation with the gRPC server.
func RegisterQueryServer(s interface{}, srv QueryServer) {}
