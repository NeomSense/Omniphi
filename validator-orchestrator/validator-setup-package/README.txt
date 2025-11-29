============================================
  OMNIPHI TESTNET VALIDATOR SETUP
============================================

Welcome to the Omniphi Testnet! This package contains
everything you need to run a validator node.

============================================
  REQUIREMENTS
============================================

- Windows 10/11 (64-bit)
- At least 4GB RAM
- 20GB free disk space
- Stable internet connection
- curl installed (comes with Windows 10+)

============================================
  QUICK START (RECOMMENDED)
============================================

1. Extract this folder to any location
2. Double-click "setup-validator.bat"
3. Enter your validator name when prompted
4. Wait for setup to complete
5. Run the generated start script

That's it! Your validator will start syncing with the network.

============================================
  MANUAL SETUP
============================================

If you prefer manual setup:

1. Create validator home directory:
   mkdir %USERPROFILE%\.omniphi-validator

2. Copy posd.exe to:
   %USERPROFILE%\.omniphi-validator\bin\posd.exe

3. Initialize the node:
   posd.exe init YOUR_NAME --chain-id omniphi-testnet-1 --home %USERPROFILE%\.omniphi-validator

4. Download genesis file:
   curl -s http://46.202.179.182:26657/genesis > genesis_response.json
   (Extract the "result.genesis" part to config/genesis.json)

5. Edit config/config.toml and set:
   persistent_peers = "66a136f96b171ecb7b4b0bc42062fde959623b4e@46.202.179.182:26656"

6. Start the validator:
   posd.exe start --home %USERPROFILE%\.omniphi-validator

============================================
  NETWORK INFORMATION
============================================

Chain ID:        omniphi-testnet-1
VPS Node ID:     66a136f96b171ecb7b4b0bc42062fde959623b4e
VPS IP:          46.202.179.182
P2P Port:        26656
RPC Port:        26657

Persistent Peer: 66a136f96b171ecb7b4b0bc42062fde959623b4e@46.202.179.182:26656

============================================
  USEFUL COMMANDS
============================================

Check node status:
  curl http://127.0.0.1:26657/status

Check connected peers:
  curl http://127.0.0.1:26657/net_info

Check VPS status:
  curl http://46.202.179.182:26657/status

View logs (in PowerShell):
  Get-Content %USERPROFILE%\.omniphi-validator\logs\*.log -Tail 50 -Wait

============================================
  TROUBLESHOOTING
============================================

Q: "AppHash mismatch" error
A: You may have a different posd.exe version. Use the exact
   same binary as provided in this package.

Q: "Connection refused" when checking status
A: The node may still be starting. Wait 30 seconds and try again.

Q: Node not syncing (stuck at block 0)
A: Check your firewall allows outbound connections on port 26656.
   Also verify the persistent_peers setting in config.toml.

Q: "Genesis file not found"
A: Run the setup script again or manually download genesis
   from http://46.202.179.182:26657/genesis

============================================
  SUPPORT
============================================

For help, contact the Omniphi team or check the GitHub repository.

============================================
  FILES IN THIS PACKAGE
============================================

- setup-validator.bat  : Automated setup script (run this first)
- posd.exe             : Omniphi blockchain binary
- README.txt           : This file

============================================
