# Omniphi Local Validator Backend - Complete Setup Guide

**Complete documentation for running the Omniphi validator orchestrator backend API and local validator desktop app.**

**Last Updated:** 2025-11-20 | **Version:** 1.0

---

## 📋 Table of Contents

1. [System Overview](#-system-overview)
2. [Backend API Setup](#-backend-api-setup)
3. [Local Validator Desktop App](#-local-validator-desktop-app)
4. [Running Everything Together](#-running-everything-together)
5. [API Reference](#-api-reference)
6. [Troubleshooting](#-troubleshooting)

---

## 🎯 System Overview

The Omniphi validator orchestrator consists of two main components:

### 1. Backend API (FastAPI)
- **Location:** `validator-orchestrator/backend/`
- **Purpose:** REST API for validator management, provisioning, and heartbeat tracking
- **Port:** 8000 (default)
- **Tech Stack:** Python 3.11+, FastAPI, SQLAlchemy, PostgreSQL/SQLite

### 2. Local Validator Desktop App (Electron)
- **Location:** `validator-orchestrator/local-validator-app/`
- **Purpose:** Desktop application for running validators locally
- **Port:** 15000 (HTTP bridge)
- **Tech Stack:** Electron, React, TypeScript, Vite

---

## 🚀 Backend API Setup

### Location
```bash
cd ~/omniphi/pos/validator-orchestrator/backend
```

On Windows:
```bash
cd c:\Users\herna\omniphi\pos\validator-orchestrator\backend
```

### Prerequisites

- **Python 3.11+**
- **PostgreSQL 15+** (or SQLite for development)
- **Docker** (optional, for cloud validator provisioning)

### Step 1: Create Virtual Environment

```bash
# Navigate to backend directory
cd ~/omniphi/pos/validator-orchestrator/backend

# Create virtual environment
python -m venv venv

# Activate virtual environment
# Linux/macOS:
source venv/bin/activate

# Windows:
venv\Scripts\activate
```

### Step 2: Install Dependencies

```bash
# Install all required packages
pip install -r requirements.txt
```

**Requirements (requirements.txt):**
- fastapi
- uvicorn[standard]
- sqlalchemy
- alembic
- psycopg2-binary (PostgreSQL)
- python-multipart
- python-jose[cryptography]
- passlib[bcrypt]
- httpx
- docker
- slowapi (rate limiting)
- python-dotenv

### Step 3: Configure Environment

```bash
# Copy example environment file
cp .env.example .env

# Edit .env with your configuration
nano .env
```

**Key Environment Variables:**

```bash
# Application
PROJECT_NAME="Omniphi Validator Orchestrator"
VERSION="1.0.0"
API_V1_STR="/api/v1"
DEBUG=true

# Security
SECRET_KEY="your-secret-key-here-change-in-production"
ACCESS_TOKEN_EXPIRE_MINUTES=30

# Database
DATABASE_URL="sqlite:///./validator_orchestrator.db"
# Or for PostgreSQL:
# DATABASE_URL="postgresql://user:password@localhost:5432/validator_orchestrator"

# Omniphi Chain
OMNIPHI_CHAIN_ID="omniphi-1"
OMNIPHI_RPC_URL="http://localhost:26657"
OMNIPHI_GRPC_URL="localhost:9090"

# CORS (adjust for production)
BACKEND_CORS_ORIGINS=["http://localhost:3000","http://localhost:5173","http://localhost:15000"]

# Rate Limiting
RATE_LIMIT_ENABLED=true
RATE_LIMIT_PER_MINUTE=60
RATE_LIMIT_PER_HOUR=1000
```

### Step 4: Initialize Database

```bash
# Create database tables
# Alembic is already configured

# Run migrations
alembic upgrade head
```

### Step 5: Start Backend Server

```bash
# Development mode (with auto-reload)
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000

# Or use Python directly
python -m app.main
```

### Verify Backend is Running

```bash
# Check health endpoint
curl http://localhost:8000/api/v1/health

# Expected response:
# {"status":"healthy","version":"1.0.0","timestamp":"2025-11-20T..."}
```

### Access API Documentation

Once the server is running, access:

- **Swagger UI:** http://localhost:8000/docs
- **ReDoc:** http://localhost:8000/redoc
- **Root:** http://localhost:8000/

---

## 💻 Local Validator Desktop App

### Location
```bash
cd ~/omniphi/pos/validator-orchestrator/local-validator-app
```

On Windows:
```bash
cd c:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app
```

### Prerequisites

- **Node.js >= 18**
- **npm or yarn**
- **posd binary** (Omniphi blockchain validator binary)

### Step 1: Install Dependencies

```bash
# Navigate to local-validator-app directory
cd ~/omniphi/pos/validator-orchestrator/local-validator-app

# Install Node.js dependencies
npm install
```

### Step 2: Add posd Binary

The app needs the `posd` binary to run validators.

**Create bin directory if it doesn't exist:**
```bash
mkdir -p bin
```

**Copy posd binary to bin directory:**

From your main blockchain directory:
```bash
# Linux/macOS
cp ~/omniphi/pos/posd ~/omniphi/pos/validator-orchestrator/local-validator-app/bin/

# Windows (PowerShell)
Copy-Item c:\Users\herna\omniphi\pos\posd.exe c:\Users\herna\omniphi\pos\validator-orchestrator\local-validator-app\bin\

# Or build fresh
cd ~/omniphi/pos
go build -o posd ./cmd/posd
cp posd ~/omniphi/pos/validator-orchestrator/local-validator-app/bin/
```

**Make binary executable (Linux/macOS):**
```bash
chmod +x ~/omniphi/pos/validator-orchestrator/local-validator-app/bin/posd
```

### Step 3: Run in Development Mode

```bash
npm run dev
```

This will:
1. Start Vite dev server on port 3000
2. Start Electron with hot-reload enabled
3. Open the desktop app window
4. Start HTTP bridge server on port 15000

### Step 4: Build for Production (Optional)

**Windows:**
```bash
npm run build:win
```

**macOS:**
```bash
npm run build:mac
```

**Linux:**
```bash
npm run build:linux
```

Built applications will be in the `dist/` directory.

---

## 🔄 Running Everything Together

### ⚠️ IMPORTANT: Choose Your Validator Mode

**You CANNOT run both the manual blockchain AND the desktop app validator at the same time!**

They will conflict because:
- Both use the same home directory (`~/.omniphi`)
- Both try to use ports 26657, 26656, 9090
- Both access the same consensus key
- Both try to lock the same database

**Choose ONE of these setups:**

---

### Setup A: Desktop App Manages Validator (Recommended)

**Don't start the blockchain manually.** Let the desktop app do it.

#### 1. Start Backend API (terminal 1)

```bash
cd ~/omniphi/pos/validator-orchestrator/backend
source venv/bin/activate  # Windows: .\venv\Scripts\Activate.ps1
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

#### 2. Start Desktop App (terminal 2)

```bash
cd ~/omniphi/pos/validator-orchestrator/local-validator-app
npm run dev
```

#### 3. Use Desktop App UI

- Click "Start Validator" in the app
- The app will start `posd` for you
- Monitor status in the UI
- Send heartbeats to backend

---

### Setup B: Manual Blockchain + Backend Only

**Don't use the desktop app's "Start Validator" button.**

#### 1. Start the Blockchain (terminal 1)

```bash
cd ~/omniphi/pos
./posd start
```

Wait for blocks to start producing.

#### 2. Start Backend API (in terminal 2)

```bash
cd ~/omniphi/pos/validator-orchestrator/backend

# Activate virtual environment
source venv/bin/activate  # or venv\Scripts\activate on Windows

# Start server
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

You should see:
```
INFO:     Uvicorn running on http://0.0.0.0:8000
INFO:     Application startup complete.
```

#### 3. Start Local Validator App (in terminal 3)

```bash
cd ~/omniphi/pos/validator-orchestrator/local-validator-app

# Start app
npm run dev
```

The Electron app window will open.

### Quick Test

#### Test Backend API:
```bash
curl http://localhost:8000/api/v1/health
```

#### Test HTTP Bridge (from desktop app):
```bash
curl http://localhost:15000/health
```

#### Test Full Integration:

1. Open desktop app
2. Go to **Dashboard** tab
3. Click **Start Validator**
4. Wait for validator to sync
5. Go to **Keys** tab
6. Click **Refresh** to load consensus public key
7. Enter your wallet address in the heartbeat section
8. Click **Send Heartbeat**
9. Check backend logs to see heartbeat received

---

## 📡 API Reference

### Backend API Endpoints

All endpoints are under `/api/v1/`

#### Health Check
```http
GET /api/v1/health
```

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2025-11-20T..."
}
```

#### Create Validator Setup Request
```http
POST /api/v1/validators/setup-requests
```

**Request Body:**
```json
{
  "walletAddress": "omni1abc...",
  "validatorName": "My Validator",
  "website": "https://myvalidator.com",
  "description": "Running Omniphi validator",
  "commissionRate": 0.10,
  "runMode": "local",
  "provider": "local"
}
```

**Response:**
```json
{
  "setupRequest": {
    "id": "uuid",
    "status": "pending",
    "walletAddress": "omni1abc...",
    "validatorName": "My Validator",
    "runMode": "local",
    "consensusPubkey": null,
    "createdAt": "2025-11-20T...",
    "updatedAt": "2025-11-20T..."
  }
}
```

#### Submit Heartbeat (Local Mode)
```http
POST /api/v1/validators/heartbeat
```

**Request Body:**
```json
{
  "walletAddress": "omni1abc...",
  "consensusPubkey": "base64...",
  "blockHeight": 12345,
  "uptimeSeconds": 3600,
  "localRpcPort": 26657,
  "localP2pPort": 26656
}
```

**Response:**
```json
{
  "message": "Heartbeat received",
  "heartbeat": {
    "id": "uuid",
    "walletAddress": "omni1abc...",
    "blockHeight": 12345,
    "lastSeen": "2025-11-20T..."
  }
}
```

#### Get Validators by Wallet
```http
GET /api/v1/validators/by-wallet/{walletAddress}
```

**Response:**
```json
[
  {
    "setupRequest": {...},
    "node": {...},
    "heartbeat": {
      "blockHeight": 12345,
      "uptimeSeconds": 3600,
      "lastSeen": "2025-11-20T..."
    }
  }
]
```

### Desktop App HTTP Bridge API

Running on **port 15000** (localhost only)

#### Get Consensus Public Key
```http
GET http://localhost:15000/consensus-pubkey
```

**Response:**
```json
{
  "success": true,
  "pubkey": {
    "@type": "/cosmos.crypto.ed25519.PubKey",
    "key": "base64_encoded_key"
  }
}
```

#### Get Validator Status
```http
GET http://localhost:15000/status
```

**Response:**
```json
{
  "success": true,
  "status": {
    "running": true,
    "blockHeight": 12345,
    "syncing": false,
    "peers": 10,
    "moniker": "my-validator",
    "network": "omniphi-1"
  }
}
```

#### Get Logs
```http
GET http://localhost:15000/logs?lines=100
```

**Response:**
```json
{
  "success": true,
  "logs": ["log line 1", "log line 2", ...]
}
```

#### Health Check
```http
GET http://localhost:15000/health
```

**Response:**
```json
{
  "success": true,
  "service": "omniphi-local-validator-bridge",
  "version": "1.0.0"
}
```

---

## 🔧 Troubleshooting

### Backend Issues

#### Issue: "ModuleNotFoundError" when starting backend

**Cause:** Virtual environment not activated or dependencies not installed

**Solution:**
```bash
cd ~/omniphi/pos/validator-orchestrator/backend
source venv/bin/activate  # or venv\Scripts\activate on Windows
pip install -r requirements.txt
```

#### Issue: "database connection failed"

**Cause:** PostgreSQL not running or DATABASE_URL incorrect

**Solution:**
```bash
# For SQLite (development), use:
DATABASE_URL="sqlite:///./validator_orchestrator.db"

# For PostgreSQL, check it's running:
sudo systemctl status postgresql  # Linux
# Or check PostgreSQL is installed and running
```

#### Issue: Port 8000 already in use

**Cause:** Another service using port 8000

**Solution:**
```bash
# Find process using port 8000
# Linux/macOS:
lsof -i :8000

# Windows:
netstat -ano | findstr :8000

# Kill the process or use different port:
uvicorn app.main:app --reload --port 8001
```

#### Issue: "alembic command not found"

**Cause:** Alembic not installed or venv not activated

**Solution:**
```bash
source venv/bin/activate
pip install alembic
alembic upgrade head
```

### Desktop App Issues

#### Issue: "posd binary not found"

**Cause:** posd binary missing from bin/ directory

**Solution:**
```bash
# Copy from main blockchain directory
cd ~/omniphi/pos
go build -o posd ./cmd/posd
cp posd ~/omniphi/pos/validator-orchestrator/local-validator-app/bin/
chmod +x ~/omniphi/pos/validator-orchestrator/local-validator-app/bin/posd
```

#### Issue: Port 15000 already in use

**Cause:** Another instance running or port conflict

**Solution:**
```bash
# Find what's using port 15000
# Linux/macOS:
lsof -i :15000

# Windows:
netstat -ano | findstr :15000

# Kill the process or change port in electron/http-bridge.js
```

#### Issue: "Failed to connect to orchestrator"

**Cause:** Backend API not running

**Solution:**
```bash
# Start backend API first
cd ~/omniphi/pos/validator-orchestrator/backend
source venv/bin/activate
uvicorn app.main:app --reload

# Then verify it's accessible:
curl http://localhost:8000/api/v1/health
```

#### Issue: Validator won't start in desktop app

**Cause:** posd binary permissions or missing dependencies

**Solution:**
```bash
# Check binary exists and is executable
cd ~/omniphi/pos/validator-orchestrator/local-validator-app
ls -lh bin/posd

# Make executable
chmod +x bin/posd

# Test binary directly
./bin/posd version

# View logs in app's "Logs" tab for error details
```

### Common Integration Issues

#### Issue: Heartbeat not reaching backend

**Solution:**
1. Verify backend is running: `curl http://localhost:8000/api/v1/health`
2. Check CORS settings in backend `.env` file
3. Verify orchestrator URL in desktop app settings matches backend URL
4. Check backend logs for errors

#### Issue: Can't create validator on-chain

**Solution:**
1. Ensure blockchain is running: `cd ~/omniphi/pos && ./posd status`
2. Verify you have consensus public key from desktop app
3. Check wallet has sufficient balance
4. Ensure wallet is connected and has signing permissions

---

## 📁 File Locations

### Backend Files
```
validator-orchestrator/backend/
├── app/
│   ├── main.py                    # FastAPI application entry point
│   ├── api/v1/
│   │   ├── validators.py          # Validator endpoints
│   │   ├── health.py              # Health check endpoint
│   │   └── auth.py                # Authentication endpoints
│   ├── core/
│   │   └── config.py              # Settings & configuration
│   ├── db/
│   │   ├── base_class.py          # SQLAlchemy base
│   │   └── session.py             # Database session
│   ├── models/                     # Database models
│   ├── schemas/                    # Pydantic schemas
│   └── services/                   # Business logic
├── .env                            # Environment variables (IMPORTANT!)
├── requirements.txt                # Python dependencies
├── alembic.ini                     # Database migrations config
└── validator_orchestrator.db       # SQLite database (development)
```

### Desktop App Files
```
validator-orchestrator/local-validator-app/
├── electron/
│   ├── main.js                    # Electron main process
│   ├── preload.js                 # Security bridge
│   ├── ipc-handlers.js            # Node management
│   └── http-bridge.js             # HTTP API server
├── src/
│   ├── components/                # React components
│   ├── App.tsx                    # Main app component
│   └── main.tsx                   # React entry point
├── bin/
│   └── posd                       # Omniphi validator binary ⚠️ MUST BE HERE
├── package.json
└── vite.config.ts
```

### Validator Data (Created by Desktop App)
```
~/.omniphi/                        # Or C:\Users\<username>\.omniphi\ on Windows
├── config/
│   ├── config.toml                # Node configuration
│   ├── app.toml                   # App configuration
│   ├── genesis.json               # Genesis file
│   └── priv_validator_key.json   # ⚠️ CONSENSUS PRIVATE KEY - BACKUP!
└── data/
    └── priv_validator_state.json  # Validator state
```

---

## 🚀 Quick Reference

### Start Backend
```bash
cd ~/omniphi/pos/validator-orchestrator/backend
source venv/bin/activate
uvicorn app.main:app --reload
```

### Start Desktop App
```bash
cd ~/omniphi/pos/validator-orchestrator/local-validator-app
npm run dev
```

### Test Backend Health
```bash
curl http://localhost:8000/api/v1/health
```

### Test Desktop App HTTP Bridge
```bash
curl http://localhost:15000/health
```

### View Backend API Docs
```
http://localhost:8000/docs
```

---

## 🆘 Getting Help

- **Backend API Docs:** http://localhost:8000/docs (when running)
- **GitHub Issues:** https://github.com/omniphi/pos/issues
- **Discord:** https://discord.gg/omniphi
- **Main Blockchain Docs:** `~/omniphi/pos/BLOCKCHAIN_QUICKSTART_UBUNTU.md`

---

**Built with ❤️ for Omniphi Blockchain** | Last Updated: 2025-11-20
