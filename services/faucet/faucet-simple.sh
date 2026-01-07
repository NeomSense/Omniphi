#!/bin/bash
# Omniphi Simple Faucet Script
# Runs on the VPS alongside the validator node

set -e

# Configuration
FAUCET_KEY="faucet"
CHAIN_ID="omniphi-testnet-1"
DENOM="uomni"
DISTRIBUTION_AMOUNT="10000000000"  # 10,000 OMNI
NODE_HOME="$HOME/.pos"
KEYRING_BACKEND="test"
GAS_PRICES="0.025uomni"
FAUCET_LOG="/var/log/omniphi-faucet.log"

# Rate limiting file
RATE_LIMIT_FILE="/tmp/faucet_ratelimit.txt"
COOLDOWN_HOURS=24

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$FAUCET_LOG"
}

check_rate_limit() {
    local address="$1"
    local current_time=$(date +%s)
    local cooldown_seconds=$((COOLDOWN_HOURS * 3600))

    if [ -f "$RATE_LIMIT_FILE" ]; then
        local last_request=$(grep "^$address:" "$RATE_LIMIT_FILE" | cut -d: -f2)
        if [ -n "$last_request" ]; then
            local elapsed=$((current_time - last_request))
            if [ $elapsed -lt $cooldown_seconds ]; then
                local remaining=$(((cooldown_seconds - elapsed) / 60))
                echo "Rate limited. Please wait $remaining minutes before requesting again."
                return 1
            fi
        fi
    fi
    return 0
}

update_rate_limit() {
    local address="$1"
    local current_time=$(date +%s)

    # Remove old entry if exists
    if [ -f "$RATE_LIMIT_FILE" ]; then
        grep -v "^$address:" "$RATE_LIMIT_FILE" > "${RATE_LIMIT_FILE}.tmp" || true
        mv "${RATE_LIMIT_FILE}.tmp" "$RATE_LIMIT_FILE"
    fi

    # Add new entry
    echo "$address:$current_time" >> "$RATE_LIMIT_FILE"
}

send_tokens() {
    local recipient="$1"

    # Validate address format
    if [[ ! "$recipient" =~ ^omni1[a-z0-9]{38}$ ]]; then
        echo "Invalid address format. Must be omni1..."
        return 1
    fi

    # Check rate limit
    if ! check_rate_limit "$recipient"; then
        return 1
    fi

    log "Sending $DISTRIBUTION_AMOUNT $DENOM to $recipient"

    # Send tokens
    result=$(posd tx bank send "$FAUCET_KEY" "$recipient" "${DISTRIBUTION_AMOUNT}${DENOM}" \
        --chain-id "$CHAIN_ID" \
        --keyring-backend "$KEYRING_BACKEND" \
        --home "$NODE_HOME" \
        --gas-prices "$GAS_PRICES" \
        --gas auto \
        --gas-adjustment 1.5 \
        --yes \
        --output json 2>&1)

    if echo "$result" | grep -q '"code":0'; then
        tx_hash=$(echo "$result" | jq -r '.txhash')
        update_rate_limit "$recipient"
        log "SUCCESS: Sent to $recipient (tx: $tx_hash)"
        echo "Success! Transaction hash: $tx_hash"
        echo "Amount: 10,000 OMNI"
        return 0
    else
        error=$(echo "$result" | jq -r '.raw_log // .error // "Unknown error"')
        log "FAILED: Could not send to $recipient - $error"
        echo "Failed to send tokens: $error"
        return 1
    fi
}

get_faucet_balance() {
    local faucet_addr=$(posd keys show "$FAUCET_KEY" --keyring-backend "$KEYRING_BACKEND" --home "$NODE_HOME" -a)
    local balance=$(posd query bank balances "$faucet_addr" --home "$NODE_HOME" --output json 2>/dev/null | jq -r '.balances[] | select(.denom=="uomni") | .amount')

    if [ -n "$balance" ]; then
        local omni=$((balance / 1000000))
        echo "Faucet balance: $omni OMNI"
    else
        echo "Faucet balance: 0 OMNI"
    fi
}

show_help() {
    echo "Omniphi Testnet Faucet"
    echo ""
    echo "Usage: $0 <command> [args]"
    echo ""
    echo "Commands:"
    echo "  send <address>  - Send test tokens to an address"
    echo "  balance         - Show faucet balance"
    echo "  setup           - Set up faucet key from mnemonic"
    echo "  help            - Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 send omni1abc..."
    echo "  $0 balance"
}

setup_faucet_key() {
    echo "Setting up faucet key..."
    echo "Enter the faucet mnemonic (24 words):"
    read -r mnemonic

    echo "$mnemonic" | posd keys add "$FAUCET_KEY" \
        --keyring-backend "$KEYRING_BACKEND" \
        --home "$NODE_HOME" \
        --recover \
        --index 0

    echo "Faucet key created!"
    posd keys show "$FAUCET_KEY" --keyring-backend "$KEYRING_BACKEND" --home "$NODE_HOME"
}

# Main
case "${1:-help}" in
    send)
        if [ -z "$2" ]; then
            echo "Usage: $0 send <address>"
            exit 1
        fi
        send_tokens "$2"
        ;;
    balance)
        get_faucet_balance
        ;;
    setup)
        setup_faucet_key
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo "Unknown command: $1"
        show_help
        exit 1
        ;;
esac
