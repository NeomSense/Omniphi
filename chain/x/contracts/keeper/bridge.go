package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	storetypes "cosmossdk.io/store/types"

	"pos/x/contracts/types"
)

// BridgeValidationRequest is the JSON schema read from the validation_requests dir.
type BridgeValidationRequest struct {
	RequestID     string `json:"request_id"`
	SchemaID      string `json:"schema_id"`
	Method        string `json:"method"`
	CurrentState  string `json:"current_state"`
	ProposedState string `json:"proposed_state"`
	Sender        string `json:"sender"`
	Epoch         uint64 `json:"epoch"`
}

// BridgeValidationResponse is the JSON schema written to the validation_responses dir.
type BridgeValidationResponse struct {
	RequestID string `json:"request_id"`
	Valid     bool   `json:"valid"`
	Reason    string `json:"reason"`
	GasUsed   uint64 `json:"gas_used"`
}

// ProcessValidationRequests polls the validation_requests directory, validates each
// request against stored schema + Wasm, and writes responses. Called from EndBlocker.
func (k Keeper) ProcessValidationRequests(ctx context.Context) int {
	if k.bridgeDir == "" {
		return 0
	}

	requestDir := filepath.Join(k.bridgeDir, "validation_requests")
	responseDir := filepath.Join(k.bridgeDir, "validation_responses")

	if err := os.MkdirAll(responseDir, 0o755); err != nil {
		k.logger.Error("contracts.bridge: failed to create response dir", "error", err)
		return 0
	}

	entries, err := os.ReadDir(requestDir)
	if err != nil {
		return 0
	}

	processed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		reqPath := filepath.Join(requestDir, entry.Name())
		data, err := os.ReadFile(reqPath)
		if err != nil {
			continue
		}

		var req BridgeValidationRequest
		if err := json.Unmarshal(data, &req); err != nil {
			_ = os.Remove(reqPath)
			continue
		}

		resp := k.validateConstraintBridge(ctx, req)

		respData, _ := json.Marshal(resp)
		respPath := filepath.Join(responseDir, entry.Name())
		if err := os.WriteFile(respPath, respData, 0o644); err != nil {
			continue
		}

		_ = os.Remove(reqPath)
		processed++

		k.logger.Info("contracts.bridge: validated",
			"request_id", req.RequestID,
			"method", req.Method,
			"valid", resp.Valid,
		)
	}

	if processed > 0 {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"contract_validations_processed",
			sdk.NewAttribute("count", fmt.Sprintf("%d", processed)),
		))
	}

	return processed
}

// validateConstraintBridge validates a constraint against stored schema and Wasm.
func (k Keeper) validateConstraintBridge(ctx context.Context, req BridgeValidationRequest) BridgeValidationResponse {
	schemaBytes, err := hex.DecodeString(req.SchemaID)
	if err != nil || len(schemaBytes) != 32 {
		return BridgeValidationResponse{RequestID: req.RequestID, Valid: false, Reason: "invalid schema_id"}
	}
	var schemaID [32]byte
	copy(schemaID[:], schemaBytes)

	schema, exists := k.GetSchema(ctx, schemaID)
	if !exists {
		return BridgeValidationResponse{RequestID: req.RequestID, Valid: false, Reason: "schema not found"}
	}
	if schema.Status != "ACTIVE" {
		return BridgeValidationResponse{RequestID: req.RequestID, Valid: false, Reason: "schema not active"}
	}

	// Validate method
	methodOK := false
	for _, is := range schema.IntentSchemas {
		if is.Method == req.Method {
			methodOK = true
			break
		}
	}
	if !methodOK {
		return BridgeValidationResponse{RequestID: req.RequestID, Valid: false, Reason: "method not in schema", GasUsed: 100}
	}

	// Validate state size
	proposedState, err := hex.DecodeString(req.ProposedState)
	if err != nil {
		return BridgeValidationResponse{RequestID: req.RequestID, Valid: false, Reason: "invalid proposed_state hex"}
	}
	if uint64(len(proposedState)) > schema.MaxStateBytes {
		return BridgeValidationResponse{RequestID: req.RequestID, Valid: false, Reason: "state exceeds max", GasUsed: 100}
	}

	// Verify Wasm
	wasmBytes, wasmExists := k.GetWasm(ctx, schemaID)
	if !wasmExists || len(wasmBytes) == 0 {
		return BridgeValidationResponse{RequestID: req.RequestID, Valid: true, Reason: "structural only (no wasm)", GasUsed: 200}
	}
	wasmHash := sha256.Sum256(wasmBytes)
	if hex.EncodeToString(wasmHash[:]) != schema.ValidatorHash {
		return BridgeValidationResponse{RequestID: req.RequestID, Valid: false, Reason: "wasm hash mismatch"}
	}

	// Invoke Wasm if validator attached
	if k.wasmValidator != nil {
		if !k.wasmValidator.IsCompiled(schemaID) {
			if _, err := k.wasmValidator.CompileAndCache(ctx, schemaID, wasmBytes); err != nil {
				return BridgeValidationResponse{RequestID: req.RequestID, Valid: false, Reason: fmt.Sprintf("compile error: %v", err)}
			}
		}
		currentState, _ := hex.DecodeString(req.CurrentState)
		senderBz, _ := hex.DecodeString(req.Sender)
		var sender [32]byte
		if len(senderBz) == 32 {
			copy(sender[:], senderBz)
		}
		result, err := k.wasmValidator.Validate(ctx, schemaID, proposedState, currentState, nil, req.Epoch, sender)
		if err != nil {
			return BridgeValidationResponse{RequestID: req.RequestID, Valid: false, Reason: fmt.Sprintf("wasm error: %v", err), GasUsed: 500}
		}
		gas := uint64(500) + uint64(len(proposedState))
		if gas > schema.MaxGasPerCall {
			gas = schema.MaxGasPerCall
		}
		return BridgeValidationResponse{RequestID: req.RequestID, Valid: result.Valid, Reason: result.Reason, GasUsed: gas}
	}

	gas := uint64(500) + uint64(len(proposedState))
	if gas > schema.MaxGasPerCall {
		gas = schema.MaxGasPerCall
	}
	return BridgeValidationResponse{RequestID: req.RequestID, Valid: true, Reason: "validated: structural + wasm hash", GasUsed: gas}
}

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
