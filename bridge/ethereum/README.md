# Omniphi Ethereum Bridge

Lock-and-mint bridge between Ethereum and the Omniphi blockchain. Users lock
ERC-20 tokens (or native ETH) on Ethereum and receive wrapped equivalents on
Omniphi, or burn wrapped tokens on Omniphi to release the originals back to
Ethereum.

## Architecture

```
Ethereum                          Omniphi
+-----------------+               +-------------------+
| OmniphiBridge   |  <-- relay -> | x/bridge module   |
| (Solidity)      |               | (Cosmos SDK)      |
+-----------------+               +-------------------+
  |                                  |
  | Deposit events                   | BurnAndBridge events
  v                                  v
+-----------------+               +-------------------+
| Relay Service   | ------------> | MsgBridgeMint     |
| (Go)            | <------------ | (attestation tx)  |
+-----------------+               +-------------------+
```

### Security Model: M-of-N Multisig

The bridge uses a threshold signature scheme:

- **N** relayer addresses are registered in the Ethereum bridge contract.
- **M** (threshold) signatures are required to authorize any withdrawal.
- Signatures must be from distinct relayers, sorted by address (ascending).
- The contract enforces signature uniqueness, replay prevention (nonce tracking),
  and EIP-2 signature malleability protection.

Example: with 5 relayers and a threshold of 3, any 3 of the 5 must co-sign a
withdrawal before funds are released.

## Prerequisites

- Go 1.24+
- Solidity compiler (solc 0.8.24+) or Foundry / Hardhat
- Access to an Ethereum RPC endpoint (Infura, Alchemy, or local node)
- A running Omniphi node with the bridge module enabled
- At least M relayer keys for withdrawal authorization

## Contract Deployment

### Using Foundry

```bash
# Install foundry if you haven't already
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Compile
forge build --contracts bridge/ethereum/contracts

# Deploy (example with 3 relayers and threshold of 2)
forge create \
  --rpc-url $ETH_RPC_URL \
  --private-key $DEPLOYER_KEY \
  bridge/ethereum/contracts/OmniphiBridge.sol:OmniphiBridge \
  --constructor-args \
    "[0xRelayer1,0xRelayer2,0xRelayer3]" \
    2
```

### Using Hardhat

```bash
npx hardhat compile
npx hardhat run scripts/deploy.js --network mainnet
```

### Constructor Arguments

| Argument           | Type        | Description                                  |
|--------------------|-------------|----------------------------------------------|
| `initialRelayers`  | `address[]` | Array of initial relayer Ethereum addresses   |
| `initialThreshold` | `uint256`   | Number of signatures required for withdrawals |

The deployer becomes the contract owner and can later add/remove relayers and
adjust the threshold.

## Relay Service

### Configuration

Create a JSON config file (e.g., `relay-config.json`):

```json
{
  "ethereum_rpc": "https://mainnet.infura.io/v3/YOUR_KEY",
  "bridge_contract_address": "0xYOUR_BRIDGE_CONTRACT",
  "private_key": "0xYOUR_RELAYER_PRIVATE_KEY",
  "eth_chain_id": 1,
  "eth_confirmations": 12,
  "eth_poll_interval": "15s",
  "eth_start_block": 0,

  "omniphi_rpc": "http://127.0.0.1:26657",
  "omniphi_chain_id": "omniphi-1",
  "omniphi_key_name": "relayer",
  "omniphi_keyring_backend": "file",
  "omniphi_keyring_dir": "/home/relayer/.pos",
  "omniphi_gas_price": "0.025uomni",
  "omniphi_gas_adjustment": 1.5,

  "retry_max_attempts": 10,
  "retry_base_delay": "2s",
  "retry_max_delay": "5m",
  "batch_size": 50,

  "health_addr": ":8081",
  "log_level": "info"
}
```

### Configuration Reference

| Field                      | Default              | Description                                        |
|----------------------------|----------------------|----------------------------------------------------|
| `ethereum_rpc`             | `http://127.0.0.1:8545` | Ethereum JSON-RPC endpoint                     |
| `bridge_contract_address`  | (required)           | Deployed OmniphiBridge contract address             |
| `private_key`              | (required)           | Relayer's Ethereum private key (hex, with 0x)       |
| `eth_chain_id`             | `1`                  | Ethereum chain ID (1=mainnet, 11155111=sepolia)     |
| `eth_confirmations`        | `12`                 | Blocks to wait before acting on Ethereum events     |
| `eth_poll_interval`        | `15s`                | How often to poll for new Ethereum logs             |
| `eth_start_block`          | `0` (latest)         | Block to begin scanning from (0 = latest at start)  |
| `omniphi_rpc`              | `http://127.0.0.1:26657` | Omniphi Tendermint RPC endpoint               |
| `omniphi_chain_id`         | `omniphi-testnet-2`  | Omniphi chain ID                                    |
| `omniphi_key_name`         | (optional)           | Keyring key name for signing Cosmos txs             |
| `omniphi_keyring_backend`  | `test`               | Keyring backend: test, file, or os                  |
| `omniphi_keyring_dir`      | (optional)           | Path to the keyring directory                       |
| `omniphi_gas_price`        | `0.025uomni`         | Gas price for Omniphi transactions                  |
| `omniphi_gas_adjustment`   | `1.5`                | Gas estimation multiplier                           |
| `retry_max_attempts`       | `10`                 | Maximum retry attempts per operation                |
| `retry_base_delay`         | `2s`                 | Initial retry delay (doubles each attempt)          |
| `retry_max_delay`          | `5m`                 | Maximum retry delay cap                             |
| `batch_size`               | `50`                 | Max events to process per poll cycle                |
| `health_addr`              | `:8081`              | Health check HTTP server bind address               |
| `log_level`                | `info`               | Log level: debug, info, warn, error                 |

### Building and Running

```bash
cd bridge/ethereum/relay

# Build
go build -o omniphi-relay ./...

# Or run directly
go run . relay-config.json
```

For the relay to be runnable as a standalone binary, create a `main.go` in your
cmd directory:

```go
package main

import relay "github.com/omniphi/bridge-relay"

func main() {
    relay.Main()
}
```

### Health Check Endpoints

The relay exposes two HTTP endpoints:

- **`GET /healthz`** — Returns 200 if the relay is operating normally, 503 if
  degraded. Response body is JSON:
  ```json
  {
    "status": "ok",
    "eth_address": "0x...",
    "bridge_contract": "0x...",
    "last_eth_poll_unix": 1711500000,
    "last_omni_poll_unix": 1711500000,
    "last_eth_block": 19500000
  }
  ```

- **`GET /readyz`** — Returns 200 once the relay has completed at least one
  successful poll cycle. Use this for Kubernetes readiness probes.

## Bridging Tokens

### Ethereum to Omniphi (Deposit)

1. **Approve the bridge contract** (ERC-20 only):
   ```
   IERC20(token).approve(bridgeAddress, amount)
   ```

2. **Call deposit()**:
   ```
   // For ERC-20:
   bridge.deposit(tokenAddress, amount, "omni1abc...")

   // For native ETH:
   bridge.deposit{value: 1 ether}(address(0), 0, "omni1abc...")
   ```

3. **Wait for confirmation**: The relay watches for the `Deposit` event, waits
   for the configured number of block confirmations, and then submits a
   `MsgBridgeMint` to Omniphi.

4. **Receive wrapped tokens**: The Omniphi bridge module mints the wrapped
   equivalent to the specified recipient address.

### Omniphi to Ethereum (Withdrawal)

1. **Burn wrapped tokens on Omniphi**:
   ```bash
   posd tx bridge burn-and-bridge \
     --token 0xOriginalToken \
     --amount 1000000 \
     --recipient 0xYourEthAddress \
     --from mykey \
     --chain-id omniphi-1
   ```

2. **Relayers sign**: Each relay observes the `BurnAndBridge` event and produces
   an ECDSA signature over the withdrawal parameters.

3. **Submit to Ethereum**: Once M signatures are collected, the relay (or any
   party) calls `withdraw()` on the bridge contract with the concatenated
   signatures.

4. **Receive tokens**: The bridge contract verifies the signatures and releases
   the locked tokens to the specified Ethereum recipient.

## Contract Administration

All admin functions are restricted to the contract owner.

### Relayer Management

```solidity
// Add a new relayer
bridge.addRelayer(0xNewRelayer)

// Remove an existing relayer (fails if it would make threshold unreachable)
bridge.removeRelayer(0xOldRelayer)

// Change the signature threshold
bridge.setThreshold(3)
```

### Emergency Controls

```solidity
// Halt all deposits and withdrawals
bridge.pause()

// Resume normal operation
bridge.unpause()
```

### Ownership Transfer (Two-Step)

```solidity
// Step 1: Current owner initiates transfer
bridge.transferOwnership(0xNewOwner)

// Step 2: New owner accepts
bridge.acceptOwnership()  // must be called from newOwner
```

## Security Considerations

### Protections Built In

- **Reentrancy guard**: All state-changing functions use the checks-effects-
  interactions pattern plus a reentrancy lock.
- **Replay prevention**: Every deposit gets a unique monotonic nonce. Every
  withdrawal nonce can only be processed once.
- **Signature malleability**: Enforces low-S values (EIP-2) and rejects
  non-canonical signatures.
- **Duplicate signatures**: Requires signers to be in strictly ascending address
  order, eliminating the need for O(n^2) uniqueness checks.
- **Fee-on-transfer tokens**: The deposit function measures the actual received
  balance rather than trusting the `amount` parameter.
- **Domain separation**: Withdrawal message hashes include `block.chainid` and
  the contract address, preventing cross-chain and cross-contract replay.
- **Two-step ownership**: Prevents accidental ownership transfer to a wrong
  address.
- **Pausable**: Owner can halt the bridge in an emergency.

### Operational Recommendations

1. **Run multiple relayers**: Deploy at least N >= 5 relayers with a threshold
   of M >= 3. Distribute keys across different infrastructure providers.

2. **Monitor health endpoints**: Set up alerting on `/healthz` returning 503
   or `last_eth_poll_unix` being stale.

3. **Use hardware security modules (HSMs)**: Store relayer private keys in HSMs
   or cloud KMS rather than plaintext config files.

4. **Confirmation depth**: Use at least 12 block confirmations for Ethereum
   mainnet (more for high-value transfers). For L2s, adjust according to the
   chain's finality guarantees.

5. **Rate limiting**: Consider adding per-block or per-epoch deposit caps in the
   Omniphi bridge module to limit maximum exposure.

6. **Audits**: Have the Solidity contract audited by at least two independent
   firms before mainnet deployment.

## File Structure

```
bridge/ethereum/
  contracts/
    OmniphiBridge.sol          # Main bridge contract
    interfaces/
      IERC20.sol               # Standard ERC-20 interface
  relay/
    relay.go                   # Relay service implementation
    config.go                  # Configuration types and validation
    go.mod                     # Go module definition
  README.md                    # This file
```
