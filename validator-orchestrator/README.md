# Omniphi One-Click Validator System

**Production-grade validator onboarding system for Omniphi blockchain**

Enable users to become validators in seconds with either Cloud or Local mode.

---

## 🎯 System Overview

This system allows users to become Omniphi validators through two modes:

### Cloud Mode
1. User connects wallet
2. Enters validator profile
3. System provisions cloud validator node (Docker container)
4. Consensus keypair generated inside container (secure!)
5. User signs MsgCreateValidator transaction
6. Validator goes live

### Local Mode
1. User connects wallet
2. Downloads desktop app
3. Desktop app runs validator locally
4. App submits heartbeat to API
5. User signs MsgCreateValidator transaction
6. Validator goes live

---

## 📁 Project Structure

```
validator-orchestrator/
├── backend/                    Backend Orchestrator (FastAPI)
│   ├── app/
│   │   ├── api/
│   │   │   └── v1/
│   │   │       └── validators.py      REST API endpoints
│   │   ├── core/
│   │   │   └── config.py              Settings & configuration
│   │   ├── db/
│   │   │   ├── base_class.py          SQLAlchemy base
│   │   │   └── session.py             Database session
│   │   ├── models/                     Database models
│   │   │   ├── validator_setup_request.py
│   │   │   ├── validator_node.py
│   │   │   └── local_validator_heartbeat.py
│   │   ├── schemas/
│   │   │   └── validators.py          Pydantic schemas
│   │   ├── services/
│   │   │   ├── chain_client.py        Cosmos SDK client
│   │   │   ├── docker_manager.py      Docker management
│   │   │   └── provisioning.py        Cloud provisioning
│   │   ├── workers/
│   │   │   └── provisioner.py         Background tasks
│   │   └── main.py                    FastAPI application
│   ├── requirements.txt
│   ├── .env.example
│   └── Dockerfile                     (to be created)
├── frontend/                          (Phase 3 - to be built)
├── local-validator-app/               (Phase 4 - to be built)
├── infra/                             (Infrastructure - to be built)
└── README.md                          This file
```

---

## 🚀 Quick Start

### ⚠️ Complete Setup Guide Available

**For detailed setup instructions, see:**
👉 **[BACKEND_LOCAL_VALIDATOR_GUIDE.md](BACKEND_LOCAL_VALIDATOR_GUIDE.md)**

This comprehensive guide includes:
- Complete backend API setup with all commands
- Local validator desktop app installation
- Running everything together
- Full API reference
- Troubleshooting guide

### Quick Backend Start

```bash
cd backend
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate
pip install -r requirements.txt
cp .env.example .env  # Edit with your config
alembic upgrade head
uvicorn app.main:app --reload
```

### Quick Desktop App Start

```bash
cd local-validator-app
npm install
# Copy posd binary to bin/ directory
cp ../../posd bin/  # Adjust path as needed
npm run dev
```

### Access

- **Backend API Docs**: http://localhost:8000/docs
- **Backend Health**: http://localhost:8000/api/v1/health
- **Desktop App**: Electron window opens automatically
- **HTTP Bridge**: http://localhost:15000/health

---

## 📡 API Endpoints

All endpoints are under `/api/v1/validators`

### 1. Create Setup Request

**`POST /api/v1/validators/setup-requests`**

Create a new validator setup request.

**Request:**
```json
{
  "walletAddress": "omni1abc...",
  "validatorName": "My Validator",
  "website": "https://myvalidator.com",
  "description": "Running Omniphi validator",
  "commissionRate": 0.10,
  "runMode": "cloud",
  "provider": "omniphi_cloud"
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
    "runMode": "cloud",
    "consensusPubkey": null,
    "createdAt": "2025-01-...",
    "updatedAt": "2025-01-..."
  }
}
```

### 2. Poll Setup Status

**`GET /api/v1/validators/setup-requests/{id}`**

Poll for provisioning status and consensus pubkey.

**Response:**
```json
{
  "setupRequest": {
    "id": "uuid",
    "status": "ready_for_chain_tx",
    "consensusPubkey": "base64_encoded_pubkey...",
    ...
  },
  "node": {
    "id": "uuid",
    "status": "running",
    "rpcEndpoint": "http://...:26657",
    "p2pEndpoint": "tcp://...:26656"
  }
}
```

**Status Flow:**
- `pending` → `provisioning` → `ready_for_chain_tx` → `active`

### 3. Get Validators by Wallet

**`GET /api/v1/validators/by-wallet/{walletAddress}`**

Get all validators for a wallet address.

**Response:**
```json
[
  {
    "setupRequest": {...},
    "node": {...},
    "chainInfo": {
      "isActiveValidator": true,
      "votingPower": "100000",
      "jailed": false
    },
    "heartbeat": null
  }
]
```

### 4. Stop Validator

**`POST /api/v1/validators/stop`**

Stop a cloud validator node.

**Request:**
```json
{
  "setupRequestId": "uuid"
}
```

### 5. Submit Heartbeat (Local Mode)

**`POST /api/v1/validators/heartbeat`**

Called by desktop app to report local validator status.

**Request:**
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

---

## 🗄️ Database Schema

### validator_setup_requests
- `id` (UUID) - Primary key
- `wallet_address` - User's wallet address
- `validator_name` - Validator moniker
- `commission_rate` - Commission (0.0-1.0)
- `website` - Validator website
- `description` - Validator description
- `run_mode` - "cloud" or "local"
- `provider` - Cloud provider name
- `consensus_pubkey` - Generated consensus public key
- `status` - Current status (enum)
- `created_at`, `updated_at`, `completed_at`

### validator_nodes
- `id` (UUID) - Primary key
- `setup_request_id` (FK) - Link to setup request
- `provider` - Cloud provider
- `node_internal_id` - Docker container ID
- `rpc_endpoint` - RPC URL
- `p2p_endpoint` - P2P URL
- `grpc_endpoint` - gRPC URL
- `status` - Node status (enum)
- `logs_url` - Logs URL
- `last_block_height` - Current block height
- `last_health_check` - Last health check time

### local_validator_heartbeats
- `id` (UUID) - Primary key
- `wallet_address` - Validator wallet
- `consensus_pubkey` - Consensus public key
- `block_height` - Current block height
- `uptime_seconds` - Uptime
- `local_rpc_port` - Local RPC port
- `local_p2p_port` - Local P2P port
- `first_seen`, `last_seen`

---

## 🔧 Services

### Chain Client (`services/chain_client.py`)

Cosmos SDK compatible chain integration:

- `get_validator_info(wallet_address)` - Query validator status
- `build_create_validator_tx()` - Build MsgCreateValidator
- `broadcast_tx()` - Broadcast signed transaction
- `get_block_height()` - Get current block height

### Provisioning Service (`services/provisioning.py`)

Cloud validator provisioning:

- `provision_cloud_validator()` - Async provisioning job
- `health_check_worker()` - Continuous health monitoring

**MVP Note:** Currently generates placeholder values. Production would:
- Create real Docker containers
- Use actual Omniphi node image
- Extract real consensus pubkeys
- Configure persistent storage

### Docker Manager (`services/docker_manager.py`)

Docker container management:

- `create_validator_container()` - Provision container
- `stop_container()` - Graceful shutdown
- `get_container_status()` - Health check
- `get_container_logs()` - View logs

---

## 🔐 Security

### Key Management

✅ **Consensus Keys (Cloud Mode)**
- Generated inside Docker container
- Never transmitted over network
- Never stored in orchestrator database

✅ **Consensus Keys (Local Mode)**
- Generated on user's local machine
- Encrypted on disk
- User-controlled backup

✅ **Wallet Keys**
- Always remain in user's wallet
- Never touched by orchestrator
- User signs transactions client-side

### API Security

- Input validation with Pydantic
- SQL injection prevention (SQLAlchemy ORM)
- CORS properly configured
- Rate limiting (to be added)
- HTTPS required in production

---

## 📊 Monitoring & Logging

### Structured Logging

All services use Python's `logging` module with structured output:

```python
logger.info(f"Starting provisioning for request {request_id}")
logger.error(f"Failed to provision: {error}", exc_info=True)
```

### Health Checks

- **API Health**: `GET /health`
- **Database**: Connection check
- **Nodes**: Periodic container health checks

### Metrics (Future)

- Prometheus metrics endpoint
- Grafana dashboards
- Alert manager integration

---

## 🧪 Testing

### Unit Tests (To Be Created)

```bash
# Install test dependencies
pip install pytest pytest-asyncio pytest-cov

# Run tests
pytest tests/ -v --cov=app
```

### Test Files to Create

- `tests/test_api_validators.py` - API endpoint tests
- `tests/test_chain_client.py` - Chain client tests
- `tests/test_provisioning.py` - Provisioning logic tests
- `tests/test_docker_manager.py` - Docker management tests

---

## 🚢 Deployment

### Docker

**Build backend image:**
```bash
cd backend
docker build -t omniphi/validator-orchestrator:latest .
```

### Docker Compose (To Be Created)

```bash
cd infra
docker-compose up -d
```

### Kubernetes (To Be Created)

```bash
kubectl apply -f infra/k8s/
```

### Terraform (To Be Created)

```bash
cd infra/terraform
terraform init
terraform plan
terraform apply
```

---

## 📈 Roadmap

### ✅ Phase 1: Backend Foundation (COMPLETE)
- Database models
- REST API endpoints
- Chain client
- Pydantic schemas

### ✅ Phase 2: Docker Integration (COMPLETE)
- Docker manager service
- Cloud provisioning worker
- Health check system

### 🚧 Phase 3: Frontend Portal (IN PROGRESS)
- React + Vite + TypeScript
- Validator setup wizard
- Dashboard
- Transaction signing UI

### 📅 Phase 4: Desktop App (PLANNED)
- Electron app
- Local node management
- HTTP bridge for portal
- Consensus key backup

### 📅 Infrastructure (PLANNED)
- Dockerfiles
- docker-compose.yml
- Kubernetes manifests
- Terraform templates

---

## 🛠️ Development

### Code Style

- **Backend**: PEP 8, type hints
- **Imports**: Organized (standard, third-party, local)
- **Logging**: Structured with context
- **Error Handling**: Explicit try/except with logging

### Environment Variables

See `.env.example` for all configuration options.

**Required:**
- `POSTGRES_*` - Database connection
- `OMNIPHI_CHAIN_ID` - Chain ID
- `OMNIPHI_RPC_URL` - Chain RPC endpoint

**Optional:**
- `DEBUG` - Enable debug mode
- `SECRET_KEY` - JWT secret
- Cloud provider credentials

---

## 📚 Additional Documentation

- **API Spec**: http://localhost:8000/docs (when running)
- **Implementation Status**: `IMPLEMENTATION_STATUS.md`
- **Architecture**: See diagrams in `IMPLEMENTATION_STATUS.md`

---

## 🤝 Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

---

## 📄 License

Copyright © 2025 Omniphi

---

## 🆘 Support

For issues or questions:
1. Check API documentation at `/docs`
2. Review logs for errors
3. Check database connection
4. Verify Docker is running (cloud mode)

---

**Built with enterprise standards for Omniphi blockchain** 🚀
