# Governance Extension Module (govx)

This module extends the standard Cosmos SDK governance module with **proposal validation** to prevent invalid proposals from being submitted. It validates governance proposals at submission time, catching errors before deposits are made and voting periods begin.

## Problem Solved

In standard Cosmos SDK governance, proposals are only validated at **execution time** (after voting ends). This means:

1. ❌ Invalid proposals can be submitted
2. ❌ Deposits are wasted on proposals that will fail
3. ❌ 48+ hour voting periods are wasted
4. ❌ Users discover errors only after the proposal "passes" but fails to execute

### Real-World Example

The Omniphi testnet experienced this issue when a consensus params proposal was submitted with missing `evidence` and `validator` fields:

```json
{
  "messages": [{
    "@type": "/cosmos.consensus.v1.MsgUpdateParams",
    "authority": "...",
    "block": { "max_gas": "60000000" },
    "evidence": null,    // ❌ Should not be null
    "validator": null,   // ❌ Should not be null
    "abci": null
  }]
}
```

The proposal:
- ✅ Was submitted successfully
- ✅ Received 100% YES votes
- ❌ Failed at execution: `"all parameters must be present"`

## Solution

This module adds a **ProposalValidationDecorator** to the ante handler chain that:

1. ✅ Validates proposal messages at submission time
2. ✅ Rejects invalid proposals immediately
3. ✅ Protects user deposits
4. ✅ Provides clear error messages

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Transaction Flow                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  MsgSubmitProposal                                          │
│       │                                                      │
│       ▼                                                      │
│  ┌─────────────────────────────────────────┐                │
│  │     ProposalValidationDecorator         │ ◄── NEW        │
│  │                                          │                │
│  │  1. ValidateBasic on messages           │                │
│  │  2. Check message routing (handler)      │                │
│  │  3. Message-specific validation          │                │
│  │  4. Simulate message execution           │                │
│  └─────────────────────────────────────────┘                │
│       │                                                      │
│       ▼ (if valid)                                          │
│  ┌─────────────────────────────────────────┐                │
│  │        Standard Gov Module              │                │
│  │  - Accept proposal                       │                │
│  │  - Collect deposit                       │                │
│  │  - Start voting period                   │                │
│  └─────────────────────────────────────────┘                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Validation Rules

### Consensus Params (`MsgUpdateParams`)

**All fields are REQUIRED**, even if unchanged:

| Field | Required Contents |
|-------|-------------------|
| `block` | `max_bytes`, `max_gas` |
| `evidence` | `max_age_num_blocks`, `max_age_duration`, `max_bytes` |
| `validator` | `pub_key_types` |
| `abci` | Can be empty `{}` but must be present |

### Duration Format

Use seconds format for durations:
- ✅ `"172800s"` (correct)
- ❌ `"48h0m0s"` (may not work)
- ❌ `172800000000000` (nanoseconds won't work)

### Proposal Metadata

| Field | Max Length | Required |
|-------|------------|----------|
| `title` | 140 chars | Yes |
| `summary` | 10,000 chars | Yes |
| `metadata` | 10,000 bytes | No |

## Error Messages

The module provides clear error messages:

```
proposal validation failed: message 0 (/cosmos.consensus.v1.MsgUpdateParams)
failed specific validation: consensus MsgUpdateParams requires 'evidence' field
- all parameters must be present
```

## Configuration

The validator can be configured via `ProposalValidatorConfig`:

```go
config := keeper.ProposalValidatorConfig{
    EnableSimulation: true,      // Enable message simulation
    MaxGasLimit:      10_000_000, // Max gas for simulation
}
```

## Files

| File | Purpose |
|------|---------|
| `x/gov/types/errors.go` | Custom error definitions |
| `x/gov/keeper/proposal_validator.go` | Core validation logic |
| `x/gov/ante/proposal_validation_decorator.go` | Ante handler decorator |
| `x/gov/module/module.go` | Module registration |
| `app/ante.go` | Ante handler integration |

## Testing

Run unit tests:
```bash
go test ./x/gov/keeper/... -v
```

Run benchmarks:
```bash
go test ./x/gov/keeper/... -bench=. -benchmem
```

## Upgrade Instructions

This feature requires a **chain upgrade** to deploy:

1. Build the new binary with the governance extension
2. Coordinate upgrade with validators
3. Execute chain upgrade at agreed height
4. After upgrade, invalid proposals will be rejected at submission time

## Security Considerations

1. **Simulation Gas Limit**: The validator uses a gas limit for message simulation to prevent DoS attacks. Default is 10M gas.

2. **No State Changes**: Simulation uses a cached context - no state changes are committed.

3. **Authority Validation**: The validator checks the governance module authority address.

4. **Error Handling**: All errors are wrapped with context for debugging without exposing internals.

## Industry Standard Comparison

| Feature | Cosmos SDK | Compound Governor | Omniphi (this) |
|---------|-----------|-------------------|----------------|
| Validate at submission | ❌ | ✅ | ✅ |
| Simulate before vote | ❌ | ✅ | ✅ |
| Clear error messages | ❌ | ✅ | ✅ |
| Protect deposits | ❌ | N/A | ✅ |

## Contributing

When adding new message types that can be included in governance proposals, add validation in `validateMessageSpecific()`:

```go
case "/your.module.v1.MsgUpdateParams":
    return pv.validateYourModuleUpdateParams(ctx, msg)
```
