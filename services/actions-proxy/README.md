# Actions Proxy — Omniphi Governance Tx Backend

Secure HTTP backend that enables the React governance dashboard to trigger **Confirm Execution** for CRITICAL proposals without putting private keys in the browser.

## Architecture

```
services/actions-proxy/
  cmd/actions-proxy/main.go         Entry point
  internal/
    config/config.go                Env-var configuration
    chain/client.go                 CLI-based posd interaction
    server/server.go                HTTP handlers, auth, CORS
    ratelimit/limiter.go            Token bucket rate limiter
    types/types.go                  Shared request/response types
  configs/example.env               All env vars with comments
  deploy/actions-proxy.service      systemd unit template
  Dockerfile                        Container build
```

## Prerequisites

- `posd` binary in `$PATH` (built from `chain/`)
- Running Omniphi node (Tendermint RPC)
- Funded key with governance authority (to sign `MsgConfirmExecution`)
- API key for authentication

## Quick Start

```bash
# Build
cd services/actions-proxy
go build -o actions-proxy ./cmd/actions-proxy

# Configure
cp configs/example.env .env
source .env

# Run
./actions-proxy
```

## Configuration

All configuration is via environment variables. See [configs/example.env](configs/example.env) for the full list.

### Required

| Variable | Description | Default |
|----------|-------------|---------|
| `API_KEY` | API key for `X-API-Key` header auth | (required) |
| `POSD_CHAIN_ID` | Chain ID | (required) |
| `KEY_NAME` | Keyring key name for signing txs | (required) |
| `TX_FEES` or `TX_GAS_PRICES` | Transaction fee config | (required) |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `BIND_ADDR` | HTTP listen address | `127.0.0.1:8090` |
| `POSD_BIN` | Path to posd binary | `posd` |
| `POSD_NODE` | Tendermint RPC endpoint | `http://localhost:26657` |
| `POSD_HOME` | posd home directory | (system default) |
| `KEYRING_BACKEND` | Keyring backend | `test` |
| `TX_GAS` | Gas setting | `auto` |
| `TX_GAS_ADJUSTMENT` | Gas adjustment multiplier | `1.3` |
| `TX_BROADCAST_MODE` | Broadcast mode | `sync` |
| `TX_TIMEOUT_SECONDS` | CLI command timeout | `30` |
| `RATE_LIMIT_RPS` | Requests per second | `0.2` |
| `RATE_LIMIT_BURST` | Burst capacity | `2` |
| `CORS_ALLOW_ORIGINS` | Comma-separated origins | `*` |

## API

### GET /health

```bash
curl http://localhost:8090/health
```

Response:
```json
{"ok": true, "service": "actions-proxy", "version": "v1"}
```

### POST /confirm-execution

Confirms execution of a CRITICAL proposal that requires a second confirmation.

**Headers:**
- `Content-Type: application/json`
- `X-API-Key: <your-api-key>`

**Request:**
```json
{"proposal_id": "123"}
```

**Example:**
```bash
curl -X POST http://localhost:8090/confirm-execution \
  -H "X-API-Key: change-me-in-production" \
  -H "Content-Type: application/json" \
  -d '{"proposal_id":"123"}'
```

**Responses:**

| Status | Result | When |
|--------|--------|------|
| 200 | `submitted` | Tx broadcast succeeded |
| 200 | `already_confirmed` | Second confirmation already received |
| 400 | `rejected` | Invalid request body or proposal_id |
| 401 | — | Missing or invalid API key |
| 409 | `rejected` | Proposal not eligible (wrong gate state, not CRITICAL, etc.) |
| 429 | — | Rate limit exceeded |
| 500 | `rejected` | CLI or chain error |

**Success response:**
```json
{
  "proposal_id": 123,
  "eligible": true,
  "action": "confirm-execution",
  "result": "submitted",
  "tx": {"code": 0, "txhash": "A1B2C3...", "raw_log": ""},
  "message": "Execution confirmed. TxHash: A1B2C3..."
}
```

## Preflight Validation

Before broadcasting, the proxy validates on-chain state:

1. Query `posd query guard queued-execution <id>` — proposal must exist
2. `gate_state` must be `READY`
3. `requires_second_confirm` must be `true`
4. `second_confirm_received` must be `false` (idempotent: returns 200 if already confirmed)
5. Query `posd query guard risk-report <id>` — tier must be `CRITICAL`

If any check fails, the request is rejected *before* broadcasting.

## React UI Integration

The governance dashboard calls this proxy when the Confirm Execution button is clicked:

```
VITE_BACKEND_ACTIONS_URL=http://localhost:8090
VITE_BACKEND_API_KEY=change-me-in-production
```

The API key is sent via `X-API-Key` header. For production, do NOT embed the key in the frontend build — use a reverse proxy that injects the header, or restrict CORS origins.

## Security

- **API key auth**: Every request requires `X-API-Key` header
- **Rate limiting**: Token bucket (default 1 req/5s, burst 2) prevents abuse
- **Preflight validation**: On-chain state checked before broadcasting
- **No browser signing**: Private keys never leave the server
- **CORS**: Configurable allowed origins (restrict in production)
- **Idempotent**: Already-confirmed proposals return 200, not error

### Production Hardening

- Run behind a firewall (not exposed to internet)
- Use `file` or `os` keyring backend (not `test`)
- Set `CORS_ALLOW_ORIGINS` to your dashboard origin only
- Rotate `API_KEY` regularly
- Use a reverse proxy (nginx/caddy) with TLS termination
- Consider IP allowlisting at the firewall level

## Deployment

### systemd

```bash
sudo cp deploy/actions-proxy.service /etc/systemd/system/
sudo cp configs/example.env /etc/omniphi/actions-proxy.env
# Edit /etc/omniphi/actions-proxy.env with production values
sudo systemctl enable --now actions-proxy
```

### Docker

```bash
docker build -t actions-proxy .
docker run -d \
  --name actions-proxy \
  -p 8090:8090 \
  -v /path/to/posd:/usr/local/bin/posd \
  --env-file .env \
  actions-proxy
```

## Testing

```bash
cd services/actions-proxy
go test ./... -v -count=1
```
