# Package Validator 2 for Transfer to Computer 2 - Windows PowerShell

$OUTPUT_DIR = ".\testnet-2nodes"
$PACKAGE_NAME = "validator2-package.zip"

if (-not (Test-Path "$OUTPUT_DIR\validator2")) {
    Write-Host "Error: Validator 2 directory not found!" -ForegroundColor Red
    Write-Host "Please run setup_2node_testnet.ps1 first!"
    exit 1
}

Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  Packaging Validator 2" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""

# Remove old package if exists
if (Test-Path $PACKAGE_NAME) {
    Remove-Item $PACKAGE_NAME -Force
}

# Create temporary directory for package
$TEMP_PKG = ".\validator2-package-temp"
if (Test-Path $TEMP_PKG) {
    Remove-Item -Recurse -Force $TEMP_PKG
}
New-Item -ItemType Directory -Path $TEMP_PKG | Out-Null

# Copy validator2 directory
Write-Host "Copying validator2 configuration..." -ForegroundColor Yellow
Copy-Item -Recurse "$OUTPUT_DIR\validator2" "$TEMP_PKG\"

# Copy binary if it exists
if (Test-Path ".\posd.exe") {
    Write-Host "Adding posd.exe binary to package..." -ForegroundColor Yellow
    Copy-Item ".\posd.exe" "$TEMP_PKG\"
}

# Copy start script
if (Test-Path ".\start_validator2.ps1") {
    Write-Host "Adding start script to package..." -ForegroundColor Yellow
    Copy-Item ".\start_validator2.ps1" "$TEMP_PKG\"
}

# Create README for Computer 2
$readme = @"
VALIDATOR 2 PACKAGE
===================

This package contains everything needed to run Validator 2 on Computer 2.

CONTENTS:
- validator2/          : Validator configuration and keys
- posd.exe            : Binary (if included)
- start_validator2.ps1: Start script

SETUP INSTRUCTIONS:
1. Extract this package to a directory on Computer 2
2. If posd.exe is not included, build it:
   - Install Go (https://golang.org/dl/)
   - Clone the repository
   - Run: go build -o posd.exe ./cmd/posd
   - Copy posd.exe to this directory

3. Start the validator:
   .\start_validator2.ps1

4. Verify it's running:
   .\posd.exe status --home .\validator2

PORTS USED:
- P2P: 26656 (must be accessible from Computer 1)
- RPC: 26657
- gRPC: 9090
- API: 1317

FIREWALL:
Make sure port 26656 is open for incoming connections!

Windows Firewall:
New-NetFirewallRule -DisplayName "Omniphi P2P" -Direction Inbound -LocalPort 26656 -Protocol TCP -Action Allow

TROUBLESHOOTING:
- Check if validator is syncing: posd.exe status --home .\validator2
- Check logs in: validator2\posd.log
- Verify peer connection in: validator2\config\config.toml

For more help, see MULTI_NODE_TESTNET_GUIDE.md
"@

$readme | Set-Content "$TEMP_PKG\README.txt"

# Create ZIP package
Write-Host "Creating ZIP package..." -ForegroundColor Yellow
Compress-Archive -Path "$TEMP_PKG\*" -DestinationPath $PACKAGE_NAME -Force

# Clean up temp directory
Remove-Item -Recurse -Force $TEMP_PKG

$packageInfo = Get-Item $PACKAGE_NAME
$PACKAGE_SIZE = "{0:N2} MB" -f ($packageInfo.Length / 1MB)

Write-Host ""
Write-Host "Package created: $PACKAGE_NAME ($PACKAGE_SIZE)" -ForegroundColor Green
Write-Host ""
Write-Host "======================================" -ForegroundColor Cyan
Write-Host "  Transfer Instructions" -ForegroundColor Cyan
Write-Host "======================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "1. Transfer $PACKAGE_NAME to Computer 2" -ForegroundColor Yellow
Write-Host ""
Write-Host "2. On Computer 2, extract the package" -ForegroundColor Yellow
Write-Host ""
Write-Host "3. If posd.exe not included, build it on Computer 2" -ForegroundColor Yellow
Write-Host ""
Write-Host "4. Start validator 2" -ForegroundColor Yellow
Write-Host ""
Write-Host "5. See README.txt in package for detailed instructions" -ForegroundColor Yellow
Write-Host ""
