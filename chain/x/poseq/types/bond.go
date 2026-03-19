package types

// BondState represents the lifecycle state of an operator bond.
//
// Transitions:
//
//	Active → PartiallySlashed (slash executed, bond reduced but not exhausted)
//	Active → Jailed (slash + jailed status)
//	PartiallySlashed → Jailed
//	PartiallySlashed → Active (bond topped up — future extension)
//	Jailed → Exhausted (further slashing depletes remaining bond)
//	Jailed → Retired (operator withdraws, bond cleaned up)
//	* → Retired (operator withdraws normally)
type BondState string

const (
	BondStateActive           BondState = "Active"
	BondStatePartiallySlashed BondState = "PartiallySlashed"
	BondStateJailed           BondState = "Jailed"
	BondStateExhausted        BondState = "Exhausted"
	BondStateRetired          BondState = "Retired"
)

// OperatorBond represents a PoSeq operator's bond registered on the slow lane.
//
// Design: bond is keyed by operator_address (bech32). One operator may bond
// multiple nodes by registering separate bonds per node_id.
type OperatorBond struct {
	// OperatorAddress is the bech32 Cosmos address of the operator.
	OperatorAddress string `json:"operator_address"`

	// NodeID is the 32-byte hex node identity this bond covers.
	NodeID string `json:"node_id"`

	// BondAmount is the declared bond in uomni (or chain denom).
	BondAmount uint64 `json:"bond_amount"`

	// BondDenom is the token denomination (e.g. "uomni").
	BondDenom string `json:"bond_denom"`

	// BondedSinceEpoch is the epoch when the bond was first registered.
	BondedSinceEpoch uint64 `json:"bonded_since_epoch"`

	// IsActive indicates the bond is currently declared (not withdrawn).
	IsActive bool `json:"is_active"`

	// WithdrawnAtEpoch is set when the operator withdraws their bond declaration.
	// Zero means not withdrawn.
	WithdrawnAtEpoch uint64 `json:"withdrawn_at_epoch,omitempty"`

	// State tracks the bond lifecycle. Defaults to Active.
	State BondState `json:"state,omitempty"`

	// SlashedAmount is the cumulative amount slashed from this bond, in uomni.
	SlashedAmount uint64 `json:"slashed_amount,omitempty"`

	// AvailableBond is BondAmount minus SlashedAmount. Maintained in sync
	// on every slash execution.
	AvailableBond uint64 `json:"available_bond,omitempty"`

	// LastSlashEpoch is the epoch of the most recent slash execution.
	// Zero if never slashed.
	LastSlashEpoch uint64 `json:"last_slash_epoch,omitempty"`

	// SlashCount is the total number of slash executions against this bond.
	SlashCount uint32 `json:"slash_count,omitempty"`
}

// MsgDeclareOperatorBond is a governance-visible bond declaration.
// Does not move tokens in Phase 5 — just associates operator + node + amount.
type MsgDeclareOperatorBond struct {
	OperatorAddress string `json:"operator_address"`
	NodeID          string `json:"node_id"`
	BondAmount      uint64 `json:"bond_amount"`
	BondDenom       string `json:"bond_denom"`
	Epoch           uint64 `json:"epoch"`
}

func (m *MsgDeclareOperatorBond) ProtoMessage()  {}
func (m *MsgDeclareOperatorBond) Reset()         {}
func (m *MsgDeclareOperatorBond) String() string { return m.OperatorAddress }

// MsgWithdrawOperatorBond withdraws the operator's bond declaration.
type MsgWithdrawOperatorBond struct {
	OperatorAddress string `json:"operator_address"`
	NodeID          string `json:"node_id"`
	Epoch           uint64 `json:"epoch"`
}

func (m *MsgWithdrawOperatorBond) ProtoMessage()  {}
func (m *MsgWithdrawOperatorBond) Reset()         {}
func (m *MsgWithdrawOperatorBond) String() string { return m.OperatorAddress }

// SlashQueueEntry is a pending slash action waiting for governance review
// or automatic execution (future phases).
//
// Phase 5: queue entries are created but NOT executed. Execution is Phase 6.
type SlashQueueEntry struct {
	// EntryID is SHA256(operator_address | node_id_hex | epoch_be(8))
	EntryID []byte `json:"entry_id"`

	OperatorAddress string `json:"operator_address"`
	NodeID          string `json:"node_id"`

	// EvidenceRef is the packet_hash of the triggering EvidencePacket.
	EvidenceRef []byte `json:"evidence_ref"`

	// Severity: "Minor", "Moderate", "Severe", "Critical"
	Severity string `json:"severity"`

	// SlashBps is the recommended slash in basis points (0–10000).
	SlashBps uint32 `json:"slash_bps"`

	// Epoch when the entry was created.
	Epoch uint64 `json:"epoch"`

	// Reason is a human-readable description.
	Reason string `json:"reason"`

	// Executed is false until Phase 6 executes the slash.
	Executed bool `json:"executed"`
}

func (s *SlashQueueEntry) ProtoMessage()  {}
func (s *SlashQueueEntry) Reset()         {}
func (s *SlashQueueEntry) String() string { return s.NodeID }
