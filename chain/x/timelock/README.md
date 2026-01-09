# Timelock Governance Module

## Overview

The Timelock module implements a mandatory delay period between governance proposal passage and execution. This provides a security window for stakeholders to review, verify, and potentially cancel malicious proposals before they take effect.

## Security Model

### Threat Model

| Threat | Mitigation |
|--------|------------|
| Flash loan governance attacks | Execution delay allows community response |
| Compromised validator keys | Guardian can cancel during delay |
| Social engineering attacks | Community has time to identify malicious proposals |
| Front-running execution | Deterministic execution time prevents MEV |
| Malicious parameter updates | Review period for all changes |

### Industry Standards Implemented

- **Compound Timelock Pattern**: Delay + execution window + cancellation
- **OpenZeppelin TimelockController**: Role-based access control
- **MakerDAO GSM (Governance Security Module)**: Emergency shutdown capability

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Governance Flow                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  [Proposal Submitted]                                           │
│         │                                                        │
│         ▼                                                        │
│  [Voting Period] ◄─── Standard Cosmos SDK Gov                   │
│         │                                                        │
│         ▼                                                        │
│  [Proposal Passed]                                              │
│         │                                                        │
│         ▼                                                        │
│  ┌─────────────────────────────────────────┐                    │
│  │         TIMELOCK MODULE                  │ ◄── NEW           │
│  │                                          │                    │
│  │  1. Queue operation (delay starts)      │                    │
│  │  2. Wait for delay period               │                    │
│  │  3. Execute after delay                 │                    │
│  │                                          │                    │
│  │  [Guardian can cancel during delay]     │                    │
│  └─────────────────────────────────────────┘                    │
│         │                                                        │
│         ▼                                                        │
│  [Messages Executed]                                            │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_delay` | Duration | 24h | Minimum delay before execution |
| `max_delay` | Duration | 14d | Maximum delay (prevents indefinite queuing) |
| `grace_period` | Duration | 7d | Window after delay during which execution is valid |
| `guardian` | Address | Multisig | Address that can cancel operations |
| `emergency_delay` | Duration | 1h | Reduced delay for emergency operations |

## Operations

### 1. Queue Operation

When a governance proposal passes, its messages are automatically queued:

```go
type QueuedOperation struct {
    Id            uint64           // Unique operation ID
    ProposalId    uint64           // Source governance proposal
    Messages      []sdk.Msg        // Messages to execute
    QueuedAt      time.Time        // When operation was queued
    ExecutableAt  time.Time        // Earliest execution time
    ExpiresAt     time.Time        // Latest execution time
    Status        OperationStatus  // QUEUED, EXECUTED, CANCELLED, EXPIRED
    Executor      string           // Who can execute (usually gov module)
}
```

### 2. Cancel Operation (Guardian Only)

```go
message MsgCancelOperation {
    string authority = 1;      // Must be guardian or gov module
    uint64 operation_id = 2;
    string reason = 3;
}
```

### 3. Execute Operation

```go
message MsgExecuteOperation {
    string executor = 1;       // Must match queued executor
    uint64 operation_id = 2;
}
```

### 4. Emergency Execute (Guardian Only)

For critical security fixes with reduced delay:

```go
message MsgEmergencyExecute {
    string authority = 1;      // Must be guardian
    uint64 operation_id = 2;
    string justification = 3;  // Required explanation
}
```

## Security Features

### 1. Operation Hashing

All operations are identified by a cryptographic hash:

```go
func ComputeOperationHash(proposalId uint64, messages []sdk.Msg, salt []byte) []byte {
    h := sha256.New()
    h.Write(sdk.Uint64ToBigEndian(proposalId))
    for _, msg := range messages {
        h.Write([]byte(sdk.MsgTypeURL(msg)))
        bz, _ := proto.Marshal(msg)
        h.Write(bz)
    }
    h.Write(salt)
    return h.Sum(nil)
}
```

### 2. Replay Protection

- Each operation has a unique ID
- Operations can only be executed once
- Expired operations are automatically marked as such

### 3. Guardian Role

The guardian is a trusted address (typically a multisig) that can:
- Cancel queued operations
- Execute emergency operations with reduced delay
- Cannot bypass the minimum delay (even 1h is enforced)

Guardian should be a 3-of-5 or 4-of-7 multisig of trusted community members.

### 4. Immutable Fields

Once queued, these fields CANNOT be modified:
- Messages
- Proposal ID
- Operation hash
- Queued timestamp
- Executor

### 5. Event Emission

All operations emit events for transparency:

```go
EventOperationQueued {
    operation_id: uint64
    proposal_id: uint64
    executable_at: timestamp
    expires_at: timestamp
    operation_hash: bytes
}

EventOperationExecuted {
    operation_id: uint64
    executor: string
    success: bool
}

EventOperationCancelled {
    operation_id: uint64
    canceller: string
    reason: string
}
```

## State Machine

```
                    ┌─────────────────┐
                    │     QUEUED      │
                    └────────┬────────┘
                             │
           ┌─────────────────┼─────────────────┐
           │                 │                 │
           ▼                 ▼                 ▼
    ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
    │  CANCELLED  │   │  EXECUTED   │   │   EXPIRED   │
    └─────────────┘   └─────────────┘   └─────────────┘
         │                  │                  │
         └──────────────────┴──────────────────┘
                            │
                            ▼
                      [TERMINAL]
```

## Integration with Cosmos SDK Gov

The timelock module integrates via a **PostProposalHandler**:

```go
// Called when proposal passes
func (k Keeper) OnProposalPassed(ctx sdk.Context, proposal govv1.Proposal) error {
    // Queue all messages from the proposal
    return k.QueueProposalMessages(ctx, proposal.Id, proposal.Messages)
}
```

This replaces direct execution with queued execution.

## Query Interface

```protobuf
service Query {
    // Get a specific operation
    rpc Operation(QueryOperationRequest) returns (QueryOperationResponse);

    // List all queued operations
    rpc QueuedOperations(QueryQueuedOperationsRequest) returns (QueryQueuedOperationsResponse);

    // List operations ready for execution
    rpc ExecutableOperations(QueryExecutableOperationsRequest) returns (QueryExecutableOperationsResponse);

    // Get timelock parameters
    rpc Params(QueryParamsRequest) returns (QueryParamsResponse);

    // Check if an operation hash exists
    rpc OperationExists(QueryOperationExistsRequest) returns (QueryOperationExistsResponse);
}
```

## CLI Commands

```bash
# Query operations
posd query timelock operation [operation-id]
posd query timelock queued
posd query timelock executable
posd query timelock params

# Execute operations (usually automated)
posd tx timelock execute [operation-id] --from executor

# Guardian actions
posd tx timelock cancel [operation-id] --reason "Security concern" --from guardian
posd tx timelock emergency-execute [operation-id] --justification "Critical fix" --from guardian
```

## Automated Execution

The module includes an **EndBlocker** that:
1. Checks for executable operations
2. Automatically marks expired operations
3. Does NOT auto-execute (requires explicit transaction)

This ensures transparency - all executions are on-chain transactions.

## Audit Checklist

- [ ] Minimum delay cannot be set below 1 hour
- [ ] Grace period prevents indefinite queueing
- [ ] Only guardian can cancel operations
- [ ] Operation hash includes all message content
- [ ] Replay protection via unique operation IDs
- [ ] Events emitted for all state changes
- [ ] Emergency execute still requires minimum 1h delay
- [ ] Guardian cannot modify operation content
- [ ] Expired operations cannot be executed
- [ ] Cancelled operations cannot be re-queued

## References

- [Compound Timelock](https://github.com/compound-finance/compound-protocol/blob/master/contracts/Timelock.sol)
- [OpenZeppelin TimelockController](https://docs.openzeppelin.com/contracts/4.x/api/governance#TimelockController)
- [MakerDAO GSM](https://docs.makerdao.com/smart-contract-modules/governance-module)
- [Cosmos SDK Governance](https://docs.cosmos.network/main/modules/gov)
