package types

// Proto interface stubs for manual message types.
// These allow poseq messages to satisfy the sdk.Msg interface
// required by tx.GenerateOrBroadcastTxCLI without full protobuf generation.

// ─── MsgSubmitExportBatch ────────────────────────────────────

func (*MsgSubmitExportBatch) ProtoMessage()             {}
func (*MsgSubmitExportBatch) Reset()                    {}
func (m *MsgSubmitExportBatch) String() string          { return "MsgSubmitExportBatch" }

// ─── MsgSubmitEvidencePacket ─────────────────────────────────

func (*MsgSubmitEvidencePacket) ProtoMessage()           {}
func (*MsgSubmitEvidencePacket) Reset()                  {}
func (m *MsgSubmitEvidencePacket) String() string        { return "MsgSubmitEvidencePacket" }

// ─── MsgSubmitCheckpointAnchor ───────────────────────────────

func (*MsgSubmitCheckpointAnchor) ProtoMessage()         {}
func (*MsgSubmitCheckpointAnchor) Reset()                {}
func (m *MsgSubmitCheckpointAnchor) String() string      { return "MsgSubmitCheckpointAnchor" }

// ─── MsgUpdateParams ─────────────────────────────────────────

func (*MsgUpdateParams) ProtoMessage()                   {}
func (*MsgUpdateParams) Reset()                          {}
func (m *MsgUpdateParams) String() string                { return "MsgUpdateParams" }

// ─── MsgCommitExecution ──────────────────────────────────────

func (*MsgCommitExecution) ProtoMessage()                {}
func (*MsgCommitExecution) Reset()                       {}
func (m *MsgCommitExecution) String() string             { return "MsgCommitExecution" }

// ─── MsgRegisterSequencer ────────────────────────────────────

func (*MsgRegisterSequencer) ProtoMessage()              {}
func (*MsgRegisterSequencer) Reset()                     {}
func (m *MsgRegisterSequencer) String() string           { return "MsgRegisterSequencer" }

// ─── MsgActivateSequencer ────────────────────────────────────

func (*MsgActivateSequencer) ProtoMessage()              {}
func (*MsgActivateSequencer) Reset()                     {}
func (m *MsgActivateSequencer) String() string           { return "MsgActivateSequencer" }

// ─── MsgDeactivateSequencer ──────────────────────────────────

func (*MsgDeactivateSequencer) ProtoMessage()            {}
func (*MsgDeactivateSequencer) Reset()                   {}
func (m *MsgDeactivateSequencer) String() string         { return "MsgDeactivateSequencer" }

// ─── MsgExecuteSlash ─────────────────────────────────────────

func (*MsgExecuteSlash) ProtoMessage()                   {}
func (*MsgExecuteSlash) Reset()                          {}
func (m *MsgExecuteSlash) String() string                { return "MsgExecuteSlash" }

// ─── MsgAnchorSettlement ─────────────────────────────────────

func (*MsgAnchorSettlement) ProtoMessage()               {}
func (*MsgAnchorSettlement) Reset()                      {}
func (m *MsgAnchorSettlement) String() string            { return "MsgAnchorSettlement" }

// ─── MsgSubmitCommitteeSnapshot ──────────────────────────────

func (*MsgSubmitCommitteeSnapshot) ProtoMessage()        {}
func (*MsgSubmitCommitteeSnapshot) Reset()               {}
func (m *MsgSubmitCommitteeSnapshot) String() string     { return "MsgSubmitCommitteeSnapshot" }

// ─── MsgTransitionSequencer ──────────────────────────────────

func (*MsgTransitionSequencer) ProtoMessage()  {}
func (*MsgTransitionSequencer) Reset()         {}
func (m *MsgTransitionSequencer) String() string { return "MsgTransitionSequencer" }

// ─── MsgDeclareOperatorBond ──────────────────────────────────
// (ProtoMessage/Reset/String already defined in bond.go — no re-declaration needed)

// ─── MsgWithdrawOperatorBond ─────────────────────────────────
// (ProtoMessage/Reset/String already defined in bond.go — no re-declaration needed)
