# Gov Copilot — Layer 3 Advisory Intelligence Service

Off-chain, non-binding advisory service for Omniphi governance proposals. Polls `x/gov` for new proposals, fetches `x/guard` risk data, generates structured JSON reports using **DeepSeek** (or a deterministic template fallback), and posts `MsgSubmitAdvisoryLink` on-chain.

## Architecture

```
services/gov-copilot/
  cmd/gov-copilot/main.go          Entry point (polling loop)
  internal/
    config/config.go               Env-var configuration
    chain/client.go                CLI-based posd integration
    ai/deepseek/client.go          DeepSeek Chat API client
    report/schema.go               Report schema + template generator
    store/local/store.go           Atomic local file storage
    state/state.go                 Persistent state (last_seen_proposal_id)
    uri/uri.go                     Public URI generation and normalization
  configs/example.env              All env vars with comments
  reports/                         Default report output directory
```

## Prerequisites

- `posd` binary in `$PATH` (built from `chain/`)
- Running Omniphi node (Tendermint RPC)
- Funded key in the keyring (to pay tx fees for `MsgSubmitAdvisoryLink`)
- DeepSeek API key (optional — template mode works without it)

## Quick Start

```bash
# Build
cd services/gov-copilot
go build -o gov-copilot ./cmd/gov-copilot

# Configure (copy and edit)
cp configs/example.env .env
source .env

# Run
./gov-copilot
```

## Configuration

All configuration is via environment variables. See [configs/example.env](configs/example.env) for the full list.

### Required

| Variable | Description | Default |
|----------|-------------|---------|
| `POSD_CHAIN_ID` | Chain ID | (required) |
| `KEY_NAME` | Keyring key name for signing txs | (required) |
| `TX_FEES` or `TX_GAS_PRICES` | Transaction fee configuration | (required) |

### DeepSeek

| Variable | Description | Default |
|----------|-------------|---------|
| `DEEPSEEK_API_KEY` | API key from platform.deepseek.com | (required if `AI_MODE=deepseek`) |
| `DEEPSEEK_BASE_URL` | API base URL | `https://api.deepseek.com` |
| `DEEPSEEK_MODEL` | Model name | `deepseek-chat` |
| `AI_MODE` | `deepseek` or `template` | `deepseek` |
| `AI_TIMEOUT_SECONDS` | Per-request timeout | `20` |
| `AI_MAX_RETRIES` | Retry count with exponential backoff | `2` |

### Public Report Hosting (Required for UI Verification)

| Variable | Description | Default |
|----------|-------------|---------|
| `REPORT_PUBLIC_ENABLED` | Generate public HTTP URIs | `true` |
| `REPORT_PUBLIC_BASE_URL` | Base URL for reports (no trailing `/`) | (required if enabled) |
| `REPORT_HTTP_SERVE_ENABLED` | Built-in file server (dev/testnet) | `false` |
| `REPORT_HTTP_BIND_ADDR` | File server bind address | `127.0.0.1:8088` |

When `REPORT_PUBLIC_ENABLED=true`, advisory links posted on-chain use `https://.../<id>.json` URIs that browsers can fetch and hash-verify. When disabled, falls back to `file://` URIs (not browser-fetchable).

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `POSD_NODE` | Tendermint RPC endpoint | `http://localhost:26657` |
| `KEYRING_BACKEND` | Keyring backend | `test` |
| `POLL_INTERVAL_SECONDS` | Polling frequency | `10` |
| `REPORT_DIR` | Report output directory | `./reports` |
| `REPORTER_ID` | Reporter tag in reports | `gov-copilot-v1` |
| `STATE_FILE` | Persistent state file | `./state.json` |

## How It Works

1. **Poll** `posd query gov proposals` for new proposals
2. **Check** `posd query guard advisory-link <id>` — skip if link already exists
3. **Fetch** `posd query guard risk-report <id>` and `queued-execution <id>` for guard data
4. **Generate** structured JSON report using DeepSeek API (or template fallback)
5. **Store** report to `REPORT_DIR/<proposal_id>.json` (atomic write + SHA256)
6. **Generate URI**: `${REPORT_PUBLIC_BASE_URL}/<id>.json` (or `file://` if disabled)
7. **Submit** `posd tx guard submit-advisory-link <id> <uri> <hash>` on-chain
8. **Persist** processed state to `STATE_FILE`

## Report Schema

```json
{
  "proposal_id": 42,
  "chain_id": "omniphi-1",
  "created_at": "2026-02-17T00:00:00Z",
  "reporter": "gov-copilot-v1",
  "ai_provider": "deepseek",
  "risk": {
    "tier_rules": "RISK_TIER_HIGH",
    "tier_ai": "RISK_TIER_MED",
    "tier_final": "RISK_TIER_HIGH",
    "treasury_bps": 500,
    "churn_bps": 0
  },
  "timeline": {
    "current_gate": "EXECUTION_GATE_CONDITIONAL_EXECUTION",
    "earliest_exec_height": 120960,
    "notes": ""
  },
  "summary": "...",
  "key_changes": ["..."],
  "what_could_go_wrong": ["..."],
  "recommended_safety_actions": ["..."]
}
```

## Verification

```bash
# Check on-chain advisory link
posd query guard advisory-link 42 -o json

# Verify report hash
sha256sum reports/42.json
```

## Testing

```bash
cd services/gov-copilot
go test ./... -v -count=1
```

Tests cover:
- Report schema JSON roundtrip
- DeepSeek response parsing (mocked HTTP server)
- Markdown-wrapped JSON handling
- Retry with exponential backoff
- State file persistence and atomic writes
- Local store atomic write and SHA256
- Public URI generation and normalization

## Public Report Hosting

The governance dashboard verifies advisory report hashes in-browser using `crypto.subtle.digest('SHA-256', bytes)`. This requires reports to be fetchable over HTTP(S).

### Production: nginx / caddy

Serve `REPORT_DIR` behind a reverse proxy with CORS headers:

**nginx:**
```nginx
server {
    listen 443 ssl;
    server_name copilot.omniphi.org;

    location /reports/ {
        alias /var/lib/gov-copilot/reports/;
        add_header Access-Control-Allow-Origin *;
        add_header Cache-Control "public, max-age=3600";
    }
}
```

**caddy:**
```
copilot.omniphi.org {
    handle /reports/* {
        root * /var/lib/gov-copilot/reports
        file_server
        header Access-Control-Allow-Origin *
    }
}
```

Then set:
```bash
REPORT_PUBLIC_BASE_URL=https://copilot.omniphi.org/reports
```

### Dev / Testnet: built-in server

For local development, enable the built-in HTTP file server:

```bash
REPORT_HTTP_SERVE_ENABLED=true
REPORT_HTTP_BIND_ADDR=0.0.0.0:8088
REPORT_PUBLIC_BASE_URL=http://localhost:8088/reports
```

This serves `REPORT_DIR` at `http://<host>:8088/reports/<id>.json`. Not recommended for production.

## Template Fallback

When `AI_MODE=template` or when DeepSeek fails, the service generates a deterministic template report based on guard module data:

- Summary derived from tier and proposal title
- Risks keyed to tier level (CRITICAL/HIGH/MED/LOW)
- Safety actions based on message types (upgrade, treasury, slashing)
- No external API calls required

## Security

- **Non-binding**: Reports are advisory only; no on-chain enforcement
- **Idempotent**: Skips proposals that already have an advisory link
- **Safe fallback**: API failures produce template reports (never crash)
- **Atomic writes**: Reports written via temp-file-then-rename
- **Report hash**: SHA256 of the full JSON, verified on-chain via `MsgSubmitAdvisoryLink`
