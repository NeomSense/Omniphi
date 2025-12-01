# Docker Integration Setup Guide

## Prerequisites

1. **Docker Desktop** must be installed and running
   - Download from: https://www.docker.com/products/docker-desktop/
   - Ensure WSL 2 backend is enabled (Settings → General → Use WSL 2 based engine)
   - Start Docker Desktop before running commands

2. **posd binary** copied to docker build context (already done ✅)

## Building the Validator Node Image

```bash
cd /c/Users/herna/omniphi/pos/validator-orchestrator/docker/validator-node

# Build the image
docker build -t omniphi/validator-node:latest .

# Verify the image was created
docker images | grep omniphi
```

Expected output:
```
omniphi/validator-node   latest    abc123def456   Just now   ~250MB
```

## Testing the Image Manually

### 1. Start a test container

```bash
docker run -d \
  --name test-validator \
  -p 26656:26656 \
  -p 26657:26657 \
  -e CHAIN_ID=omniphi-localnet-1 \
  -e MONIKER=test-validator \
  omniphi/validator-node:latest
```

### 2. Check container logs

```bash
docker logs -f test-validator
```

You should see:
```
==================================================
Starting Omniphi Validator Node
Chain ID: omniphi-localnet-1
Moniker: test-validator
==================================================
Initializing node...
Node initialized successfully
==================================================
Consensus Public Key:
{"@type":"/cosmos.crypto.ed25519.PubKey","key":"..."}
==================================================
Starting validator node...
```

### 3. Query the RPC endpoint

```bash
curl http://localhost:26657/status
```

### 4. Get consensus pubkey

```bash
docker exec test-validator posd tendermint show-validator --home /home/validator/.omniphi
```

### 5. Stop and remove test container

```bash
docker stop test-validator
docker rm test-validator
```

## Integration with Backend Orchestrator

Once Docker is running, the backend will automatically:

1. **Create containers** via `docker_manager.create_validator_container()`
2. **Extract consensus pubkeys** from running containers
3. **Monitor container health** via RPC queries
4. **Manage lifecycle** (stop, restart, remove)

## Enable Real Docker Provisioning

The provisioning service is ready to use real Docker. To switch from MVP mode to production:

1. **Start Docker Desktop**
2. **Build the image** (see above)
3. **Restart the backend** - it will automatically detect Docker and use real provisioning

The code in `backend/app/services/provisioning.py` is already configured for real Docker integration!

## Environment Configuration

Update `.env` if needed:

```env
DOCKER_NETWORK=omniphi-validator-network
DOCKER_IMAGE=omniphi/validator-node:latest
```

## Troubleshooting

### Docker not found

```bash
# Check if Docker is running
docker ps

# If not, start Docker Desktop application
```

### Permission errors

```bash
# On Linux, add user to docker group
sudo usermod -aG docker $USER
newgrp docker
```

### Container fails to start

```bash
# Check container logs
docker logs <container_id>

# Check Docker daemon logs
# Windows: Docker Desktop → Troubleshoot → View logs
```

### Port conflicts

If ports 26656 or 26657 are already in use:

```bash
# Find what's using the port
netstat -ano | findstr :26657

# Kill the process or use different ports in the orchestrator
```

## Next Steps

Once Docker is running:

1. **Test the full flow:**
   ```bash
   # Create a validator via API
   curl -X POST http://localhost:8000/api/v1/validators/setup-requests \
     -H "Content-Type: application/json" \
     -d '{
       "walletAddress": "omni1test",
       "validatorName": "Docker Test",
       "commissionRate": 0.10,
       "runMode": "cloud",
       "provider": "omniphi_cloud"
     }'
   ```

2. **Monitor the provisioning:**
   ```bash
   # Check setup request status
   curl http://localhost:8000/api/v1/validators/setup-requests/<id>

   # List running containers
   docker ps
   ```

3. **View container logs:**
   ```bash
   docker logs <container_id>
   ```

## Production Deployment

For production, consider:

1. **Use Docker Compose** for multi-container setups
2. **Implement volume persistence** for validator data
3. **Add monitoring** (Prometheus, Grafana)
4. **Set resource limits** (CPU, memory)
5. **Configure logging** (centralized log aggregation)

Example docker-compose.yml:

```yaml
version: '3.8'

services:
  orchestrator:
    build: ./backend
    ports:
      - "8000:8000"
    environment:
      - DATABASE_URL=postgresql://user:pass@db/validator_orchestrator
    depends_on:
      - db

  db:
    image: postgres:15
    volumes:
      - postgres_data:/var/lib/postgresql/data
    environment:
      - POSTGRES_DB=validator_orchestrator
      - POSTGRES_USER=omniphi
      - POSTGRES_PASSWORD=secure_password

volumes:
  postgres_data:
```

## Security Considerations

1. **Consensus keys** are generated inside containers and never leave them
2. **Network isolation** via Docker bridge networks
3. **User permissions** - containers run as non-root user
4. **Volume permissions** - proper ownership of mounted volumes
5. **Resource limits** - prevent DoS via resource exhaustion

---

**Status**: Docker infrastructure ready, waiting for Docker Desktop to be running.
