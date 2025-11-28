# 🚀 Start Testing - Quick Guide

**Both servers are running! Here's how to test everything.**

---

## ✅ System Status

**Backend:** ✅ Running at http://localhost:8000
**Frontend:** ✅ Running at http://localhost:8080

---

## 🌐 Open in Your Browser

### 1. Main Application (START HERE!)

```
http://localhost:8080
```

**What you'll see:**
- Modern landing page with "Omniphi Validators" branding
- "Connect Wallet" button
- Navigation menu
- Beautiful glass-card design

### 2. API Documentation (For Developers)

```
http://localhost:8000/docs
```

**What you can do:**
- Test all API endpoints interactively
- See request/response formats
- No coding needed - just click "Try it out"

---

## 🧪 Test Scenarios

### Scenario 1: Visual Tour (2 minutes)

1. **Open:** http://localhost:8080
2. **Explore:**
   - Click on navigation items
   - Try the "Connect Wallet" button
   - Resize browser window (test responsive design)
   - Look for any visual bugs

### Scenario 2: Create a Validator (5 minutes)

1. **Open:** http://localhost:8080
2. **Connect wallet** (use mock mode)
3. **Click "Create Validator"** or navigate to `/wizard`
4. **Fill in details:**
   - Moniker: "My Test Validator"
   - Commission: 10%
   - Description: "Testing the system"
5. **Click through the wizard steps**
6. **Deploy!**
7. **Watch the provisioning status update**

### Scenario 3: Dashboard Tour (3 minutes)

1. **Open:** http://localhost:8080/dashboard
2. **Check the status cards:**
   - Validator status
   - Uptime
   - Commission
   - Rewards
3. **Try the action buttons:**
   - Refresh (circular arrow)
   - Stop (if cloud mode)
   - Redeploy (if cloud mode)

### Scenario 4: API Testing (5 minutes)

1. **Open:** http://localhost:8000/docs
2. **Find "Health Check" endpoint**
3. **Click "Try it out" → Execute**
4. **See the response:** `{"status": "ok"}`
5. **Try other endpoints:**
   - POST /validators/setup-requests
   - GET /validators/by-wallet/{address}

---

## 🎯 What to Look For

### ✅ Good Signs
- Pages load quickly
- No JavaScript errors in console (F12)
- Buttons are clickable and responsive
- Forms validate properly
- Toast notifications appear for actions
- Status updates happen automatically
- API calls succeed (check Network tab)

### ⚠️ Issues to Report
- Console errors (press F12 to check)
- Buttons that don't work
- Forms that don't validate
- Missing text or broken layouts
- Slow page loads
- Failed API calls

---

## 🔍 Developer Tools

**Press F12 in your browser to open DevTools:**

### Console Tab
- See JavaScript logs
- Check for errors (red text)
- View API call logs

### Network Tab
- Monitor API requests
- Check response times
- Verify status codes (200 = success)
- Inspect request/response data

### Application Tab
- Check Local Storage → "validator-storage"
- See persisted state data
- Clear storage to reset

---

## 🎨 UI Features to Test

### Navigation
- [ ] Click between pages
- [ ] Back button works
- [ ] URLs update correctly

### Wizard Flow
- [ ] Step navigation works
- [ ] Form validation triggers
- [ ] Progress bar updates
- [ ] Deploy button enabled when valid

### Dashboard
- [ ] Cards display data
- [ ] Status updates
- [ ] Action buttons work
- [ ] Toasts appear

### Responsive Design
- [ ] Works on desktop
- [ ] Works on tablet (resize window)
- [ ] Works on mobile (narrow window)

---

## 📱 Quick Test on Mobile

**From another device on same network:**

1. Find your computer's IP address:
   ```bash
   # Windows
   ipconfig
   # Look for IPv4 Address like 192.168.x.x
   ```

2. Open on mobile:
   ```
   http://YOUR_IP:8080
   ```

3. Test the responsive design!

---

## 🐛 Common Issues & Fixes

### Issue: Page won't load

**Fix:**
1. Check servers are running
2. Verify URL: http://localhost:8080 (not 5173)
3. Try hard refresh: Ctrl+F5

### Issue: "Failed to fetch" errors

**Fix:**
1. Check backend: http://localhost:8000/api/v1/health
2. Check .env file has: `VITE_API_URL=http://localhost:8000`
3. Restart frontend dev server

### Issue: Buttons don't work

**Fix:**
1. Press F12 → Console tab
2. Look for JavaScript errors
3. Check Network tab for failed API calls
4. Report what you see

### Issue: Validator won't deploy

**Fix:**
1. Check backend logs
2. May need database migrations:
   ```bash
   cd validator-orchestrator/backend
   alembic upgrade head
   ```

---

## 📝 Testing Checklist

**Basic Tests:**
- [ ] Homepage loads
- [ ] No console errors
- [ ] Can navigate between pages
- [ ] Buttons are clickable
- [ ] Forms have validation

**Wizard Tests:**
- [ ] Can open wizard
- [ ] Can fill in form
- [ ] Validation works
- [ ] Can navigate steps
- [ ] Deploy button triggers API

**Dashboard Tests:**
- [ ] Dashboard loads
- [ ] Status card shows data
- [ ] Action buttons present
- [ ] Refresh works
- [ ] Toasts appear

**API Tests:**
- [ ] Health endpoint works
- [ ] Can create validator
- [ ] Can check status
- [ ] CORS is configured

---

## 🎉 Success Criteria

**Test is successful if:**
1. ✅ All pages load without errors
2. ✅ Wizard completes full flow
3. ✅ API calls succeed
4. ✅ Toast notifications work
5. ✅ Dashboard shows status
6. ✅ No console errors

---

## 📚 More Information

**Detailed Testing:**
- See [TEST_GUIDE.md](TEST_GUIDE.md) for comprehensive test scenarios

**API Integration:**
- See [FRONTEND_BACKEND_INTEGRATION.md](FRONTEND_BACKEND_INTEGRATION.md)

**New Features:**
- See [OPTIONAL_ENHANCEMENTS_COMPLETE.md](OPTIONAL_ENHANCEMENTS_COMPLETE.md)

---

## 🚀 Ready to Start?

1. **Open your browser**
2. **Go to:** http://localhost:8080
3. **Start clicking around!**
4. **Have fun testing!** 🎉

**Servers will keep running in the background.**

**To stop servers later:**
- Backend and frontend will stop when you close the terminal
- Or use Ctrl+C if running in foreground

---

**Happy Testing! 🚀**
