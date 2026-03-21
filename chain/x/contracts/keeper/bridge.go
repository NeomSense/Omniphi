package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	sdk "github.com/cosmos/cosmos-sdk/types"
	storetypes "cosmossdk.io/store/types"

	"pos/x/contracts/types"
)

// ExportSchemasToDir writes all active contract schemas to JSON files in the
// specified directory. PoSeq nodes poll this directory to import schemas.
//
// Each schema is written as <schema_id>.schema.json.
func (k Keeper) ExportSchemasToDir(ctx context.Context, exportDir string) error {
	if exportDir == "" {
		return nil // no export configured
	}

	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return fmt.Errorf("failed to create schema export dir: %w", err)
	}

	kvStore := k.storeService.OpenKVStore(ctx)

	// Iterate all schemas at prefix 0x02
	iter, err := kvStore.Iterator(types.KeyPrefixSchema, storetypes.PrefixEndBytes(types.KeyPrefixSchema))
	if err != nil {
		return fmt.Errorf("failed to create schema iterator: %w", err)
	}
	defer iter.Close()

	exported := 0
	for ; iter.Valid(); iter.Next() {
		var schema types.ContractSchema
		if err := json.Unmarshal(iter.Value(), &schema); err != nil {
			k.logger.Error("failed to unmarshal schema during export", "error", err)
			continue
		}

		if schema.Status != types.ContractStatusActive {
			continue
		}

		filename := filepath.Join(exportDir, schema.SchemaID+".schema.json")

		// Skip if already exported (idempotent)
		if _, err := os.Stat(filename); err == nil {
			continue
		}

		bz, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			k.logger.Error("failed to marshal schema for export", "error", err, "schema_id", schema.SchemaID)
			continue
		}

		if err := os.WriteFile(filename, bz, 0o644); err != nil {
			k.logger.Error("failed to write schema export", "error", err, "path", filename)
			continue
		}

		exported++
	}

	if exported > 0 {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		k.logger.Info("exported contract schemas",
			"count", exported,
			"dir", exportDir,
			"height", sdkCtx.BlockHeight(),
		)
	}

	return nil
}

// ExportSchemaWithWasm writes both the schema JSON and the Wasm bytecode
// for a single schema. Used after deployment to immediately make the
// contract available to PoSeq nodes.
func (k Keeper) ExportSchemaWithWasm(ctx context.Context, schemaID [32]byte, exportDir string) error {
	if exportDir == "" {
		return nil
	}

	schema, exists := k.GetSchema(ctx, schemaID)
	if !exists {
		return fmt.Errorf("schema not found")
	}

	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return err
	}

	// Export schema JSON
	schemaPath := filepath.Join(exportDir, schema.SchemaID+".schema.json")
	bz, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(schemaPath, bz, 0o644); err != nil {
		return err
	}

	// Export Wasm bytecode
	wasm, exists := k.GetWasm(ctx, schemaID)
	if exists {
		wasmPath := filepath.Join(exportDir, schema.SchemaID+".wasm")
		if err := os.WriteFile(wasmPath, wasm, 0o644); err != nil {
			return err
		}
	}

	return nil
}
