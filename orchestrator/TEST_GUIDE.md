# 🧪 Frontend Testing Guide

**Both servers are running successfully!**

---

## ✅ Current Status

**Backend:** ✅ Running at http://localhost:8000
**Frontend:** ✅ Running at http://localhost:8080

---

## 🌐 Access the Application

**Open your browser and navigate to:**

```
http://localhost:8080
```

You should see the Omniphi Validator Hub homepage!

---

## 🧪 Test Scenarios

### 1. Homepage Test

**URL:** http://localhost:8080

**What to check:**
- ✅ Page loads without errors
- ✅ "Connect Wallet" button visible
- ✅ Navigation menu present
- ✅ Responsive design (try resizing window)

**Expected:** Modern landing page with validator setup options

---

### 2. Wallet Connection (Mock Mode)

**Steps:**
1. Click "Connect Wallet" button
2. Should show wallet connection dialog
3. Use mock mode (no real wallet needed for testing)

**What to check:**
- ✅ Wallet connects successfully
- ✅ Wallet address shows in UI
- ✅ "Create Validator" or "Dashboard" options appear

---

### 3. Validator Setup Wizard

**URL:** http://localhost:8080/wizard

**Steps:**
1. Navigate to wizard (or click "Create Validator")
2. Select **Cloud Mode**
3. Fill in validator details:
   - **Moniker:** "Test Validator 1"
   - **Website:** "https://test.com" (optional)
   - **Description:** "My first test validator"
   - **Commission Rate:** 10% (default)
4. Click **Next** through steps
5. Review details on confirmation page
6. Click **Deploy**

**What to check:**
- ✅ Form validation works (try submitting empty form)
- ✅ Navigation between steps works
- ✅ Progress bar updates
- ✅ Deploy button triggers API call

**Expected Result:**
- Frontend calls `POST /api/v1/validators/setup-requests`
- Backend returns setupRequestId
- Wizard advances to "Provisioning" step
- Status polling begins (every 3 seconds)

---

### 4. Provisioning Status Polling

**What happens automatically:**
1. Frontend polls `GET /api/v1/validators/setup-requests/{id}` every 3 seconds
2. Backend updates status: `pending` → `provisioning` → `ready_for_chain_tx`
3. Progress bar shows current stage
4. When complete, consensus pubkey is displayed

**What to check:**
- ✅ Status updates in real-time
- ✅ Progress bar animates
- ✅ Status messages change
- ✅ Eventually shows "Ready for transaction" with pubkey

**Open Browser DevTools (F12) to see:**
- Network tab → Should see repeated API calls every 3 seconds
- Console → Should see no errors

---

### 5. Validator Dashboard

**URL:** http://localhost:8080/dashboard

**Steps:**
1. After completing wizard, navigate to dashboard
2. Or go directly if you have a validator already

**What to check:**
- ✅ Validator status card shows current state
- ✅ Uptime percentage displayed
- ✅ Commission rate shown
- ✅ Action buttons present:
  - Edit (shows "coming soon" toast)
  - Redeploy (enabled for cloud validators)
  - Stop (enabled for cloud validators)
  - Refresh (updates status)

**Test Dashboard Actions:**

**Stop Validator:**
1. Click "Stop" button
2. Should call `POST /api/v1/validators/stop`
3. Shows success toast if setupRequestId exists
4. Status updates to "stopped"

**Redeploy Validator:**
1. Click "Redeploy" button
2. Should call `POST /api/v1/validators/redeploy`
3. Shows success toast
4. Triggers new provisioning
5. Can monitor status with polling

**Refresh Status:**
1. Click refresh button (circular arrow)
2. Fetches latest validator data
3. Updates UI with new information

---

### 6. API Integration Tests

**Open Browser Console (F12) and run:**

```javascript
// Test health endpoint
fetch('http://localhost:8000/api/v1/health')
  .then(r => r.json())
  .then(console.log)

// Test create validator (with mock data)
fetch('http://localhost:8000/api/v1/validators/setup-requests', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    walletAddress: 'omni1test123',
    validatorName: 'Console Test Validator',
    website: 'https://test.com',
    description: 'Testing from console',
    commissionRate: 0.10,
    runMode: 'cloud',
    provider: 'docker'
  })
})
  .then(r => r.json())
  .then(data => {
    console.log('Setup request created:', data);

    // Test get status
    const id = data.setupRequest.id;
    fetch(`http://localhost:8000/api/v1/validators/setup-requests/${id}`)
      .then(r => r.json())
      .then(console.log)
  })
```

**Expected Results:**
- Health check returns: `{ status: "ok", version: "1.0.0" }`
- Create validator returns: `{ setupRequest: { id: "...", status: "pending" } }`
- Get status returns: Full validator details

---

### 7. Test New Enhancement Components

**Validator List View:**
```
http://localhost:8080/validators
```
- Currently not routed (need to add route)
- Shows all validators for connected wallet
- Has search, filter, summary stats

**Delegation Manager:**
```
http://localhost:8080/delegations
```
- Currently not routed (need to add route)
- Delegate/undelegate/redelegate UI
- Mock transaction building

**Rewards Manager:**
```
http://localhost:8080/rewards
```
- Currently not routed (need to add route)
- Shows total rewards
- Claim rewards functionality

**Transaction History:**
```
http://localhost:8080/transactions
```
- Currently not routed (need to add route)
- Transaction table with filters
- Export to CSV

**Analytics Dashboard:**
```
http://localhost:8080/analytics
```
- Currently not routed (need to add route)
- 4 interactive charts
- Performance metrics

**Note:** To enable these routes, see [OPTIONAL_ENHANCEMENTS_COMPLETE.md](OPTIONAL_ENHANCEMENTS_COMPLETE.md)

---

## 🔍 Debugging Tools

### Check Backend Logs

The backend is running in background. To see logs:

```bash
# Check what the backend is doing
curl http://localhost:8000/api/v1/health

# View backend API docs
# Open in browser: http://localhost:8000/docs
```

### Check Frontend in Browser

**Open DevTools (F12):**

1. **Console Tab:**
   - Look for JavaScript errors
   - Should see API calls being made
   - Check for CORS errors (should be none)

2. **Network Tab:**
   - Watch API requests/responses
   - Check status codes (should be 200)
   - Inspect request/response payloads

3. **Application Tab:**
   - Check Local Storage → `validator-storage`
   - Should see persisted state (wallet address, setupRequestId, etc.)

---

## 🐛 Common Issues

### Issue: "Failed to deploy validator"

**Cause:** Backend not responding or CORS issue

**Fix:**
1. Check backend is running: `curl http://localhost:8000/api/v1/health`
2. Check `.env` file has correct URL: `VITE_API_URL=http://localhost:8000`
3. Restart frontend: Kill and run `npm run dev` again

---

### Issue: "No setup request ID found"

**Cause:** Wizard didn't save setupRequestId to store

**Fix:**
1. Complete the wizard flow again
2. Check browser console for errors
3. Check Local Storage for `setupRequestId`

---

### Issue: Components not rendering

**Cause:** Missing routes or import errors

**Fix:**
1. Check browser console for errors
2. Verify component imports are correct
3. Check React error overlay (red screen)

---

### Issue: API calls to wrong URL

**Cause:** Environment variable not loaded

**Fix:**
1. Restart dev server after creating `.env`
2. Check `.env` file exists in frontend root
3. Verify `VITE_API_URL=http://localhost:8000`

---

## ✅ Test Checklist

**Basic Functionality:**
- [ ] Homepage loads
- [ ] Wallet connects (mock mode)
- [ ] Wizard opens
- [ ] Form validation works
- [ ] Navigation between steps
- [ ] Deploy button works
- [ ] API call succeeds

**Provisioning Flow:**
- [ ] Status polling starts
- [ ] Status updates in UI
- [ ] Progress bar animates
- [ ] Consensus pubkey shown when ready
- [ ] Can proceed to sign transaction

**Dashboard:**
- [ ] Dashboard loads
- [ ] Status card shows data
- [ ] Stop button works (cloud mode)
- [ ] Redeploy button works (cloud mode)
- [ ] Refresh button updates data
- [ ] Toast notifications appear

**API Integration:**
- [ ] Backend responds to health check
- [ ] Can create setup request
- [ ] Can poll status
- [ ] Can get validators by wallet
- [ ] Can stop validator
- [ ] Can redeploy validator

**DevTools:**
- [ ] No console errors
- [ ] Network requests succeed (200)
- [ ] CORS headers present
- [ ] Local storage persists data

---

## 🎯 Expected Behavior Summary

**1. Normal Flow:**
```
Homepage → Connect Wallet → Wizard → Deploy → Provisioning → Sign Transaction → Dashboard
```

**2. API Flow:**
```
POST /setup-requests → Returns ID → Poll GET /setup-requests/{id} → Status updates → Ready
```

**3. Dashboard Actions:**
```
Click Stop → POST /stop → Success toast → Status updated
Click Redeploy → POST /redeploy → Success toast → New provisioning started
```

---

## 🚀 Next Steps After Testing

Once basic testing is complete:

1. **Enable new routes** - Add validator list, delegations, rewards, etc.
2. **Test with real wallet** - Integrate Keplr/Leap
3. **Connect to blockchain** - Point to actual RPC endpoints
4. **Test transaction signing** - Real MsgCreateValidator
5. **Deploy to staging** - Test in production-like environment

---

## 📊 Performance Testing

**Load Test the API:**

```bash
# Install Apache Bench (if not installed)
# Then test API performance:

# Test health endpoint (100 requests)
ab -n 100 -c 10 http://localhost:8000/api/v1/health

# Test create validator endpoint (10 requests)
ab -n 10 -c 2 -p validator-request.json -T application/json \
  http://localhost:8000/api/v1/validators/setup-requests
```

**Expected:**
- Health endpoint: >1000 requests/sec
- Create validator: >50 requests/sec
- Average response time: <100ms

---

## 🎉 Success Criteria

**Test is successful if:**
- ✅ Both servers start without errors
- ✅ Frontend loads in browser
- ✅ Wizard completes full flow
- ✅ API calls succeed (200 status)
- ✅ Status polling works
- ✅ Dashboard actions work
- ✅ No console errors
- ✅ Toast notifications appear
- ✅ Data persists in Local Storage

---

**Happy Testing! 🚀**

If you encounter any issues, check the browser console first, then backend logs.
