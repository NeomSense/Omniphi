# 🎯 START HERE - Omniphi Validator Orchestrator

**Welcome! This is your entry point for running the validator orchestrator system.**

---

## 🪟 Windows Users - Read This First!

**If you're on Windows, use the Windows-specific guide:**
👉 **[WINDOWS_QUICK_START.md](WINDOWS_QUICK_START.md)**

This guide has:
- PowerShell-specific commands (not `source`, use `.\venv\Scripts\Activate.ps1`)
- Windows path formats (`C:\Users\...`)
- Windows troubleshooting

---

## 📚 Documentation Structure

We have **4 documentation files** - choose based on your needs:

### 0. 🪟 [WINDOWS_QUICK_START.md](WINDOWS_QUICK_START.md) - Windows Users
**Use this if:** You're on Windows (PowerShell/cmd commands)

### 1. 🚀 [QUICK_START.md](QUICK_START.md) - 2 Minutes
**Use this if:** You've already set everything up and just need the start commands.

Quick reference card with:
- Start commands for backend and desktop app
- URLs for accessing services
- Common troubleshooting

### 2. 📖 [BACKEND_LOCAL_VALIDATOR_GUIDE.md](BACKEND_LOCAL_VALIDATOR_GUIDE.md) - 15 Minutes
**Use this if:** First time setup or need detailed instructions.

Complete guide with:
- Step-by-step backend API setup
- Step-by-step desktop app setup
- Full API reference
- File locations
- Comprehensive troubleshooting

### 3. 📋 [README.md](README.md) - 5 Minutes
**Use this if:** You want system overview and architecture details.

High-level overview with:
- System architecture
- Project structure
- Database schema
- Development roadmap

---

## 🎬 First Time Setup (15 minutes)

### Step 1: Backend API

```bash
cd ~/omniphi/pos/validator-orchestrator/backend

# Create virtual environment
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt

# Setup environment
cp .env.example .env

# Initialize database
alembic upgrade head

# Start server
uvicorn app.main:app --reload
```

**Verify:** http://localhost:8000/api/v1/health

### Step 2: Desktop App

```bash
cd ~/omniphi/pos/validator-orchestrator/local-validator-app

# Install dependencies
npm install

# Copy posd binary
cp ~/omniphi/pos/posd bin/
chmod +x bin/posd

# Start app
npm run dev
```

**Verify:** Electron window opens + http://localhost:15000/health

---

## ⚡ Daily Usage (After Setup)

**Terminal 1 - Backend:**
```bash
cd ~/omniphi/pos/validator-orchestrator/backend
source venv/bin/activate
uvicorn app.main:app --reload
```

**Terminal 2 - Desktop App:**
```bash
cd ~/omniphi/pos/validator-orchestrator/local-validator-app
npm run dev
```

---

## 🧭 What Each Component Does

### Backend API (FastAPI)
- **Location:** `validator-orchestrator/backend/`
- **Port:** 8000
- **Purpose:** REST API for validator management
- **Use it to:** Create validator requests, track heartbeats, query status

### Desktop App (Electron)
- **Location:** `validator-orchestrator/local-validator-app/`
- **Port:** 15000 (HTTP bridge)
- **Purpose:** Run validators locally on your computer
- **Use it to:** Start/stop validators, view status, manage keys

### The Flow
1. User opens desktop app
2. Desktop app runs `posd` (validator node) locally
3. Desktop app sends heartbeats to backend API
4. Backend API tracks all active validators
5. User can query backend API to see all validators

---

## 🔧 Prerequisites

Make sure you have:
- ✅ **Python 3.11+** (for backend)
- ✅ **Node.js 18+** (for desktop app)
- ✅ **posd binary** (built blockchain binary)
- ✅ **SQLite** (or PostgreSQL for production)

---

## 🆘 Quick Help

### Backend won't start?
```bash
cd ~/omniphi/pos/validator-orchestrator/backend
source venv/bin/activate
pip install -r requirements.txt
```

### Desktop app won't start?
```bash
cd ~/omniphi/pos/validator-orchestrator/local-validator-app
npm install
```

### Desktop app can't find posd?
```bash
# Build fresh posd
cd ~/omniphi/pos
go build -o posd ./cmd/posd

# Copy to desktop app
cp posd ~/omniphi/pos/validator-orchestrator/local-validator-app/bin/
chmod +x ~/omniphi/pos/validator-orchestrator/local-validator-app/bin/posd
```

---

## 📖 Next Steps

1. **First time?** → Read [BACKEND_LOCAL_VALIDATOR_GUIDE.md](BACKEND_LOCAL_VALIDATOR_GUIDE.md)
2. **Already setup?** → Use [QUICK_START.md](QUICK_START.md)
3. **Want architecture details?** → Read [README.md](README.md)
4. **Need blockchain docs?** → See `~/omniphi/pos/BLOCKCHAIN_QUICKSTART_UBUNTU.md`

---

## 🌐 Access URLs (When Running)

| Service | URL |
|---------|-----|
| Backend API Swagger | http://localhost:8000/docs |
| Backend Health | http://localhost:8000/api/v1/health |
| Desktop App | Electron window |
| HTTP Bridge API | http://localhost:15000/health |

---

**Happy Validating! 🚀**

For issues, check the troubleshooting section in [BACKEND_LOCAL_VALIDATOR_GUIDE.md](BACKEND_LOCAL_VALIDATOR_GUIDE.md)
