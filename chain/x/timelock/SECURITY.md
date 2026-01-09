# Timelock Module Security Audit Checklist

## Overview

This document outlines the security measures implemented in the Timelock governance module and provides a checklist for security audits.

## Threat Model

### Identified Threats

| ID | Threat | Severity | Mitigation |
|----|--------|----------|------------|
| T1 | Flash loan governance attacks | Critical | Minimum 24h delay allows community response |
| T2 | Compromised validator keys | High | Guardian can cancel malicious operations |
| T3 | Social engineering attacks | High | Delay period for community verification |
| T4 | Front-running execution | Medium | Deterministic execution time |
| T5 | Guardian key compromise | High | Guardian change requires governance vote |
| T6 | Replay attacks | High | Unique operation ID and hash per operation |
| T7 | Parameter manipulation | Medium | Rate limiting on parameter changes |
| T8 | Denial of service via spam | Low | Standard Cosmos SDK tx fees apply |

## Security Constants

### Absolute Minimums (Hardcoded - Cannot Be Changed)

```go
AbsoluteMinDelay       = 1 hour   // Even emergency operations wait 1 hour
AbsoluteMaxDelay       = 30 days  // Prevents indefinite queueing
AbsoluteMinGracePeriod = 1 hour   // Minimum execution window
```

### Default Values (Can Be Changed via Governance)

```go
DefaultMinDelay       = 24 hours  // Standard delay period
DefaultMaxDelay       = 14 days   // Maximum delay
DefaultGracePeriod    = 7 days    // Execution window after delay
DefaultEmergencyDelay = 1 hour    // Emergency execution delay
```

## Security Audit Checklist

### Parameter Validation

- [x] `min_delay` cannot be set below `AbsoluteMinDelay` (1 hour)
- [x] `max_delay` cannot exceed `AbsoluteMaxDelay` (30 days)
- [x] `min_delay` must be <= `max_delay`
- [x] `emergency_delay` must be >= `AbsoluteMinDelay` (1 hour)
- [x] `emergency_delay` must be < `min_delay`
- [x] `grace_period` must be >= `AbsoluteMinGracePeriod` (1 hour)
- [x] Parameter changes are rate-limited (max 50% reduction per update)

### Operation Security

- [x] Each operation has a unique ID (monotonically increasing)
- [x] Operation hash includes: proposal ID, operation ID, messages, queued time
- [x] Hash is verified before execution
- [x] Operations can only be executed once (status transitions are terminal)
- [x] Expired operations are automatically marked and cannot be executed
- [x] Cancelled operations cannot be re-executed

### Access Control

- [x] Only governance can update module parameters
- [x] Only governance can change the guardian address
- [x] Only guardian or governance can cancel operations
- [x] Only guardian can trigger emergency execution
- [x] Executor must match the queued executor address
- [x] Authority checks use address comparison, not signature verification

### Cancellation Security

- [x] Cancellation requires a reason (minimum 10 characters)
- [x] Reason is stored on-chain for transparency
- [x] Cancelled operations emit events for monitoring
- [x] Only QUEUED operations can be cancelled

### Emergency Execution Security

- [x] Emergency execution still requires minimum 1 hour delay
- [x] Emergency execution requires detailed justification (minimum 20 characters)
- [x] Emergency executions are logged with WARNING level
- [x] Emergency execution emits distinct event type

### State Integrity

- [x] All state changes emit events
- [x] Genesis state is validated on import
- [x] Operation IDs are sequential (no gaps in normal operation)
- [x] Hash index is maintained for O(1) lookup by hash

### Message Execution

- [x] Messages are executed via standard Cosmos SDK message router
- [x] Each message's handler is verified to exist before queuing
- [x] Execution failures are recorded (status = FAILED)
- [x] Failed executions preserve the error message

## Known Limitations

1. **No Partial Execution**: If any message in an operation fails, the entire operation is marked as failed. Partial execution is not supported.

2. **Guardian Single Point**: The guardian is a single address. For production, use a multisig address.

3. **No Re-queuing**: Cancelled or failed operations cannot be re-queued. A new governance proposal is required.

4. **Clock Dependency**: The module relies on block time. Clock manipulation by validators could affect timing.

## Recommended Guardian Setup

For production deployments, the guardian should be:

1. **Multisig Wallet**: 3-of-5 or 4-of-7 multisig
2. **Geographically Distributed**: Signers in different jurisdictions
3. **Operationally Separate**: Signers should not be validators
4. **Response Plan**: Documented procedure for emergency actions

## Event Monitoring

The following events should be monitored:

| Event | Action |
|-------|--------|
| `operation_queued` | Log for tracking |
| `operation_executed` | Verify expected outcome |
| `operation_cancelled` | Alert - investigate reason |
| `operation_expired` | Review why execution didn't happen |
| `emergency_execution` | **HIGH ALERT** - verify justification |
| `guardian_updated` | **CRITICAL ALERT** - verify authorization |
| `params_updated` | Review parameter changes |

## Upgrade Considerations

When upgrading the timelock module:

1. Export genesis state before upgrade
2. Verify all queued operations are preserved
3. Validate new parameters against security constants
4. Test execution of queued operations post-upgrade

## Incident Response

### If Guardian Key is Compromised

1. Immediately submit governance proposal to change guardian
2. Monitor for unauthorized cancellations
3. Consider emergency parameter update to extend delays

### If Malicious Proposal Passes Vote

1. Guardian should cancel operation during delay period
2. Document reason in cancellation message
3. Submit new proposal with corrected parameters

## Audit Trail

All security-relevant operations are logged:

```
INFO  operation queued      operation_id=X proposal_id=Y executable_at=Z
INFO  operation executed    operation_id=X executor=Y
INFO  operation cancelled   operation_id=X canceller=Y reason=Z
WARN  emergency executed    operation_id=X guardian=Y justification=Z
WARN  guardian updated      old=X new=Y
INFO  params updated        old_min_delay=X new_min_delay=Y
```

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-01-08 | Initial implementation |
