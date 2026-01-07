# Omniphi Testnet Faucet

Production-grade token distribution service for the Omniphi testnet.

## Features

- **Rate Limiting**: Per-address cooldown and daily distribution caps
- **Web UI**: User-friendly interface for requesting tokens
- **REST API**: Programmatic access for developers
- **Health Checks**: Monitoring and status endpoints
- **CORS Support**: Cross-origin requests for web applications
- **Docker Ready**: Production deployment with Docker Compose

## Quick Start

### Environment Variables

Create a `.env` file:

```bash
FAUCET_MNEMONIC="your faucet wallet mnemonic phrase here"
CHAIN_ID=omniphi-testnet-1
RPC_ENDPOINT=http://localhost:26657
GRPC_ENDPOINT=localhost:9090
DISTRIBUTION_AMOUNT=10000000000  # 10,000 OMNI
COOLDOWN_SECONDS=86400  # 24 hours
DAILY_CAP=1000
```

### Run with Docker

```bash
docker-compose up -d
```

### Run Locally

```bash
export FAUCET_MNEMONIC="your mnemonic here"
go run .
```

## API Endpoints

### POST /faucet
Request tokens for an address.

```bash
curl -X POST http://localhost:8080/faucet \
  -H "Content-Type: application/json" \
  -d '{"address": "omni1..."}'
```

Response:
```json
{
  "success": true,
  "tx_hash": "ABC123...",
  "amount": "10000 OMNI",
  "message": "Tokens sent successfully!"
}
```

### GET /health
Health check endpoint.

```json
{
  "status": "healthy",
  "faucet_address": "omni1...",
  "chain_id": "omniphi-testnet-1",
  "daily_remaining": 950
}
```

### GET /stats
Distribution statistics.

```json
{
  "total_distributed_today": 50,
  "daily_cap": 1000,
  "cooldown_seconds": 86400,
  "distribution_amount": "10000 OMNI"
}
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `FAUCET_PORT` | 8080 | HTTP server port |
| `FAUCET_HOST` | 0.0.0.0 | HTTP server host |
| `CHAIN_ID` | omniphi-testnet-1 | Blockchain chain ID |
| `RPC_ENDPOINT` | http://localhost:26657 | Tendermint RPC endpoint |
| `DENOM` | uomni | Token denomination |
| `BECH32_PREFIX` | omni | Address prefix |
| `FAUCET_MNEMONIC` | (required) | Faucet wallet mnemonic |
| `DISTRIBUTION_AMOUNT` | 10000000000 | Amount per request (in uomni) |
| `COOLDOWN_SECONDS` | 86400 | Cooldown between requests |
| `DAILY_CAP` | 1000 | Max distributions per day |
| `ALLOWED_ORIGINS` | * | CORS allowed origins |

## Security

- Store `FAUCET_MNEMONIC` securely (use secrets management in production)
- Run behind a reverse proxy (nginx) in production
- Enable rate limiting at the proxy level for additional protection
- Monitor faucet balance and set up alerts

## Monitoring

The faucet exposes metrics at `/health` and `/stats` for monitoring:

```bash
# Check health
curl http://localhost:8080/health

# Get stats
curl http://localhost:8080/stats
```

## License

MIT License - Omniphi Network
