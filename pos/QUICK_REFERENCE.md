# Omniphi Quick Reference Card

## ⚠️ Critical: Always Use Local Binary

**NEVER run:** `posd` (system-wide command)
**ALWAYS run:** `./posd` (local binary in project directory)

---

## 🔨 After Any Code Changes

```bash
cd ~/omniphi/pos
rm -f posd
go build -o posd ./cmd/posd
```

**Why?** Old binaries will have old bugs. Always rebuild to get the latest fixes.

---

## 🚀 Daily Commands

### Start the Blockchain
```bash
cd ~/omniphi/pos
./posd start
```

### Check Status
```bash
cd ~/omniphi/pos
./posd status
```

### Check Block Height
```bash
cd ~/omniphi/pos
./posd status | jq '.sync_info.latest_block_height'
```

### Reset Blockchain Data (Fresh Start)
```bash
cd ~/omniphi/pos
./posd comet unsafe-reset-all
./posd start
```

---

## 🐛 Common Issues

### Issue: "module 'feemarket' is missing a type URL"
**Cause:** Running old binary
**Fix:**
```bash
cd ~/omniphi/pos
rm -f posd
go build -o posd ./cmd/posd
./posd start
```

### Issue: "command not found: posd"
**Cause:** Not in the right directory or forgot `./`
**Fix:**
```bash
cd ~/omniphi/pos
./posd status
```

### Issue: Changes not taking effect
**Cause:** Running old binary
**Fix:** Rebuild (see top of this document)

---

## 📁 Important Files

| File | Purpose |
|------|---------|
| `./posd` | **The binary you run** (always use this) |
| `~/.pos/config/genesis.json` | Blockchain initial state |
| `~/.pos/config/config.toml` | Node configuration |
| `~/.pos/config/app.toml` | Application configuration |
| `~/.pos/data/` | Blockchain database |

---

## 🔑 Quick Tips

1. **Always** be in `~/omniphi/pos` directory
2. **Always** use `./posd` not `posd`
3. **Always** rebuild after code changes
4. Check binary timestamp: `ls -lh ~/omniphi/pos/posd`
5. If stuck, rebuild and reset:
   ```bash
   cd ~/omniphi/pos
   rm -f posd
   go build -o posd ./cmd/posd
   ./posd comet unsafe-reset-all
   ./posd start
   ```

---

**Last Updated:** 2025-11-20
