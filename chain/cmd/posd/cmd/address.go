package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"pos/pkg/address"
)

// AddressCmd returns the address utility commands
func AddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address",
		Short: "Address format utilities for Omniphi",
		Long: `Utilities for working with Omniphi addresses.

Omniphi uses Ethereum-style addresses internally (0x...) but displays them
with the 1x... prefix for branding. This command helps convert between formats
and provides information about addresses.`,
	}

	cmd.AddCommand(
		AddressInfoCmd(),
		AddressConvertCmd(),
		AddressValidateCmd(),
	)

	return cmd
}

// AddressInfoCmd returns the address-info subcommand
func AddressInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info [address]",
		Short: "Get detailed information about an address",
		Long: `Analyze an address and return detailed information including:
- Original format
- Display format (1x...)
- Ethereum format (0x...)
- Address type
- Validation status
- Byte length

Supports both 0x and 1x address formats.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addressStr := args[0]

			info := address.GetAddressInfo(addressStr)

			// Format as JSON
			jsonOutput, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}

			fmt.Println(string(jsonOutput))
			return nil
		},
	}

	return cmd
}

// AddressConvertCmd returns the address-convert subcommand
func AddressConvertCmd() *cobra.Command {
	var toFormat string

	cmd := &cobra.Command{
		Use:   "convert [address]",
		Short: "Convert address between 0x and 1x formats",
		Long: `Convert an address between different formats:
- 0x format (Ethereum standard)
- 1x format (Omniphi branding)

Examples:
  posd address convert 0x1234... --to 1x
  posd address convert 1x1234... --to 0x`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addressStr := args[0]

			// Validate address first
			if !address.IsValidOmniAddress(addressStr) {
				return fmt.Errorf("invalid address format: %s", addressStr)
			}

			var result string
			switch toFormat {
			case "1x", "omni", "omniphi":
				result = address.FormatOmniAddress(addressStr)
			case "0x", "eth", "ethereum":
				result = address.NormalizeToEthAddress(addressStr)
			default:
				return fmt.Errorf("invalid format: %s (use '0x' or '1x')", toFormat)
			}

			output := map[string]string{
				"input":  addressStr,
				"output": result,
				"format": toFormat,
			}

			jsonOutput, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}

			fmt.Println(string(jsonOutput))
			return nil
		},
	}

	cmd.Flags().StringVar(&toFormat, "to", "1x", "Target format: '0x' (Ethereum) or '1x' (Omniphi)")

	return cmd
}

// AddressValidateCmd returns the address-validate subcommand
func AddressValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [address]",
		Short: "Validate an address format",
		Long: `Validate whether an address string is in a valid format.

Accepts both 0x and 1x formats.

Exit codes:
  0 - Address is valid
  1 - Address is invalid`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addressStr := args[0]

			valid := address.IsValidOmniAddress(addressStr)

			output := map[string]interface{}{
				"address": addressStr,
				"valid":   valid,
			}

			if valid {
				// Parse to get more details
				parsed, err := address.ParseOmniAddress(addressStr)
				if err == nil {
					output["length"] = len(parsed.Bytes())
					output["display"] = address.AccAddressToOmni(parsed)
					output["ethereum"] = address.AccAddressToEth(parsed)
				}
			}

			jsonOutput, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}

			fmt.Println(string(jsonOutput))

			if !valid {
				return fmt.Errorf("address is invalid")
			}

			return nil
		},
	}

	return cmd
}
