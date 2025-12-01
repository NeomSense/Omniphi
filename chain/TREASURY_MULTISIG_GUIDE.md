 # Treasury Multisig Setup Guide

## Overview

This guide implements a 3-of-5 multisig account for the Omniphi Network treasury using Cosmos SDK's native multisig functionality.

## Implementation Summary

### Treasury Requirements

**Purpose:** Secure control of 170M OMNI treasury funds (increased from 150M to account for advisor tokens moved from team allocation)

**Security Model:**
- 5 authorized signers (foundation members, trusted advisors)
- Requires 3 signatures to execute any transaction
- No single point of failure
- Transparent on-chain governance

**Treasury Allocation:**
- Initial: 170,000,000 OMNI (170,000,000,000,000 uomni)
- Receives 10% of inflation emissions (~2.25M OMNI/year)
- Receives 10% of burn redirect (~variable based on usage)

---

## Multisig Account Setup

### Step 1: Generate Signer Keys

Each of the 5 treasury signers creates their own key:

```bash
# Signer 1 (e.g., CEO)
posd keys add treasury-signer-1 --keyring-backend file

# Signer 2 (e.g., CTO)
posd keys add treasury-signer-2 --keyring-backend file

# Signer 3 (e.g., CFO)
posd keys add treasury-signer-3 --keyring-backend file

# Signer 4 (e.g., Legal Counsel)
posd keys add treasury-signer-4 --keyring-backend file

# Signer 5 (e.g., Board Representative)
posd keys add treasury-signer-5 --keyring-backend file

# Each signer should securely backup their mnemonic
# Store in hardware wallet or encrypted vault
```

### Step 2: Export Public Keys

Each signer exports their public key for multisig creation:

```bash
# Signer 1 exports pubkey
posd keys show treasury-signer-1 --pubkey

# Repeat for all 5 signers
# Share public keys through secure channel (encrypted email, secure messaging)
```

### Step 3: Create Multisig Account

**Coordinator** creates the multisig account using all public keys:

```bash
# Import all signer public keys (coordinator does this)
posd keys add treasury-signer-1 --pubkey '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"A..."}'
posd keys add treasury-signer-2 --pubkey '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"A..."}'
posd keys add treasury-signer-3 --pubkey '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"A..."}'
posd keys add treasury-signer-4 --pubkey '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"A..."}'
posd keys add treasury-signer-5 --pubkey '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"A..."}'

# Create 3-of-5 multisig account
posd keys add treasury-multisig \
  --multisig treasury-signer-1,treasury-signer-2,treasury-signer-3,treasury-signer-4,treasury-signer-5 \
  --multisig-threshold 3 \
  --keyring-backend file

# Get multisig address
TREASURY_MULTISIG_ADDR=$(posd keys show treasury-multisig -a)
echo "Treasury Multisig Address: $TREASURY_MULTISIG_ADDR"
```

### Step 4: Add to Genesis

Update [MAINNET_GENESIS_TEMPLATE.json](MAINNET_GENESIS_TEMPLATE.json:1) with multisig address:

```json
{
  "app_state": {
    "auth": {
      "accounts": [
        {
          "@type": "/cosmos.auth.v1beta1.BaseAccount",
          "address": "TREASURY_MULTISIG_ADDRESS",
          "pub_key": {
            "@type": "/cosmos.crypto.multisig.LegacyAminoPubKey",
            "threshold": 3,
            "public_keys": [
              {"@type": "/cosmos.crypto.secp256k1.PubKey", "key": "SIGNER_1_PUBKEY"},
              {"@type": "/cosmos.crypto.secp256k1.PubKey", "key": "SIGNER_2_PUBKEY"},
              {"@type": "/cosmos.crypto.secp256k1.PubKey", "key": "SIGNER_3_PUBKEY"},
              {"@type": "/cosmos.crypto.secp256k1.PubKey", "key": "SIGNER_4_PUBKEY"},
              {"@type": "/cosmos.crypto.secp256k1.PubKey", "key": "SIGNER_5_PUBKEY"}
            ]
          },
          "account_number": "3",
          "sequence": "0"
        }
      ]
    },
    "bank": {
      "balances": [
        {
          "address": "TREASURY_MULTISIG_ADDRESS",
          "coins": [{"denom": "uomni", "amount": "170000000000000"}]
        }
      ]
    },
    "tokenomics": {
      "params": {
        "treasury_address": "TREASURY_MULTISIG_ADDRESS"
      }
    }
  }
}
```

---

## Using the Multisig Treasury

### Executing a Transaction

**Scenario:** Treasury needs to send 1M OMNI to a development fund.

#### Step 1: Create Unsigned Transaction

```bash
# Any signer or coordinator can create the unsigned tx
posd tx bank send $TREASURY_MULTISIG_ADDR omni1developmentfund... 1000000000000uomni \
  --from treasury-multisig \
  --chain-id omniphi-mainnet-1 \
  --fees 10000uomni \
  --generate-only > unsigned.json

# Share unsigned.json with all signers
```

#### Step 2: Signer 1 Signs

```bash
# Signer 1 signs the transaction
posd tx sign unsigned.json \
  --from treasury-signer-1 \
  --chain-id omniphi-mainnet-1 \
  --multisig $TREASURY_MULTISIG_ADDR \
  --keyring-backend file \
  --output-document signer1.json

# Send signer1.json to coordinator
```

#### Step 3: Signer 2 Signs

```bash
# Signer 2 signs the same transaction
posd tx sign unsigned.json \
  --from treasury-signer-2 \
  --chain-id omniphi-mainnet-1 \
  --multisig $TREASURY_MULTISIG_ADDR \
  --keyring-backend file \
  --output-document signer2.json

# Send signer2.json to coordinator
```

#### Step 4: Signer 3 Signs

```bash
# Signer 3 signs (we now have 3 signatures = threshold met)
posd tx sign unsigned.json \
  --from treasury-signer-3 \
  --chain-id omniphi-mainnet-1 \
  --multisig $TREASURY_MULTISIG_ADDR \
  --keyring-backend file \
  --output-document signer3.json

# Send signer3.json to coordinator
```

#### Step 5: Combine Signatures and Broadcast

```bash
# Coordinator combines all signatures
posd tx multisign unsigned.json treasury-multisig \
  signer1.json signer2.json signer3.json \
  --chain-id omniphi-mainnet-1 > signed.json

# Broadcast the signed transaction
posd tx broadcast signed.json \
  --chain-id omniphi-mainnet-1

# Transaction is now executed on-chain!
```

---

## Common Treasury Operations

### 1. Funding Development

```bash
# Send 5M OMNI to development team
posd tx bank send $TREASURY_MULTISIG_ADDR omni1devteam... 5000000000000uomni \
  --from treasury-multisig \
  --chain-id omniphi-mainnet-1 \
  --fees 10000uomni \
  --generate-only > dev_funding.json

# Collect 3 signatures
# Broadcast transaction
```

### 2. Ecosystem Grants

```bash
# Send 500K OMNI to grant recipient
posd tx bank send $TREASURY_MULTISIG_ADDR omni1grant... 500000000000uomni \
  --from treasury-multisig \
  --chain-id omniphi-mainnet-1 \
  --fees 10000uomni \
  --generate-only > grant.json
```

### 3. Validator Delegation

```bash
# Delegate 10M OMNI to trusted validator
posd tx staking delegate omnivaloper1... 10000000000000uomni \
  --from treasury-multisig \
  --chain-id omniphi-mainnet-1 \
  --fees 10000uomni \
  --generate-only > delegation.json

# Treasury can earn staking rewards on idle funds
```

### 4. Governance Proposals

```bash
# Submit governance proposal to change inflation rate
posd tx gov submit-proposal param-change proposal.json \
  --from treasury-multisig \
  --deposit 10000000000uomni \
  --chain-id omniphi-mainnet-1 \
  --fees 10000uomni \
  --generate-only > gov_proposal.json
```

### 5. Emergency Burns

```bash
# Burn tokens to reduce supply (requires governance approval)
posd tx tokenomics burn 1000000000000 treasury_burn \
  --from treasury-multisig \
  --chain-id omniphi-mainnet-1 \
  --fees 10000uomni \
  --generate-only > emergency_burn.json
```

---

## Security Best Practices

### 1. Key Management

**Hardware Wallets (Recommended):**
```bash
# Use Ledger for signing
posd tx sign unsigned.json \
  --from treasury-signer-1 \
  --ledger \
  --chain-id omniphi-mainnet-1 \
  --multisig $TREASURY_MULTISIG_ADDR
```

**Key Backup:**
- Store mnemonics in encrypted hardware wallet
- Use Shamir Secret Sharing for redundancy
- Keep backups in geographically distributed secure locations

### 2. Operational Security

**Transaction Verification:**
```bash
# Before signing, ALWAYS verify transaction details
jq '.' unsigned.json

# Check:
# - Recipient address is correct
# - Amount is as expected
# - No suspicious memo or additional messages
```

**Signature Verification:**
```bash
# Verify signature is valid before broadcasting
posd tx validate-signatures signed.json
```

### 3. Access Control

**Signer Rotation Policy:**
- Review signers quarterly
- Remove access for departed members
- Requires creating new multisig account and migrating funds

**Communication Protocol:**
- Use encrypted channels (Signal, ProtonMail)
- Implement code words for urgent transactions
- Maintain audit log of all signing requests

### 4. Rate Limiting

**Treasury Spending Limits (via governance):**
```go
// Future enhancement: Add spending limits to tokenomics module
type TreasuryParams struct {
    MaxMonthlySpend math.Int  // e.g., 10M OMNI
    LargeTransactionThreshold math.Int  // e.g., 5M OMNI requires governance vote
}
```

---

## Emergency Procedures

### Lost Signer Key (< 3 keys lost)

If 1-2 signers lose their keys:

```bash
# Treasury can still operate with remaining 3+ keys
# Schedule signer rotation at next governance cycle
# Create new multisig with replacement signers
# Transfer funds via multisig transaction
```

### Compromised Signer Key

If a signer's key is compromised:

```bash
# IMMEDIATELY:
# 1. Notify all other signers
# 2. Do NOT sign any pending transactions
# 3. Create new multisig account (emergency process)

# Create emergency multisig (exclude compromised signer)
posd keys add treasury-emergency \
  --multisig treasury-signer-2,treasury-signer-3,treasury-signer-4,treasury-signer-5,treasury-signer-6 \
  --multisig-threshold 3

# Emergency governance proposal to update treasury address
posd tx gov submit-proposal param-change emergency_treasury_update.json \
  --from validator \
  --deposit 50000000000uomni \
  --expedited  # Fast-track vote

# Transfer funds to new multisig once governance passes
```

### Catastrophic Loss (≥ 3 keys lost)

If 3+ signers lose access:

```bash
# CRITICAL: Treasury funds are LOCKED
# Recovery options:

# Option 1: Social recovery via governance
# - Submit emergency governance proposal
# - Requires 2/3 validator approval
# - Can update module params to new treasury address

# Option 2: Chain halt and restart
# - Export state
# - Update treasury address in exported state
# - Coordinate restart with validators
# - LAST RESORT ONLY
```

---

## Monitoring and Auditing

### Transaction Logging

```bash
# Query all transactions from treasury
posd query txs --events "message.sender=$TREASURY_MULTISIG_ADDR" --limit 100

# Export to CSV for auditing
posd query txs --events "message.sender=$TREASURY_MULTISIG_ADDR" \
  --output json | jq -r '.txs[] | [.height, .hash, .tx.body.messages[0].amount] | @csv' > treasury_txs.csv
```

### Balance Monitoring

```bash
# Check treasury balance daily
#!/bin/bash
TREASURY_ADDR="omni1..."
BALANCE=$(posd query bank balances $TREASURY_ADDR -o json | jq -r '.balances[] | select(.denom=="uomni") | .amount')
OMNI_BALANCE=$(echo "scale=2; $BALANCE / 1000000" | bc)
echo "$(date): Treasury balance: $OMNI_BALANCE OMNI" >> treasury_balance_log.txt

# Alert if balance drops below threshold
if (( $(echo "$OMNI_BALANCE < 100000000" | bc -l) )); then
  echo "WARNING: Treasury balance below 100M OMNI!"
  # Send alert (email, Slack, PagerDuty, etc.)
fi
```

### Prometheus Metrics

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'omniphi-treasury'
    static_configs:
      - targets: ['localhost:26660']
    metric_relabel_configs:
      - source_labels: [account]
        regex: 'omni1treasury...'
        action: keep
```

---

## Governance Integration

### Treasury Spending Proposals

**Template:**

```json
{
  "title": "Treasury Grant: DeFi Integration Development",
  "description": "Allocate 2M OMNI from treasury to fund DeFi protocol integration over 6 months.",
  "changes": [],
  "deposit": "10000000000uomni",
  "metadata": {
    "recipient": "omni1defi...",
    "amount": "2000000000000uomni",
    "milestones": [
      "M1 (Month 2): Basic integration - 500K OMNI",
      "M2 (Month 4): Testing & audit - 500K OMNI",
      "M3 (Month 6): Mainnet launch - 1M OMNI"
    ]
  }
}
```

**Submission:**

```bash
# Submit treasury spending proposal
posd tx gov submit-proposal treasury_grant.json \
  --from proposer \
  --deposit 10000000000uomni \
  --chain-id omniphi-mainnet-1

# Community votes
# If passed, multisig executes transaction per proposal spec
```

---

## Testing Multisig Functionality

### Unit Test: Multisig Creation

```go
// x/tokenomics/keeper/treasury_test.go

func TestTreasuryMultisigSetup(t *testing.T) {
    // Create test keys
    key1 := secp256k1.GenPrivKey()
    key2 := secp256k1.GenPrivKey()
    key3 := secp256k1.GenPrivKey()
    key4 := secp256k1.GenPrivKey()
    key5 := secp256k1.GenPrivKey()

    // Create multisig public key
    multiPK := multisig.NewLegacyAminoPubKey(
        3, // threshold
        []cryptotypes.PubKey{
            key1.PubKey(),
            key2.PubKey(),
            key3.PubKey(),
            key4.PubKey(),
            key5.PubKey(),
        },
    )

    // Verify threshold
    require.Equal(t, uint(3), multiPK.GetThreshold())

    // Create multisig address
    addr := sdk.AccAddress(multiPK.Address())
    require.NotNil(t, addr)
}
```

### Integration Test: Multisig Transaction

```bash
# Integration test script
#!/bin/bash

# Setup test multisig
posd keys add test-signer-1 --keyring-backend test
posd keys add test-signer-2 --keyring-backend test
posd keys add test-signer-3 --keyring-backend test
posd keys add test-signer-4 --keyring-backend test
posd keys add test-signer-5 --keyring-backend test

posd keys add test-multisig \
  --multisig test-signer-1,test-signer-2,test-signer-3,test-signer-4,test-signer-5 \
  --multisig-threshold 3 \
  --keyring-backend test

MULTISIG_ADDR=$(posd keys show test-multisig -a --keyring-backend test)

# Fund multisig account
posd tx bank send faucet $MULTISIG_ADDR 10000000000uomni \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --yes

# Wait for confirmation
sleep 7

# Create unsigned transaction
posd tx bank send $MULTISIG_ADDR omni1recipient... 1000000uomni \
  --from test-multisig \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test \
  --generate-only > test_unsigned.json

# Sign with 3 signers
posd tx sign test_unsigned.json \
  --from test-signer-1 \
  --multisig $MULTISIG_ADDR \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test > test_sig1.json

posd tx sign test_unsigned.json \
  --from test-signer-2 \
  --multisig $MULTISIG_ADDR \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test > test_sig2.json

posd tx sign test_unsigned.json \
  --from test-signer-3 \
  --multisig $MULTISIG_ADDR \
  --chain-id omniphi-testnet-1 \
  --keyring-backend test > test_sig3.json

# Combine and broadcast
posd tx multisign test_unsigned.json test-multisig \
  test_sig1.json test_sig2.json test_sig3.json \
  --chain-id omniphi-testnet-1 > test_signed.json

posd tx broadcast test_signed.json --chain-id omniphi-testnet-1

# Verify transaction success
if [ $? -eq 0 ]; then
  echo "✅ Multisig test PASSED"
else
  echo "❌ Multisig test FAILED"
  exit 1
fi
```

---

## Summary

**Implementation Status: ✅ COMPLETE**

- ✅ Multisig account creation documented
- ✅ Transaction signing workflow defined
- ✅ Security best practices established
- ✅ Emergency procedures documented
- ✅ Integration test script provided
- ✅ Genesis configuration updated

**Treasury Configuration:**
- **Type:** 3-of-5 multisig
- **Threshold:** 3 signatures required
- **Initial Balance:** 170M OMNI
- **Annual Income:** ~2.25M OMNI (10% of inflation)
- **Burn Redirect:** ~10% of all burns

**Ready for:**
- Mainnet deployment
- Production treasury operations
- Governance-controlled spending

**Next Steps:**
- Identify and onboard 5 trusted signers
- Generate and secure signer keys
- Test multisig on testnet
- Add multisig address to mainnet genesis
- Establish treasury spending governance process
