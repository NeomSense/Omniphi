# Validator Operations Documentation

**Day-to-day operational guides for running Omniphi validators.**

---

## Overview

This directory contains operational guides for managing validator nodes after initial setup. These guides cover routine operations, maintenance, and best practices for keeping your validator running smoothly.

---

## Operational Guides

### 1. [State Sync](STATE_SYNC.md) ⚡

**Fast-sync your validator in minutes instead of days.**

**Topics covered:**
- Quick state sync setup (5 steps)
- Finding trusted RPC endpoints
- Automated state sync scripts
- Troubleshooting sync issues
- When to use state sync vs fast sync

**When to use:**
- ✅ Bootstrapping new validator
- ✅ Recovering from corruption
- ✅ Testing/development nodes
- ❌ Archive nodes (need full history)

**Time to sync:** 5-30 minutes (vs days for full sync)

---

### 2. [Backups and Restore](BACKUPS.md) 💾

**Protect your validator with proper backup strategies.**

**Topics covered:**
- Essential backups (consensus key, state, configuration)
- Manual and automated backup procedures
- Cloud backup solutions (AWS S3, Backblaze B2, Restic)
- Restore procedures for various scenarios
- Testing backups (monthly verification)

**Critical files to backup:**
1. `priv_validator_key.json` - Consensus private key
2. `priv_validator_state.json` - Validator state (prevents double-signing)
3. Wallet mnemonic - Operator key (write on paper/metal)
4. Configuration files - Network settings

**Backup strategy:**
- 📅 Daily automated backups
- 🔐 Encrypted with GPG (AES256)
- 📍 3 locations (local, external, cloud)
- ✅ Tested monthly

---

### 3. [Monitoring and Alerts](MONITORING.md) 📊

**Monitor validator health and prevent downtime.**

**Topics covered:**
- Quick health check scripts
- Prometheus + Grafana setup
- Key metrics to monitor (block height, missed blocks, peers)
- Alerting with Alertmanager
- External uptime monitoring
- Log monitoring and analysis

**Essential metrics:**
- Block height (always increasing)
- Missed blocks (< 1000)
- Peer count (> 5)
- Disk space (< 85%)
- Memory usage (< 90%)
- CPU usage (< 90%)

**Monitoring stack:**
- **Prometheus** - Metrics collection
- **Grafana** - Visualization
- **Alertmanager** - Alert routing
- **node_exporter** - System metrics
- **Uptime Robot** - External monitoring

---

## Operations Workflow

### Daily Operations

**Morning Check (5 minutes):**
```bash
# Quick health check
~/scripts/quick-health-check.sh

# Check block explorer
# Verify validator is signing blocks

# Review any overnight alerts
```

**Expected output:**
- ✓ Process running
- ✓ Caught up with network
- ✓ 10+ peers connected
- ✓ Missed blocks < 100
- ✓ Disk < 80%

---

### Weekly Maintenance

**Every Monday (15 minutes):**

1. **Review monitoring dashboards:**
   - Open Grafana: `http://your-server:3000`
   - Check for anomalies in metrics
   - Review alert history

2. **Check system updates:**
   ```bash
   sudo apt update
   sudo apt list --upgradable

   # Apply security updates (if any)
   sudo apt upgrade -y
   ```

3. **Review backup logs:**
   ```bash
   tail -50 /var/log/omniphi-backup.log

   # Verify backups are running daily
   ls -lh /mnt/backups/omniphi/ | tail -10
   ```

4. **Check disk space trends:**
   ```bash
   df -h ~/.omniphi

   # If > 70%, consider adjusting pruning
   ```

---

### Monthly Tasks

**First of each month (30 minutes):**

1. **Test backup restoration:**
   ```bash
   ~/scripts/test-backup.sh

   # Verify critical files can be restored
   ```

2. **Review validator performance:**
   - Uptime percentage (should be > 99.5%)
   - Missed blocks total
   - Commission earnings
   - Delegations (any changes?)

3. **Security audit:**
   ```bash
   # Check SSH access logs
   sudo grep "Accepted" /var/log/auth.log | tail -50

   # Check failed login attempts
   sudo grep "Failed" /var/log/auth.log | wc -l

   # Review firewall rules
   sudo ufw status verbose
   ```

4. **Update documentation:**
   - Any configuration changes made?
   - New procedures discovered?
   - Update runbooks

---

### Quarterly Tasks

**Every 3 months (2 hours):**

1. **Full disaster recovery test:**
   - Restore validator on test server
   - Verify all procedures work
   - Update recovery documentation

2. **Performance review:**
   - Analyze resource usage trends
   - Optimize configuration if needed
   - Consider hardware upgrades

3. **Security hardening review:**
   - Update OS and packages
   - Rotate SSH keys (if policy requires)
   - Review and update firewall rules
   - Check for new security best practices

4. **Validator software updates:**
   - Check for new Omniphi releases
   - Review upgrade instructions
   - Plan upgrade window

---

## Common Operations

### Checking Validator Status

```bash
# Quick status
posd status | jq '.SyncInfo'

# Detailed validator info
posd query staking validator $(posd keys show my-validator --bech val -a)

# Check if jailed
posd query staking validator $(posd keys show my-validator --bech val -a) | jq '.jailed'

# Missed blocks count
VALIDATOR_ADDR=$(posd tendermint show-validator)
posd query slashing signing-info $VALIDATOR_ADDR | jq '.missed_blocks_counter'
```

---

### Restarting Validator

```bash
# Graceful restart
sudo systemctl restart posd

# Check logs for issues
sudo journalctl -u posd -f

# Verify validator is back online
posd status | jq '.SyncInfo.catching_up'
# Should be: false
```

---

### Updating Validator Metadata

**Edit validator description, website, etc:**

```bash
posd tx staking edit-validator \
  --moniker="My Updated Validator" \
  --website="https://my-validator.com" \
  --details="Omniphi validator operated by..." \
  --chain-id=omniphi-1 \
  --from=my-validator-wallet \
  --gas=auto \
  --gas-adjustment=1.5 \
  --gas-prices=0.025uomni
```

---

### Changing Commission Rate

```bash
# Check current commission
posd query staking validator $(posd keys show my-validator --bech val -a) | jq '.commission'

# Update commission (max once per day, limited by max-change-rate)
posd tx staking edit-validator \
  --commission-rate="0.08" \
  --chain-id=omniphi-1 \
  --from=my-validator-wallet \
  --gas=auto \
  --gas-adjustment=1.5 \
  --gas-prices=0.025uomni
```

---

### Withdrawing Rewards

```bash
# Check accumulated rewards
posd query distribution rewards $(posd keys show my-wallet -a)

# Withdraw validator commission
posd tx distribution withdraw-rewards \
  $(posd keys show my-validator --bech val -a) \
  --commission \
  --from=my-wallet \
  --chain-id=omniphi-1 \
  --gas=auto \
  --gas-adjustment=1.5 \
  --gas-prices=0.025uomni

# Withdraw all delegation rewards
posd tx distribution withdraw-all-rewards \
  --from=my-wallet \
  --chain-id=omniphi-1 \
  --gas=auto \
  --gas-adjustment=1.5 \
  --gas-prices=0.025uomni
```

---

## Troubleshooting Quick Reference

### Issue: Validator Not Signing Blocks

**Check:**
1. Process running? `pgrep posd`
2. Caught up? `posd status | jq '.SyncInfo.catching_up'` (should be `false`)
3. Peers connected? `curl localhost:26657/net_info | jq '.result.n_peers'`
4. Jailed? `posd query staking validator $(posd keys show my-validator --bech val -a) | jq '.jailed'`

**Fix:**
```bash
# If jailed, unjail:
posd tx slashing unjail --from=my-validator-wallet --chain-id=omniphi-1

# If not caught up, check logs:
sudo journalctl -u posd -f

# If low peers, check firewall:
sudo ufw status | grep 26656
```

---

### Issue: High Disk Usage

**Check:**
```bash
df -h ~/.omniphi
du -sh ~/.omniphi/data/*
```

**Fix:**
```bash
# Adjust pruning (see PRUNING_STRATEGIES.md)
nano ~/.omniphi/config/app.toml
# Change: pruning = "custom"
# Set: pruning-keep-recent = "100"

sudo systemctl restart posd
```

---

### Issue: High Memory Usage

**Check:**
```bash
free -h
ps aux | grep posd
```

**Fix:**
```bash
# Reduce IAVL cache
nano ~/.omniphi/config/app.toml
# Change: iavl-cache-size = 400000

# Disable inter-block cache (if necessary)
# Change: inter-block-cache = false

sudo systemctl restart posd
```

---

### Issue: Sync Stuck

**Check:**
```bash
# Is height increasing?
watch -n 5 'posd status | jq .SyncInfo.latest_block_height'

# Check peers
curl localhost:26657/net_info | jq '.result.peers[] | {id: .node_info.id, ip: .remote_ip}'
```

**Fix:**
```bash
# Try state sync (see STATE_SYNC.md)
# Or add more peers:
nano ~/.omniphi/config/config.toml
# Update persistent_peers with known good peers

sudo systemctl restart posd
```

---

## Emergency Procedures

### Validator Compromised

**Immediate actions:**

1. **Stop validator:**
   ```bash
   sudo systemctl stop posd
   ```

2. **Disconnect from network:**
   ```bash
   sudo ufw deny out
   ```

3. **Backup evidence:**
   ```bash
   tar -czf /tmp/evidence-$(date +%Y%m%d-%H%M%S).tar.gz /var/log/ ~/.bash_history
   ```

4. **Notify team and begin incident response**

5. **Follow recovery procedures in [../security/SLASHING_PROTECTION.md](../security/SLASHING_PROTECTION.md)**

---

### Server Failure

**Recovery steps:**

1. **Provision new server** (same specs or better)

2. **Restore from backup:**
   ```bash
   # Copy backup to new server
   scp omniphi-backup-latest.tar.gz.gpg new-server:~/

   # Decrypt and restore (see BACKUPS.md)
   ```

3. **Sync blockchain** (use state sync for speed)

4. **Verify validator signing** before considering old server

---

## Operations Checklist

### Daily
- [ ] Validator signing blocks (check block explorer)
- [ ] No critical alerts
- [ ] Automated backups running

### Weekly
- [ ] Review Grafana dashboards
- [ ] Check system updates
- [ ] Verify backup logs
- [ ] Check disk space trends

### Monthly
- [ ] Test backup restoration
- [ ] Review validator performance
- [ ] Security audit (logs, firewall)
- [ ] Update documentation

### Quarterly
- [ ] Full disaster recovery test
- [ ] Performance optimization review
- [ ] Security hardening review
- [ ] Plan software upgrades

---

## Additional Resources

### Related Documentation
- [TRADITIONAL_SETUP.md](../TRADITIONAL_SETUP.md) - Initial validator setup
- [Security Guides](../security/) - Key management, firewall, slashing protection
- [Configuration Templates](../../infra/configs/) - Config files and tuning

### External Resources
- **Omniphi Docs:** https://docs.omniphi.io
- **Block Explorer:** https://explorer.omniphi.io
- **Discord Community:** https://discord.gg/omniphi
- **Governance Forum:** https://forum.omniphi.io

### Tools
- **Prometheus:** https://prometheus.io
- **Grafana:** https://grafana.com
- **Uptime Robot:** https://uptimerobot.com
- **Better Uptime:** https://betteruptime.com

---

## Support

**Need help with operations?**

- **Community Discord:** https://discord.gg/omniphi (validator channel)
- **Validator Forum:** https://forum.omniphi.io
- **Emergency Contact:** validators@omniphi.io

**Report issues:**
- GitHub: https://github.com/omniphi/validator-orchestrator/issues

---

**Last Updated:** 2025-11-20
