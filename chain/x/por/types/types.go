package types

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
)

// Dec is an alias for math.LegacyDec, used by CLI parsing
type Dec = math.LegacyDec

// ParseDec parses a string decimal (e.g., "0.95") into a Dec
func ParseDec(s string) (Dec, error) {
	return math.LegacyNewDecFromStr(s)
}

// ============================================================================
// Enumerations
// ============================================================================

// BatchStatus represents the lifecycle state of a batch commitment
type BatchStatus uint32

const (
	BatchStatusSubmitted BatchStatus = 0 // Initial state after MsgSubmitBatch
	BatchStatusPending   BatchStatus = 1 // Minimum attestations met + quorum reached
	BatchStatusFinalized BatchStatus = 2 // Challenge window passed with no valid challenges
	BatchStatusRejected  BatchStatus = 3 // Valid challenge proven, batch is invalid
)

func (s BatchStatus) String() string {
	switch s {
	case BatchStatusSubmitted:
		return "SUBMITTED"
	case BatchStatusPending:
		return "PENDING"
	case BatchStatusFinalized:
		return "FINALIZED"
	case BatchStatusRejected:
		return "REJECTED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// IsValid returns true if the status is a known value
func (s BatchStatus) IsValid() bool {
	return s <= BatchStatusRejected
}

// ChallengeType represents the category of fraud proof
type ChallengeType uint32

const (
	ChallengeTypeInvalidRoot     ChallengeType = 0 // Merkle root does not match records
	ChallengeTypeDoubleInclusion ChallengeType = 1 // Same record included in multiple batches
	ChallengeTypeMissingRecord   ChallengeType = 2 // Claimed records missing from tree
	ChallengeTypeInvalidSchema   ChallengeType = 3 // Records do not conform to registered schema
)

func (ct ChallengeType) String() string {
	switch ct {
	case ChallengeTypeInvalidRoot:
		return "INVALID_ROOT"
	case ChallengeTypeDoubleInclusion:
		return "DOUBLE_INCLUSION"
	case ChallengeTypeMissingRecord:
		return "MISSING_RECORD"
	case ChallengeTypeInvalidSchema:
		return "INVALID_SCHEMA"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", ct)
	}
}

func (ct ChallengeType) IsValid() bool {
	return ct <= ChallengeTypeInvalidSchema
}

// ChallengeStatus represents the resolution state of a challenge
type ChallengeStatus uint32

const (
	ChallengeStatusOpen            ChallengeStatus = 0 // Challenge submitted, not yet resolved
	ChallengeStatusResolvedValid   ChallengeStatus = 1 // Challenge was correct (fraud proven)
	ChallengeStatusResolvedInvalid ChallengeStatus = 2 // Challenge was wrong (no fraud)
)

func (cs ChallengeStatus) String() string {
	switch cs {
	case ChallengeStatusOpen:
		return "OPEN"
	case ChallengeStatusResolvedValid:
		return "RESOLVED_VALID"
	case ChallengeStatusResolvedInvalid:
		return "RESOLVED_INVALID"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", cs)
	}
}

// AppStatus represents the lifecycle state of a registered application
type AppStatus uint32

const (
	AppStatusActive       AppStatus = 0
	AppStatusSuspended    AppStatus = 1
	AppStatusDeregistered AppStatus = 2
)

func (as AppStatus) String() string {
	switch as {
	case AppStatusActive:
		return "ACTIVE"
	case AppStatusSuspended:
		return "SUSPENDED"
	case AppStatusDeregistered:
		return "DEREGISTERED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", as)
	}
}

// ============================================================================
// State Types
// ============================================================================

// App represents a registered application that submits record batches
type App struct {
	AppId           uint64    `json:"app_id"`
	Name            string    `json:"name"`
	Owner           string    `json:"owner"`            // bech32 address of app owner
	SchemaCid       string    `json:"schema_cid"`       // IPFS CID of record schema definition
	ChallengePeriod int64     `json:"challenge_period"`  // challenge window duration in seconds
	MinVerifiers    uint32    `json:"min_verifiers"`     // minimum attestations required
	Status          AppStatus `json:"status"`
	CreatedAt       int64     `json:"created_at"`        // unix timestamp
}

func (a *App) Reset()         { *a = App{} }
func (a *App) String() string { return fmt.Sprintf("%+v", *a) }
func (*App) ProtoMessage()    {}

// VerifierMember represents a single verifier within a verifier set
type VerifierMember struct {
	Address  string   `json:"address"`   // bech32 address
	Weight   math.Int `json:"weight"`    // stake-weighted voting power
	JoinedAt int64    `json:"joined_at"` // unix timestamp
}

func (vm *VerifierMember) Reset()         { *vm = VerifierMember{} }
func (vm *VerifierMember) String() string { return fmt.Sprintf("%+v", *vm) }
func (*VerifierMember) ProtoMessage()     {}

// VerifierSet represents an epoch-scoped set of verifiers for an application
type VerifierSet struct {
	VerifierSetId   uint64           `json:"verifier_set_id"`
	Epoch           uint64           `json:"epoch"`
	Members         []VerifierMember `json:"members"`
	MinAttestations uint32           `json:"min_attestations"` // minimum number of attestations needed
	QuorumPct       math.LegacyDec  `json:"quorum_pct"`       // required weighted quorum (e.g. 0.67)
	AppId           uint64           `json:"app_id"`            // associated application
}

func (vs *VerifierSet) Reset()         { *vs = VerifierSet{} }
func (vs *VerifierSet) String() string { return fmt.Sprintf("%+v", *vs) }
func (*VerifierSet) ProtoMessage()     {}

// GetTotalWeight returns the sum of all member weights
func (vs *VerifierSet) GetTotalWeight() math.Int {
	total := math.ZeroInt()
	for _, m := range vs.Members {
		total = total.Add(m.Weight)
	}
	return total
}

// IsMember checks if an address is a member of this verifier set
func (vs *VerifierSet) IsMember(addr string) bool {
	for _, m := range vs.Members {
		if m.Address == addr {
			return true
		}
	}
	return false
}

// GetMemberWeight returns the weight of a member, or zero if not a member
func (vs *VerifierSet) GetMemberWeight(addr string) math.Int {
	for _, m := range vs.Members {
		if m.Address == addr {
			return m.Weight
		}
	}
	return math.ZeroInt()
}

// BatchCommitment represents an on-chain anchor for a batch of off-chain records
type BatchCommitment struct {
	BatchId          uint64      `json:"batch_id"`
	Epoch            uint64      `json:"epoch"`
	RecordMerkleRoot []byte      `json:"record_merkle_root"` // SHA256 merkle root
	RecordCount      uint64      `json:"record_count"`
	AppId            uint64      `json:"app_id"`
	VerifierSetId    uint64      `json:"verifier_set_id"`
	Submitter        string      `json:"submitter"`          // bech32 address
	ChallengeEndTime int64       `json:"challenge_end_time"` // unix timestamp
	Status           BatchStatus `json:"status"`
	SubmittedAt      int64       `json:"submitted_at"`       // unix timestamp
	FinalizedAt      int64       `json:"finalized_at"`       // 0 if not finalized

	// F2/F6: DA layer commitment hash (32 bytes, optional)
	DACommitmentHash []byte `json:"da_commitment_hash,omitempty"`

	// F8: PoSeq commitment hash reference (32 bytes, optional)
	PoSeqCommitmentHash []byte `json:"poseq_commitment_hash,omitempty"`
}

func (bc *BatchCommitment) Reset()         { *bc = BatchCommitment{} }
func (bc *BatchCommitment) String() string { return fmt.Sprintf("%+v", *bc) }
func (*BatchCommitment) ProtoMessage()     {}

// IsTerminal returns true if the batch is in a final state (FINALIZED or REJECTED)
func (bc *BatchCommitment) IsTerminal() bool {
	return bc.Status == BatchStatusFinalized || bc.Status == BatchStatusRejected
}

// Attestation represents a verifier's sign-off on a batch commitment
type Attestation struct {
	BatchId          uint64         `json:"batch_id"`
	VerifierAddress  string         `json:"verifier_address"`  // bech32 address
	Signature        []byte         `json:"signature"`
	ConfidenceWeight math.LegacyDec `json:"confidence_weight"` // [0, 1] confidence factor
	Timestamp        int64          `json:"timestamp"`         // unix timestamp
}

func (a *Attestation) Reset()         { *a = Attestation{} }
func (a *Attestation) String() string { return fmt.Sprintf("%+v", *a) }
func (*Attestation) ProtoMessage()    {}

// Challenge represents a fraud proof submitted against a pending batch
type Challenge struct {
	ChallengeId   uint64          `json:"challenge_id"`
	BatchId       uint64          `json:"batch_id"`
	Challenger    string          `json:"challenger"`      // bech32 address
	ChallengeType ChallengeType   `json:"challenge_type"`
	ProofData     []byte          `json:"proof_data"`      // encoded fraud proof
	Status        ChallengeStatus `json:"status"`
	Timestamp     int64           `json:"timestamp"`       // unix timestamp of submission
	ResolvedAt    int64           `json:"resolved_at"`     // 0 if not resolved
	ResolvedBy    string          `json:"resolved_by"`     // governance or auto
	BondAmount    math.Int        `json:"bond_amount"`     // F4: bond locked for this challenge
}

func (c *Challenge) Reset()         { *c = Challenge{} }
func (c *Challenge) String() string { return fmt.Sprintf("%+v", *c) }
func (*Challenge) ProtoMessage()    {}

// VerifierReputation tracks the accuracy and reliability of a verifier
type VerifierReputation struct {
	Address             string   `json:"address"`
	TotalAttestations   uint64   `json:"total_attestations"`
	CorrectAttestations uint64   `json:"correct_attestations"`
	SlashedCount        uint64   `json:"slashed_count"`
	ReputationScore     math.Int `json:"reputation_score"` // derived metric
}

func (vr *VerifierReputation) Reset()         { *vr = VerifierReputation{} }
func (vr *VerifierReputation) String() string { return fmt.Sprintf("%+v", *vr) }
func (*VerifierReputation) ProtoMessage()     {}

// PoSeqCommitment represents a registered sequencer state root (F8)
type PoSeqCommitment struct {
	CommitmentHash []byte `json:"commitment_hash"` // SHA256 hash of the PoSeq state root
	StateRoot      []byte `json:"state_root"`      // The actual state root
	SequencerAddr  string `json:"sequencer_addr"`  // Bech32 address of the submitting sequencer
	BlockHeight    uint64 `json:"block_height"`    // PoSeq block height
	Timestamp      int64  `json:"timestamp"`
}

func (p *PoSeqCommitment) Reset()         { *p = PoSeqCommitment{} }
func (p *PoSeqCommitment) String() string { return fmt.Sprintf("%+v", *p) }
func (*PoSeqCommitment) ProtoMessage()    {}

// PoSeqSequencerSet defines the known set of valid sequencers (F8)
type PoSeqSequencerSet struct {
	Sequencers []string `json:"sequencers"` // Bech32 addresses of authorized sequencers
	Threshold  uint32   `json:"threshold"`  // Minimum attestations required
}

func (p *PoSeqSequencerSet) Reset()         { *p = PoSeqSequencerSet{} }
func (p *PoSeqSequencerSet) String() string { return fmt.Sprintf("%+v", *p) }
func (*PoSeqSequencerSet) ProtoMessage()    {}

// IsAuthorizedSequencer checks if an address is in the sequencer set
func (p *PoSeqSequencerSet) IsAuthorizedSequencer(addr string) bool {
	for _, s := range p.Sequencers {
		if s == addr {
			return true
		}
	}
	return false
}

// ============================================================================
// Message Types
// ============================================================================

// MsgRegisterApp registers a new application in the PoR module
type MsgRegisterApp struct {
	Owner           string `json:"owner"`
	Name            string `json:"name"`
	SchemaCid       string `json:"schema_cid"`
	ChallengePeriod int64  `json:"challenge_period"`
	MinVerifiers    uint32 `json:"min_verifiers"`
}

type MsgRegisterAppResponse struct {
	AppId uint64 `json:"app_id"`
}

func (m *MsgRegisterApp) Reset()             { *m = MsgRegisterApp{} }
func (m *MsgRegisterApp) String() string     { return fmt.Sprintf("%+v", *m) }
func (*MsgRegisterApp) ProtoMessage()        {}
func (m *MsgRegisterAppResponse) Reset()     { *m = MsgRegisterAppResponse{} }
func (m *MsgRegisterAppResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgRegisterAppResponse) ProtoMessage() {}

// MsgCreateVerifierSet creates a new verifier set for an application
type MsgCreateVerifierSet struct {
	Creator         string           `json:"creator"`
	AppId           uint64           `json:"app_id"`
	Epoch           uint64           `json:"epoch"`
	Members         []VerifierMember `json:"members"`
	MinAttestations uint32           `json:"min_attestations"`
	QuorumPct       math.LegacyDec  `json:"quorum_pct"`
}

type MsgCreateVerifierSetResponse struct {
	VerifierSetId uint64 `json:"verifier_set_id"`
}

func (m *MsgCreateVerifierSet) Reset()             { *m = MsgCreateVerifierSet{} }
func (m *MsgCreateVerifierSet) String() string     { return fmt.Sprintf("%+v", *m) }
func (*MsgCreateVerifierSet) ProtoMessage()        {}
func (m *MsgCreateVerifierSetResponse) Reset()     { *m = MsgCreateVerifierSetResponse{} }
func (m *MsgCreateVerifierSetResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgCreateVerifierSetResponse) ProtoMessage() {}

// MsgSubmitBatch submits a batch commitment (merkle root) to the chain
type MsgSubmitBatch struct {
	Submitter        string   `json:"submitter"`
	AppId            uint64   `json:"app_id"`
	Epoch            uint64   `json:"epoch"`
	RecordMerkleRoot []byte   `json:"record_merkle_root"`
	RecordCount      uint64   `json:"record_count"`
	VerifierSetId    uint64   `json:"verifier_set_id"`
	DACommitmentHash []byte   `json:"da_commitment_hash,omitempty"`    // F2/F6: optional DA layer commitment
	LeafHashes       [][]byte `json:"leaf_hashes,omitempty"`           // F3: optional per-record leaf hashes
	PoSeqCommitmentHash []byte `json:"poseq_commitment_hash,omitempty"` // F8: optional PoSeq reference
}

type MsgSubmitBatchResponse struct {
	BatchId uint64 `json:"batch_id"`
}

func (m *MsgSubmitBatch) Reset()             { *m = MsgSubmitBatch{} }
func (m *MsgSubmitBatch) String() string     { return fmt.Sprintf("%+v", *m) }
func (*MsgSubmitBatch) ProtoMessage()        {}
func (m *MsgSubmitBatchResponse) Reset()     { *m = MsgSubmitBatchResponse{} }
func (m *MsgSubmitBatchResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgSubmitBatchResponse) ProtoMessage() {}

// MsgSubmitAttestation submits a verifier attestation for a batch
type MsgSubmitAttestation struct {
	Verifier         string         `json:"verifier"`
	BatchId          uint64         `json:"batch_id"`
	Signature        []byte         `json:"signature"`
	ConfidenceWeight math.LegacyDec `json:"confidence_weight"`
}

type MsgSubmitAttestationResponse struct {
	AttestationCount uint32 `json:"attestation_count"`
	MetQuorum        bool   `json:"met_quorum"`
}

func (m *MsgSubmitAttestation) Reset()             { *m = MsgSubmitAttestation{} }
func (m *MsgSubmitAttestation) String() string     { return fmt.Sprintf("%+v", *m) }
func (*MsgSubmitAttestation) ProtoMessage()        {}
func (m *MsgSubmitAttestationResponse) Reset()     { *m = MsgSubmitAttestationResponse{} }
func (m *MsgSubmitAttestationResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgSubmitAttestationResponse) ProtoMessage() {}

// MsgChallengeBatch submits a fraud proof against a pending batch
type MsgChallengeBatch struct {
	Challenger    string        `json:"challenger"`
	BatchId       uint64        `json:"batch_id"`
	ChallengeType ChallengeType `json:"challenge_type"`
	ProofData     []byte        `json:"proof_data"`
}

type MsgChallengeBatchResponse struct {
	ChallengeId uint64 `json:"challenge_id"`
}

func (m *MsgChallengeBatch) Reset()             { *m = MsgChallengeBatch{} }
func (m *MsgChallengeBatch) String() string     { return fmt.Sprintf("%+v", *m) }
func (*MsgChallengeBatch) ProtoMessage()        {}
func (m *MsgChallengeBatchResponse) Reset()     { *m = MsgChallengeBatchResponse{} }
func (m *MsgChallengeBatchResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (*MsgChallengeBatchResponse) ProtoMessage() {}

// MsgFinalizeBatch finalizes a batch after the challenge window expires
type MsgFinalizeBatch struct {
	Authority string `json:"authority"` // governance module or EndBlocker
	BatchId   uint64 `json:"batch_id"`
}

type MsgFinalizeBatchResponse struct{}

func (m *MsgFinalizeBatch) Reset()             { *m = MsgFinalizeBatch{} }
func (m *MsgFinalizeBatch) String() string     { return fmt.Sprintf("%+v", *m) }
func (*MsgFinalizeBatch) ProtoMessage()        {}
func (m *MsgFinalizeBatchResponse) Reset()     { *m = MsgFinalizeBatchResponse{} }
func (m *MsgFinalizeBatchResponse) String() string { return "" }
func (*MsgFinalizeBatchResponse) ProtoMessage() {}

// MsgUpdateParams updates the module parameters (governance only)
type MsgUpdateParams struct {
	Authority string `json:"authority"`
	Params    Params `json:"params"`
}

type MsgUpdateParamsResponse struct{}

func (m *MsgUpdateParams) Reset()             { *m = MsgUpdateParams{} }
func (m *MsgUpdateParams) String() string     { return fmt.Sprintf("%+v", *m) }
func (*MsgUpdateParams) ProtoMessage()        {}
func (m *MsgUpdateParamsResponse) Reset()     { *m = MsgUpdateParamsResponse{} }
func (m *MsgUpdateParamsResponse) String() string { return "" }
func (*MsgUpdateParamsResponse) ProtoMessage() {}

// MsgRegisterPoSeqCommitment registers a PoSeq state root commitment on-chain (F8)
type MsgRegisterPoSeqCommitment struct {
	Authority      string `json:"authority"`       // governance or authorized sequencer
	CommitmentHash []byte `json:"commitment_hash"` // SHA256 hash
	StateRoot      []byte `json:"state_root"`      // The actual state root
	BlockHeight    uint64 `json:"block_height"`    // PoSeq block height
}

type MsgRegisterPoSeqCommitmentResponse struct{}

func (m *MsgRegisterPoSeqCommitment) Reset()             { *m = MsgRegisterPoSeqCommitment{} }
func (m *MsgRegisterPoSeqCommitment) String() string     { return fmt.Sprintf("%+v", *m) }
func (*MsgRegisterPoSeqCommitment) ProtoMessage()        {}
func (m *MsgRegisterPoSeqCommitmentResponse) Reset()     { *m = MsgRegisterPoSeqCommitmentResponse{} }
func (m *MsgRegisterPoSeqCommitmentResponse) String() string { return "" }
func (*MsgRegisterPoSeqCommitmentResponse) ProtoMessage() {}

// MsgUpdatePoSeqSequencerSet updates the authorized sequencer set (F8, governance only)
type MsgUpdatePoSeqSequencerSet struct {
	Authority  string   `json:"authority"`
	Sequencers []string `json:"sequencers"`
	Threshold  uint32   `json:"threshold"`
}

type MsgUpdatePoSeqSequencerSetResponse struct{}

func (m *MsgUpdatePoSeqSequencerSet) Reset()             { *m = MsgUpdatePoSeqSequencerSet{} }
func (m *MsgUpdatePoSeqSequencerSet) String() string     { return fmt.Sprintf("%+v", *m) }
func (*MsgUpdatePoSeqSequencerSet) ProtoMessage()        {}
func (m *MsgUpdatePoSeqSequencerSetResponse) Reset()     { *m = MsgUpdatePoSeqSequencerSetResponse{} }
func (m *MsgUpdatePoSeqSequencerSetResponse) String() string { return "" }
func (*MsgUpdatePoSeqSequencerSetResponse) ProtoMessage() {}

// ============================================================================
// Query Types
// ============================================================================

type QueryParamsRequest struct{}
type QueryParamsResponse struct {
	Params Params `json:"params"`
}

type QueryAppRequest struct {
	AppId uint64 `json:"app_id"`
}
type QueryAppResponse struct {
	App App `json:"app"`
}

type QueryAppsRequest struct{}
type QueryAppsResponse struct {
	Apps []App `json:"apps"`
}

type QueryVerifierSetRequest struct {
	VerifierSetId uint64 `json:"verifier_set_id"`
}
type QueryVerifierSetResponse struct {
	VerifierSet VerifierSet `json:"verifier_set"`
}

type QueryBatchRequest struct {
	BatchId uint64 `json:"batch_id"`
}
type QueryBatchResponse struct {
	Batch BatchCommitment `json:"batch"`
}

type QueryBatchesRequest struct {
	Status *BatchStatus `json:"status,omitempty"` // optional filter
}
type QueryBatchesResponse struct {
	Batches []BatchCommitment `json:"batches"`
}

type QueryBatchesByEpochRequest struct {
	Epoch uint64 `json:"epoch"`
}
type QueryBatchesByEpochResponse struct {
	Batches []BatchCommitment `json:"batches"`
}

type QueryAttestationsRequest struct {
	BatchId uint64 `json:"batch_id"`
}
type QueryAttestationsResponse struct {
	Attestations []Attestation `json:"attestations"`
}

type QueryChallengesRequest struct {
	BatchId uint64 `json:"batch_id"`
}
type QueryChallengesResponse struct {
	Challenges []Challenge `json:"challenges"`
}

type QueryVerifierReputationRequest struct {
	Address string `json:"address"`
}
type QueryVerifierReputationResponse struct {
	Reputation VerifierReputation `json:"reputation"`
}

// ============================================================================
// Service Interfaces
// ============================================================================

// MsgServer defines the msg service interface for the PoR module
type MsgServer interface {
	RegisterApp(ctx context.Context, msg *MsgRegisterApp) (*MsgRegisterAppResponse, error)
	CreateVerifierSet(ctx context.Context, msg *MsgCreateVerifierSet) (*MsgCreateVerifierSetResponse, error)
	SubmitBatch(ctx context.Context, msg *MsgSubmitBatch) (*MsgSubmitBatchResponse, error)
	SubmitAttestation(ctx context.Context, msg *MsgSubmitAttestation) (*MsgSubmitAttestationResponse, error)
	ChallengeBatch(ctx context.Context, msg *MsgChallengeBatch) (*MsgChallengeBatchResponse, error)
	FinalizeBatch(ctx context.Context, msg *MsgFinalizeBatch) (*MsgFinalizeBatchResponse, error)
	UpdateParams(ctx context.Context, msg *MsgUpdateParams) (*MsgUpdateParamsResponse, error)
	// F8: PoSeq commitment management
	RegisterPoSeqCommitment(ctx context.Context, msg *MsgRegisterPoSeqCommitment) (*MsgRegisterPoSeqCommitmentResponse, error)
	UpdatePoSeqSequencerSet(ctx context.Context, msg *MsgUpdatePoSeqSequencerSet) (*MsgUpdatePoSeqSequencerSetResponse, error)
}

// QueryServer defines the query service interface for the PoR module
type QueryServer interface {
	Params(ctx context.Context, req *QueryParamsRequest) (*QueryParamsResponse, error)
	App(ctx context.Context, req *QueryAppRequest) (*QueryAppResponse, error)
	Apps(ctx context.Context, req *QueryAppsRequest) (*QueryAppsResponse, error)
	VerifierSet(ctx context.Context, req *QueryVerifierSetRequest) (*QueryVerifierSetResponse, error)
	Batch(ctx context.Context, req *QueryBatchRequest) (*QueryBatchResponse, error)
	Batches(ctx context.Context, req *QueryBatchesRequest) (*QueryBatchesResponse, error)
	BatchesByEpoch(ctx context.Context, req *QueryBatchesByEpochRequest) (*QueryBatchesByEpochResponse, error)
	Attestations(ctx context.Context, req *QueryAttestationsRequest) (*QueryAttestationsResponse, error)
	Challenges(ctx context.Context, req *QueryChallengesRequest) (*QueryChallengesResponse, error)
	VerifierReputation(ctx context.Context, req *QueryVerifierReputationRequest) (*QueryVerifierReputationResponse, error)
}

// ============================================================================
// Service Registration Stubs
// ============================================================================

// RegisterMsgServer registers the MsgServer implementation with the gRPC server.
// For manual types (non-protoc-generated), this is a no-op placeholder.
// The actual registration happens via module.Configurator in RegisterServices.
func RegisterMsgServer(s interface{}, srv MsgServer) {
	// In manual types mode, registration is handled by the module's RegisterServices method.
	// When proto-generated code is available, this will be replaced with proper gRPC registration.
}

// RegisterQueryServer registers the QueryServer implementation with the gRPC server.
func RegisterQueryServer(s interface{}, srv QueryServer) {
	// Same as above - placeholder for manual types mode.
}
