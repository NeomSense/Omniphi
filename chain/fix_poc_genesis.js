#!/usr/bin/env node
const fs = require('fs');
const path = require('path');
const os = require('os');

const homeDir = path.join(os.homedir(), '.pos');
const genesisPath = path.join(homeDir, 'config', 'genesis.json');

console.log(`Reading genesis from: ${genesisPath}`);

try {
    const genesis = JSON.parse(fs.readFileSync(genesisPath, 'utf8'));

    console.log('Current PoC state:', JSON.stringify(genesis.app_state.poc, null, 2));

    // Update PoC module genesis
    genesis.app_state.poc = {
        params: {
            quorum_pct: "0.670000000000000000",
            base_reward_unit: "1000",
            inflation_share: "0.000000000000000000",
            max_per_block: 10,
            tiers: [
                { name: "bronze", cutoff: "1000" },
                { name: "silver", cutoff: "10000" },
                { name: "gold", cutoff: "100000" }
            ],
            reward_denom: "omniphi"
        },
        contributions: [],
        credits: [],
        next_contribution_id: "1"
    };

    // Write back
    fs.writeFileSync(genesisPath, JSON.stringify(genesis, null, 2));

    console.log('✓ PoC genesis updated successfully!');
    console.log('New PoC state:', JSON.stringify(genesis.app_state.poc, null, 2));

} catch (err) {
    console.error(`Error: ${err.message}`);
    process.exit(1);
}
