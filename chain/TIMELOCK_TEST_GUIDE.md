# Timelock Module Testing Guide

This guide walks through testing the complete timelock governance integration.

## Overview

The timelock module ensures all governance proposals are delayed by 24 hours before execution, providing time for the community to react to malicious proposals.

## Prerequisites

1. Chain deployed and running
2. At least one validator account with tokens
3. Guardian address configured (optional for basic testing)

## Phase 1: Verify Module Integration

### 1.1 Check Timelock Parameters

```bash
posd query timelock params
```

Expected output:
```json
{
  "params": {
    "min_delay_seconds": "86400",  // 24 hours
    "guardian": ""                  // Empty initially
  }
}
```

### 1.2 Verify No Pending Operations

```bash
posd query timelock queued
posd query timelock executable
```

Both should return empty lists initially.

## Phase 2: Set Guardian Address

### 2.1 Create Guardian Proposal

First, generate a guardian address (or use an existing one):

```bash
# Get your validator address
GUARDIAN=$(posd keys show validator -a)
echo "Guardian address: $GUARDIAN"
```

### 2.2 Submit Governance Proposal to Set Guardian

```bash
# Create proposal JSON
cat > guardian-proposal.json <<EOF
{
  "messages": [
    {
      "@type": "/pos.timelock.v1.MsgUpdateGuardian",
      "authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700jdv5h4y",
      "new_guardian": "$GUARDIAN"
    }
  ],
  "metadata": "ipfs://CID",
  "deposit": "10000000omniphi",
  "title": "Set Timelock Guardian",
  "summary": "Set the guardian address for timelock emergency operations"
}
EOF

# Submit the proposal
posd tx gov submit-proposal guardian-proposal.json \
  --from validator \
  --chain-id pos \
  --gas auto \
  --gas-adjustment 1.5 \
  --yes

# Get the proposal ID (should be 1 for first proposal)
PROPOSAL_ID=1
```

### 2.3 Vote and Pass the Proposal

```bash
# Vote yes
posd tx gov vote $PROPOSAL_ID yes \
  --from validator \
  --chain-id pos \
  --yes

# Wait for voting period to end (default: ~1 minute for testing)
sleep 70

# Check proposal status
posd query gov proposal $PROPOSAL_ID
```

### 2.4 Verify Guardian Was Set

**CRITICAL**: This proposal should be queued in timelock, not executed immediately!

```bash
# Check if guardian proposal is queued
posd query timelock queued

# The proposal should show up as operation ID 1
# It should NOT be executable for 24 hours
```

## Phase 3: Test Governance Proposal Queueing

### 3.1 Create a Test Parameter Update Proposal

```bash
# Create a simple parameter change proposal
cat > param-proposal.json <<EOF
{
  "messages": [
    {
      "@type": "/cosmos.bank.v1beta1.MsgUpdateParams",
      "authority": "omni10d07y265gmmuvt4z0w9aw880jnsr700jdv5h4y",
      "params": {
        "send_enabled": [],
        "default_send_enabled": true
      }
    }
  ],
  "metadata": "ipfs://CID",
  "deposit": "10000000omniphi",
  "title": "Test Timelock",
  "summary": "Testing timelock integration with governance"
}
EOF

# Submit proposal
posd tx gov submit-proposal param-proposal.json \
  --from validator \
  --chain-id pos \
  --gas auto \
  --gas-adjustment 1.5 \
  --yes

PROPOSAL_ID=2
```

### 3.2 Vote and Pass

```bash
# Vote yes
posd tx gov vote $PROPOSAL_ID yes \
  --from validator \
  --chain-id pos \
  --yes

# Wait for voting period
sleep 70
```

### 3.3 Verify Proposal Is Queued in Timelock

```bash
# Check gov proposal status
posd query gov proposal $PROPOSAL_ID
# Should show status: PROPOSAL_STATUS_PASSED

# Check timelock queue
posd query timelock queued
# Should show the proposal as a queued operation

# Check operation details
posd query timelock operation 2
# Should show:
# - status: QUEUED
# - executable_time: 24 hours from now
# - proposal_id: 2
```

## Phase 4: Test Operation Execution

### 4.1 Wait for Delay Period (or simulate)

For testing, you can either:
- Wait 24 hours
- Modify params to use a shorter delay (e.g., 60 seconds) via governance

### 4.2 Execute the Operation

```bash
# Check if operation is executable
posd query timelock executable

# If it appears, execute it
posd tx timelock execute 2 \
  --from validator \
  --chain-id pos \
  --yes

# Verify execution
posd query timelock operation 2
# Status should now be: EXECUTED
```

## Phase 5: Test Guardian Emergency Execution

### 5.1 Create Another Proposal

```bash
# Submit another test proposal (ID will be 3)
posd tx gov submit-proposal param-proposal.json \
  --from validator \
  --chain-id pos \
  --yes

# Vote and pass
posd tx gov vote 3 yes --from validator --chain-id pos --yes
sleep 70
```

### 5.2 Guardian Emergency Execute

```bash
# Guardian can execute immediately without waiting
posd tx timelock emergency-execute 3 "Critical security fix" \
  --from validator \
  --chain-id pos \
  --yes

# Check operation status
posd query timelock operation 3
# Should show: EXECUTED (without waiting 24h)
```

## Phase 6: Test Operation Cancellation

### 6.1 Create and Queue Another Proposal

```bash
posd tx gov submit-proposal param-proposal.json --from validator --chain-id pos --yes
posd tx gov vote 4 yes --from validator --chain-id pos --yes
sleep 70
```

### 6.2 Cancel the Operation

```bash
# Guardian or governance can cancel
posd tx timelock cancel 4 "Found vulnerability in proposal" \
  --from validator \
  --chain-id pos \
  --yes

# Verify cancellation
posd query timelock operation 4
# Should show: CANCELLED
```

## Verification Checklist

- [ ] Timelock params loaded successfully
- [ ] Guardian proposal was queued (not executed immediately)
- [ ] Test proposals are queued in timelock after passing governance
- [ ] Operations show correct executable time (24h delay)
- [ ] Operations can be executed after delay
- [ ] Guardian can emergency execute immediately
- [ ] Operations can be cancelled
- [ ] Events are emitted for all operations

## Expected Behavior

### ✅ Correct Behavior
1. All passed governance proposals are queued in timelock
2. Operations cannot execute before delay period
3. Guardian can emergency execute or cancel
4. Regular execution works after delay

### ❌ Incorrect Behavior (Issues to Fix)
1. Proposals execute immediately without timelock
2. Operations executable before delay period
3. Non-guardian can emergency execute
4. Operation IDs not incrementing

## Troubleshooting

### Proposal Executes Immediately
- Check EndBlocker order in app_config.go
- Verify timelock runs BEFORE gov module
- Check gov hooks are attached

### Operation Not Found
- Verify proposal passed governance
- Check gov hooks fired: `posd query txs --events 'proposal_timelocked'`
- Check keeper has gov keeper reference

### Permission Denied
- Verify guardian address matches params
- Check authority is governance module address

## Next Steps

After successful testing:
1. Deploy to mainnet with production guardian
2. Set min_delay to 86400 (24 hours)
3. Monitor operations via queries
4. Set up alerting for new queued operations
