//! Omniphi Intent Contract Toolkit — CLI entry point.
//!
//! Provides commands for the full contract development lifecycle:
//! - `init`     — Scaffold a new contract project
//! - `build`    — Compile contract schema from YAML to JSON
//! - `validate` — Check schema validity without compiling
//! - `test`     — Run local contract tests
//! - `deploy`   — Deploy compiled schema to chain (stub)

mod compiler;
mod schema;
mod tester;
mod validator;

use clap::{Parser, Subcommand};
use std::path::{Path, PathBuf};
use std::process;

#[derive(Parser)]
#[command(
    name = "omniphi-contracts",
    version = "0.1.0",
    about = "CLI toolkit for building, testing, and deploying Omniphi Intent Contracts",
    long_about = "The Omniphi Contract Toolkit provides a complete development workflow for Intent \
                  Contracts. Define contracts in YAML, compile them to the runtime's JSON schema \
                  format, test locally with YAML test cases, and deploy to the chain."
)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Scaffold a new contract project with example files
    Init {
        /// Name of the contract project to create
        name: String,
        /// Template to use (counter, escrow, or blank)
        #[arg(short, long, default_value = "counter")]
        template: String,
    },

    /// Compile contract schema from YAML to JSON
    Build {
        /// Path to the contract project directory (default: current directory)
        #[arg(short, long)]
        path: Option<PathBuf>,
        /// Output file path (default: build/<name>.json)
        #[arg(short, long)]
        output: Option<PathBuf>,
    },

    /// Validate schema without compiling
    Validate {
        /// Path to the contract project directory or schema YAML file
        #[arg(short, long)]
        path: Option<PathBuf>,
    },

    /// Run local contract tests
    Test {
        /// Path to the contract project directory
        #[arg(short, long)]
        path: Option<PathBuf>,
        /// Run only tests matching this name filter
        #[arg(short, long)]
        filter: Option<String>,
    },

    /// Deploy compiled schema to the Omniphi chain
    Deploy {
        /// Path to the compiled JSON schema file
        #[arg(short, long)]
        path: Option<PathBuf>,
        /// RPC endpoint URL
        #[arg(long)]
        rpc: String,
        /// Path to the signing key file
        #[arg(long)]
        key: String,
        /// Chain ID (default: omniphi-testnet-2)
        #[arg(long, default_value = "omniphi-testnet-2")]
        chain_id: String,
        /// Gas limit for the deploy transaction
        #[arg(long, default_value = "500000")]
        gas: u64,
        /// Dry run: print the payload without sending
        #[arg(long, default_value = "false")]
        dry_run: bool,
    },
}

fn main() {
    let cli = Cli::parse();

    let result = match cli.command {
        Commands::Init { name, template } => cmd_init(&name, &template),
        Commands::Build { path, output } => cmd_build(path.as_deref(), output.as_deref()),
        Commands::Validate { path } => cmd_validate(path.as_deref()),
        Commands::Test { path, filter } => cmd_test(path.as_deref(), filter.as_deref()),
        Commands::Deploy {
            path,
            rpc,
            key,
            chain_id,
            gas,
            dry_run,
        } => cmd_deploy(path.as_deref(), &rpc, &key, &chain_id, gas, dry_run),
    };

    if let Err(e) = result {
        eprintln!("Error: {}", e);
        process::exit(1);
    }
}

// ---------------------------------------------------------------------------
// init
// ---------------------------------------------------------------------------

fn cmd_init(name: &str, template: &str) -> Result<(), String> {
    // Validate name
    if name.is_empty() {
        return Err("project name must not be empty".to_string());
    }
    if !name
        .chars()
        .next()
        .unwrap()
        .is_ascii_alphabetic()
    {
        return Err("project name must start with a letter".to_string());
    }
    if !name
        .chars()
        .all(|c| c.is_ascii_alphanumeric() || c == '-' || c == '_')
    {
        return Err(
            "project name must contain only alphanumeric characters, hyphens, or underscores"
                .to_string(),
        );
    }

    let project_dir = PathBuf::from(name);
    if project_dir.exists() {
        return Err(format!("directory '{}' already exists", name));
    }

    // Create directory structure
    std::fs::create_dir_all(project_dir.join("build"))
        .map_err(|e| format!("failed to create directory: {}", e))?;

    // Write contract.toml
    let manifest = format!(
        r#"[contract]
name = "{name}"
version = "0.1.0"
description = "An Omniphi Intent Contract"
authors = []

[build]
schema = "schema.yaml"
tests = "tests.yaml"
output = "build"
"#,
        name = name
    );
    write_file(&project_dir.join("contract.toml"), &manifest)?;

    // Write schema and test files based on template
    let (schema_content, test_content) = match template {
        "counter" => (counter_template_schema(name), counter_template_tests(name)),
        "escrow" => (escrow_template_schema(name), escrow_template_tests(name)),
        "blank" => (blank_template_schema(name), blank_template_tests(name)),
        _ => {
            return Err(format!(
                "unknown template '{}': must be one of: counter, escrow, blank",
                template
            ))
        }
    };

    write_file(&project_dir.join("schema.yaml"), &schema_content)?;
    write_file(&project_dir.join("tests.yaml"), &test_content)?;

    // Write .gitignore
    write_file(&project_dir.join(".gitignore"), "build/\n")?;

    println!("Created contract project '{}'", name);
    println!();
    println!("  cd {}", name);
    println!("  omniphi-contracts validate");
    println!("  omniphi-contracts build");
    println!("  omniphi-contracts test");
    println!();
    println!("Edit schema.yaml to define your contract.");
    println!("Edit tests.yaml to add test cases.");

    Ok(())
}

fn counter_template_schema(name: &str) -> String {
    format!(
        r#"# {name} — Omniphi Intent Contract
# Generated by omniphi-contracts init

name: {name}
version: "0.1.0"
description: "A simple counter contract"

intents:
  - name: increment
    description: Increase the counter by a given amount
    params:
      - name: amount
        param_type: Uint128
        required: true
        description: Amount to add to the counter
    postconditions:
      - "count increases by amount"

  - name: decrement
    description: Decrease the counter by a given amount
    params:
      - name: amount
        param_type: Uint128
        required: true
        description: Amount to subtract from the counter
    preconditions:
      - "count >= amount"
    postconditions:
      - "count decreases by amount"

state_fields:
  - name: count
    field_type: Uint128
    default_value: "0"
    description: The current counter value

  - name: owner
    field_type: Address
    description: The address that deployed this contract

constraints:
  - name: non_negative_count
    constraint_type: StateCheck
    params:
      field: count
      op: gte
      value: "0"
    applies_to:
      - decrement

max_gas_per_call: 500000
max_state_bytes: 4096
"#,
        name = name
    )
}

fn counter_template_tests(name: &str) -> String {
    format!(
        r#"# Test suite for {name}
# Generated by omniphi-contracts init

contract: {name}
schema_path: schema.yaml

tests:
  - name: increment from zero
    intent: increment
    params:
      amount: "10"
    state_before:
      count: "0"
    state_after:
      count: "10"
    expect_valid: true

  - name: increment from nonzero
    intent: increment
    params:
      amount: "5"
    state_before:
      count: "10"
    state_after:
      count: "15"
    expect_valid: true

  - name: decrement within bounds
    intent: decrement
    params:
      amount: "3"
    state_before:
      count: "10"
    state_after:
      count: "7"
    expect_valid: true

  - name: decrement to zero
    intent: decrement
    params:
      amount: "10"
    state_before:
      count: "10"
    state_after:
      count: "0"
    expect_valid: true

  - name: decrement below zero fails
    intent: decrement
    params:
      amount: "15"
    state_before:
      count: "10"
    state_after:
      count: "-5"
    expect_valid: false
    expect_error: non_negative

  - name: missing amount rejected
    intent: increment
    params: {{}}
    expect_valid: false
    expect_error: missing required
"#,
        name = name
    )
}

fn escrow_template_schema(name: &str) -> String {
    format!(
        r#"# {name} — Omniphi Intent Contract
# Generated by omniphi-contracts init

name: {name}
version: "0.1.0"
description: "An escrow contract with arbiter-controlled release and refund"

intents:
  - name: deposit
    description: Deposit funds into the escrow
    params:
      - name: sender
        param_type: Address
        required: true
        description: The depositor's address
      - name: amount
        param_type: Uint128
        required: true
        description: Amount to deposit
      - name: recipient
        param_type: Address
        required: true
        description: The intended recipient
    preconditions:
      - "deposited == false"
    postconditions:
      - "deposited == true"
      - "amount field is set"

  - name: release
    description: Release escrowed funds to the recipient
    params:
      - name: arbiter
        param_type: Address
        required: true
        description: The arbiter authorizing the release
    preconditions:
      - "deposited == true"
      - "released == false"
      - "refunded == false"
    postconditions:
      - "released == true"

  - name: refund
    description: Refund escrowed funds to the sender
    params:
      - name: arbiter
        param_type: Address
        required: true
        description: The arbiter authorizing the refund
    preconditions:
      - "deposited == true"
      - "released == false"
      - "refunded == false"
    postconditions:
      - "refunded == true"

state_fields:
  - name: deposited
    field_type: Bool
    default_value: "false"
    description: Whether funds have been deposited

  - name: released
    field_type: Bool
    default_value: "false"
    description: Whether funds have been released to recipient

  - name: refunded
    field_type: Bool
    default_value: "false"
    description: Whether funds have been refunded to sender

  - name: arbiter
    field_type: Address
    description: The arbiter who can release or refund

  - name: sender
    field_type: Address
    description: The original depositor

  - name: recipient
    field_type: Address
    description: The intended recipient of the funds

  - name: amount
    field_type: Uint128
    default_value: "0"
    description: The escrowed amount

constraints:
  - name: only_arbiter_release
    constraint_type: OwnershipCheck
    params:
      object_field: arbiter
    applies_to:
      - release

  - name: only_arbiter_refund
    constraint_type: OwnershipCheck
    params:
      object_field: arbiter
    applies_to:
      - refund

  - name: not_already_released
    constraint_type: StateCheck
    params:
      field: released
      op: eq
      value: "false"
    applies_to:
      - refund

  - name: not_already_refunded
    constraint_type: StateCheck
    params:
      field: refunded
      op: eq
      value: "false"
    applies_to:
      - release

max_gas_per_call: 500000
max_state_bytes: 8192
"#,
        name = name
    )
}

fn escrow_template_tests(name: &str) -> String {
    format!(
        r#"# Test suite for {name}
# Generated by omniphi-contracts init

contract: {name}
schema_path: schema.yaml

tests:
  - name: deposit succeeds
    intent: deposit
    params:
      sender: "test_alice"
      amount: "1000"
      recipient: "test_bob"
    state_before:
      deposited: "false"
      released: "false"
      refunded: "false"
      amount: "0"
    state_after:
      deposited: "true"
      released: "false"
      refunded: "false"
      amount: "1000"
    expect_valid: true

  - name: release by arbiter succeeds
    intent: release
    sender: test_arbiter
    params:
      arbiter: "test_arbiter"
    state_before:
      deposited: "true"
      released: "false"
      refunded: "false"
      amount: "1000"
    state_after:
      deposited: "true"
      released: "true"
      refunded: "false"
      amount: "1000"
    expect_valid: true

  - name: refund by arbiter succeeds
    intent: refund
    sender: test_arbiter
    params:
      arbiter: "test_arbiter"
    state_before:
      deposited: "true"
      released: "false"
      refunded: "false"
      amount: "1000"
    state_after:
      deposited: "true"
      released: "false"
      refunded: "true"
      amount: "1000"
    expect_valid: true

  - name: release after refund fails
    intent: release
    sender: test_arbiter
    params:
      arbiter: "test_arbiter"
    state_before:
      deposited: "true"
      released: "false"
      refunded: "true"
      amount: "1000"
    state_after:
      deposited: "true"
      released: "true"
      refunded: "true"
      amount: "1000"
    expect_valid: false
    expect_error: not_already_refunded

  - name: refund after release fails
    intent: refund
    sender: test_arbiter
    params:
      arbiter: "test_arbiter"
    state_before:
      deposited: "true"
      released: "true"
      refunded: "false"
      amount: "1000"
    state_after:
      deposited: "true"
      released: "true"
      refunded: "true"
      amount: "1000"
    expect_valid: false
    expect_error: not_already_released

  - name: release by non-arbiter fails
    intent: release
    sender: test_attacker
    params:
      arbiter: "test_attacker"
    state_before:
      deposited: "true"
      released: "false"
      refunded: "false"
      arbiter: "test_arbiter"
      amount: "1000"
    state_after:
      deposited: "true"
      released: "true"
      refunded: "false"
      amount: "1000"
    expect_valid: false
    expect_error: OwnershipCheck
"#,
        name = name
    )
}

fn blank_template_schema(name: &str) -> String {
    format!(
        r#"# {name} — Omniphi Intent Contract
# Edit this file to define your contract schema.

name: {name}
version: "0.1.0"
description: "TODO: describe your contract"

intents:
  - name: my_intent
    description: "TODO: describe this intent"
    params:
      - name: my_param
        param_type: Uint128
        required: true
        description: "TODO: describe this parameter"
    preconditions: []
    postconditions: []

state_fields:
  - name: my_field
    field_type: Uint128
    default_value: "0"
    description: "TODO: describe this field"

constraints: []

max_gas_per_call: 1000000
max_state_bytes: 65536
"#,
        name = name
    )
}

fn blank_template_tests(name: &str) -> String {
    format!(
        r#"# Test suite for {name}
# Add test cases below.

contract: {name}
schema_path: schema.yaml

tests:
  - name: example test
    intent: my_intent
    params:
      my_param: "100"
    state_before:
      my_field: "0"
    state_after:
      my_field: "100"
    expect_valid: true
"#,
        name = name
    )
}

// ---------------------------------------------------------------------------
// build
// ---------------------------------------------------------------------------

fn cmd_build(path: Option<&Path>, output: Option<&Path>) -> Result<(), String> {
    let project_dir = path.unwrap_or_else(|| Path::new("."));
    let manifest = load_manifest(project_dir)?;

    let schema_path = project_dir.join(&manifest.build.schema);
    if !schema_path.exists() {
        return Err(format!(
            "schema file not found: {}",
            schema_path.display()
        ));
    }

    let output_dir = project_dir.join(&manifest.build.output);
    std::fs::create_dir_all(&output_dir)
        .map_err(|e| format!("failed to create output directory: {}", e))?;

    let output_path = output.map(PathBuf::from).unwrap_or_else(|| {
        output_dir.join(format!("{}.json", manifest.contract.name.to_lowercase()))
    });

    println!("Compiling {}...", schema_path.display());

    let (compiled, warnings) = compiler::compile_file(&schema_path, &output_path)
        .map_err(|e| format!("{}", e))?;

    for warning in &warnings {
        println!("  {}", warning);
    }

    println!();
    println!("  Schema ID:  {}", compiled.schema_id);
    println!("  Name:       {}", compiled.name);
    println!("  Version:    {}", compiled.version);
    println!("  Domain tag: {}", compiled.domain_tag);
    println!("  Intents:    {}", compiled.intent_schemas.len());
    println!("  State:      {} fields", compiled.state_fields.len());
    println!("  Constraints:{}", compiled.constraints.len());
    println!();
    println!("Compiled schema written to: {}", output_path.display());

    Ok(())
}

// ---------------------------------------------------------------------------
// validate
// ---------------------------------------------------------------------------

fn cmd_validate(path: Option<&Path>) -> Result<(), String> {
    let project_dir = path.unwrap_or_else(|| Path::new("."));

    // Support passing a direct YAML file path
    let schema_path = if project_dir.is_file()
        && project_dir
            .extension()
            .map_or(false, |ext| ext == "yaml" || ext == "yml")
    {
        project_dir.to_path_buf()
    } else {
        let manifest = load_manifest(project_dir)?;
        project_dir.join(&manifest.build.schema)
    };

    if !schema_path.exists() {
        return Err(format!(
            "schema file not found: {}",
            schema_path.display()
        ));
    }

    println!("Validating {}...", schema_path.display());

    let schema = compiler::parse_yaml_file(&schema_path)
        .map_err(|e| format!("{}", e))?;

    let messages = validator::validate_schema(&schema);

    let errors: Vec<_> = messages
        .iter()
        .filter(|m| m.severity == validator::Severity::Error)
        .collect();
    let warnings: Vec<_> = messages
        .iter()
        .filter(|m| m.severity == validator::Severity::Warning)
        .collect();

    for msg in &messages {
        println!("  {}", msg);
    }

    println!();
    if errors.is_empty() {
        println!(
            "Schema is valid. ({} warning{})",
            warnings.len(),
            if warnings.len() == 1 { "" } else { "s" }
        );
        Ok(())
    } else {
        Err(format!(
            "Schema has {} error{} and {} warning{}",
            errors.len(),
            if errors.len() == 1 { "" } else { "s" },
            warnings.len(),
            if warnings.len() == 1 { "" } else { "s" }
        ))
    }
}

// ---------------------------------------------------------------------------
// test
// ---------------------------------------------------------------------------

fn cmd_test(path: Option<&Path>, filter: Option<&str>) -> Result<(), String> {
    let project_dir = path.unwrap_or_else(|| Path::new("."));
    let manifest = load_manifest(project_dir)?;

    let schema_path = project_dir.join(&manifest.build.schema);
    let tests_path = project_dir.join(&manifest.build.tests);

    if !schema_path.exists() {
        return Err(format!(
            "schema file not found: {}",
            schema_path.display()
        ));
    }
    if !tests_path.exists() {
        return Err(format!(
            "test file not found: {}",
            tests_path.display()
        ));
    }

    // Parse schema
    let schema = compiler::parse_yaml_file(&schema_path)
        .map_err(|e| format!("schema error: {}", e))?;

    // Validate schema first
    let validation = validator::validate_schema(&schema);
    let schema_errors: Vec<_> = validation
        .iter()
        .filter(|m| m.severity == validator::Severity::Error)
        .collect();
    if !schema_errors.is_empty() {
        eprintln!("Schema has errors; fix them before running tests:");
        for err in &schema_errors {
            eprintln!("  {}", err);
        }
        return Err("schema validation failed".to_string());
    }

    // Parse test suite
    let mut suite = tester::parse_test_file(&tests_path)?;

    // Apply filter if provided
    if let Some(filter_str) = filter {
        let filter_lower = filter_str.to_lowercase();
        suite.tests.retain(|t| t.name.to_lowercase().contains(&filter_lower));
        if suite.tests.is_empty() {
            return Err(format!("no tests match filter '{}'", filter_str));
        }
    }

    println!(
        "Running {} test{} for {}...\n",
        suite.tests.len(),
        if suite.tests.len() == 1 { "" } else { "s" },
        manifest.contract.name
    );

    let summary = tester::run_tests(&schema, &suite);
    print!("{}", summary);

    if summary.is_success() {
        Ok(())
    } else {
        Err(format!("{} test(s) failed", summary.failed))
    }
}

// ---------------------------------------------------------------------------
// deploy
// ---------------------------------------------------------------------------

fn cmd_deploy(
    path: Option<&Path>,
    rpc: &str,
    key: &str,
    chain_id: &str,
    gas: u64,
    dry_run: bool,
) -> Result<(), String> {
    let project_dir = path.unwrap_or_else(|| Path::new("."));

    // Find the compiled JSON
    let json_path = if project_dir.is_file()
        && project_dir
            .extension()
            .map_or(false, |ext| ext == "json")
    {
        project_dir.to_path_buf()
    } else {
        let manifest = load_manifest(project_dir)?;
        let output_dir = project_dir.join(&manifest.build.output);
        let json_name = format!("{}.json", manifest.contract.name.to_lowercase());
        output_dir.join(json_name)
    };

    if !json_path.exists() {
        return Err(format!(
            "compiled schema not found: {} (run 'omniphi-contracts build' first)",
            json_path.display()
        ));
    }

    // Load and parse the compiled schema
    let json_content =
        std::fs::read_to_string(&json_path).map_err(|e| format!("failed to read: {}", e))?;
    let compiled: CompiledSchema =
        serde_json::from_str(&json_content).map_err(|e| format!("invalid compiled schema: {}", e))?;

    // Build the deploy transaction payload
    let deploy_payload = serde_json::json!({
        "body": {
            "messages": [
                {
                    "@type": "/omniphi.contracts.v1.MsgDeployContract",
                    "deployer": format!("(from key: {})", key),
                    "schema": compiled,
                }
            ],
            "memo": format!("Deploy {} v{}", compiled.name, compiled.version),
            "timeout_height": "0",
            "extension_options": [],
            "non_critical_extension_options": []
        },
        "auth_info": {
            "signer_infos": [],
            "fee": {
                "amount": [{"denom": "uomni", "amount": format!("{}", gas * 25)}],
                "gas_limit": format!("{}", gas),
                "payer": "",
                "granter": ""
            }
        },
        "signatures": []
    });

    println!("Deploy Intent Contract");
    println!("======================");
    println!("  Schema ID: {}", compiled.schema_id);
    println!("  Name:      {}", compiled.name);
    println!("  Version:   {}", compiled.version);
    println!("  RPC:       {}", rpc);
    println!("  Chain ID:  {}", chain_id);
    println!("  Key:       {}", key);
    println!("  Gas:       {}", gas);
    println!();

    if dry_run {
        println!("--- DRY RUN: Transaction payload ---");
        println!(
            "{}",
            serde_json::to_string_pretty(&deploy_payload)
                .map_err(|e| format!("JSON serialization failed: {}", e))?
        );
        println!("---");
        println!();
        println!("Dry run complete. No transaction was sent.");
        println!(
            "To deploy for real, remove --dry-run or use:\n  posd tx contracts deploy {} --from {} --chain-id {} --gas {} --node {}",
            json_path.display(), key, chain_id, gas, rpc
        );
    } else {
        // Print the payload that would be sent
        println!("Transaction payload:");
        println!(
            "{}",
            serde_json::to_string_pretty(&deploy_payload)
                .map_err(|e| format!("JSON serialization failed: {}", e))?
        );
        println!();
        println!("NOTE: RPC submission is not yet implemented in this version.");
        println!("To deploy manually, use the posd CLI:");
        println!(
            "  posd tx contracts deploy {} --from {} --chain-id {} --gas {} --node {}",
            json_path.display(), key, chain_id, gas, rpc
        );
    }

    Ok(())
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn load_manifest(project_dir: &Path) -> Result<schema::ProjectManifest, String> {
    let manifest_path = project_dir.join("contract.toml");
    if !manifest_path.exists() {
        return Err(format!(
            "contract.toml not found in {} (is this a contract project directory?)",
            project_dir.display()
        ));
    }

    let content = std::fs::read_to_string(&manifest_path)
        .map_err(|e| format!("failed to read contract.toml: {}", e))?;

    toml::from_str(&content).map_err(|e| format!("failed to parse contract.toml: {}", e))
}

fn write_file(path: &Path, content: &str) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)
            .map_err(|e| format!("failed to create directory {}: {}", parent.display(), e))?;
    }
    std::fs::write(path, content)
        .map_err(|e| format!("failed to write {}: {}", path.display(), e))
}

// Re-export CompiledSchema for the deploy command's deserialization
use schema::CompiledSchema;
