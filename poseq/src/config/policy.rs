#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum SubmissionClass {
    Transfer,
    Swap,
    YieldAllocate,
    TreasuryRebalance,
    GoalPacket,
    AgentSubmission,
    Other(String),
}

impl SubmissionClass {
    pub fn priority_weight(&self) -> u32 {
        match self {
            SubmissionClass::TreasuryRebalance => 1000,
            SubmissionClass::GoalPacket        => 800,
            SubmissionClass::Swap              => 600,
            SubmissionClass::YieldAllocate     => 500,
            SubmissionClass::Transfer          => 400,
            SubmissionClass::AgentSubmission   => 200,
            SubmissionClass::Other(_)          => 100,
        }
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SubmissionClassPolicy {
    pub class: SubmissionClass,
    pub max_per_batch: usize,    // 0 = unlimited
    pub allowed: bool,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub enum TieBreakRule {
    /// Lower hash bytes win (lexicographic ascending on submission_id)
    LexicographicAscending,
    /// Higher nonce wins
    HigherNonce,
    /// Lower nonce wins
    LowerNonce,
    /// Sender lexicographic ascending
    SenderAscending,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct OrderingPolicyConfig {
    /// Primary: higher priority_weight first
    /// Secondary: apply tie_break_rule
    pub tie_break: TieBreakRule,
    /// If true, same sender's submissions are ordered by nonce ascending within their priority group
    pub enforce_sender_nonce_order: bool,
}

impl Default for OrderingPolicyConfig {
    fn default() -> Self {
        OrderingPolicyConfig {
            tie_break: TieBreakRule::LexicographicAscending,
            enforce_sender_nonce_order: true,
        }
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct BatchPolicy {
    pub max_submissions_per_batch: usize,  // default 256
    pub max_payload_bytes_per_submission: usize,  // default 65536
    pub max_pending_queue_size: usize,     // default 4096; 0 = unlimited
}

impl Default for BatchPolicy {
    fn default() -> Self {
        BatchPolicy {
            max_submissions_per_batch: 256,
            max_payload_bytes_per_submission: 65536,
            max_pending_queue_size: 4096,
        }
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct PoSeqPolicy {
    pub version: u32,
    pub batch: BatchPolicy,
    pub ordering: OrderingPolicyConfig,
    pub class_policies: Vec<SubmissionClassPolicy>,
}

impl PoSeqPolicy {
    pub fn default_policy() -> Self {
        PoSeqPolicy {
            version: 1,
            batch: BatchPolicy::default(),
            ordering: OrderingPolicyConfig::default(),
            class_policies: vec![
                SubmissionClassPolicy { class: SubmissionClass::Transfer,          max_per_batch: 0,   allowed: true },
                SubmissionClassPolicy { class: SubmissionClass::Swap,              max_per_batch: 0,   allowed: true },
                SubmissionClassPolicy { class: SubmissionClass::YieldAllocate,     max_per_batch: 50,  allowed: true },
                SubmissionClassPolicy { class: SubmissionClass::TreasuryRebalance, max_per_batch: 10,  allowed: true },
                SubmissionClassPolicy { class: SubmissionClass::GoalPacket,        max_per_batch: 0,   allowed: true },
                SubmissionClassPolicy { class: SubmissionClass::AgentSubmission,   max_per_batch: 20,  allowed: true },
            ],
        }
    }

    pub fn is_class_allowed(&self, class: &SubmissionClass) -> bool {
        self.class_policies.iter()
            .find(|p| &p.class == class)
            .map(|p| p.allowed)
            .unwrap_or(true)
    }

    pub fn max_per_batch(&self, class: &SubmissionClass) -> usize {
        self.class_policies.iter()
            .find(|p| &p.class == class)
            .map(|p| p.max_per_batch)
            .unwrap_or(0)
    }
}
