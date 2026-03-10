package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: fix_guard_genesis <genesis.json>")
		os.Exit(1)
	}
	path := os.Args[1]
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
		os.Exit(1)
	}

	var genesis map[string]interface{}
	if err := json.Unmarshal(data, &genesis); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	appState, ok := genesis["app_state"].(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Error: app_state not found")
		os.Exit(1)
	}

	// Set guard params for testnet
	appState["guard"] = map[string]interface{}{
		"delay_low_blocks":                    "10",
		"delay_med_blocks":                    "50",
		"delay_high_blocks":                   "200",
		"delay_critical_blocks":               "500",
		"visibility_window_blocks":            "100",
		"shock_absorber_window_blocks":        "50",
		"threshold_default_bps":               "5000",
		"threshold_high_bps":                  "6700",
		"threshold_critical_bps":              "7500",
		"treasury_throttle_enabled":           false,
		"treasury_max_outflow_bps_per_day":    "0",
		"enable_stability_checks":             false,
		"max_validator_churn_bps":             "0",
		"advisory_ai_enabled":                 false,
		"binding_ai_enabled":                  false,
		"ai_shadow_mode":                      false,
		"critical_requires_second_confirm":    false,
		"critical_second_confirm_window_blocks": "0",
		"extension_high_blocks":               "21600",
		"extension_critical_blocks":           "64800",
		"max_proposals_per_block":             "5",
		"max_queue_scan_depth":                "100",
		// timelock_integration_enabled omitted (proto3 default false, enable via gov proposal)
	}

	// Also fix gov params for testnet
	if gov, ok := appState["gov"].(map[string]interface{}); ok {
		if params, ok := gov["params"].(map[string]interface{}); ok {
			params["voting_period"] = "300s"
			params["expedited_voting_period"] = "60s"
			if minDep, ok := params["min_deposit"].([]interface{}); ok && len(minDep) > 0 {
				if dep, ok := minDep[0].(map[string]interface{}); ok {
					dep["amount"] = "10000000000"
					dep["denom"] = "omniphi"
				}
			}
			// Expedited min deposit must be > min deposit
			if expDep, ok := params["expedited_min_deposit"].([]interface{}); ok && len(expDep) > 0 {
				if dep, ok := expDep[0].(map[string]interface{}); ok {
					dep["amount"] = "50000000000"
					dep["denom"] = "omniphi"
				}
			}
		}
	}

	// Also fix staking params
	if staking, ok := appState["staking"].(map[string]interface{}); ok {
		if params, ok := staking["params"].(map[string]interface{}); ok {
			params["bond_denom"] = "omniphi"
			params["max_validators"] = float64(125)
		}
	}

	// Fix consensus block params
	if consensus, ok := genesis["consensus"].(map[string]interface{}); ok {
		if params, ok := consensus["params"].(map[string]interface{}); ok {
			if block, ok := params["block"].(map[string]interface{}); ok {
				block["max_gas"] = "60000000"
			}
		}
	}

	// Fix crisis constant fee
	if crisis, ok := appState["crisis"].(map[string]interface{}); ok {
		crisis["constant_fee"] = map[string]interface{}{
			"denom":  "omniphi",
			"amount": "1000000000000",
		}
	}

	// Fix feemarket treasury address (required to avoid consensus failure)
	// Use the validator address passed as 2nd arg, or read from keyring
	if feemarket, ok := appState["feemarket"].(map[string]interface{}); ok {
		treasuryAddr := ""
		if len(os.Args) >= 3 {
			treasuryAddr = os.Args[2]
		}
		if treasuryAddr != "" {
			feemarket["treasury_address"] = treasuryAddr
		}
	}

	// Fix bank denom metadata
	if bank, ok := appState["bank"].(map[string]interface{}); ok {
		bank["denom_metadata"] = []interface{}{
			map[string]interface{}{
				"description": "The native staking token of Omniphi",
				"denom_units": []interface{}{
					map[string]interface{}{"denom": "omniphi", "exponent": float64(0), "aliases": []interface{}{"microomni", "uomni"}},
					map[string]interface{}{"denom": "mOMNI", "exponent": float64(3), "aliases": []interface{}{"milliomni"}},
					map[string]interface{}{"denom": "OMNI", "exponent": float64(6), "aliases": []interface{}{}},
				},
				"base":    "omniphi",
				"display": "OMNI",
				"name":    "Omniphi",
				"symbol":  "OMNI",
			},
		}
	}

	out, err := json.MarshalIndent(genesis, "", " ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(path, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
		os.Exit(1)
	}

	fmt.Println("Genesis fixed successfully")
}
