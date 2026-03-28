# Omniphi Python SDK

Production-quality Python client SDK for the [Omniphi blockchain](https://omniphi.network). Built on Cosmos SDK with secp256k1 cryptography, BIP-39/BIP-44 key derivation, and bech32 address encoding.

## Installation

```bash
pip install omniphi
```

Or install from source:

```bash
cd sdk/python
pip install -e ".[dev]"
```

### Requirements

- Python >= 3.10
- Dependencies: `httpx`, `bech32`, `mnemonic`, `ecdsa`

## Quick Start

```python
from omniphi import OmniphiClient, Wallet

# Create a new wallet
wallet = Wallet.create()
print(f"Address:  {wallet.address}")
print(f"Mnemonic: {wallet.mnemonic}")

# Connect to a node
client = OmniphiClient(
    rpc_url="http://localhost:26657",
    rest_url="http://localhost:1317",
)

# Check the block height
height = client.get_block_height()
print(f"Block height: {height}")
```

## Wallet Management

### Create a New Wallet

```python
from omniphi import Wallet

# 24-word mnemonic (256-bit entropy) -- default
wallet = Wallet.create()

# 12-word mnemonic (128-bit entropy)
wallet = Wallet.create(strength=128)

print(wallet.address)         # omni1...
print(wallet.mnemonic)        # 24 space-separated words
print(wallet.public_key_hex)  # compressed secp256k1 pubkey
```

### Restore from Mnemonic

```python
wallet = Wallet.from_mnemonic(
    "abandon abandon abandon abandon abandon abandon "
    "abandon abandon abandon abandon abandon about"
)
```

### Import from Private Key

```python
wallet = Wallet.from_private_key("deadbeef..." )  # 64-char hex
```

### Sign and Verify

```python
message = b"hello omniphi"
signature = wallet.sign(message)       # 64-byte secp256k1 signature
is_valid = wallet.verify(message, signature)  # True
```

## Querying the Chain

```python
from omniphi import OmniphiClient

client = OmniphiClient()

# Chain info
height = client.get_block_height()
chain_id = client.get_chain_id()

# Balances
balance = client.get_balance("omni1...", denom="omniphi")
all_balances = client.get_all_balances("omni1...")

# Validators
validators = client.get_validators()

# Account info (needed for signing)
account_number, sequence = client.get_account_number_and_sequence("omni1...")
```

## Sending Tokens

```python
from omniphi import OmniphiClient, Wallet

wallet = Wallet.from_mnemonic("your mnemonic here ...")
client = OmniphiClient()

result = client.send_tokens(
    sender=wallet.address,
    recipient="omni1recipient...",
    amount=1_000_000,       # in smallest denomination
    denom="omniphi",
    wallet=wallet,
    memo="payment for services",
)
print(f"TX Hash: {result['tx_response']['txhash']}")
```

## Delegating to Validators

```python
result = client.delegate(
    delegator=wallet.address,
    validator="omnivaloper1...",
    amount=5_000_000,
    wallet=wallet,
)
```

## Proof-of-Contribution (PoC)

### Submit a Contribution

```python
import hashlib

content = b"my contribution content"
content_hash = hashlib.sha256(content).hexdigest()

result = client.submit_contribution(
    sender=wallet.address,
    ctype="code",              # "code", "record", "relay", "green"
    uri="ipfs://QmYourHash",
    content_hash=content_hash,
    wallet=wallet,
)
```

### Query Contributions

```python
# Single contribution
contribution = client.query_contribution(contribution_id=42)

# All contributions with filters
contributions = client.query_contributions(
    contributor="omni1...",
    ctype="code",
    verified=1,   # -1=all, 0=unverified, 1=verified
)

# PoC credits and tier
credits = client.query_poc_credits("omni1...")
```

## Tokenomics Queries

```python
# Token supply metrics
supply = client.query_token_supply()
print(f"Total supply: {supply['current_total_supply']}")
print(f"Supply cap:   {supply['total_supply_cap']}")

# Inflation
inflation = client.query_inflation()
print(f"Rate: {inflation['current_inflation_rate']}")

# Emission distribution
emissions = client.query_emissions()

# DAO treasury
treasury = client.query_treasury()

# Adaptive burn rate
burn_rate = client.query_tokenomics_burn_rate()
```

## Fee Market

```python
base_fee = client.query_base_fee()
utilization = client.query_block_utilization()
```

## Guard Module

```python
# Risk report for a governance proposal
report = client.query_risk_report(proposal_id=5)

# Queued execution state
queued = client.query_guard_queued(proposal_id=5)
```

## Timelock Module

```python
operations = client.query_timelock_operations()
queued = client.query_timelock_queued()
executable = client.query_timelock_executable()
```

## Module Parameters

Query parameters for any supported module:

```python
# Custom modules
poc_params = client.query_module_params("poc")
guard_params = client.query_module_params("guard")
tokenomics_params = client.query_module_params("tokenomics")
feemarket_params = client.query_module_params("feemarket")
timelock_params = client.query_module_params("timelock")

# Standard Cosmos SDK modules
staking_params = client.query_module_params("staking")
gov_params = client.query_module_params("gov")
```

## Building Transactions Manually

For advanced use cases, you can build and sign transactions directly:

```python
from omniphi import build_send_msg, build_delegate_msg, sign_tx

# Build messages
msg1 = build_send_msg("omni1from", "omni1to", 1000)
msg2 = build_delegate_msg("omni1del", "omnivaloper1val", 2000)

# Sign
signed_tx = sign_tx(
    msgs=[msg1, msg2],
    wallet=wallet,
    chain_id="omniphi-testnet-2",
    account_number=42,
    sequence=7,
    fee=10000,
    gas=400000,
    memo="batch transaction",
)

# Broadcast
result = client.broadcast_tx(signed_tx)
```

### Available Message Builders

```python
from omniphi import (
    build_send_msg,
    build_delegate_msg,
    build_contribution_msg,
    build_endorse_msg,
    build_withdraw_poc_rewards_msg,
    build_advisory_link_msg,
)
```

## Error Handling

```python
from omniphi import (
    OmniphiError,
    OmniphiConnectionError,
    OmniphiTxError,
    OmniphiBroadcastError,
)

try:
    result = client.send_tokens(
        sender=wallet.address,
        recipient="omni1...",
        amount=1000,
        wallet=wallet,
    )
except OmniphiConnectionError as e:
    print(f"Cannot reach node: {e.url}")
except OmniphiBroadcastError as e:
    print(f"TX failed: code={e.code}, log={e.raw_log}")
except OmniphiTxError as e:
    print(f"Signing error: {e}")
except OmniphiError as e:
    print(f"General error: {e}")
```

## Constants

```python
from omniphi import (
    BECH32_PREFIX,    # "omni"
    DENOM,            # "omniphi"
    COIN_TYPE,        # 60
    HD_PATH,          # "m/44'/60'/0'/0/0"
    DEFAULT_GAS,      # 200000
    DEFAULT_FEE,      # 5000
    MODULE_NAMES,     # ("poc", "por", "poseq", ...)
    MSG_TYPE_URLS,    # {"send": "/cosmos.bank.v1beta1.MsgSend", ...}
    REST_PATHS,       # {"balances": "/cosmos/bank/...", ...}
)
```

## Running Tests

```bash
pip install -e ".[dev]"
pytest tests/ -v
```

## License

MIT
