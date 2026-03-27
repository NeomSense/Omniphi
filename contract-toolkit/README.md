# Omniphi Intent Contract Toolkit

A CLI and DSL for building, testing, and deploying Intent Contracts on the Omniphi blockchain.

Intent Contracts are constraint schemas. Rather than executing logic directly, they declare what valid state transitions look like. Solvers compete to fulfill user intents, and the constraint validator (running in the Omniphi runtime) checks whether a solver's proposed state change satisfies the contract's rules.

This toolkit provides the developer workflow:

1. **Define** contracts in human-readable YAML
2. **Compile** them to the runtime's JSON schema format
3. **Test** locally with YAML test cases
4. **Deploy** to the Omniphi chain

## Installation

```bash
cd contract-toolkit
cargo build --release
# The binary is at target/release/omniphi-contracts
```

## Quickstart

```bash
# 1. Scaffold a new contract project
omniphi-contracts init my-counter --template counter

# 2. Enter the project directory
cd my-counter

# 3. Validate the schema
omniphi-contracts validate

# 4. Compile YAML to JSON
omniphi-contracts build

# 5. Run tests
omniphi-contracts test

# 6. Deploy (dry run)
omniphi-contracts deploy --rpc http://localhost:26657 --key ~/.pos/keyring-test/validator.info --dry-run
```

## Project Structure

After `omniphi-contracts init my-contract`, you get:

```
my-contract/
  contract.toml     # Project manifest (name, version, paths)
  schema.yaml       # Contract schema definition
  tests.yaml        # Test cases
  build/            # Compiled output (generated)
  .gitignore
```

### contract.toml

```toml
[contract]
name = "my-contract"
version = "0.1.0"
description = "My contract description"
authors = ["Your Name"]

[build]
schema = "schema.yaml"    # Path to schema YAML
tests = "tests.yaml"      # Path to test suite YAML
output = "build"           # Output directory for compiled JSON
```

## Schema Reference

The schema YAML file defines the complete contract. Here is the full structure:

```yaml
# Required: contract metadata
name: MyContract                    # Alphanumeric, hyphens, underscores
version: "1.0.0"                    # Semantic version
description: "What this does"       # Human-readable description

# Required: at least one intent
intents:
  - name: my_intent                 # Valid identifier (method selector)
    description: "What it does"
    params:                         # Parameters the caller provides
      - name: amount
        param_type: Uint128         # See "Parameter Types" below
        required: true              # Default: true
        description: "How much"
    preconditions:                  # Must hold before state change
      - "balance >= amount"
    postconditions:                 # Must hold after state change
      - "balance decreased by amount"

# Optional: persistent state fields
state_fields:
  - name: balance
    field_type: Uint128
    default_value: "0"              # Initial value (string representation)
    description: "Account balance"

# Optional: constraints (validated by runtime)
constraints:
  - name: sufficient_balance
    constraint_type: BalanceCheck   # See "Constraint Types" below
    params:
      asset: "uomni"
      min_amount: "100"
    applies_to:                     # Empty = all intents
      - withdraw

# Optional: resource limits
max_gas_per_call: 1000000           # Default: 1,000,000
max_state_bytes: 65536              # Default: 65,536
```

### Parameter Types

| Type      | Aliases              | Description                         | Example Default    |
|-----------|----------------------|-------------------------------------|--------------------|
| `Uint128` | `u128`               | 128-bit unsigned integer            | `"0"`              |
| `Address` | `addr`               | 32-byte Ed25519 public key (hex)    | `"zero"`, `"sender"` |
| `Bytes`   | `binary`             | Arbitrary bytes (hex-encoded)       | `""`               |
| `String`  | `str`, `text`        | UTF-8 text                          | `""`               |
| `Bool`    | `boolean`            | Boolean flag                        | `"true"`, `"false"` |

### Constraint Types

#### BalanceCheck

Verifies that an account holds sufficient balance of an asset.

```yaml
- name: has_funds
  constraint_type: BalanceCheck
  params:
    asset: "uomni"           # Asset denomination
    min_amount: "1000"       # Minimum required balance
```

#### OwnershipCheck

Verifies that the caller owns a specific object (the sender matches the address in a state field).

```yaml
- name: is_owner
  constraint_type: OwnershipCheck
  params:
    object_field: "owner"    # State field containing the owner address
```

#### TimeCheck

Verifies the current epoch satisfies a time condition.

```yaml
- name: not_expired
  constraint_type: TimeCheck
  params:
    op: "lt"                 # Operator: gte, lte, gt, lt
    value: "1000"            # Epoch value to compare against
```

#### StateCheck

Verifies a state field satisfies a condition against the proposed new state.

```yaml
- name: positive_balance
  constraint_type: StateCheck
  params:
    field: "balance"         # Must match a defined state field
    op: "gte"                # Operator: gte, lte, gt, lt, eq, neq
    value: "0"
```

#### Custom

Custom constraints evaluated by the contract's Wasm validator. In local testing, simple expressions are supported.

```yaml
- name: custom_rule
  constraint_type: Custom
  params:
    expression: "count >= 0"  # Simple field comparison (local testing)
```

## Testing Guide

### Test File Format

```yaml
contract: MyContract
schema_path: schema.yaml

tests:
  - name: "descriptive test name"
    intent: increment               # Intent to invoke
    params:                         # Input parameters
      amount: "10"
    sender: "test_alice"            # Caller identity
    epoch: 42                       # Current epoch (default: 1)
    state_before:                   # State before the intent
      count: "5"
    state_after:                    # Expected state after
      count: "15"
    expect_valid: true              # Should constraints pass?

  - name: "expected failure"
    intent: decrement
    params:
      amount: "100"
    state_before:
      count: "5"
    state_after:
      count: "-95"
    expect_valid: false             # We expect this to fail
    expect_error: "non_negative"    # Error must contain this string
```

### Test Execution

The tester simulates the constraint evaluation pipeline:

1. Resolves the intent by name from the schema
2. Validates required parameters are present
3. Validates parameter types match definitions
4. Checks state_before fields against schema
5. Evaluates all applicable constraints (global + intent-specific)
6. Compares the result against `expect_valid`
7. If `expect_valid: false`, checks that the error contains `expect_error`

### Filtering Tests

Run only tests matching a name pattern:

```bash
omniphi-contracts test --filter "decrement"
```

## CLI Commands

### `omniphi-contracts init <name>`

Scaffold a new contract project.

```
Options:
  -t, --template <TEMPLATE>  Template: counter, escrow, blank [default: counter]
```

### `omniphi-contracts build`

Compile the contract schema from YAML to JSON.

```
Options:
  -p, --path <PATH>    Project directory [default: .]
  -o, --output <PATH>  Output JSON file [default: build/<name>.json]
```

The compiled JSON includes a deterministic `schema_id` computed as:
```
SHA256("OMNIPHI_CONTRACT_SCHEMA_V1" || canonical_json_with_empty_schema_id)
```

### `omniphi-contracts validate`

Check schema validity without compiling. Reports errors and warnings.

```
Options:
  -p, --path <PATH>  Project directory or schema YAML file [default: .]
```

### `omniphi-contracts test`

Run local contract tests.

```
Options:
  -p, --path <PATH>      Project directory [default: .]
  -f, --filter <FILTER>  Run only tests matching this name
```

### `omniphi-contracts deploy`

Deploy a compiled schema to the chain.

```
Options:
  -p, --path <PATH>          Project directory or compiled JSON file [default: .]
      --rpc <URL>            RPC endpoint URL (required)
      --key <PATH>           Path to signing key file (required)
      --chain-id <ID>        Chain ID [default: omniphi-testnet-2]
      --gas <GAS>            Gas limit [default: 500000]
      --dry-run              Print payload without sending
```

## Examples

### Counter Contract

The simplest possible contract. See `templates/counter.yaml`.

- **State**: a single `count` field (Uint128)
- **Intents**: `increment(amount)`, `decrement(amount)`, `reset()`
- **Constraints**: count must remain non-negative after decrement

### Escrow Contract

A real-world escrow pattern. See `templates/escrow.yaml`.

- **State**: deposited, released, refunded (booleans), plus arbiter/sender/recipient/amount
- **Intents**: `deposit(sender, amount, recipient, arbiter)`, `release(arbiter)`, `refund(arbiter)`
- **Constraints**:
  - Only the arbiter can release or refund
  - Cannot release if already refunded (and vice versa)
  - Cannot deposit twice
  - Must be deposited before release or refund

## How It Works

Omniphi Intent Contracts follow the constraint-based architecture:

1. **Users submit intents** (not transactions). An intent says "I want X to happen" rather than "execute these steps."

2. **Solvers propose state transitions**. Competing solvers figure out how to fulfill the intent and submit a proposed new state.

3. **The runtime validates constraints**. The contract's constraints check whether the proposed state transition is valid, without executing any logic.

4. **The best solution wins**. The runtime's settlement layer picks the optimal solver solution and applies the state change.

This toolkit helps you define step 3: the constraints that govern valid state transitions.

## Architecture

```
                                  YAML Schema
                                      |
                                  [validate]
                                      |
                                  [compile]
                                      |
                              Compiled JSON Schema
                                      |
                         +------------+------------+
                         |                         |
                    [local test]              [deploy]
                         |                         |
                   Test Results            Omniphi Chain
```

The compiled JSON matches the `ContractSchemaDef` type from the `omniphi-contract-sdk` crate, ensuring compatibility with the runtime's contract registration system.
