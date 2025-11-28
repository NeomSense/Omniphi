# Omniphi Local Validator Desktop App

Electron-based desktop application for running Omniphi validators locally on your machine.

---

## 📖 Complete Documentation Available

**For full setup instructions and troubleshooting, see:**
👉 **[../BACKEND_LOCAL_VALIDATOR_GUIDE.md](../BACKEND_LOCAL_VALIDATOR_GUIDE.md)**

Or start here: **[../START_HERE.md](../START_HERE.md)**

---

## Features

- **One-Click Validator Management**: Start/stop your local validator node with a single click
- **Real-time Status Monitoring**: Live block height, peer count, sync status, and uptime
- **Consensus Key Management**: Generate, view, and backup your validator consensus keys
- **HTTP Bridge Server**: Exposes validator status on port 15000 for external access
- **Heartbeat to Orchestrator**: Automatically send status updates to the backend orchestrator
- **Log Viewer**: Real-time log streaming from your validator node
- **Secure Key Storage**: Keys stored in OS keychain (Windows Credential Manager, macOS Keychain, Linux Secret Service)

## Prerequisites

- Node.js >= 18
- npm or yarn
- **posd binary** - The Omniphi blockchain validator binary

## Installation

### 1. Install Dependencies

```bash
cd local-validator-app
npm install
```

### 2. Add posd Binary

Place the `posd` binary (or `posd.exe` on Windows) in the `bin/` directory:

```
local-validator-app/
└── bin/
    └── posd          # Linux/macOS
    └── posd.exe      # Windows
```

Download the binary from:
- GitHub Releases: https://github.com/omniphi/omniphi/releases
- Build from source: https://github.com/omniphi/omniphi

## Development

### Run in Development Mode

```bash
npm run dev
```

This will:
1. Start Vite dev server on port 3000
2. Start Electron with hot-reload enabled
3. Open DevTools automatically

### Build for Production

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

## Usage

### First Time Setup

1. **Launch the app**
2. **App will check for `posd` binary**
   - If not found, you'll see setup instructions
   - Place binary in `bin/` directory
   - Click "Check Again"

### Starting Your Validator

1. Navigate to **Dashboard** tab
2. Click **Start Validator**
3. Wait for node initialization (30-60 seconds)
4. Monitor sync status in real-time

### Getting Your Consensus Public Key

1. Navigate to **Keys** tab
2. Click **Refresh** to load your consensus public key
3. Click **Copy to Clipboard**
4. Use this key when creating your validator on-chain

### Sending Heartbeats

1. Ensure validator is running
2. On **Dashboard** tab, find "Send Heartbeat to Orchestrator"
3. Enter your wallet address (omni...)
4. Click **Send Heartbeat**
5. Status is sent to backend orchestrator at http://localhost:8000

### Viewing Logs

1. Navigate to **Logs** tab
2. Logs stream in real-time
3. Use **Auto-scroll** checkbox for automatic scrolling
4. Click **Refresh** to reload logs
5. Click **Clear** to clear the display

### Configuring Settings

1. Navigate to **Settings** tab
2. Update configuration:
   - **Validator Moniker**: Your validator display name
   - **Chain ID**: Network to connect to
   - **Orchestrator URL**: Backend API endpoint
   - **Heartbeat Interval**: How often to send status (in seconds)
   - **Auto-start**: Automatically start validator when app launches
3. Click **Save Settings**

## HTTP Bridge API

The app runs an HTTP server on **port 15000** with the following endpoints:

### GET /consensus-pubkey

Get consensus public key.

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

### GET /status

Get validator node status.

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
    "network": "omniphi-mainnet-1"
  }
}
```

### GET /logs?lines=100

Get recent logs.

**Response:**
```json
{
  "success": true,
  "logs": ["log line 1", "log line 2", ...]
}
```

### GET /health

Health check endpoint.

**Response:**
```json
{
  "success": true,
  "service": "omniphi-local-validator-bridge",
  "version": "1.0.0"
}
```

## File Locations

### Validator Data

**Linux/macOS:**
```
~/.omniphi/
├── config/
│   ├── config.toml
│   ├── app.toml
│   ├── genesis.json
│   └── priv_validator_key.json  # Consensus private key
└── data/
    └── priv_validator_state.json
```

**Windows:**
```
C:\Users\<username>\.omniphi\
├── config\
│   ├── config.toml
│   ├── app.toml
│   ├── genesis.json
│   └── priv_validator_key.json  # Consensus private key
└── data\
    └── priv_validator_state.json
```

### App Configuration

Stored via `electron-store`:
- **Windows**: `%APPDATA%\omniphi-local-validator\config.json`
- **macOS**: `~/Library/Application Support/omniphi-local-validator/config.json`
- **Linux**: `~/.config/omniphi-local-validator/config.json`

### Secure Key Storage

Consensus keys are also stored in OS keychain:
- **Windows**: Windows Credential Manager
- **macOS**: Keychain Access
- **Linux**: Secret Service API (GNOME Keyring, KDE Wallet)

## Security

### Key Management

- **Consensus private keys** are generated locally and NEVER leave your machine
- Keys are stored encrypted in OS keychain
- Backup exports are password-encrypted
- **Wallet keys** are never touched by this app (managed by your wallet)

### Network Security

- HTTP bridge only listens on localhost (127.0.0.1)
- No external access to sensitive operations
- All IPC communication uses contextBridge for security

### Best Practices

1. **Backup your consensus keys** regularly
2. **Use a strong password** for key exports
3. **Keep backups offline** and secure
4. **Don't share your private keys** with anyone
5. **Update the app** regularly for security patches

## Troubleshooting

### Validator Won't Start

**Check binary exists:**
```bash
ls -la bin/posd        # Linux/macOS
dir bin\posd.exe      # Windows
```

**Check binary permissions:**
```bash
chmod +x bin/posd     # Linux/macOS
```

**View error logs:**
- Navigate to Logs tab
- Look for initialization errors

### Port Already in Use

If port 26657 (RPC) or 26656 (P2P) is already in use:

1. Stop other blockchain nodes
2. Or change ports in `~/.omniphi/config/config.toml`:
   ```toml
   [rpc]
   laddr = "tcp://127.0.0.1:26657"

   [p2p]
   laddr = "tcp://0.0.0.0:26656"
   ```

### HTTP Bridge Port 15000 Conflict

If port 15000 is already in use, you'll see an error on startup.

**Find what's using the port:**
```bash
# Linux/macOS
lsof -i :15000

# Windows
netstat -ano | findstr :15000
```

### Can't Connect to Orchestrator

**Check orchestrator is running:**
```bash
curl http://localhost:8000/api/v1/health
```

**Update orchestrator URL in Settings tab**

### Node Not Syncing

- Check internet connection
- Verify genesis file matches network
- Check peer configuration
- View logs for connection errors

## Development Notes

### Project Structure

```
local-validator-app/
├── electron/
│   ├── main.js          # Main process
│   ├── preload.js       # Security bridge
│   ├── ipc-handlers.js  # IPC handlers (node management)
│   └── http-bridge.js   # HTTP API server
├── src/
│   ├── components/      # React components
│   ├── types/           # TypeScript types
│   ├── App.tsx          # Main app component
│   ├── App.css          # App styles
│   └── main.tsx         # React entry point
├── package.json
├── vite.config.ts       # Vite configuration
└── tsconfig.json        # TypeScript configuration
```

### Adding New Features

1. **New IPC Handler**: Add to `electron/ipc-handlers.js`
2. **Expose to Renderer**: Update `electron/preload.js`
3. **TypeScript Types**: Add to `src/types/index.ts`
4. **UI Component**: Create in `src/components/`

## Support

- **Issues**: https://github.com/omniphi/validator-orchestrator/issues
- **Documentation**: https://docs.omniphi.io
- **Discord**: https://discord.gg/omniphi

## License

MIT
