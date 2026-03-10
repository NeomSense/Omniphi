# Omniphi Governance Dashboard

A lightweight React dashboard that displays Cosmos governance proposals alongside the **x/guard** module's 3-layer risk assessment pipeline.

## Features

- **Proposal list** with real-time polling from the Cosmos `x/gov` module
- **Guard risk panel** — shows rules-based tier, AI tier, final tier, treasury BPS, delay blocks
- **Execution timeline** — horizontal stepper showing gate progression (Visibility → Shock Absorber → Conditional → Ready → Executed/Aborted) with block countdown
- **Advisory panel** — DeepSeek copilot report URI, hash, and verification instructions
- **Confirm Execution** button for CRITICAL proposals requiring second confirmation (calls backend proxy — no browser signing)
- **Raw JSON viewer** for every data source (collapsible)
- Dark theme with responsive layout

## Quick Start

```bash
# 1. Install dependencies
npm install

# 2. Configure endpoints
cp .env.example .env
# Edit .env with your LCD URL

# 3. Run dev server
npm run dev
```

Open http://localhost:5173 in your browser.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_LCD_URL` | `http://localhost:1317` | Cosmos LCD (REST) endpoint |
| `VITE_GUARD_REST_URL` | Same as LCD | Guard module REST (if separate) |
| `VITE_POLL_MS` | `8000` | Polling interval in ms |
| `VITE_BACKEND_ACTIONS_URL` | *(empty)* | Backend proxy for tx actions |

## Expected REST Endpoints

The dashboard queries these endpoints. All paths are centralized in `src/api/gov.js` and `src/api/guard.js` for easy adjustment.

### Cosmos Gov (standard)

| Endpoint | Description |
|----------|-------------|
| `GET /cosmos/gov/v1/proposals` | List proposals |
| `GET /cosmos/gov/v1/proposals/{id}` | Single proposal |
| `GET /cosmos/gov/v1/proposals/{id}/tally` | Tally results |
| `GET /cosmos/base/tendermint/v1beta1/blocks/latest` | Latest block height |

### Omniphi Guard (custom module)

| Endpoint | Description |
|----------|-------------|
| `GET /omniphi/guard/v1/guard_status/{id}` | Combined risk + execution (preferred) |
| `GET /omniphi/guard/v1/risk_report/{id}` | Risk report fallback |
| `GET /omniphi/guard/v1/queued/{id}` | Queued execution fallback |
| `GET /omniphi/guard/v1/advisory/{id}` | Advisory link |
| `GET /omniphi/guard/v1/params` | Module parameters |

### Backend Proxy (optional)

| Endpoint | Method | Body | Description |
|----------|--------|------|-------------|
| `{BACKEND}/confirm-execution` | POST | `{ "proposal_id": "42" }` | Confirm CRITICAL proposal execution |

## Troubleshooting

### CORS errors

If the browser blocks LCD requests with CORS errors, configure your node to allow the dashboard origin:

```toml
# app.toml
[api]
enabled-unsafe-cors = true
```

Or run a local proxy (e.g., `caddy reverse-proxy --from :1317 --to node:1317`).

### Wrong LCD URL

Check that `VITE_LCD_URL` points to the correct endpoint. You can verify it works by running:

```bash
curl http://localhost:1317/cosmos/gov/v1/proposals
```

### Guard routes not found

If guard endpoints return 404, the module may not be registered on your chain or may use different REST paths. Check `src/api/guard.js` and update the `PATHS` object.

### No proposals shown

If the proposal list is empty, your chain may not have any proposals yet. Submit a test proposal:

```bash
posd tx gov submit-proposal draft_proposal.json --from mykey --fees 1000uomni
```

## Tech Stack

- React 19 + Vite
- react-router-dom v7
- Plain CSS (no Tailwind, no CSS-in-JS)
- `fetch()` only (no axios)
- Polling-based updates (no WebSocket)

## Architecture

```
src/
  api/           # HTTP client + endpoint definitions
    client.js    #   Base fetch wrapper with timeout
    gov.js       #   Cosmos gov queries
    guard.js     #   x/guard queries + confirm action
  components/    # Reusable UI components
    Layout.jsx   #   App shell (header + outlet)
    ProposalList.jsx
    ProposalRow.jsx
    GuardPanel.jsx
    Timeline.jsx
    AdvisoryPanel.jsx
    JsonViewer.jsx
    Loading.jsx
    ErrorBanner.jsx
  pages/         # Route-level pages
    Home.jsx     #   Proposal list with guard columns
    ProposalPage.jsx  # Detail view with all panels
  utils/         # Shared helpers
    format.js    #   Display formatting
    gates.js     #   Gate definitions + time math
    polling.js   #   usePolling hook
  App.jsx        # Router setup
  main.jsx       # Entry point
  styles.css     # All styles
```
