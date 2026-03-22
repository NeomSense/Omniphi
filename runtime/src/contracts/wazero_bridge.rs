//! Wazero Validation Bridge
//!
//! Connects the Rust settlement engine to the Go chain's wazero constraint
//! validator via a file-based exchange. The Go chain writes validation results
//! to a shared directory; the Rust side reads them during plan validation.
//!
//! Flow:
//! 1. Solver submits plan with `proposed_state` in metadata
//! 2. Rust PlanValidator writes a validation request to the bridge dir
//! 3. Go chain's EndBlocker polls the request dir, invokes wazero, writes result
//! 4. Rust reads the result file and caches it in ValidationCache
//! 5. Settlement engine checks cache before applying ContractStateTransition
//!
//! This is a temporary bridge until native FFI or gRPC is available.

use crate::contracts::validation_cache::{CachedValidation, ValidationCache, ValidationCacheKey, state_hash};
use crate::contracts::validator::{ConstraintResult, ConstraintValidatorBridge, ValidationContext};
use crate::objects::base::SchemaId;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::path::{Path, PathBuf};

/// A validation request written to the bridge directory.
#[derive(Debug, Serialize, Deserialize)]
pub struct ValidationRequest {
    pub request_id: String,
    pub schema_id: String, // hex
    pub method: String,
    pub current_state: String,  // hex-encoded bytes
    pub proposed_state: String, // hex-encoded bytes
    pub sender: String,         // hex-encoded 32 bytes
    pub epoch: u64,
}

/// A validation response read from the bridge directory.
#[derive(Debug, Serialize, Deserialize)]
pub struct ValidationResponse {
    pub request_id: String,
    pub valid: bool,
    pub reason: String,
    pub gas_used: u64,
}

/// File-based bridge to the Go chain's wazero validator.
pub struct WazeroBridge {
    /// Directory where validation requests are written.
    request_dir: PathBuf,
    /// Directory where validation responses are read.
    response_dir: PathBuf,
    /// Local validation cache.
    pub cache: ValidationCache,
}

impl WazeroBridge {
    pub fn new(bridge_dir: &Path) -> Self {
        let request_dir = bridge_dir.join("validation_requests");
        let response_dir = bridge_dir.join("validation_responses");

        // Create directories if they don't exist
        let _ = std::fs::create_dir_all(&request_dir);
        let _ = std::fs::create_dir_all(&response_dir);

        WazeroBridge {
            request_dir,
            response_dir,
            cache: ValidationCache::new(20, 10_000), // 20 epoch TTL, 10K entries max
        }
    }

    /// Write a validation request for the Go chain to process.
    pub fn submit_request(&self, req: &ValidationRequest) -> Result<(), String> {
        let path = self.request_dir.join(format!("{}.json", req.request_id));
        let json = serde_json::to_string_pretty(req).map_err(|e| e.to_string())?;
        std::fs::write(&path, json).map_err(|e| format!("failed to write request: {}", e))
    }

    /// Poll for a validation response. Returns None if not yet available.
    pub fn poll_response(&self, request_id: &str) -> Option<ValidationResponse> {
        let path = self.response_dir.join(format!("{}.json", request_id));
        let data = std::fs::read_to_string(&path).ok()?;
        let resp: ValidationResponse = serde_json::from_str(&data).ok()?;
        // Clean up the response file after reading
        let _ = std::fs::remove_file(&path);
        Some(resp)
    }

    /// Poll all pending responses and cache them.
    pub fn poll_all_responses(&mut self, current_epoch: u64) -> usize {
        let entries = match std::fs::read_dir(&self.response_dir) {
            Ok(e) => e,
            Err(_) => return 0,
        };

        let mut count = 0;
        for entry in entries.flatten() {
            let path = entry.path();
            if path.extension().map(|e| e == "json").unwrap_or(false) {
                if let Ok(data) = std::fs::read_to_string(&path) {
                    if let Ok(resp) = serde_json::from_str::<ValidationResponse>(&data) {
                        // Parse request_id to extract cache key components
                        // Format: "{schema_hex}_{method}_{current_hash}_{proposed_hash}"
                        if let Some(key) = parse_request_id_to_cache_key(&resp.request_id) {
                            self.cache.insert(key, CachedValidation {
                                valid: resp.valid,
                                reason: resp.reason.clone(),
                                cached_at_epoch: current_epoch,
                                gas_used: resp.gas_used,
                            });
                            count += 1;
                        }
                        let _ = std::fs::remove_file(&path);
                    }
                }
            }
        }
        count
    }

    /// Check if a constraint has been pre-validated.
    pub fn is_validated(
        &self,
        schema_id: &SchemaId,
        current_state: &[u8],
        proposed_state: &[u8],
        method: &str,
        current_epoch: u64,
    ) -> Option<bool> {
        let current_hash = state_hash(current_state);
        let proposed_hash = state_hash(proposed_state);
        self.cache.is_pre_validated(schema_id, &current_hash, &proposed_hash, method, current_epoch)
    }

    /// Generate a deterministic request ID for a validation.
    pub fn make_request_id(
        schema_id: &SchemaId,
        current_state: &[u8],
        proposed_state: &[u8],
        method: &str,
    ) -> String {
        let current_hash = state_hash(current_state);
        let proposed_hash = state_hash(proposed_state);
        format!(
            "{}_{}_{}_{}",
            hex::encode(&schema_id[..8]),
            method,
            hex::encode(&current_hash[..8]),
            hex::encode(&proposed_hash[..8]),
        )
    }
}

impl ConstraintValidatorBridge for WazeroBridge {
    fn validate(
        &self,
        schema_id: &SchemaId,
        proposed_state: &[u8],
        current_state: &[u8],
        _intent_params: &[u8],
        context: &ValidationContext,
    ) -> ConstraintResult {
        // Check cache first
        let current_hash = state_hash(current_state);
        let proposed_hash = state_hash(proposed_state);
        let cache_key = ValidationCacheKey::compute(
            schema_id, &current_hash, &proposed_hash, &context.method_selector,
        );

        if let Some(cached) = self.cache.get(&cache_key, context.epoch) {
            return ConstraintResult {
                valid: cached.valid,
                reason: cached.reason.clone(),
                gas_used: cached.gas_used,
            };
        }

        // Not cached — write request for Go chain to process
        let request_id = Self::make_request_id(schema_id, current_state, proposed_state, &context.method_selector);
        let req = ValidationRequest {
            request_id: request_id.clone(),
            schema_id: hex::encode(schema_id),
            method: context.method_selector.clone(),
            current_state: hex::encode(current_state),
            proposed_state: hex::encode(proposed_state),
            sender: hex::encode(context.sender),
            epoch: context.epoch,
        };

        if let Err(e) = self.submit_request(&req) {
            return ConstraintResult {
                valid: false,
                reason: format!("failed to submit validation request: {}", e),
                gas_used: 0,
            };
        }

        // For synchronous validation, poll immediately (Go may have processed it
        // in the same block). If not available, accept optimistically — the next
        // epoch will catch invalid transitions via the safety kernel.
        if let Some(resp) = self.poll_response(&request_id) {
            ConstraintResult {
                valid: resp.valid,
                reason: resp.reason,
                gas_used: resp.gas_used,
            }
        } else {
            // Optimistic acceptance — constraint will be verified asynchronously.
            // Invalid transitions are caught by the safety kernel at epoch boundary.
            ConstraintResult {
                valid: true,
                reason: "optimistic: pending async validation".to_string(),
                gas_used: 0,
            }
        }
    }
}

/// Parse a request_id back into a ValidationCacheKey.
/// Format: "{schema_8hex}_{method}_{current_8hex}_{proposed_8hex}"
fn parse_request_id_to_cache_key(request_id: &str) -> Option<ValidationCacheKey> {
    let parts: Vec<&str> = request_id.splitn(4, '_').collect();
    if parts.len() != 4 {
        return None;
    }

    // Reconstruct the cache key from the request_id components
    // We use SHA256 of the request_id as the cache key since we don't
    // have the full 32-byte hashes from the truncated hex.
    let mut hasher = Sha256::new();
    hasher.update(b"OMNIPHI_VALIDATION_CACHE_V1");
    hasher.update(request_id.as_bytes());
    let hash = hasher.finalize();
    let mut key = [0u8; 32];
    key.copy_from_slice(&hash);
    Some(ValidationCacheKey(key))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    fn make_temp_bridge() -> (WazeroBridge, TempDir) {
        let dir = TempDir::new().unwrap();
        let bridge = WazeroBridge::new(dir.path());
        (bridge, dir)
    }

    #[test]
    fn test_submit_and_poll_request() {
        let (bridge, _dir) = make_temp_bridge();

        let req = ValidationRequest {
            request_id: "test_req_1".to_string(),
            schema_id: hex::encode([0xAA; 32]),
            method: "fund".to_string(),
            current_state: hex::encode(b"current"),
            proposed_state: hex::encode(b"proposed"),
            sender: hex::encode([1u8; 32]),
            epoch: 5,
        };

        bridge.submit_request(&req).unwrap();

        // Simulate Go writing a response
        let resp = ValidationResponse {
            request_id: "test_req_1".to_string(),
            valid: true,
            reason: String::new(),
            gas_used: 100,
        };
        let resp_path = bridge.response_dir.join("test_req_1.json");
        fs::write(&resp_path, serde_json::to_string(&resp).unwrap()).unwrap();

        let result = bridge.poll_response("test_req_1");
        assert!(result.is_some());
        assert!(result.unwrap().valid);
    }

    #[test]
    fn test_cache_hit() {
        let (mut bridge, _dir) = make_temp_bridge();

        let schema = [0xAA; 32];
        let current = state_hash(b"current");
        let proposed = state_hash(b"proposed");
        let key = ValidationCacheKey::compute(&schema, &current, &proposed, "fund");

        bridge.cache.insert(key, CachedValidation {
            valid: true,
            reason: String::new(),
            cached_at_epoch: 5,
            gas_used: 50,
        });

        let result = bridge.is_validated(&schema, b"current", b"proposed", "fund", 5);
        assert_eq!(result, Some(true));
    }

    #[test]
    fn test_make_request_id_deterministic() {
        let schema = [0xBB; 32];
        let id1 = WazeroBridge::make_request_id(&schema, b"state1", b"state2", "release");
        let id2 = WazeroBridge::make_request_id(&schema, b"state1", b"state2", "release");
        assert_eq!(id1, id2);

        let id3 = WazeroBridge::make_request_id(&schema, b"state1", b"state3", "release");
        assert_ne!(id1, id3);
    }
}
