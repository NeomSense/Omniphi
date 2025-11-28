#!/bin/bash
# Treasury Multisig Integration Test Script
# Tests 3-of-5 multisig functionality on testnet

set -e

# Configuration
CHAIN_ID="omniphi-testnet-1"
KEYRING="test"
NODE="tcp://localhost:26657"

echo "=== Treasury Multisig Integration Test ==="
echo ""

# Step 1: Create 5 test signers
echo "Step 1: Creating 5 signer keys..."
posd keys add test-signer-1 --keyring-backend $KEYRING 2>/dev/null || echo "test-signer-1 already exists"
posd keys add test-signer-2 --keyring-backend $KEYRING 2>/dev/null || echo "test-signer-2 already exists"
posd keys add test-signer-3 --keyring-backend $KEYRING 2>/dev/null || echo "test-signer-3 already exists"
posd keys add test-signer-4 --keyring-backend $KEYRING 2>/dev/null || echo "test-signer-4 already exists"
posd keys add test-signer-5 --keyring-backend $KEYRING 2>/dev/null || echo "test-signer-5 already exists"

echo "✅ Signer keys created"
echo ""

# Step 2: Create 3-of-5 multisig account
echo "Step 2: Creating 3-of-5 multisig account..."
posd keys add test-multisig \
  --multisig test-signer-1,test-signer-2,test-signer-3,test-signer-4,test-signer-5 \
  --multisig-threshold 3 \
  --keyring-backend $KEYRING 2>/dev/null || echo "test-multisig already exists"

MULTISIG_ADDR=$(posd keys show test-multisig -a --keyring-backend $KEYRING)
echo "✅ Multisig address: $MULTISIG_ADDR"
echo ""

# Step 3: Fund multisig account from faucet
echo "Step 3: Funding multisig account with 10,000 OMNI..."
FAUCET_ADDR=$(posd keys show faucet -a --keyring-backend $KEYRING 2>/dev/null || posd keys list --keyring-backend $KEYRING | grep -A 1 "name: faucet" | grep "address:" | awk '{print $2}')

if [ -z "$FAUCET_ADDR" ]; then
  echo "❌ ERROR: Faucet account not found. Please create faucet account first."
  exit 1
fi

posd tx bank send $FAUCET_ADDR $MULTISIG_ADDR 10000000000uomni \
  --chain-id $CHAIN_ID \
  --keyring-backend $KEYRING \
  --fees 5000uomni \
  --node $NODE \
  --yes

echo "Waiting for transaction confirmation (7 seconds)..."
sleep 7

# Verify balance
BALANCE=$(posd query bank balances $MULTISIG_ADDR --node $NODE -o json | jq -r '.balances[] | select(.denom=="uomni") | .amount')
echo "✅ Multisig balance: $BALANCE uomni"
echo ""

# Step 4: Create recipient address
echo "Step 4: Creating test recipient..."
posd keys add test-recipient --keyring-backend $KEYRING 2>/dev/null || echo "test-recipient already exists"
RECIPIENT_ADDR=$(posd keys show test-recipient -a --keyring-backend $KEYRING)
echo "✅ Recipient address: $RECIPIENT_ADDR"
echo ""

# Step 5: Create unsigned transaction
echo "Step 5: Creating unsigned transaction (send 1 OMNI)..."
posd tx bank send $MULTISIG_ADDR $RECIPIENT_ADDR 1000000uomni \
  --from test-multisig \
  --chain-id $CHAIN_ID \
  --keyring-backend $KEYRING \
  --fees 5000uomni \
  --generate-only > test_unsigned.json

echo "✅ Unsigned transaction created: test_unsigned.json"
cat test_unsigned.json | jq '.'
echo ""

# Step 6: Collect 3 signatures (threshold)
echo "Step 6: Collecting signatures from 3 signers..."

echo "  - Signer 1 signing..."
posd tx sign test_unsigned.json \
  --from test-signer-1 \
  --multisig $MULTISIG_ADDR \
  --chain-id $CHAIN_ID \
  --keyring-backend $KEYRING \
  --node $NODE > test_sig1.json
echo "  ✅ Signature 1 collected"

echo "  - Signer 2 signing..."
posd tx sign test_unsigned.json \
  --from test-signer-2 \
  --multisig $MULTISIG_ADDR \
  --chain-id $CHAIN_ID \
  --keyring-backend $KEYRING \
  --node $NODE > test_sig2.json
echo "  ✅ Signature 2 collected"

echo "  - Signer 3 signing..."
posd tx sign test_unsigned.json \
  --from test-signer-3 \
  --multisig $MULTISIG_ADDR \
  --chain-id $CHAIN_ID \
  --keyring-backend $KEYRING \
  --node $NODE > test_sig3.json
echo "  ✅ Signature 3 collected (threshold reached)"
echo ""

# Step 7: Combine signatures
echo "Step 7: Combining signatures into multisig transaction..."
posd tx multisign test_unsigned.json test-multisig \
  test_sig1.json test_sig2.json test_sig3.json \
  --chain-id $CHAIN_ID \
  --keyring-backend $KEYRING > test_signed.json

echo "✅ Multisig transaction created: test_signed.json"
echo ""

# Step 8: Validate signatures
echo "Step 8: Validating signatures..."
posd tx validate-signatures test_signed.json --chain-id $CHAIN_ID --keyring-backend $KEYRING

if [ $? -eq 0 ]; then
  echo "✅ Signatures valid"
else
  echo "❌ Signature validation FAILED"
  exit 1
fi
echo ""

# Step 9: Broadcast transaction
echo "Step 9: Broadcasting multisig transaction..."
TX_RESULT=$(posd tx broadcast test_signed.json --chain-id $CHAIN_ID --node $NODE -o json)
TX_CODE=$(echo $TX_RESULT | jq -r '.code')

if [ "$TX_CODE" = "0" ] || [ "$TX_CODE" = "null" ]; then
  echo "✅ Transaction broadcast successful"
  echo "Transaction hash: $(echo $TX_RESULT | jq -r '.txhash')"
else
  echo "❌ Transaction broadcast FAILED"
  echo "Error: $(echo $TX_RESULT | jq -r '.raw_log')"
  exit 1
fi
echo ""

# Step 10: Verify transaction
echo "Step 10: Waiting for transaction confirmation (7 seconds)..."
sleep 7

RECIPIENT_BALANCE=$(posd query bank balances $RECIPIENT_ADDR --node $NODE -o json | jq -r '.balances[] | select(.denom=="uomni") | .amount')
echo "Recipient balance: $RECIPIENT_BALANCE uomni"

if [ "$RECIPIENT_BALANCE" -ge "1000000" ]; then
  echo "✅ Transaction confirmed - recipient received funds"
else
  echo "❌ Transaction NOT confirmed - recipient balance is $RECIPIENT_BALANCE"
  exit 1
fi
echo ""

# Cleanup
echo "Cleaning up test files..."
rm -f test_unsigned.json test_sig1.json test_sig2.json test_sig3.json test_signed.json
echo "✅ Test files removed"
echo ""

# Summary
echo "=========================================="
echo "✅ MULTISIG INTEGRATION TEST PASSED"
echo "=========================================="
echo ""
echo "Test Summary:"
echo "  - Created 3-of-5 multisig account"
echo "  - Funded with 10,000 OMNI"
echo "  - Sent 1 OMNI to recipient"
echo "  - Required 3 signatures (threshold met)"
echo "  - Transaction confirmed on-chain"
echo ""
echo "Multisig Address: $MULTISIG_ADDR"
echo "Recipient Address: $RECIPIENT_ADDR"
echo "Amount Sent: 1.0 OMNI"
echo "Status: SUCCESS ✅"
echo ""
