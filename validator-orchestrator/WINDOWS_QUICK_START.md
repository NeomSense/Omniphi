# Windows Quick Start Guide

**Special guide for Windows users with PowerShell-specific commands**

---

## ⚡ Quick Start Commands (Windows)

### Backend API

```powershell
# Navigate to backend
cd C:\Users\herna\omniphi\pos\validator-orchestrator\backend

# Activate virtual environment (PowerShell)
.\venv\Scripts\Activate.ps1

# Start server
python -m uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

**Or if using Command Prompt (cmd):**
```cmd
cd C:\Users\herna\omniphi\pos\validator-orchestrator\backend
venv\Scripts\activate.bat
python -m uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

### Desktop App

```powershell
# Navigate to desktop app
cd C:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app

# Start app
npm run dev
```

---

## 🔧 First Time Setup (Windows)

### Backend Setup

```powershell
cd C:\Users\herna\omniphi\pos\validator-orchestrator\backend

# Create virtual environment
python -m venv venv

# Activate (PowerShell)
.\venv\Scripts\Activate.ps1

# Install dependencies
pip install -r requirements.txt

# Copy environment file
Copy-Item .env.example .env

# Initialize database
alembic upgrade head
```

### Desktop App Setup

```powershell
cd C:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app

# Install dependencies
npm install

# Copy posd binary (adjust source path as needed)
Copy-Item C:\Users\herna\omniphi\pos\posd.exe bin\

# Verify binary
ls bin\posd.exe
```

---

## 🌐 Access URLs

Once running:
- **Backend API Docs:** http://localhost:8000/docs
- **Backend Health:** http://localhost:8000/api/v1/health
- **Desktop App:** Electron window opens automatically
- **HTTP Bridge:** http://localhost:15000/health

---

## 💡 Windows-Specific Tips

### PowerShell vs Command Prompt

**PowerShell (Recommended):**
```powershell
.\venv\Scripts\Activate.ps1
```

**Command Prompt:**
```cmd
venv\Scripts\activate.bat
```

### If You Get "Execution Policy" Error

If PowerShell blocks the activation script:

```powershell
# Check current policy
Get-ExecutionPolicy

# Set policy to allow (run as Administrator)
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser

# Then try activating again
.\venv\Scripts\Activate.ps1
```

### Path Separators

Windows uses backslashes `\` in paths:
- ✅ `C:\Users\herna\omniphi\pos`
- ❌ `C:/Users/herna/omniphi/pos` (works in some contexts but not all)

### Testing with PowerShell

```powershell
# Test backend health
Invoke-WebRequest -Uri http://localhost:8000/api/v1/health

# Or use curl if installed
curl http://localhost:8000/api/v1/health
```

---

## ⚠️ Important: Desktop App vs Manual Blockchain

### Can the Desktop App Run Alongside a Manual Blockchain?

**NO** - The desktop app will conflict with a manually running blockchain because they both:
- Use the same home directory: `C:\Users\herna\.omniphi\`
- Use the same ports: 26657 (RPC), 26656 (P2P), 9090 (gRPC)
- Use the same consensus key
- Access the same blockchain database

### What to Do

**Option 1: Use Desktop App (Recommended)**
```powershell
# Stop manual blockchain
taskkill /IM posd.exe /F

# Start desktop app - it will use the same validator
cd C:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app
npm run dev
```

**Option 2: Use Manual Blockchain Only**
```powershell
# Don't click "Start Validator" in desktop app
# Keep your manual blockchain running
cd C:\Users\herna\omniphi\pos
.\posd start
```

**You cannot run both at the same time** without:
- Configuring different home directories
- Changing port configurations
- Creating a separate validator on-chain

---

## 🐛 Windows-Specific Troubleshooting

### Issue: "Cannot be loaded because running scripts is disabled"

**Solution:**
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### Issue: "python: command not found"

**Solution:**
```powershell
# Use py launcher instead
py -m venv venv
py -m pip install -r requirements.txt
py -m uvicorn app.main:app --reload
```

### Issue: Port already in use (Windows)

**Find what's using the port:**
```powershell
netstat -ano | findstr :8000
```

**Kill the process:**
```powershell
# Replace <PID> with the Process ID from netstat
taskkill /PID <PID> /F
```

### Issue: Can't copy posd.exe

**Solution:**
```powershell
# Check source exists
Test-Path C:\Users\herna\omniphi\pos\posd.exe

# Copy with full paths
Copy-Item -Path C:\Users\herna\omniphi\pos\posd.exe -Destination C:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app\bin\

# Verify
ls C:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app\bin\posd.exe
```

---

## 📂 Windows File Locations

### Backend Files
```
C:\Users\herna\omniphi\pos\validator-orchestrator\backend\
├── venv\                          Virtual environment
├── .env                           Configuration (EDIT THIS)
├── requirements.txt               Dependencies
└── app\main.py                    Entry point
```

### Desktop App Files
```
C:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app\
├── bin\posd.exe                   Validator binary (MUST BE HERE)
├── electron\                      Electron main process
├── src\                           React app
└── package.json                   Dependencies
```

### Validator Data
```
C:\Users\herna\.omniphi\
├── config\
│   ├── config.toml
│   ├── app.toml
│   ├── genesis.json
│   └── priv_validator_key.json    ⚠️ BACKUP THIS FILE!
└── data\
```

---

## ✅ Verification Checklist

After starting services, verify:

1. **Backend Running:**
   ```powershell
   curl http://localhost:8000/api/v1/health
   # Should return: {"status":"ok",...}
   ```

2. **Backend API Docs Accessible:**
   - Open browser: http://localhost:8000/docs

3. **Desktop App Launched:**
   - Electron window should open

4. **HTTP Bridge Running:**
   ```powershell
   curl http://localhost:15000/health
   # Should return: {"success":true,...}
   ```

---

## 🚀 Daily Workflow (Windows)

### Open 2 PowerShell Windows

**Terminal 1 - Backend:**
```powershell
cd C:\Users\herna\omniphi\pos\validator-orchestrator\backend
.\venv\Scripts\Activate.ps1
python -m uvicorn app.main:app --reload
```

**Terminal 2 - Desktop App:**
```powershell
cd C:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app
npm run dev
```

---

## 📚 Full Documentation

For complete details, see:
- [BACKEND_LOCAL_VALIDATOR_GUIDE.md](BACKEND_LOCAL_VALIDATOR_GUIDE.md) - Complete guide
- [START_HERE.md](START_HERE.md) - Overview
- [QUICK_START.md](QUICK_START.md) - Quick reference (Linux/macOS commands)

---

**Windows PowerShell Commands Summary:**

```powershell
# Backend
cd C:\Users\herna\omniphi\pos\validator-orchestrator\backend
.\venv\Scripts\Activate.ps1
python -m uvicorn app.main:app --reload

# Desktop App
cd C:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app
npm run dev

# Test
curl http://localhost:8000/api/v1/health
curl http://localhost:15000/health
```
   
---

**Last Updated:** 2025-11-20 | **Platform:** Windows 10/11
