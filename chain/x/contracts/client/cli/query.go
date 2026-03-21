package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// GetQueryCmd returns the query commands for the contracts module.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contracts",
		Short: "Intent Contract query commands",
	}

	cmd.AddCommand(
		CmdQuerySchema(),
		CmdQuerySchemas(),
		CmdQueryInstance(),
	)

	return cmd
}

// CmdQuerySchema queries a single contract schema by ID.
func CmdQuerySchema() *cobra.Command {
	return &cobra.Command{
		Use:   "schema [schema-id]",
		Short: "Query a contract schema by its hex ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Query via gRPC when proto types are wired.
			// For now, print usage hint.
			fmt.Printf("Query schema: %s\n", args[0])
			fmt.Println("(gRPC query will be available after proto wiring)")
			return nil
		},
	}
}

// CmdQuerySchemas lists all deployed contract schemas.
func CmdQuerySchemas() *cobra.Command {
	return &cobra.Command{
		Use:   "schemas",
		Short: "List all deployed contract schemas",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Listing all schemas...")
			fmt.Println("(gRPC query will be available after proto wiring)")
			return nil
		},
	}
}

// CmdQueryInstance queries a contract instance by ID.
func CmdQueryInstance() *cobra.Command {
	return &cobra.Command{
		Use:   "instance [instance-id]",
		Short: "Query a contract instance by its numeric ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Query instance: %s\n", args[0])
			fmt.Println("(gRPC query will be available after proto wiring)")
			return nil
		},
	}
}
