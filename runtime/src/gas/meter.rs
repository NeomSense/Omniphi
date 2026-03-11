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
}

impl GasMeter {
    pub fn new(limit: u64) -> Self {
        Self {
            limit,
            consumed: 0,
            costs: GasCosts::default_costs(),
        }
    }

    /// Consume `amount` gas units, returning Err if the limit is exceeded.
    pub fn consume(&mut self, amount: u64) -> Result<(), RuntimeError> {
        self.consumed = self
            .consumed
            .checked_add(amount)
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

    pub fn remaining(&self) -> u64 {
        self.limit.saturating_sub(self.consumed)
    }
}
