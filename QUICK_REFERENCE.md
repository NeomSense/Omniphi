# Omniphi Quick Reference Card

Essential commands for local testnet testing

---

## 🚀 Setup & Start

```bash
# Build binary
cd chain && go build -o build/posd ./cmd/posd

# Create key
posd keys add validator --keyring-backend test

# Initialize testnet
./scripts/init_testnet.sh validator1

# Start node
posd start

# Check status
posd status
```

---

## 🧪 PoC Module

### Submit Contribution
```bash
posd tx poc submit-contribution \
  "code" \
  "https://github.com/omniphi/omniphi/pull/42" \
  "$(echo -n 'https://github.com/omniphi/omniphi/pull/42' | sha256sum | cut -d' ' -f1)" \
  --from validator \
  --chain-id omniphi-testnet-2 \
  --keyring-backend test \
  --fees 100000omniphi \
  --gas auto --gas-adjustment 1.5 \
  -y
```

### Query Commands
```bash
# Query contribution by ID
posd query poc contribution <ID>

# Query PoC parameters
posd query poc params

# Query C-Score for address
posd query poc cscore <ADDRESS>

# List contributions (if implemented)
posd query poc list-contributions
```

---

## 🏛️ Governance

### Create Proposal
```bash
# Generate proposal template
./scripts/create_proposal.sh poc \
  --title "Update PoC Cap" \
  --summary "Increase C-Score cap to 200K" \
  --output /tmp/proposal.json

# Submit proposal
posd tx gov submit-proposal /tmp/proposal.json \
  --from validator \
  --chain-id omniphi-testnet-2 \
  --keyring-backend test \
  --fees 100000omniphi \
  --gas 500000 \
  -y
```

### Query Proposals
```bash
# List all proposals
posd query gov proposals

# Query specific proposal
posd query gov proposal <PROPOSAL_ID>

# Query vote
posd query gov vote <PROPOSAL_ID> <VOTER_ADDRESS>

# Query tally
posd query gov tally <PROPOSAL_ID>
```

### Vote on Proposal
```bash
posd tx gov vote <PROPOSAL_ID> yes \
  --from validator \
  --chain-id omniphi-testnet-2 \
  --keyring-backend test \
  --fees 50000omniphi \
  -y

# Vote options: yes, no, abstain, no_with_veto
```

---

## 💰 Bank & Balances

```bash
# Check balance
posd query bank balances <ADDRESS>

# Send tokens
posd tx bank send <FROM_KEY> <TO_ADDRESS> <AMOUNT>omniphi \
  --chain-id omniphi-testnet-2 \
  --keyring-backend test \
  --fees 50000omniphi \
  -y

# Get validator address
posd keys show validator --keyring-backend test -a
```

---

## 🔍 Query Transaction

```bash
# Query by hash
posd query tx <TX_HASH>

# Query and extract events
posd query tx <TX_HASH> | jq '.events'

# Get specific event attribute
posd query tx <TX_HASH> | jq '.events[] | select(.type=="poc_submit")'
```

---

## 🔐 Keys Management

```bash
# Create new key
posd keys add <KEY_NAME> --keyring-backend test

# List all keys
posd keys list --keyring-backend test

# Show specific key
posd keys show <KEY_NAME> --keyring-backend test

# Export key (backup)
posd keys export <KEY_NAME> --keyring-backend test

# Delete key
posd keys delete <KEY_NAME> --keyring-backend test
```

---

## 🛠️ Troubleshooting

```bash
# View logs (if running in background)
tail -f ~/.pos/node.log

# Check node sync status
posd status | jq '.sync_info'

# Validate genesis
posd genesis validate-genesis

# Reset node data (keeps config)
posd comet unsafe-reset-all

# Get node info
posd status | jq '.node_info'

# Get validator info
posd query staking validator <VALIDATOR_ADDRESS>
```

---

## 📊 Monitoring

```bash
# Check latest block
posd status | jq '.sync_info.latest_block_height'

# Check validator set
posd query staking validators

# Check module accounts
posd query auth module-accounts

# Get governance module address
posd query auth module-account gov
```

---

## 🧹 Cleanup

```bash
# Stop node
pkill posd

# Remove data only (safe)
rm -rf ~/.pos/data
rm -f ~/.pos/config/genesis.json

# Remove everything (WARNING: backs up keys first!)
cp -r ~/.pos/keyring-test ~/backup-keys-$(date +%s)
rm -rf ~/.pos
```

---

## 🎯 Integration Test

```bash
# Run full integration test
cd chain/scripts
bash test_local_integration.sh

# Test includes:
# ✓ PoC contribution submission
# ✓ PoC parameter queries
# ✓ Governance proposal creation
# ✓ Governance voting
# ✓ Tally queries
```

---

## 📝 Configuration Locations

| File | Location |
|------|----------|
| Home directory | `~/.pos/` |
| Genesis file | `~/.pos/config/genesis.json` |
| Node config | `~/.pos/config/config.toml` |
| App config | `~/.pos/config/app.toml` |
| Keyring | `~/.pos/keyring-test/` |
| Node data | `~/.pos/data/` |

---

## 💡 Common Parameters

### Token Denominations
- **Base unit**: `omniphi` (micro-OMNI, 6 decimals)
- **Display unit**: `OMNI`
- **Conversion**: 1 OMNI = 1,000,000 omniphi

### Example Amounts
```
10 OMNI = 10000000omniphi
1K OMNI = 1000000000omniphi
10K OMNI = 10000000000omniphi
100M OMNI = 100000000000000omniphi
```

### Governance
- **Min deposit**: 10,000 OMNI (10000000000omniphi)
- **Voting period**: 300s (5 minutes - testnet)
- **Expedited period**: 60s (1 minute - testnet)

### Gas & Fees
- **Typical fees**: 50000-100000omniphi
- **Gas limit**: Auto or 200000-500000
- **Gas adjustment**: 1.5 (recommended)

---

## 🔗 Useful Links

- **Documentation**: [LOCAL_TESTNET_GUIDE.md](LOCAL_TESTNET_GUIDE.md)
- **PoC Tests**: [TEST_RESULTS_HIGH2_POC.md](TEST_RESULTS_HIGH2_POC.md)
- **CI/CD**: [.github/workflows/poc-tests.yml](.github/workflows/poc-tests.yml)
- **Session Summary**: [SESSION_SUMMARY_2026-02-03.md](SESSION_SUMMARY_2026-02-03.md)

---

**Last Updated**: February 3, 2026
**Chain ID**: omniphi-testnet-2
**Node**: tcp://localhost:26657
