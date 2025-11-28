# Validator Orchestrator Quick Start Card

## 📍 Locations

**Backend API:** `~/omniphi/pos/validator-orchestrator/backend/`
**Desktop App:** `~/omniphi/pos/validator-orchestrator/local-validator-app/`

---

## 🚀 Start Backend API

```bash
cd ~/omniphi/pos/validator-orchestrator/backend
source venv/bin/activate
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

**Expected Output:**
```
INFO:     Uvicorn running on http://0.0.0.0:8000
INFO:     Application startup complete.
```

**Test it:**
```bash
curl http://localhost:8000/api/v1/health
```

---

## 💻 Start Desktop App

```bash
cd ~/omniphi/pos/validator-orchestrator/local-validator-app
npm run dev
```

**Test HTTP Bridge:**
```bash
curl http://localhost:15000/health
```

---

## ⚠️ Before First Run

### Backend Setup (One-Time)
```bash
cd ~/omniphi/pos/validator-orchestrator/backend
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
cp .env.example .env  # Edit if needed
alembic upgrade head
```

### Desktop App Setup (One-Time)
```bash
cd ~/omniphi/pos/validator-orchestrator/local-validator-app
npm install

# Copy posd binary
cp ~/omniphi/pos/posd bin/
chmod +x bin/posd
```

---

## 🔗 Access Points

| Service | URL | Purpose |
|---------|-----|---------|
| Backend API Docs | http://localhost:8000/docs | Swagger UI |
| Backend Health | http://localhost:8000/api/v1/health | Health check |
| Desktop App | Electron window | UI |
| HTTP Bridge | http://localhost:15000/ | Desktop app API |

---

## 🐛 Common Issues

### Backend won't start
```bash
cd ~/omniphi/pos/validator-orchestrator/backend
source venv/bin/activate
pip install -r requirements.txt
```

### Desktop app can't find posd
```bash
cp ~/omniphi/pos/posd ~/omniphi/pos/validator-orchestrator/local-validator-app/bin/
chmod +x ~/omniphi/pos/validator-orchestrator/local-validator-app/bin/posd
```

### Port conflicts
- Backend: Change `--port 8000` to `--port 8001`
- Desktop HTTP bridge: Edit `electron/http-bridge.js`

---

## 📚 Full Documentation

See [BACKEND_LOCAL_VALIDATOR_GUIDE.md](BACKEND_LOCAL_VALIDATOR_GUIDE.md) for complete setup instructions, API reference, and troubleshooting.

---

**Quick tip:** Run these in separate terminal windows/tabs so you can see logs from each service.
