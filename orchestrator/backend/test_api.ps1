# Omniphi Validator Orchestrator API Test Script (PowerShell)

Write-Host "=================================================="
Write-Host "Testing Omniphi Validator Orchestrator API"
Write-Host "=================================================="
Write-Host ""

# Test 1: Health Check
Write-Host "1. Health Check:" -ForegroundColor Cyan
try {
    $health = Invoke-RestMethod -Uri "http://localhost:8000/api/v1/health"
    Write-Host "   Status: " -NoNewline
    Write-Host $health.status -ForegroundColor Green
    Write-Host "   Version: $($health.version)"
    Write-Host "   Database: $($health.database)"
} catch {
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test 2: Create Validator Setup Request
Write-Host "2. Creating Validator Setup Request:" -ForegroundColor Cyan
$body = @{
    walletAddress = "omni1test$(Get-Random -Maximum 9999)"
    validatorName = "PowerShell Test Validator"
    website = "https://test.omniphi.com"
    description = "Testing from PowerShell"
    commissionRate = 0.10
    runMode = "cloud"
    provider = "omniphi_cloud"
} | ConvertTo-Json

try {
    $createResponse = Invoke-RestMethod -Uri "http://localhost:8000/api/v1/validators/setup-requests" `
        -Method POST `
        -ContentType "application/json" `
        -Body $body

    $requestId = $createResponse.setupRequest.id
    Write-Host "   Setup Request ID: " -NoNewline
    Write-Host $requestId -ForegroundColor Yellow
    Write-Host "   Status: $($createResponse.setupRequest.status)"
    Write-Host "   Wallet: $($createResponse.setupRequest.walletAddress)"
    Write-Host "   Validator Name: $($createResponse.setupRequest.validatorName)"
} catch {
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
    exit
}
Write-Host ""

# Test 3: Wait for provisioning
Write-Host "3. Waiting for provisioning (3 seconds)..." -ForegroundColor Cyan
Start-Sleep -Seconds 3
Write-Host ""

# Test 4: Check Setup Request Status
Write-Host "4. Checking Setup Request Status:" -ForegroundColor Cyan
try {
    $statusResponse = Invoke-RestMethod -Uri "http://localhost:8000/api/v1/validators/setup-requests/$requestId"

    Write-Host "   Status: " -NoNewline
    if ($statusResponse.setupRequest.status -eq "ready_for_chain_tx") {
        Write-Host $statusResponse.setupRequest.status -ForegroundColor Green
    } else {
        Write-Host $statusResponse.setupRequest.status -ForegroundColor Yellow
    }

    Write-Host "   Consensus Pubkey: $($statusResponse.setupRequest.consensusPubkey.Substring(0, 20))..."

    if ($statusResponse.node) {
        Write-Host "   Node Status: $($statusResponse.node.status)"
        Write-Host "   RPC Endpoint: $($statusResponse.node.rpcEndpoint)"
        Write-Host "   P2P Endpoint: $($statusResponse.node.p2pEndpoint)"
    }
} catch {
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test 5: Get Validators by Wallet
Write-Host "5. Getting Validators by Wallet:" -ForegroundColor Cyan
try {
    $walletAddress = $createResponse.setupRequest.walletAddress
    $validators = Invoke-RestMethod -Uri "http://localhost:8000/api/v1/validators/by-wallet/$walletAddress"

    Write-Host "   Found $($validators.Count) validator(s)"
    foreach ($val in $validators) {
        Write-Host "   - $($val.setupRequest.validatorName) (Status: $($val.setupRequest.status))"
    }
} catch {
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

Write-Host "=================================================="
Write-Host "API Test Complete!" -ForegroundColor Green
Write-Host "=================================================="
Write-Host ""
Write-Host "Visit http://localhost:8000/docs for interactive API documentation" -ForegroundColor Cyan
