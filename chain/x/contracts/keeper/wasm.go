package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// ConstraintResult is the output of a constraint validator invocation.
type ConstraintResult struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason"`
}

// WasmValidator manages compiled Wasm modules and invokes constraint validators.
type WasmValidator struct {
	mu       sync.RWMutex
	runtime  wazero.Runtime
	compiled map[[32]byte]wazero.CompiledModule
	gasLimit uint64
}

// NewWasmValidator creates a validator backed by the wazero RISC-V runtime.
func NewWasmValidator(gasLimit uint64) *WasmValidator {
	ctx := context.Background()
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true))

	return &WasmValidator{
		runtime:  rt,
		compiled: make(map[[32]byte]wazero.CompiledModule),
		gasLimit: gasLimit,
	}
}

// CompileAndCache compiles Wasm bytecode and caches it by schema ID.
// Returns the SHA256 hash of the bytecode for verification.
func (wv *WasmValidator) CompileAndCache(ctx context.Context, schemaID [32]byte, bytecode []byte) (string, error) {
	wv.mu.Lock()
	defer wv.mu.Unlock()

	compiled, err := wv.runtime.CompileModule(ctx, bytecode)
	if err != nil {
		return "", fmt.Errorf("failed to compile wasm: %w", err)
	}

	wv.compiled[schemaID] = compiled
	hash := sha256.Sum256(bytecode)
	return hex.EncodeToString(hash[:]), nil
}

// Validate invokes the constraint validator for a schema.
//
// The Wasm module must export a function `validate` with signature:
//   (proposed_ptr, proposed_len, current_ptr, current_len, params_ptr, params_len, epoch i64, sender_ptr) -> i32
//
// Returns 0 for valid, non-zero for invalid. The reason is read from exported
// memory at a convention offset (or a default message is used).
func (wv *WasmValidator) Validate(
	ctx context.Context,
	schemaID [32]byte,
	proposedState []byte,
	currentState []byte,
	intentParams []byte,
	epoch uint64,
	sender [32]byte,
) (*ConstraintResult, error) {
	wv.mu.RLock()
	compiled, ok := wv.compiled[schemaID]
	wv.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no compiled module for schema %s", hex.EncodeToString(schemaID[:]))
	}

	// Instantiate a fresh module instance for this invocation (sandbox isolation)
	mod, err := wv.runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().
		WithName(hex.EncodeToString(schemaID[:8])))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate wasm: %w", err)
	}
	defer mod.Close(ctx)

	// Write input data to Wasm memory
	memory := mod.Memory()
	if memory == nil {
		return nil, fmt.Errorf("wasm module has no exported memory")
	}

	// Layout: proposed | current | params | sender at sequential offsets
	offset := uint32(0)

	proposedPtr := offset
	if !memory.Write(proposedPtr, proposedState) {
		return nil, fmt.Errorf("failed to write proposed_state to wasm memory")
	}
	offset += uint32(len(proposedState))

	currentPtr := offset
	if !memory.Write(currentPtr, currentState) {
		return nil, fmt.Errorf("failed to write current_state to wasm memory")
	}
	offset += uint32(len(currentState))

	paramsPtr := offset
	if !memory.Write(paramsPtr, intentParams) {
		return nil, fmt.Errorf("failed to write intent_params to wasm memory")
	}
	offset += uint32(len(intentParams))

	senderPtr := offset
	if !memory.Write(senderPtr, sender[:]) {
		return nil, fmt.Errorf("failed to write sender to wasm memory")
	}

	// Call the exported `validate` function
	validateFn := mod.ExportedFunction("validate")
	if validateFn == nil {
		// If no validate function, treat as unconditionally valid
		// (schema-only contracts that delegate all validation to solvers)
		return &ConstraintResult{Valid: true, Reason: ""}, nil
	}

	results, err := validateFn.Call(ctx,
		api.EncodeU32(proposedPtr), api.EncodeU32(uint32(len(proposedState))),
		api.EncodeU32(currentPtr), api.EncodeU32(uint32(len(currentState))),
		api.EncodeU32(paramsPtr), api.EncodeU32(uint32(len(intentParams))),
		uint64(epoch),
		api.EncodeU32(senderPtr),
	)
	if err != nil {
		return &ConstraintResult{
			Valid:  false,
			Reason: fmt.Sprintf("validator execution error: %v", err),
		}, nil
	}

	if len(results) == 0 {
		return &ConstraintResult{Valid: true, Reason: ""}, nil
	}

	resultCode := api.DecodeI32(results[0])
	if resultCode == 0 {
		return &ConstraintResult{Valid: true, Reason: ""}, nil
	}

	return &ConstraintResult{
		Valid:  false,
		Reason: fmt.Sprintf("constraint rejected (code %d)", resultCode),
	}, nil
}

// IsCompiled checks if a schema's Wasm is already compiled and cached.
func (wv *WasmValidator) IsCompiled(schemaID [32]byte) bool {
	wv.mu.RLock()
	defer wv.mu.RUnlock()
	_, ok := wv.compiled[schemaID]
	return ok
}

// Close releases all wazero resources.
func (wv *WasmValidator) Close(ctx context.Context) error {
	return wv.runtime.Close(ctx)
}

// ValidateJSON is a convenience wrapper that accepts JSON params and returns JSON.
func (wv *WasmValidator) ValidateJSON(
	ctx context.Context,
	schemaID [32]byte,
	proposedState json.RawMessage,
	currentState json.RawMessage,
	intentParams json.RawMessage,
	epoch uint64,
	sender [32]byte,
) (*ConstraintResult, error) {
	return wv.Validate(ctx, schemaID, proposedState, currentState, intentParams, epoch, sender)
}
