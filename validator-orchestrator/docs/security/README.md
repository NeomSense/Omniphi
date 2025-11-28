# Validator Security Documentation

**Comprehensive security guides for operating Omniphi validators safely.**

---

## Overview

Operating a validator requires understanding and implementing multiple layers of security. These guides cover critical security topics to protect your validator stake and maintain network integrity.

---

## Security Guides

### 1. [Key Management](KEY_MANAGEMENT.md) 🔑

**Critical guide for managing validator keys.**

**Topics covered:**
- Understanding consensus vs operator keys
- Generating and backing up keys securely
- Hardware security modules (HSM)
- Key rotation procedures
- Multi-signature wallets
- Disaster recovery scenarios

**Priority:** 🔴 **CRITICAL** - Read before running a validator

**Key takeaways:**
- Consensus key lives on validator node only
- Operator key stored in wallet/keyring
- Always backup keys in multiple secure locations
- Never run two validators with same consensus key

---

### 2. [Firewall Setup](FIREWALL_SETUP.md) 🛡️

**Essential guide for configuring firewalls to protect validator infrastructure.**

**Topics covered:**
- UFW configuration (Ubuntu/Debian)
- iptables manual setup
- Cloud provider firewalls (AWS, DigitalOcean, GCP)
- DDoS protection and rate limiting
- Sentry node architecture
- Fail2Ban integration

**Priority:** 🔴 **CRITICAL** - Configure before exposing validator to internet

**Key takeaways:**
- Only port 26656 (P2P) needs public access
- Keep RPC/API ports localhost-only
- Restrict SSH to your IP
- Use sentry nodes for high-value validators

---

### 3. [Slashing Protection](SLASHING_PROTECTION.md) ⚠️

**Critical guide for preventing slashing penalties.**

**Topics covered:**
- Double-signing protection
- Downtime prevention
- State file management
- High availability setups
- Monitoring and alerts
- Recovery procedures

**Priority:** 🔴 **CRITICAL** - Slashing results in permanent loss of stake

**Key takeaways:**
- Double-signing = 5% slash + permanent removal (tombstone)
- Downtime = 0.01% slash + temporary jailing
- Never run two validators with same key
- Monitor missed blocks continuously

---

## Security Checklist

### Before Running Validator (Pre-Launch)

#### ✅ Key Security
- [ ] Consensus key generated and backed up (encrypted)
- [ ] Operator wallet created and mnemonic stored offline
- [ ] Hardware wallet configured (recommended for large stakes)
- [ ] Key backup tested (restore from backup successfully)
- [ ] File permissions set correctly (`chmod 600` on sensitive files)

#### ✅ Server Security
- [ ] Operating system updated to latest security patches
- [ ] SSH hardened (key-based auth, no root login, custom port)
- [ ] Firewall configured (UFW or iptables)
- [ ] Fail2Ban installed and configured
- [ ] Monitoring agents installed (Prometheus, node_exporter)

#### ✅ Network Security
- [ ] Port 26656 (P2P) open to internet
- [ ] Port 26657 (RPC) restricted to localhost or specific IPs
- [ ] Port 22 (SSH) restricted to your IP or VPN
- [ ] Cloud provider security groups configured
- [ ] DDoS protection considered (Cloudflare, Sentry nodes)

#### ✅ Operational Security
- [ ] Monitoring and alerting configured
- [ ] Backup automation set up
- [ ] Disaster recovery plan documented
- [ ] Emergency contacts list created
- [ ] Tested validator start/stop procedures

---

### During Validator Operation (Ongoing)

#### ✅ Daily Checks
- [ ] Validator is signing blocks (check block explorer)
- [ ] Peer count healthy (>10 peers)
- [ ] No alerts triggered (email, Telegram, PagerDuty)
- [ ] Uptime > 99% in last 24 hours

#### ✅ Weekly Checks
- [ ] Review monitoring dashboards (Grafana)
- [ ] Check missed blocks counter (should be low)
- [ ] Verify backups are running successfully
- [ ] Review firewall logs for suspicious activity
- [ ] Check system resource usage (disk, RAM, CPU)

#### ✅ Monthly Checks
- [ ] Security updates applied to OS
- [ ] Validator software updated to latest stable version
- [ ] Backup restoration tested
- [ ] Review and rotate access credentials
- [ ] Audit SSH access logs

#### ✅ Quarterly Checks
- [ ] Full security audit
- [ ] Test disaster recovery procedures
- [ ] Review and update documentation
- [ ] Evaluate new security tools and practices
- [ ] Review insurance coverage (if applicable)

---

## Security Layers

### Layer 1: Physical/Cloud Security
- **Physical servers:** Secure data center, access controls
- **Cloud instances:** Strong account credentials, 2FA enabled
- **Network:** Isolated VPCs, private subnets

### Layer 2: Operating System Security
- **Updates:** Regular security patches
- **Users:** Principle of least privilege
- **Firewall:** UFW/iptables configured
- **SSH:** Key-based auth, no root login

### Layer 3: Application Security
- **Validator binary:** Downloaded from trusted sources, verify checksums
- **Configuration:** Secure defaults, minimal exposed services
- **Keys:** Encrypted backups, restricted file permissions

### Layer 4: Network Security
- **Firewall:** Only necessary ports open
- **DDoS protection:** Rate limiting, Fail2Ban
- **Sentry nodes:** Shield validator from direct internet exposure
- **VPN:** Consider VPN for administrative access

### Layer 5: Operational Security
- **Monitoring:** 24/7 alerting on critical metrics
- **Backups:** Automated, encrypted, tested regularly
- **Incident response:** Documented procedures
- **Team:** Trained on security best practices

---

## Common Attack Vectors & Mitigations

### Attack: SSH Brute Force

**Vector:** Attackers try thousands of password combinations on SSH port

**Mitigation:**
- Use SSH keys instead of passwords
- Install Fail2Ban (locks out after failed attempts)
- Change SSH to non-standard port (e.g., 2222)
- Restrict SSH access to specific IPs

**Reference:** [FIREWALL_SETUP.md#fail2ban-integration](FIREWALL_SETUP.md#fail2ban-integration)

---

### Attack: DDoS on P2P Port

**Vector:** Flood validator with connection requests on port 26656

**Mitigation:**
- Implement rate limiting (iptables `--connlimit`)
- Use sentry node architecture
- Enable DDoS protection (Cloudflare, cloud provider)
- Monitor connection counts

**Reference:** [FIREWALL_SETUP.md#ddos-protection](FIREWALL_SETUP.md#ddos-protection)

---

### Attack: Consensus Key Theft

**Vector:** Attacker gains access to `priv_validator_key.json`

**Mitigation:**
- Encrypt backups with strong password
- Set file permissions to 600
- Use HSM (tmkms + YubiHSM)
- Never store keys in cloud storage
- Monitor for unauthorized access

**Reference:** [KEY_MANAGEMENT.md#securing-consensus-key-on-validator](KEY_MANAGEMENT.md#securing-consensus-key-on-validator)

---

### Attack: Operator Wallet Compromise

**Vector:** Attacker steals mnemonic or wallet keyfile

**Mitigation:**
- Store mnemonic offline (paper, metal plate)
- Use hardware wallet (Ledger)
- Enable 2FA on associated accounts
- Use multi-signature wallet for organizations
- Never store mnemonic digitally (photos, cloud)

**Reference:** [KEY_MANAGEMENT.md#backing-up-operator-key](KEY_MANAGEMENT.md#backing-up-operator-key)

---

### Attack: Double-Signing Induced Slashing

**Vector:** Attacker tricks you into running two validators with same key

**Mitigation:**
- Never copy consensus key to second server
- Use time-delayed failover (5+ minutes)
- Implement monitoring for double-sign detection
- Use Horcrux for distributed signing

**Reference:** [SLASHING_PROTECTION.md#double-signing-protection](SLASHING_PROTECTION.md#double-signing-protection)

---

### Attack: Social Engineering

**Vector:** Attacker impersonates support staff to obtain keys

**Mitigation:**
- **NEVER** share private keys or mnemonic
- Verify identity before sharing validator info
- Be cautious of phishing emails/messages
- Official support will NEVER ask for keys

**Red flags:**
- "We need your keys to fix an issue"
- "Urgent: Send mnemonic to prevent slashing"
- Links to fake websites (typosquatting)

---

## Incident Response Plan

### Step 1: Detect

**How you'll know something is wrong:**
- Monitoring alerts triggered
- Unexpected slashing event
- Balance decreased without authorization
- Validator missing blocks
- Unusual server access logs

---

### Step 2: Assess

**Determine severity:**

| Severity | Examples | Response Time |
|----------|----------|---------------|
| **Critical** | Double-signing, wallet drained | Immediate (< 5 min) |
| **High** | Validator jailed, server compromised | Urgent (< 30 min) |
| **Medium** | High missed blocks, low peer count | Soon (< 2 hours) |
| **Low** | Monitoring alert false positive | Normal (< 24 hours) |

---

### Step 3: Contain

**Immediate actions based on incident type:**

**If consensus key compromised:**
```bash
# Stop validator immediately
sudo systemctl stop posd

# Investigate breach
sudo grep "Accepted" /var/log/auth.log

# Rotate consensus key (see KEY_MANAGEMENT.md)
```

**If operator wallet compromised:**
```bash
# Create new wallet
posd keys add new-wallet

# Transfer remaining funds
posd tx bank send old-wallet new-wallet <amount> ...
```

**If server compromised:**
```bash
# Disconnect from network
sudo ufw deny out

# Backup evidence
tar -czf evidence-$(date +%Y%m%d-%H%M%S).tar.gz /var/log/

# Rebuild server from clean image
```

---

### Step 4: Recover

**Follow recovery procedures:**

- **Downtime slashing:** [SLASHING_PROTECTION.md#recovering-from-downtime-slashing](SLASHING_PROTECTION.md#recovering-from-downtime-slashing)
- **Key loss:** [KEY_MANAGEMENT.md#key-disaster-recovery](KEY_MANAGEMENT.md#key-disaster-recovery)
- **Server failure:** Restore from backup, resync validator

---

### Step 5: Post-Incident Review

**After resolving incident:**

1. **Document timeline:** What happened, when, why
2. **Root cause analysis:** How did breach occur?
3. **Update procedures:** What can prevent recurrence?
4. **Notify stakeholders:** If public validator, inform delegators
5. **Report to authorities:** If criminal activity suspected

---

## Security Tools & Resources

### Recommended Tools

| Tool | Purpose | Link |
|------|---------|------|
| **UFW** | Firewall management | `apt-get install ufw` |
| **Fail2Ban** | Intrusion prevention | `apt-get install fail2ban` |
| **tmkms** | Hardware security module | https://github.com/iqlusioninc/tmkms |
| **Horcrux** | Distributed validator | https://github.com/strangelove-ventures/horcrux |
| **Prometheus** | Monitoring | https://prometheus.io |
| **Grafana** | Visualization | https://grafana.com |
| **GPG** | Encryption | `apt-get install gnupg` |
| **rkhunter** | Rootkit detection | `apt-get install rkhunter` |
| **chkrootkit** | Rootkit detection | `apt-get install chkrootkit` |
| **Auditd** | System auditing | `apt-get install auditd` |

---

### External Resources

- **Cosmos Validator Security Best Practices:** https://docs.cosmos.network/main/validators/security
- **CometBFT Security:** https://docs.cometbft.com/v0.38/core/validators
- **OWASP Top 10:** https://owasp.org/www-project-top-ten/
- **CIS Benchmarks:** https://www.cisecurity.org/cis-benchmarks

---

## Emergency Contacts

**Before running a validator, document these:**

| Contact Type | Details |
|-------------|----------|
| **Primary Operator** | Name, phone, email |
| **Backup Operator** | Name, phone, email |
| **Hosting Provider** | Support phone, account ID |
| **Omniphi Team** | Discord, Telegram, support email |
| **Legal/Compliance** | If institutional validator |

**Template emergency runbook:** [Create in your ops repo]

---

## Summary

**Security is a continuous process, not a one-time setup.**

**Minimum security requirements:**
1. ✅ Keys backed up securely (encrypted, offline)
2. ✅ Firewall configured (only P2P port public)
3. ✅ Monitoring and alerts active
4. ✅ Regular backups (automated, tested)
5. ✅ Incident response plan documented

**For high-value validators (>$100k stake), additionally:**
- Use hardware security module (tmkms + YubiHSM)
- Implement sentry node architecture
- Use multi-signature operator wallet
- Purchase cyber insurance
- Hire professional security audit

---

**Questions or security concerns?**

- **Omniphi Security Team:** security@omniphi.io
- **Community Discord:** https://discord.gg/omniphi
- **Bug Bounty Program:** [Coming soon]

---

**Last Updated:** 2025-11-20
