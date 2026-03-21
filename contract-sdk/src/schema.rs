//! Schema generation helpers.

use crate::types::{ContractSchemaDef, IntentMethodDef, ParamDef};

/// Builder for constructing contract schemas.
pub struct SchemaBuilder {
    name: String,
    description: String,
    domain_tag: String,
    methods: Vec<IntentMethodDef>,
    max_gas_per_call: u64,
    max_state_bytes: u64,
}

impl SchemaBuilder {
    pub fn new(name: &str) -> Self {
        SchemaBuilder {
            name: name.to_string(),
            description: String::new(),
            domain_tag: format!("contract.{}", name.to_lowercase()),
            methods: Vec::new(),
            max_gas_per_call: 1_000_000,
            max_state_bytes: 65_536,
        }
    }

    pub fn description(mut self, desc: &str) -> Self {
        self.description = desc.to_string();
        self
    }

    pub fn domain_tag(mut self, tag: &str) -> Self {
        self.domain_tag = tag.to_string();
        self
    }

    pub fn max_gas(mut self, gas: u64) -> Self {
        self.max_gas_per_call = gas;
        self
    }

    pub fn max_state(mut self, bytes: u64) -> Self {
        self.max_state_bytes = bytes;
        self
    }

    /// Add an intent method to the schema.
    pub fn method(mut self, name: &str, description: &str, params: Vec<(&str, &str)>) -> Self {
        self.methods.push(IntentMethodDef {
            method: name.to_string(),
            params: params
                .into_iter()
                .map(|(n, t)| ParamDef {
                    name: n.to_string(),
                    type_hint: t.to_string(),
                })
                .collect(),
            capabilities: vec!["ContractCall".to_string()],
            description: description.to_string(),
        });
        self
    }

    /// Build the schema definition.
    pub fn build(self) -> ContractSchemaDef {
        ContractSchemaDef {
            name: self.name,
            description: self.description,
            domain_tag: self.domain_tag,
            intent_schemas: self.methods,
            max_gas_per_call: self.max_gas_per_call,
            max_state_bytes: self.max_state_bytes,
        }
    }
}
