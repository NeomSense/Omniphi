#!/bin/bash

echo "Testing Omniphi Validator Orchestrator API"
echo "=========================================="
echo ""

echo "1. Health Check:"
curl -s http://localhost:8000/api/v1/health | jq .
echo ""
echo ""

echo "2. Creating Validator Setup Request:"
RESPONSE=$(curl -s -X POST http://localhost:8000/api/v1/validators/setup-requests \
  -H "Content-Type: application/json" \
  -d '{
    "walletAddress": "omni1test123abc",
    "validatorName": "Test Validator",
    "website": "https://test-validator.com",
    "description": "Testing the validator orchestrator",
    "commissionRate": 0.10,
    "runMode": "cloud",
    "provider": "omniphi_cloud"
  }')

echo "$RESPONSE" | jq .
REQUEST_ID=$(echo "$RESPONSE" | jq -r '.setupRequest.id')
echo ""
echo "Setup Request ID: $REQUEST_ID"
echo ""
echo ""

echo "3. Waiting for provisioning (3 seconds)..."
sleep 3
echo ""

echo "4. Polling Setup Status:"
curl -s "http://localhost:8000/api/v1/validators/setup-requests/$REQUEST_ID" | jq .
echo ""
echo ""

echo "5. Getting validators by wallet:"
curl -s "http://localhost:8000/api/v1/validators/by-wallet/omni1test123abc" | jq .
echo ""
echo ""

echo "=========================================="
echo "API Test Complete!"
echo ""
echo "Visit http://localhost:8000/docs for interactive API documentation"
