# Proto Generation Fix Guide (Cross-Platform)

**Version:** 1.0 | **Last Updated:** 2025-11-20

This guide provides instructions for regenerating protobuf files for the x/poc module's access control parameters on **Ubuntu/Linux, macOS, and Windows**.

---

## Problem Statement

The x/poc module's new access control parameters (fields 14-18 in params.proto) are not being serialized to state because the protobuf Marshal/Unmarshal methods were not regenerated.

## Quick Diagnosis

### Ubuntu/Linux/macOS
```bash
go test ./x/poc/keeper -v -run TestParamsSerialization
```

### Windows (PowerShell)
```powershell
go test ./x/poc/keeper -v -run TestParamsSerialization
```

**Expected output (showing the problem):**
```
Before SetParams:
  EnableCscoreGating: true
  MinCscoreForCtype: map[code:1000 governance:10000]

After GetParams:
  EnableCscoreGating: false   ❌ Should be true
  MinCscoreForCtype: map[]    ❌ Should have values
```

---

## Solution: Install Required Tools & Regenerate

### Step 1: Install Protoc Generators

<details>
<summary><b>Ubuntu/Linux/macOS</b></summary>

```bash
# Install protoc-gen-gocosmos
go install github.com/cosmos/gogoproto/protoc-gen-gocosmos@latest

# Install protoc-gen-grpc-gateway
go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@latest

# Verify installation
which protoc-gen-gocosmos
which protoc-gen-grpc-gateway

# Ensure Go bin is in PATH (if needed)
export PATH=$PATH:$(go env GOPATH)/bin
```

To make PATH changes permanent, add to `~/.bashrc` or `~/.zshrc`:
```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

</details>

<details>
<summary><b>Windows (PowerShell)</b></summary>

```powershell
# Install protoc-gen-gocosmos
go install github.com/cosmos/gogoproto/protoc-gen-gocosmos@latest

# Install protoc-gen-grpc-gateway
go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@latest

# Verify installation
$env:GOPATH = go env GOPATH
Test-Path "$env:GOPATH\bin\protoc-gen-gocosmos.exe"
Test-Path "$env:GOPATH\bin\protoc-gen-grpc-gateway.exe"

# Add Go bin to PATH (current session)
$env:PATH += ";$(go env GOPATH)\bin"
```

**To make PATH permanent:**

**Option A: PowerShell Profile**
```powershell
notepad $PROFILE
# Add this line and save:
$env:PATH += ";$((go env GOPATH).Trim())\bin"

# Reload profile
. $PROFILE
```

**Option B: System Environment Variables**
1. Press `Win + X` → System
2. Advanced System Settings → Environment Variables
3. Under "User variables", edit `Path`
4. Add: `C:\Users\YourName\go\bin` (your actual GOPATH\bin)
5. Click OK and restart PowerShell

</details>

### Step 2: Regenerate Proto Files

<details>
<summary><b>Ubuntu/Linux/macOS</b></summary>

```bash
cd proto
buf generate --template buf.gen.gogo.yaml
cd ..
```

</details>

<details>
<summary><b>Windows (PowerShell)</b></summary>

```powershell
cd proto
buf generate --template buf.gen.gogo.yaml
cd ..
```

</details>

### Step 3: Verify Fix

<details>
<summary><b>Ubuntu/Linux/macOS</b></summary>

```bash
# Test serialization
go test ./x/poc/keeper -v -run TestParamsSerialization

# Run full test suite
go test ./x/poc/keeper -v
# All 36 tests should pass
```

</details>

<details>
<summary><b>Windows (PowerShell)</b></summary>

```powershell
# Test serialization
go test ./x/poc/keeper -v -run TestParamsSerialization

# Run full test suite
go test ./x/poc/keeper -v
# All 36 tests should pass
```

</details>

---

## Alternative Methods

### Option 1: Use Docker (Cross-Platform)

<details>
<summary><b>Ubuntu/Linux/macOS</b></summary>

```bash
docker run --rm \
  -v $(pwd):/workspace \
  -w /workspace/proto \
  bufbuild/buf:latest \
  generate --template buf.gen.gogo.yaml
```

</details>

<details>
<summary><b>Windows (PowerShell)</b></summary>

```powershell
# Use backticks for line continuation in PowerShell
docker run --rm `
  -v ${PWD}:/workspace `
  -w /workspace/proto `
  bufbuild/buf:latest `
  generate --template buf.gen.gogo.yaml
```

**Note:** If Docker volume mounting fails, use absolute path:
```powershell
$currentDir = (Get-Location).Path
docker run --rm `
  -v "${currentDir}:/workspace" `
  -w /workspace/proto `
  bufbuild/buf:latest `
  generate --template buf.gen.gogo.yaml
```

</details>

### Option 2: Use Ignite CLI (Cross-Platform)

<details>
<summary><b>Ubuntu/Linux/macOS</b></summary>

```bash
# Create required workspace file
touch buf.work.yaml

# Generate protos
ignite generate proto-go --yes
```

</details>

<details>
<summary><b>Windows (PowerShell)</b></summary>

```powershell
# Create required workspace file
New-Item -ItemType File -Path "buf.work.yaml" -Force

# Generate protos
ignite generate proto-go --yes
```

**Note:** You may need to press a key to accept the metrics prompt on Windows.

</details>

### Option 3: WSL2 on Windows

If you have WSL2 installed on Windows, you can use Linux commands:

```powershell
# Enter WSL
wsl

# Navigate to your project
cd /mnt/c/Users/YourName/omniphi/pos

# Run Linux commands
cd proto
buf generate --template buf.gen.gogo.yaml
cd ..
```

---

## What Gets Changed

Only **one file** will be modified:
- `x/poc/types/params.pb.go` (or `x\poc\types\params.pb.go` on Windows)

The following methods will be updated to handle fields 14-18:
- `func (m *Params) Marshal() ([]byte, error)`
- `func (m *Params) MarshalToSizedBuffer(dAtA []byte) (int, error)`
- `func (m *Params) Unmarshal(dAtA []byte) error`
- `func (m *Params) Size() int`
- `func (m *Params) Equal(that interface{}) bool`

**Expected diff:** ~200 lines added to params.pb.go

---

## Verification Checklist

After regenerating protos, verify:

- [ ] `go build ./x/poc/...` succeeds
- [ ] `go test ./x/poc/keeper -run TestParamsSerialization` passes
- [ ] `go test ./x/poc/keeper -v` shows **36/36 tests passing**
- [ ] No changes to any files except `x/poc/types/params.pb.go`

## Expected Test Results

### Before Proto Regeneration
```
PASS: 18/18 existing tests ✅
PASS: 9/18 new tests ✅
FAIL: 9/18 new tests ❌ (serialization-dependent)
```

### After Proto Regeneration
```
PASS: 18/18 existing tests ✅
PASS: 18/18 new tests ✅
ALL TESTS: 36/36 ✅
```

---

## Troubleshooting

### Issue: "go: no such tool"

**Cause:** Protoc plugins not installed or not in PATH

<details>
<summary><b>Ubuntu/Linux/macOS Solution</b></summary>

```bash
# Ensure PATH includes Go bin
export PATH=$PATH:$(go env GOPATH)/bin

# Reinstall tools
go install github.com/cosmos/gogoproto/protoc-gen-gocosmos@latest
go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@latest

# Verify
which protoc-gen-gocosmos
which protoc-gen-grpc-gateway
```

</details>

<details>
<summary><b>Windows Solution</b></summary>

```powershell
# Check Go bin directory
$goBin = "$(go env GOPATH)\bin"
Write-Host "Go bin directory: $goBin"
Get-ChildItem $goBin | Where-Object { $_.Name -like "*protoc*" }

# Add to PATH
$env:PATH += ";$goBin"

# Reinstall tools
go install github.com/cosmos/gogoproto/protoc-gen-gocosmos@latest
go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@latest

# Verify
Get-Command protoc-gen-gocosmos.exe
Get-Command protoc-gen-grpc-gateway.exe
```

</details>

### Issue: "buf.work.yaml: file does not exist"

**Cause:** Ignite CLI expects workspace file

<details>
<summary><b>Ubuntu/Linux/macOS Solution</b></summary>

```bash
touch buf.work.yaml
# Then retry ignite command
```

</details>

<details>
<summary><b>Windows Solution</b></summary>

```powershell
New-Item -ItemType File -Path "buf.work.yaml" -Force
# Then retry ignite command
```

</details>

### Issue: Fields still not serializing after regeneration

**Cause:** Old generated files cached

<details>
<summary><b>Cross-Platform Solution</b></summary>

```bash
# Clean Go build cache (same on all platforms)
go clean -cache -modcache -testcache

# Rebuild
go build ./x/poc/...

# Retest
go test ./x/poc/keeper -v
```

</details>

---

## Quick Fix Scripts

### Ubuntu/Linux/macOS Script

Save as `fix-proto.sh`:

```bash
#!/bin/bash
set -e

echo "Installing protoc generators..."
go install github.com/cosmos/gogoproto/protoc-gen-gocosmos@latest
go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@latest

echo "Adding Go bin to PATH..."
export PATH=$PATH:$(go env GOPATH)/bin

echo "Verifying installation..."
which protoc-gen-gocosmos || exit 1
which protoc-gen-grpc-gateway || exit 1

echo "Regenerating proto files..."
cd proto
buf generate --template buf.gen.gogo.yaml
cd ..

echo "Building module..."
go build ./x/poc/...

echo "Running tests..."
go test ./x/poc/keeper -v -run TestParamsSerialization

echo "Done! Check test output above."
```

Run: `chmod +x fix-proto.sh && ./fix-proto.sh`

### Windows PowerShell Script

Save as `fix-proto.ps1`:

```powershell
# fix-proto.ps1
Write-Host "Installing protoc generators..." -ForegroundColor Cyan
go install github.com/cosmos/gogoproto/protoc-gen-gocosmos@latest
go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@latest

Write-Host "`nAdding Go bin to PATH..." -ForegroundColor Cyan
$env:PATH += ";$(go env GOPATH)\bin"

Write-Host "`nVerifying installation..." -ForegroundColor Cyan
if (Get-Command protoc-gen-gocosmos.exe -ErrorAction SilentlyContinue) {
    Write-Host "✓ protoc-gen-gocosmos installed" -ForegroundColor Green
} else {
    Write-Host "✗ protoc-gen-gocosmos NOT found" -ForegroundColor Red
    exit 1
}

if (Get-Command protoc-gen-grpc-gateway.exe -ErrorAction SilentlyContinue) {
    Write-Host "✓ protoc-gen-grpc-gateway installed" -ForegroundColor Green
} else {
    Write-Host "✗ protoc-gen-grpc-gateway NOT found" -ForegroundColor Red
    exit 1
}

Write-Host "`nRegenerating proto files..." -ForegroundColor Cyan
Set-Location proto
buf generate --template buf.gen.gogo.yaml

Write-Host "`nBuilding module..." -ForegroundColor Cyan
Set-Location ..
go build ./x/poc/...

Write-Host "`nRunning tests..." -ForegroundColor Cyan
go test ./x/poc/keeper -v -run TestParamsSerialization

Write-Host "`nDone! Check test output above." -ForegroundColor Cyan
```

Run: `.\fix-proto.ps1`

---

## Platform Differences Reference

| Aspect | Linux/macOS | Windows PowerShell |
|--------|-------------|-------------------|
| PATH separator | `:` (colon) | `;` (semicolon) |
| Line continuation | `\` (backslash) | `` ` `` (backtick) |
| Executable extension | none | `.exe` |
| Path separator | `/` (forward slash) | `\` (backslash), but `/` also works |
| Environment variable | `$VAR` | `$env:VAR` |
| Check command exists | `which command` | `Get-Command command` |
| Create file | `touch file` | `New-Item -ItemType File` |

---

## Summary

**Quickest Path to Fix:**
1. Install protoc-gen-gocosmos: `go install github.com/cosmos/gogoproto/protoc-gen-gocosmos@latest`
2. Install protoc-gen-grpc-gateway: `go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@latest`
3. Add Go bin to PATH
4. Regenerate: `cd proto && buf generate --template buf.gen.gogo.yaml`

**Verification:** Run `go test ./x/poc/keeper -v` and ensure all 36 tests pass

**Impact:** Without this fix, the access control feature will build but not function at runtime (parameters won't persist to state)

---

*For detailed implementation information, see: [POA_ACCESS_CONTROL_IMPLEMENTATION.md](POA_ACCESS_CONTROL_IMPLEMENTATION.md)*
