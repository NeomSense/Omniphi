package cmd

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/keys"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/spf13/cobra"

	"pos/pkg/address"
)

// CustomKeysCommand returns the keys command with 1x address display
func CustomKeysCommand() *cobra.Command {
	cmd := keys.Commands()

	// Add a custom subcommand to show 1x format
	cmd.AddCommand(Show1xAddressCmd())

	return cmd
}

// Show1xAddressCmd returns a command to show an address in 1x format
func Show1xAddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-1x [name]",
		Short: "Show key address in Omniphi 1x format",
		Long: `Display the address for a key in Omniphi's branded 1x format.

This command shows:
- Bech32 format (omni1...) - Internal SDK format
- Omniphi display format (1x...) - User-facing brand format
- Ethereum format (0x...) - EVM-compatible format

All three formats represent the same 20-byte Ethereum-compatible address.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			keyName := args[0]
			keyInfo, err := clientCtx.Keyring.Key(keyName)
			if err != nil {
				return fmt.Errorf("key not found: %w", err)
			}

			addr, err := keyInfo.GetAddress()
			if err != nil {
				return fmt.Errorf("failed to get address: %w", err)
			}

			pubKey, err := keyInfo.GetPubKey()
			if err != nil {
				return fmt.Errorf("failed to get public key: %w", err)
			}

			// Display all formats
			fmt.Printf("\n")
			fmt.Printf("Key Name:        %s\n", keyName)
			fmt.Printf("Key Type:        %s\n", keyInfo.GetType())
			fmt.Printf("Public Key Type: %s\n", pubKey.Type())
			fmt.Printf("\n")
			fmt.Printf("📍 Address Formats:\n")
			fmt.Printf("   Omniphi (1x):  %s\n", address.AccAddressToOmni(addr))
			fmt.Printf("   Ethereum (0x): %s\n", address.AccAddressToEth(addr))
			fmt.Printf("   Bech32:        %s\n", addr.String())
			fmt.Printf("\n")
			fmt.Printf("💡 Tip: Use '1x...' format for user-facing displays\n")
			fmt.Printf("💡 Use 'posd address convert <address> --to 1x' to convert any address\n")
			fmt.Printf("\n")

			return nil
		},
	}

	return cmd
}

// GetKeyAlgorithm returns the algorithm type for a public key
func GetKeyAlgorithm(pubKey cryptotypes.PubKey) string {
	switch pubKey.Type() {
	case "eth_secp256k1":
		return "Ethereum secp256k1 (EVM-compatible)"
	case "secp256k1":
		return "Cosmos secp256k1"
	case "ed25519":
		return "Ed25519"
	default:
		return pubKey.Type()
	}
}
