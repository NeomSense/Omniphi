use crate::errors::RuntimeError;

/// Gas costs per operation type (deterministic constants).
#[derive(Debug, Clone, Copy)]
pub struct GasCosts {
    pub base_tx: u64,
    pub object_read: u64,
    pub object_write: u64,
    pub debit_balance: u64,
    pub credit_balance: u64,
    pub swap_pool: u64,
    pub lock_balance: u64,
    pub unlock_balance: u64,
    pub update_version: u64,
    pub capability_check: u64,
    /// Gas per byte of net new state created (storage growth fee).
    pub storage_growth_per_byte: u64,
}

impl GasCosts {
    pub const fn default_costs() -> Self {
        GasCosts {
            base_tx: 1_000,
            object_read: 100,
            object_write: 500,
            debit_balance: 200,
            credit_balance: 200,
            swap_pool: 800,
            lock_balance: 300,
            unlock_balance: 300,
            update_version: 50,
            capability_check: 50,
            storage_growth_per_byte: 20,
        }
    }
}

/// A re-export alias so callers can use `GasCost` as a shorthand.
pub type GasCost = u64;

/// Tracks gas consumption during plan execution.
/// Returns Err if limit exceeded.
#[derive(Debug, Clone)]
pub struct GasMeter {
    pub limit: u64,
    pub consumed: u64,
    pub costs: GasCosts,
    /// Net new state bytes created during execution (for storage growth accounting).
    pub storage_bytes_added: u64,
    /// Batch complexity multiplier in basis points (10000 = 1.0x).
    /// Applied to all gas costs when operating in high-contention batches.
    pub batch_complexity_multiplier_bps: u64,
}

impl GasMeter {
    pub fn new(limit: u64) -> Self {
        Self {
            limit,
            consumed: 0,
            costs: GasCosts::default_costs(),
            storage_bytes_added: 0,
            batch_complexity_multiplier_bps: 10_000, // default 1.0x
        }
    }

    /// Create a gas meter with a batch complexity multiplier.
    ///
    /// The multiplier is applied to all gas consumption, increasing costs
    /// for high-contention or high-complexity settlement batches.
    /// Typical range: 10000 (1.0x normal) to 20000 (2.0x heavy contention).
    pub fn with_complexity_multiplier(limit: u64, multiplier_bps: u64) -> Self {
        Self {
            limit,
            consumed: 0,
            costs: GasCosts::default_costs(),
            storage_bytes_added: 0,
            batch_complexity_multiplier_bps: std::cmp::max(multiplier_bps, 10_000),
        }
    }

    /// Consume `amount` gas units, applying the batch complexity multiplier.
    /// Returns Err if the limit is exceeded.
    pub fn consume(&mut self, amount: u64) -> Result<(), RuntimeError> {
        // Apply complexity multiplier: effective = amount * multiplier / 10000
        let effective = if self.batch_complexity_multiplier_bps == 10_000 {
            amount // fast path: no multiplication needed at 1.0x
        } else {
            (amount as u128)
                .saturating_mul(self.batch_complexity_multiplier_bps as u128)
                .checked_div(10_000)
                .unwrap_or(u128::MAX) as u64
        };

        self.consumed = self
            .consumed
            .checked_add(effective)
            .ok_or_else(|| RuntimeError::ConstraintViolation("gas counter overflow".into()))?;
        if self.consumed > self.limit {
            Err(RuntimeError::ConstraintViolation(format!(
                "out of gas: consumed {} limit {}",
                self.consumed, self.limit
            )))
        } else {
            Ok(())
        }
    }

    /// Charge gas for net new state bytes created.
    pub fn consume_storage_growth(&mut self, bytes: u64) -> Result<(), RuntimeError> {
        self.storage_bytes_added = self.storage_bytes_added.saturating_add(bytes);
        let gas = bytes.saturating_mul(self.costs.storage_growth_per_byte);
        self.consume(gas)
    }

    pub fn remaining(&self) -> u64 {
        self.limit.saturating_sub(self.consumed)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_basic_gas_consumption() {
        let mut meter = GasMeter::new(10_000);
        assert!(meter.consume(5_000).is_ok());
        assert_eq!(meter.remaining(), 5_000);
        assert!(meter.consume(5_000).is_ok());
        assert_eq!(meter.remaining(), 0);
        assert!(meter.consume(1).is_err());
    }

    #[test]
    fn test_complexity_multiplier() {
        let mut meter = GasMeter::with_complexity_multiplier(10_000, 15_000); // 1.5x
        assert!(meter.consume(1_000).is_ok());
        // 1000 * 15000 / 10000 = 1500 consumed
        assert_eq!(meter.consumed, 1_500);
        assert_eq!(meter.remaining(), 8_500);
    }

    #[test]
    fn test_no_multiplier_fast_path() {
        let mut meter = GasMeter::new(10_000);
        assert!(meter.consume(1_000).is_ok());
        assert_eq!(meter.consumed, 1_000); // exactly 1000, no multiplier
    }

    #[test]
    fn test_storage_growth_charge() {
        let mut meter = GasMeter::new(100_000);
        assert!(meter.consume_storage_growth(100).is_ok());
        // 100 bytes * 20 gas/byte = 2000 gas
        assert_eq!(meter.consumed, 2_000);
        assert_eq!(meter.storage_bytes_added, 100);
    }

    #[test]
    fn test_storage_growth_with_multiplier() {
        let mut meter = GasMeter::with_complexity_multiplier(100_000, 20_000); // 2.0x
        assert!(meter.consume_storage_growth(100).is_ok());
        // 100 * 20 = 2000 base gas, then 2000 * 20000 / 10000 = 4000
        assert_eq!(meter.consumed, 4_000);
    }

    #[test]
    fn test_overflow_protection() {
        let mut meter = GasMeter::new(u64::MAX);
        assert!(meter.consume(u64::MAX - 1).is_ok());
        // This should detect overflow
        let result = meter.consume(u64::MAX);
        assert!(result.is_err());
    }

    #[test]
    fn test_minimum_multiplier_is_1x() {
        let meter = GasMeter::with_complexity_multiplier(10_000, 5_000); // asked for 0.5x
        assert_eq!(meter.batch_complexity_multiplier_bps, 10_000); // clamped to 1.0x minimum
    }
}
