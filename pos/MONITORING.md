# Omniphi Blockchain Monitoring Guide

Quick reference for monitoring your Omniphi testnet on both Ubuntu/Linux/Mac and Windows.

---

## Quick Health Check

Check if your node is running and producing blocks.

### Ubuntu/Linux/Mac
```bash
./health_check.sh
```

### Windows
```powershell
.\health_check.ps1
```

---

## Continuous Live Monitor

Real-time blockchain monitoring with auto-refresh.

### Ubuntu/Linux/Mac
```bash
./monitor.sh
```

### Windows
```powershell
.\monitor.ps1
```

---

## Manual Status Commands

### Get Full Status (JSON)

**Ubuntu/Linux/Mac:**
```bash
./posd status --home ./testnet-2nodes/validator1
```

**Windows:**
```powershell
.\posd.exe status --home .\testnet-2nodes\validator1
```

### Get Block Height

**Ubuntu/Linux/Mac:**
```bash
./posd status --home ./testnet-2nodes/validator1 | grep -o '"latest_block_height":"[^"]*"' | cut -d'"' -f4
```

**Windows:**
```powershell
$status = .\posd.exe status --home .\testnet-2nodes\validator1 | ConvertFrom-Json
$status.sync_info.latest_block_height
```

### Check Sync Status

**Ubuntu/Linux/Mac:**
```bash
./posd status --home ./testnet-2nodes/validator1 | grep -o '"catching_up":[^,]*' | cut -d':' -f2
```

**Windows:**
```powershell
$status = .\posd.exe status --home .\testnet-2nodes\validator1 | ConvertFrom-Json
$status.sync_info.catching_up
```

### Check Peer Count

**Ubuntu/Linux/Mac:**
```bash
./posd status --home ./testnet-2nodes/validator1 | grep -o '"n_peers":"[^"]*"' | cut -d'"' -f4
```

**Windows:**
```powershell
$status = .\posd.exe status --home .\testnet-2nodes\validator1 | ConvertFrom-Json
$status.node_info.other.n_peers
```

---

## Query Commands

### Check Validators

**Ubuntu/Linux/Mac:**
```bash
./posd query staking validators --home ./testnet-2nodes/validator1
```

**Windows:**
```powershell
.\posd.exe query staking validators --home .\testnet-2nodes\validator1
```

### Check Account Balance

**Ubuntu/Linux/Mac:**
```bash
./posd query bank balances omni1g3agstj7m9jn8rrty2nzuwew5qp3xzvurn43y8 --home ./testnet-2nodes/validator1
```

**Windows:**
```powershell
.\posd.exe query bank balances omni1g3agstj7m9jn8rrty2nzuwew5qp3xzvurn43y8 --home .\testnet-2nodes\validator1
```

### Get Latest Block

**Ubuntu/Linux/Mac:**
```bash
./posd query block --home ./testnet-2nodes/validator1
```

**Windows:**
```powershell
.\posd.exe query block --home .\testnet-2nodes\validator1
```

---

## Expected Output

### Healthy Blockchain
```
✓ Node is running
  Block Height: 136
  Peers: 1
  Status: ✓ SYNCED AND PRODUCING BLOCKS
```

### Waiting for Peer
```
✓ Node is running
  Block Height: 0
  Peers: 0
  Status: ⏳ WAITING FOR PEER
```

### Node Not Running
```
✗ Node is not responding
  Make sure the validator is running:
  ./start_validator1.sh  (Ubuntu)
  .\start_validator1.ps1 (Windows)
```

---

## Troubleshooting

### Node Not Responding

**Check if process is running:**

Ubuntu/Linux/Mac:
```bash
ps aux | grep posd
```

Windows:
```powershell
Get-Process | Where-Object {$_.ProcessName -like "*posd*"}
```

**Check logs:**

Ubuntu/Linux/Mac:
```bash
tail -f ./testnet-2nodes/validator1/posd.log
```

Windows:
```powershell
Get-Content .\testnet-2nodes\validator1\posd.log -Tail 50 -Wait
```

**Restart validator:**

Ubuntu/Linux/Mac:
```bash
./start_validator1.sh
```

Windows:
```powershell
.\start_validator1.ps1
```

### Blocks Not Increasing

1. Check both validators are running
2. Verify network connectivity between computers
3. Check firewall - Port 26656 must be open
4. Verify peer configuration in `config/config.toml`

---

## Quick Reference Table

| Task | Ubuntu/Linux/Mac | Windows |
|------|------------------|---------|
| Health Check | `./health_check.sh` | `.\health_check.ps1` |
| Live Monitor | `./monitor.sh` | `.\monitor.ps1` |
| Status | `./posd status --home ./testnet-2nodes/validator1` | `.\posd.exe status --home .\testnet-2nodes\validator1` |
| Validators | `./posd query staking validators` | `.\posd.exe query staking validators` |
| Logs | `tail -f logfile` | `Get-Content -Tail 50 -Wait` |
