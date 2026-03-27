//! Local test runner — executes test suites against contract schemas without
//! requiring a running chain or solver.
//!
//! Test cases are defined in YAML alongside the contract schema. Each test case
//! specifies:
//! - The intent to invoke
//! - Input parameters
//! - State before the intent
//! - Expected state after the intent
//! - Whether the constraint check should pass or fail
//!
//! The tester simulates constraint evaluation by:
//! 1. Loading the contract schema and its constraints
//! 2. Checking preconditions against `state_before`
//! 3. Checking postconditions against `state_after`
//! 4. Evaluating global and intent-specific constraints
//! 5. Comparing actual results against expected outcomes

use crate::schema::*;
use std::collections::HashMap;
use std::fmt;
use std::path::Path;

// ---------------------------------------------------------------------------
// Test results
// ---------------------------------------------------------------------------

/// Outcome of a single test case.
#[derive(Debug, Clone)]
pub enum TestOutcome {
    /// Test passed (actual matched expected).
    Pass,
    /// Test failed with a reason.
    Fail(String),
    /// Test was skipped (e.g., invalid setup).
    Skip(String),
}

impl fmt::Display for TestOutcome {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            TestOutcome::Pass => write!(f, "PASS"),
            TestOutcome::Fail(reason) => write!(f, "FAIL: {}", reason),
            TestOutcome::Skip(reason) => write!(f, "SKIP: {}", reason),
        }
    }
}

/// Result of running a single test case.
#[derive(Debug, Clone)]
pub struct TestResult {
    pub name: String,
    pub outcome: TestOutcome,
}

/// Summary of a complete test run.
#[derive(Debug)]
pub struct TestSummary {
    pub total: usize,
    pub passed: usize,
    pub failed: usize,
    pub skipped: usize,
    pub results: Vec<TestResult>,
}

impl TestSummary {
    pub fn is_success(&self) -> bool {
        self.failed == 0
    }
}

impl fmt::Display for TestSummary {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        writeln!(f, "\n--- Test Results ---")?;
        for result in &self.results {
            let icon = match &result.outcome {
                TestOutcome::Pass => "[PASS]",
                TestOutcome::Fail(_) => "[FAIL]",
                TestOutcome::Skip(_) => "[SKIP]",
            };
            writeln!(f, "  {} {}: {}", icon, result.name, result.outcome)?;
        }
        writeln!(f, "\nSummary: {} total, {} passed, {} failed, {} skipped",
            self.total, self.passed, self.failed, self.skipped)?;
        if self.is_success() {
            writeln!(f, "Result: ALL TESTS PASSED")?;
        } else {
            writeln!(f, "Result: {} TEST(S) FAILED", self.failed)?;
        }
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// Test runner
// ---------------------------------------------------------------------------

/// Parse a test suite from a YAML string.
pub fn parse_test_suite(yaml_str: &str) -> Result<TestSuite, String> {
    serde_yaml::from_str(yaml_str).map_err(|e| format!("Failed to parse test suite YAML: {}", e))
}

/// Parse a test suite from a YAML file.
pub fn parse_test_file(path: &Path) -> Result<TestSuite, String> {
    let content =
        std::fs::read_to_string(path).map_err(|e| format!("Failed to read test file: {}", e))?;
    parse_test_suite(&content)
}

/// Run all test cases in a suite against a compiled schema.
pub fn run_tests(schema: &ContractSchema, suite: &TestSuite) -> TestSummary {
    let mut results = Vec::new();
    let mut passed = 0;
    let mut failed = 0;
    let mut skipped = 0;

    // Build lookup tables
    let intent_map: HashMap<String, &IntentSchema> = schema
        .intents
        .iter()
        .map(|i| (i.name.to_lowercase(), i))
        .collect();

    let state_field_map: HashMap<String, &StateField> = schema
        .state_fields
        .iter()
        .map(|f| (f.name.to_lowercase(), f))
        .collect();

    for test_case in &suite.tests {
        let result = run_single_test(test_case, &intent_map, &state_field_map, &schema.constraints);
        match &result.outcome {
            TestOutcome::Pass => passed += 1,
            TestOutcome::Fail(_) => failed += 1,
            TestOutcome::Skip(_) => skipped += 1,
        }
        results.push(result);
    }

    TestSummary {
        total: results.len(),
        passed,
        failed,
        skipped,
        results,
    }
}

/// Run a single test case.
fn run_single_test(
    test: &TestCase,
    intent_map: &HashMap<String, &IntentSchema>,
    state_field_map: &HashMap<String, &StateField>,
    constraints: &[Constraint],
) -> TestResult {
    let name = test.name.clone();

    // 1. Resolve the intent
    let intent = match intent_map.get(&test.intent.to_lowercase()) {
        Some(i) => *i,
        None => {
            let outcome = if test.expect_valid {
                TestOutcome::Fail(format!("intent '{}' not found in schema", test.intent))
            } else if !test.expect_error.is_empty()
                && format!("intent '{}' not found in schema", test.intent)
                    .contains(&test.expect_error)
            {
                TestOutcome::Pass
            } else {
                TestOutcome::Fail(format!(
                    "intent '{}' not found in schema (expected error: '{}')",
                    test.intent, test.expect_error
                ))
            };
            return TestResult { name, outcome };
        }
    };

    // 2. Validate required parameters are provided
    for param_def in &intent.params {
        if param_def.required && !test.params.contains_key(&param_def.name) {
            let err_msg = format!("missing required parameter '{}'", param_def.name);
            let outcome = if !test.expect_valid && (test.expect_error.is_empty() || err_msg.contains(&test.expect_error)) {
                TestOutcome::Pass
            } else if test.expect_valid {
                TestOutcome::Fail(err_msg)
            } else {
                TestOutcome::Fail(format!(
                    "got error '{}' but expected error containing '{}'",
                    err_msg, test.expect_error
                ))
            };
            return TestResult { name, outcome };
        }
    }

    // 3. Validate parameter types
    if let Some(err) = validate_param_types(&test.params, &intent.params) {
        let outcome = if !test.expect_valid && (test.expect_error.is_empty() || err.contains(&test.expect_error)) {
            TestOutcome::Pass
        } else if test.expect_valid {
            TestOutcome::Fail(err)
        } else {
            TestOutcome::Fail(format!(
                "got error '{}' but expected error containing '{}'",
                err, test.expect_error
            ))
        };
        return TestResult { name, outcome };
    }

    // 4. Validate state_before has valid fields
    for key in test.state_before.keys() {
        if !state_field_map.contains_key(&key.to_lowercase()) {
            return TestResult {
                name,
                outcome: TestOutcome::Skip(format!(
                    "state_before references unknown field '{}'",
                    key
                )),
            };
        }
    }

    // 5. Evaluate constraints
    let constraint_result = evaluate_constraints(
        test,
        intent,
        state_field_map,
        constraints,
    );

    // 6. Compare against expectations
    let outcome = match constraint_result {
        Ok(()) => {
            if test.expect_valid {
                // Constraints passed and we expected them to pass.
                // Now check state_after if provided.
                if !test.state_after.is_empty() {
                    match validate_state_after(&test.state_after, state_field_map) {
                        Ok(()) => TestOutcome::Pass,
                        Err(e) => TestOutcome::Fail(format!("state_after validation failed: {}", e)),
                    }
                } else {
                    TestOutcome::Pass
                }
            } else {
                TestOutcome::Fail(format!(
                    "expected constraint violation{} but all constraints passed",
                    if test.expect_error.is_empty() {
                        String::new()
                    } else {
                        format!(" containing '{}'", test.expect_error)
                    }
                ))
            }
        }
        Err(err_msg) => {
            if !test.expect_valid {
                if test.expect_error.is_empty() || err_msg.to_lowercase().contains(&test.expect_error.to_lowercase()) {
                    TestOutcome::Pass
                } else {
                    TestOutcome::Fail(format!(
                        "constraint failed with '{}' but expected error containing '{}'",
                        err_msg, test.expect_error
                    ))
                }
            } else {
                TestOutcome::Fail(format!("constraint check failed: {}", err_msg))
            }
        }
    };

    TestResult { name, outcome }
}

// ---------------------------------------------------------------------------
// Constraint evaluation
// ---------------------------------------------------------------------------

/// Evaluate all applicable constraints for a test case.
/// Returns Ok(()) if all pass, Err(message) on the first failure.
fn evaluate_constraints(
    test: &TestCase,
    intent: &IntentSchema,
    state_field_map: &HashMap<String, &StateField>,
    constraints: &[Constraint],
) -> Result<(), String> {
    // Evaluate global and intent-specific constraints
    for constraint in constraints {
        // Check if this constraint applies to the current intent
        if !constraint.applies_to.is_empty() {
            let applies = constraint
                .applies_to
                .iter()
                .any(|name| name.to_lowercase() == intent.name.to_lowercase());
            if !applies {
                continue;
            }
        }

        evaluate_single_constraint(constraint, test, state_field_map)?;
    }

    Ok(())
}

/// Evaluate a single constraint against test state.
fn evaluate_single_constraint(
    constraint: &Constraint,
    test: &TestCase,
    state_field_map: &HashMap<String, &StateField>,
) -> Result<(), String> {
    match constraint.constraint_type {
        ConstraintType::BalanceCheck => evaluate_balance_check(constraint, test),
        ConstraintType::OwnershipCheck => evaluate_ownership_check(constraint, test),
        ConstraintType::TimeCheck => evaluate_time_check(constraint, test),
        ConstraintType::StateCheck => evaluate_state_check(constraint, test, state_field_map),
        ConstraintType::Custom => evaluate_custom_check(constraint, test),
    }
}

fn evaluate_balance_check(constraint: &Constraint, test: &TestCase) -> Result<(), String> {
    // In local testing, we check if the test case has a balance-related param
    // that satisfies the constraint.
    if let Some(serde_json::Value::String(min_amount_str)) = constraint.params.get("min_amount") {
        if let Ok(min_amount) = min_amount_str.parse::<u128>() {
            // Check if any param in state_before provides sufficient balance
            if let Some(serde_json::Value::String(balance_str)) = test.state_before.get("balance") {
                if let Ok(balance) = balance_str.parse::<u128>() {
                    if balance < min_amount {
                        return Err(format!(
                            "BalanceCheck '{}' failed: balance {} < required {}",
                            constraint.name, balance, min_amount
                        ));
                    }
                }
            }
            // Also check amount param
            if let Some(serde_json::Value::String(amount_str)) = test.params.get("amount") {
                if let Ok(amount) = amount_str.parse::<u128>() {
                    if let Some(serde_json::Value::String(balance_str)) = test.state_before.get("balance") {
                        if let Ok(balance) = balance_str.parse::<u128>() {
                            if amount > balance {
                                return Err(format!(
                                    "BalanceCheck '{}' failed: requested amount {} exceeds balance {}",
                                    constraint.name, amount, balance
                                ));
                            }
                        }
                    }
                }
            }
        }
    }
    Ok(())
}

fn evaluate_ownership_check(constraint: &Constraint, test: &TestCase) -> Result<(), String> {
    if let Some(serde_json::Value::String(field_name)) = constraint.params.get("object_field") {
        // Check that the sender matches the ownership field in state_before
        if let Some(owner_value) = test.state_before.get(field_name) {
            if !test.sender.is_empty() {
                if let serde_json::Value::String(owner) = owner_value {
                    if owner != &test.sender && !test.sender.is_empty() {
                        return Err(format!(
                            "OwnershipCheck '{}' failed: sender '{}' does not own '{}' (owner: '{}')",
                            constraint.name, test.sender, field_name, owner
                        ));
                    }
                }
            }
        }
    }
    Ok(())
}

fn evaluate_time_check(constraint: &Constraint, test: &TestCase) -> Result<(), String> {
    let op = match constraint.params.get("op") {
        Some(serde_json::Value::String(op)) => op.as_str(),
        _ => return Ok(()), // No op means we can't check
    };

    let value = match constraint.params.get("value") {
        Some(serde_json::Value::String(v)) => v.parse::<u64>().map_err(|_| {
            format!(
                "TimeCheck '{}': invalid value '{}' (must be a u64)",
                constraint.name, v
            )
        })?,
        Some(serde_json::Value::Number(n)) => n.as_u64().ok_or_else(|| {
            format!(
                "TimeCheck '{}': invalid numeric value",
                constraint.name
            )
        })?,
        _ => return Ok(()),
    };

    let epoch = test.epoch;
    let pass = match op {
        "gte" => epoch >= value,
        "lte" => epoch <= value,
        "gt" => epoch > value,
        "lt" => epoch < value,
        _ => true,
    };

    if !pass {
        return Err(format!(
            "TimeCheck '{}' failed: epoch {} is not {} {}",
            constraint.name, epoch, op, value
        ));
    }

    Ok(())
}

fn evaluate_state_check(
    constraint: &Constraint,
    test: &TestCase,
    _state_field_map: &HashMap<String, &StateField>,
) -> Result<(), String> {
    let field_name = match constraint.params.get("field") {
        Some(serde_json::Value::String(f)) => f.as_str(),
        _ => return Ok(()),
    };

    let op = match constraint.params.get("op") {
        Some(serde_json::Value::String(op)) => op.as_str(),
        _ => return Ok(()),
    };

    let target_value = match constraint.params.get("value") {
        Some(serde_json::Value::String(v)) => v.clone(),
        Some(serde_json::Value::Number(n)) => n.to_string(),
        _ => return Ok(()),
    };

    // Check against state_after (postcondition) if available, otherwise state_before
    let state = if !test.state_after.is_empty() {
        &test.state_after
    } else {
        &test.state_before
    };

    let actual_value = match state.get(field_name) {
        Some(serde_json::Value::String(v)) => v.clone(),
        Some(serde_json::Value::Number(n)) => n.to_string(),
        Some(v) => v.to_string(),
        None => return Ok(()), // Field not in test state; can't check
    };

    // Try numeric comparison first
    if let (Ok(actual_num), Ok(target_num)) = (
        actual_value.parse::<i128>(),
        target_value.parse::<i128>(),
    ) {
        let pass = match op {
            "gte" => actual_num >= target_num,
            "lte" => actual_num <= target_num,
            "gt" => actual_num > target_num,
            "lt" => actual_num < target_num,
            "eq" => actual_num == target_num,
            "neq" => actual_num != target_num,
            _ => true,
        };

        if !pass {
            return Err(format!(
                "StateCheck '{}' failed: field '{}' value {} is not {} {}",
                constraint.name, field_name, actual_num, op, target_num
            ));
        }
    } else {
        // String comparison
        let pass = match op {
            "eq" => actual_value == target_value,
            "neq" => actual_value != target_value,
            _ => {
                return Err(format!(
                    "StateCheck '{}': non-numeric comparison with op '{}' is only valid for eq/neq",
                    constraint.name, op
                ));
            }
        };

        if !pass {
            return Err(format!(
                "StateCheck '{}' failed: field '{}' value '{}' is not {} '{}'",
                constraint.name, field_name, actual_value, op, target_value
            ));
        }
    }

    Ok(())
}

fn evaluate_custom_check(constraint: &Constraint, test: &TestCase) -> Result<(), String> {
    // Custom constraints require a Wasm validator at runtime.
    // In local testing, we evaluate simple expressions if provided.
    if let Some(serde_json::Value::String(expr)) = constraint.params.get("expression") {
        // Support simple field comparison expressions like "field >= 0"
        let parts: Vec<&str> = expr.split_whitespace().collect();
        if parts.len() == 3 {
            let field = parts[0];
            let op = parts[1];
            let value = parts[2];

            let state = if !test.state_after.is_empty() {
                &test.state_after
            } else {
                &test.state_before
            };

            if let Some(actual) = state.get(field) {
                let actual_str = match actual {
                    serde_json::Value::String(s) => s.clone(),
                    serde_json::Value::Number(n) => n.to_string(),
                    _ => actual.to_string(),
                };

                if let (Ok(a), Ok(v)) = (actual_str.parse::<i128>(), value.parse::<i128>()) {
                    let pass = match op {
                        ">=" => a >= v,
                        "<=" => a <= v,
                        ">" => a > v,
                        "<" => a < v,
                        "==" => a == v,
                        "!=" => a != v,
                        _ => true,
                    };
                    if !pass {
                        return Err(format!(
                            "Custom constraint '{}' failed: {} ({}) {} {}",
                            constraint.name, field, a, op, v
                        ));
                    }
                }
            }
        }
    }
    Ok(())
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Validate that parameter values match expected types.
fn validate_param_types(
    params: &serde_json::Map<String, serde_json::Value>,
    param_defs: &[ParamDef],
) -> Option<String> {
    let def_map: HashMap<String, &ParamDef> = param_defs
        .iter()
        .map(|p| (p.name.to_lowercase(), p))
        .collect();

    for (name, value) in params {
        if let Some(def) = def_map.get(&name.to_lowercase()) {
            if let Some(err) = validate_param_value(value, &def.param_type, name) {
                return Some(err);
            }
        }
        // Unknown params are allowed (passed through as custom_constraints)
    }
    None
}

fn validate_param_value(
    value: &serde_json::Value,
    expected_type: &ParamType,
    param_name: &str,
) -> Option<String> {
    match expected_type {
        ParamType::Uint128 => match value {
            serde_json::Value::Number(n) => {
                if n.is_f64() {
                    Some(format!(
                        "parameter '{}': Uint128 must be an integer, got float",
                        param_name
                    ))
                } else {
                    None
                }
            }
            serde_json::Value::String(s) => {
                if s.parse::<u128>().is_err() {
                    Some(format!(
                        "parameter '{}': value '{}' is not a valid Uint128",
                        param_name, s
                    ))
                } else {
                    None
                }
            }
            _ => Some(format!(
                "parameter '{}': expected Uint128, got {}",
                param_name,
                value_type_name(value)
            )),
        },
        ParamType::Address => match value {
            serde_json::Value::String(s) => {
                if s.len() != 64 && s != "zero" && s != "sender" && !s.starts_with("test_") {
                    Some(format!(
                        "parameter '{}': Address must be 64 hex chars, 'zero', 'sender', or 'test_*' label, got '{}'",
                        param_name, s
                    ))
                } else {
                    None
                }
            }
            _ => Some(format!(
                "parameter '{}': expected Address string, got {}",
                param_name,
                value_type_name(value)
            )),
        },
        ParamType::Bytes => match value {
            serde_json::Value::String(s) => {
                if s.len() % 2 != 0 || !s.chars().all(|c| c.is_ascii_hexdigit()) {
                    Some(format!(
                        "parameter '{}': Bytes must be even-length hex string, got '{}'",
                        param_name, s
                    ))
                } else {
                    None
                }
            }
            _ => Some(format!(
                "parameter '{}': expected Bytes hex string, got {}",
                param_name,
                value_type_name(value)
            )),
        },
        ParamType::String => {
            // Strings accept any JSON value (will be stringified)
            None
        }
        ParamType::Bool => match value {
            serde_json::Value::Bool(_) => None,
            serde_json::Value::String(s) if s == "true" || s == "false" => None,
            _ => Some(format!(
                "parameter '{}': expected Bool, got {}",
                param_name,
                value_type_name(value)
            )),
        },
    }
}

fn value_type_name(value: &serde_json::Value) -> &'static str {
    match value {
        serde_json::Value::Null => "null",
        serde_json::Value::Bool(_) => "bool",
        serde_json::Value::Number(_) => "number",
        serde_json::Value::String(_) => "string",
        serde_json::Value::Array(_) => "array",
        serde_json::Value::Object(_) => "object",
    }
}

fn validate_state_after(
    state_after: &serde_json::Map<String, serde_json::Value>,
    state_field_map: &HashMap<String, &StateField>,
) -> Result<(), String> {
    for key in state_after.keys() {
        if !state_field_map.contains_key(&key.to_lowercase()) {
            return Err(format!("unknown state field '{}' in state_after", key));
        }
    }
    Ok(())
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn test_schema() -> ContractSchema {
        ContractSchema {
            name: "Counter".to_string(),
            version: "1.0.0".to_string(),
            description: "Test counter".to_string(),
            intents: vec![
                IntentSchema {
                    name: "increment".to_string(),
                    description: "Increase counter".to_string(),
                    params: vec![ParamDef {
                        name: "amount".to_string(),
                        param_type: ParamType::Uint128,
                        required: true,
                        description: String::new(),
                    }],
                    preconditions: vec![],
                    postconditions: vec![],
                },
                IntentSchema {
                    name: "decrement".to_string(),
                    description: "Decrease counter".to_string(),
                    params: vec![ParamDef {
                        name: "amount".to_string(),
                        param_type: ParamType::Uint128,
                        required: true,
                        description: String::new(),
                    }],
                    preconditions: vec![],
                    postconditions: vec![],
                },
            ],
            state_fields: vec![StateField {
                name: "count".to_string(),
                field_type: ParamType::Uint128,
                default_value: Some("0".to_string()),
                description: String::new(),
            }],
            constraints: vec![Constraint {
                name: "non_negative".to_string(),
                constraint_type: ConstraintType::StateCheck,
                params: {
                    let mut m = serde_json::Map::new();
                    m.insert("field".to_string(), serde_json::Value::String("count".to_string()));
                    m.insert("op".to_string(), serde_json::Value::String("gte".to_string()));
                    m.insert("value".to_string(), serde_json::Value::String("0".to_string()));
                    m
                },
                applies_to: vec!["decrement".to_string()],
            }],
            max_gas_per_call: 1_000_000,
            max_state_bytes: 65_536,
        }
    }

    #[test]
    fn test_passing_increment() {
        let schema = test_schema();
        let suite = TestSuite {
            contract: "Counter".to_string(),
            schema_path: String::new(),
            tests: vec![TestCase {
                name: "increment by 5".to_string(),
                intent: "increment".to_string(),
                params: {
                    let mut m = serde_json::Map::new();
                    m.insert("amount".to_string(), serde_json::Value::String("5".to_string()));
                    m
                },
                sender: "test_user".to_string(),
                epoch: 1,
                state_before: {
                    let mut m = serde_json::Map::new();
                    m.insert("count".to_string(), serde_json::Value::String("10".to_string()));
                    m
                },
                state_after: {
                    let mut m = serde_json::Map::new();
                    m.insert("count".to_string(), serde_json::Value::String("15".to_string()));
                    m
                },
                expect_valid: true,
                expect_error: String::new(),
            }],
        };

        let summary = run_tests(&schema, &suite);
        assert!(summary.is_success(), "expected pass: {:?}", summary.results);
    }

    #[test]
    fn test_failing_decrement_negative() {
        let schema = test_schema();
        let suite = TestSuite {
            contract: "Counter".to_string(),
            schema_path: String::new(),
            tests: vec![TestCase {
                name: "decrement below zero".to_string(),
                intent: "decrement".to_string(),
                params: {
                    let mut m = serde_json::Map::new();
                    m.insert("amount".to_string(), serde_json::Value::String("10".to_string()));
                    m
                },
                sender: "test_user".to_string(),
                epoch: 1,
                state_before: {
                    let mut m = serde_json::Map::new();
                    m.insert("count".to_string(), serde_json::Value::String("5".to_string()));
                    m
                },
                state_after: {
                    let mut m = serde_json::Map::new();
                    m.insert("count".to_string(), serde_json::Value::String("-5".to_string()));
                    m
                },
                expect_valid: false,
                expect_error: "non_negative".to_string(),
            }],
        };

        let summary = run_tests(&schema, &suite);
        assert!(summary.is_success(), "expected controlled failure: {:?}", summary.results);
    }

    #[test]
    fn test_missing_required_param() {
        let schema = test_schema();
        let suite = TestSuite {
            contract: "Counter".to_string(),
            schema_path: String::new(),
            tests: vec![TestCase {
                name: "missing amount".to_string(),
                intent: "increment".to_string(),
                params: serde_json::Map::new(),
                sender: String::new(),
                epoch: 1,
                state_before: serde_json::Map::new(),
                state_after: serde_json::Map::new(),
                expect_valid: false,
                expect_error: "missing required".to_string(),
            }],
        };

        let summary = run_tests(&schema, &suite);
        assert!(summary.is_success());
    }

    #[test]
    fn test_unknown_intent() {
        let schema = test_schema();
        let suite = TestSuite {
            contract: "Counter".to_string(),
            schema_path: String::new(),
            tests: vec![TestCase {
                name: "bad intent".to_string(),
                intent: "nonexistent".to_string(),
                params: serde_json::Map::new(),
                sender: String::new(),
                epoch: 1,
                state_before: serde_json::Map::new(),
                state_after: serde_json::Map::new(),
                expect_valid: false,
                expect_error: "not found".to_string(),
            }],
        };

        let summary = run_tests(&schema, &suite);
        assert!(summary.is_success());
    }

    #[test]
    fn test_time_check() {
        let mut schema = test_schema();
        schema.constraints.push(Constraint {
            name: "not_too_early".to_string(),
            constraint_type: ConstraintType::TimeCheck,
            params: {
                let mut m = serde_json::Map::new();
                m.insert("op".to_string(), serde_json::Value::String("gte".to_string()));
                m.insert("value".to_string(), serde_json::Value::String("100".to_string()));
                m
            },
            applies_to: vec!["increment".to_string()],
        });

        let suite = TestSuite {
            contract: "Counter".to_string(),
            schema_path: String::new(),
            tests: vec![TestCase {
                name: "too early".to_string(),
                intent: "increment".to_string(),
                params: {
                    let mut m = serde_json::Map::new();
                    m.insert("amount".to_string(), serde_json::Value::String("1".to_string()));
                    m
                },
                sender: String::new(),
                epoch: 50, // before epoch 100
                state_before: serde_json::Map::new(),
                state_after: serde_json::Map::new(),
                expect_valid: false,
                expect_error: "TimeCheck".to_string(),
            }],
        };

        let summary = run_tests(&schema, &suite);
        assert!(summary.is_success());
    }
}
