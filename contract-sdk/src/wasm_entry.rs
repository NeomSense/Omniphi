//! Wasm entry point macro and helpers.
//!
//! The `omniphi_contract_entry!` macro generates the Wasm-exported `validate`
//! function that the wazero runtime calls. Contract developers invoke this
//! macro once at the bottom of their contract file.
//!
//! # Usage
//!
//! ```rust,ignore
//! use omniphi_contract_sdk::*;
//!
//! #[derive(serde::Serialize, serde::Deserialize)]
//! struct MyState { /* ... */ }
//!
//! impl ContractValidator for MyState {
//!     fn validate(ctx: &ValidationContext, current: &Self, proposed: &Self) -> ConstraintResult {
//!         // ... validation logic ...
//!         ConstraintResult::accept()
//!     }
//! }
//!
//! omniphi_contract_entry!(MyState);
//! ```

/// Generates the Wasm entry point for a contract validator.
///
/// This macro creates:
/// 1. An `alloc` function for the host to write data into Wasm memory
/// 2. A `validate` function that the wazero runtime calls
/// 3. Memory management for passing results back to the host
///
/// The generated `validate` function reads input from linear memory,
/// deserializes, calls the `ContractValidator::validate_method`, and
/// returns 0 (valid) or 1 (invalid).
#[macro_export]
macro_rules! omniphi_contract_entry {
    ($state_type:ty) => {
        // Allocator for the host to write input data into Wasm memory.
        #[no_mangle]
        pub extern "C" fn alloc(size: u32) -> u32 {
            let layout = std::alloc::Layout::from_size_align(size as usize, 1).unwrap();
            unsafe { std::alloc::alloc(layout) as u32 }
        }

        // Main validation entry point called by the wazero host.
        //
        // Arguments (all are offsets/lengths into linear memory):
        //   proposed_ptr, proposed_len: proposed state bytes
        //   current_ptr, current_len: current state bytes
        //   params_ptr, params_len: context/params bytes (JSON)
        //   epoch: current epoch (u64)
        //   sender_ptr: 32-byte sender address
        //
        // Returns: 0 = valid, 1 = invalid
        #[no_mangle]
        pub extern "C" fn validate(
            proposed_ptr: u32,
            proposed_len: u32,
            current_ptr: u32,
            current_len: u32,
            params_ptr: u32,
            params_len: u32,
            _epoch: u64,
            _sender_ptr: u32,
        ) -> i32 {
            let proposed = unsafe {
                std::slice::from_raw_parts(proposed_ptr as *const u8, proposed_len as usize)
            };
            let current = unsafe {
                std::slice::from_raw_parts(current_ptr as *const u8, current_len as usize)
            };
            let context = unsafe {
                std::slice::from_raw_parts(params_ptr as *const u8, params_len as usize)
            };

            let result = $crate::validator::dispatch_validation::<$state_type>(
                current, proposed, context,
            );

            if result.valid { 0 } else { 1 }
        }
    };
}
