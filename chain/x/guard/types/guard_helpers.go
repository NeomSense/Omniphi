package types

// IsTerminal returns true if the gate state is terminal (EXECUTED or ABORTED)
func (s ExecutionGateState) IsTerminal() bool {
	return s == EXECUTION_GATE_EXECUTED || s == EXECUTION_GATE_ABORTED
}

// IsReady returns true if ready for execution
func (q *QueuedExecution) IsReady() bool {
	return q.GateState == EXECUTION_GATE_READY
}

// IsTerminal returns true if execution is in a terminal state
func (q *QueuedExecution) IsTerminal() bool {
	return q.GateState.IsTerminal()
}

// NeedsConfirmation returns true if execution requires and hasn't received second confirmation
func (q *QueuedExecution) NeedsConfirmation() bool {
	return q.RequiresSecondConfirm && !q.SecondConfirmReceived
}

// GetTierName returns human-readable tier name
func (t RiskTier) GetTierName() string {
	switch t {
	case RISK_TIER_LOW:
		return "LOW"
	case RISK_TIER_MED:
		return "MEDIUM"
	case RISK_TIER_HIGH:
		return "HIGH"
	case RISK_TIER_CRITICAL:
		return "CRITICAL"
	default:
		return "UNSPECIFIED"
	}
}

// GetGateStateName returns human-readable gate state name
func (s ExecutionGateState) GetGateStateName() string {
	switch s {
	case EXECUTION_GATE_VISIBILITY:
		return "VISIBILITY"
	case EXECUTION_GATE_SHOCK_ABSORBER:
		return "SHOCK_ABSORBER"
	case EXECUTION_GATE_CONDITIONAL_EXECUTION:
		return "CONDITIONAL_EXECUTION"
	case EXECUTION_GATE_READY:
		return "READY"
	case EXECUTION_GATE_EXECUTED:
		return "EXECUTED"
	case EXECUTION_GATE_ABORTED:
		return "ABORTED"
	default:
		return "UNSPECIFIED"
	}
}

// MaxConstraint returns the maximum of two values (AI can only constrain)
func MaxConstraint(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

// MaxConstraintTier returns the higher risk tier (AI can only constrain)
func MaxConstraintTier(a, b RiskTier) RiskTier {
	if a > b {
		return a
	}
	return b
}
