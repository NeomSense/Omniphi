# Timelock Deployment Commands

Run these commands on your VPS to deploy and test the timelock module.

## Part 1: Deploy Updated Binary

```bash
# SSH to your VPS
ssh root@167.88.35.192

# Stop the node
sudo systemctl stop posd

# Pull latest code
cd ~/omniphi/chain
git pull origin main

# Build
make build

# Install
sudo cp build/posd /usr/local/bin/posd
sudo chmod +x /usr/local/bin/posd

# Verify
posd version

# Start the node
sudo systemctl start posd

# Check status
sudo systemctl status posd
sudo journalctl -u posd -f -n 50
```

Wait for the node to sync and stabilize (30-60 seconds).

## Part 2: Step 1 - Check Timelock Parameters

```bash
# Query timelock params
posd query timelock params --node tcp://localhost:26657
```

**Expected output:**
```json
{
  "params": {
    "min_delay_seconds": "86400",
    "guardian": ""
  }
}
```

## Part 3: Step 2 - Verify No Pending Operations

```bash
# Check queued operations
posd query timelock queued --node tcp://localhost:26657

# Check executable operations
posd query timelock executable --node tcp://localhost:26657
```

**Expected output:** Both should return empty lists `[]`

## Part 4: Step 3 - Get Guardian Address

```bash
# Get your validator address to use as guardian
GUARDIAN=$(posd keys show validator -a)
echo "Guardian address: $GUARDIAN"
```

**Save this address - you'll need it for the proposal.**

## Part 5: Step 4 - Create Guardian Proposal

```bash
# Create the proposal JSON file
cat > guardian-proposal.json <<'EOF'
{
  "messages": [
    {
      "@type": "/pos.timelock.v1.MsgUpdateGuardian",
      "authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700jdv5h4y",
      "new_guardian": "PUT_YOUR_GUARDIAN_ADDRESS_HERE"
    }
  ],
  "metadata": "ipfs://QmTimelockGuardian",
  "deposit": "10000000uomni",
  "title": "Set Timelock Guardian",
  "summary": "Set the guardian address for timelock emergency operations. This address will have the ability to cancel malicious proposals and execute emergency operations."
}
EOF

# Replace the guardian address with your actual address
# Edit the file: nano guardian-proposal.json
# Replace PUT_YOUR_GUARDIAN_ADDRESS_HERE with the $GUARDIAN value from above

# Submit the proposal
posd tx gov submit-proposal guardian-proposal.json \
  --from validator \
  --chain-id pos \
  --gas auto \
  --gas-adjustment 1.5 \
  --node tcp://localhost:26657 \
  --yes

# Wait a few seconds for the transaction to be included
sleep 5

# Check if proposal was created
posd query gov proposals --node tcp://localhost:26657

# Get the proposal ID (should be the latest one)
PROPOSAL_ID=$(posd query gov proposals --node tcp://localhost:26657 --output json | jq -r '.proposals[-1].id')
echo "Proposal ID: $PROPOSAL_ID"
```

## Part 6: Vote on the Proposal

```bash
# Vote yes on the proposal
posd tx gov vote $PROPOSAL_ID yes \
  --from validator \
  --chain-id pos \
  --node tcp://localhost:26657 \
  --yes

# Check proposal status
posd query gov proposal $PROPOSAL_ID --node tcp://localhost:26657

# Check voting
posd query gov votes $PROPOSAL_ID --node tcp://localhost:26657
```

## Part 7: Wait for Voting Period to End

```bash
# Check when voting period ends
posd query gov proposal $PROPOSAL_ID --node tcp://localhost:26657 | grep voting_end_time

# Wait for voting period (default is usually 1-2 minutes for testing)
# You can monitor the chain height:
watch -n 2 'posd status 2>&1 | jq -r .sync_info.latest_block_height'
```

## Part 8: CRITICAL - Verify Proposal Is Queued (Not Executed)

**This is the key test - the proposal should be queued in timelock, NOT executed immediately!**

```bash
# After voting period ends, check if proposal is in timelock queue
posd query timelock queued --node tcp://localhost:26657

# Expected: You should see operation ID 1 with your guardian proposal
# The operation should have:
# - status: QUEUED
# - executable_time: 24 hours from now
# - proposal_id: $PROPOSAL_ID

# Also check the operation details
posd query timelock operation 1 --node tcp://localhost:26657

# Verify guardian is NOT yet set (because proposal is queued, not executed)
posd query timelock params --node tcp://localhost:26657
# Guardian should still be empty ""
```

## Expected Results

✅ **Success indicators:**
1. Proposal passes governance vote
2. Proposal appears in `posd query timelock queued`
3. Proposal is NOT executed immediately
4. Guardian parameter is still empty
5. Operation shows 24-hour delay

❌ **Failure indicators:**
1. Proposal executes immediately after voting
2. Guardian is set right away
3. Proposal doesn't appear in timelock queue
4. Any errors in logs

## Troubleshooting

If the proposal executes immediately instead of being queued:

```bash
# Check if hooks are firing
sudo journalctl -u posd -n 1000 | grep "proposal marked for timelock"

# Check EndBlocker order
sudo journalctl -u posd -n 1000 | grep "EndBlocker"

# Check for any errors
sudo journalctl -u posd -n 500 | grep -i error
```

## Next Steps

After confirming the proposal is queued:
1. Wait 24 hours (or modify min_delay via governance for faster testing)
2. Execute the operation: `posd tx timelock execute 1 --from validator --chain-id pos --yes`
3. Verify guardian is set: `posd query timelock params`

---

**For the full testing guide, see:** `chain/TIMELOCK_TEST_GUIDE.md`
