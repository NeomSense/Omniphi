#!/bin/bash
# Fix timelock params in genesis.json

set -e

echo "Fixing timelock params in genesis.json..."

# Stop the node
echo "Stopping posd..."
sudo systemctl stop posd

# Backup genesis
echo "Backing up genesis..."
cp ~/.pos/config/genesis.json ~/.pos/config/genesis.json.backup.$(date +%s)

# Update timelock params using jq
echo "Updating timelock params..."
cat ~/.pos/config/genesis.json | jq '.app_state.timelock.params = {
  "min_delay_seconds": "86400",
  "max_delay_seconds": "1209600",
  "grace_period_seconds": "604800",
  "emergency_delay_seconds": "3600",
  "guardian": ""
}' > ~/.pos/config/genesis.json.new

# Validate JSON
if jq empty ~/.pos/config/genesis.json.new 2>/dev/null; then
    echo "JSON is valid, replacing genesis..."
    mv ~/.pos/config/genesis.json.new ~/.pos/config/genesis.json
    echo "Genesis updated successfully!"
else
    echo "ERROR: Generated JSON is invalid!"
    rm ~/.pos/config/genesis.json.new
    exit 1
fi

# Show the updated params
echo ""
echo "Updated timelock params:"
jq '.app_state.timelock.params' ~/.pos/config/genesis.json

# Start the node
echo ""
echo "Starting posd..."
sudo systemctl start posd

echo ""
echo "Done! Monitor logs with: sudo journalctl -u posd -f"
