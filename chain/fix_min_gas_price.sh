#!/bin/bash
# Fix minimum-gas-prices in all posd config files

echo "==================================="
echo "Fixing minimum-gas-prices for posd"
echo "==================================="
echo ""

# Function to fix a config file
fix_config() {
    local config_file=$1

    if [ ! -f "$config_file" ]; then
        echo "⚠️  Config not found: $config_file"
        return
    fi

    # Check current value
    current_value=$(grep "^minimum-gas-prices" "$config_file" | cut -d'"' -f2)

    if [ -z "$current_value" ]; then
        echo "✅ Fixing: $config_file"
        echo "   Current: (empty)"
        echo "   New:     0.05uomni"
        sed -i 's/^minimum-gas-prices = ""/minimum-gas-prices = "0.05uomni"/' "$config_file"
    else
        echo "ℹ️  Already set: $config_file"
        echo "   Value: $current_value"

        # If it's using old denom, update it
        if [[ "$current_value" == *"omniphi"* ]]; then
            echo "   ⚠️  Using old denom, updating to 'uomni'"
            sed -i 's/minimum-gas-prices = ".*"/minimum-gas-prices = "0.05uomni"/' "$config_file"
            echo "   ✅ Updated to: 0.05uomni"
        fi
    fi
    echo ""
}

# Find and fix all posd config files
echo "Searching for posd configurations..."
echo ""

# Check common locations
for home_dir in ~ /home/funmachine /home/$USER; do
    for chain_dir in .posd .pos .pos-new .pos-temp; do
        config_file="$home_dir/$chain_dir/config/app.toml"
        if [ -f "$config_file" ]; then
            fix_config "$config_file"
        fi
    done
done

echo "==================================="
echo "✅ Fix complete!"
echo "==================================="
echo ""
echo "Now you can start your chain with:"
echo "  posd start"
echo ""
echo "Or with explicit flag:"
echo "  posd start --minimum-gas-prices=0.05uomni"
echo ""
