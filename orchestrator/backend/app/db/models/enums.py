"""
Centralized Enum Definitions for Omniphi Cloud Database

All enums used across the database models are defined here for consistency
and to avoid circular imports. These enums inherit from both str and enum.Enum
to ensure JSON serialization works correctly with Pydantic.
"""

import enum


# =============================================================================
# REGION & INFRASTRUCTURE ENUMS
# =============================================================================

class RegionCode(str, enum.Enum):
    """
    Supported geographic region codes.

    These map to major cloud provider regions for validator deployment.
    """
    US_EAST = "us-east"
    US_WEST = "us-west"
    EU_CENTRAL = "eu-central"
    EU_WEST = "eu-west"
    ASIA_PACIFIC = "asia-pacific"
    ASIA_SOUTHEAST = "asia-southeast"
    SOUTH_AMERICA = "south-america"
    AUSTRALIA = "australia"


class RegionStatus(str, enum.Enum):
    """Region operational status."""
    ACTIVE = "active"              # Fully operational
    DEGRADED = "degraded"          # Partial functionality
    MAINTENANCE = "maintenance"    # Scheduled maintenance
    OFFLINE = "offline"            # Not available
    COMING_SOON = "coming_soon"    # Planned but not yet available


class MachineType(str, enum.Enum):
    """
    Available machine types for validators.

    Specifications:
    - SMALL: 2 CPU, 4GB RAM, 100GB SSD - Light workloads
    - MEDIUM: 4 CPU, 8GB RAM, 200GB SSD - Standard validators
    - LARGE: 8 CPU, 16GB RAM, 500GB SSD - High-performance
    - XLARGE: 16 CPU, 32GB RAM, 1TB SSD - Enterprise grade
    - CUSTOM: Custom specifications
    """
    SMALL = "small"
    MEDIUM = "medium"
    LARGE = "large"
    XLARGE = "xlarge"
    CUSTOM = "custom"


class ServerStatus(str, enum.Enum):
    """Individual server operational status."""
    ACTIVE = "active"              # Healthy and available
    DEGRADED = "degraded"          # Operating with issues
    OFFLINE = "offline"            # Not reachable
    PROVISIONING = "provisioning"  # Being set up
    DECOMMISSIONING = "decommissioning"  # Being removed


# =============================================================================
# VALIDATOR DOMAIN ENUMS
# =============================================================================

class RunMode(str, enum.Enum):
    """Validator run mode - where the validator node runs."""
    CLOUD = "cloud"    # Orchestrator provisions in cloud
    LOCAL = "local"    # User runs locally with desktop app


class SetupStatus(str, enum.Enum):
    """
    Validator setup request lifecycle status.

    Flow: PENDING -> PROVISIONING -> CONFIGURING -> READY_FOR_CHAIN_TX -> ACTIVE
    Alternative flows: PENDING -> FAILED, ACTIVE -> CANCELLED
    """
    PENDING = "pending"                    # Request received, queued
    PROVISIONING = "provisioning"          # Node being created
    CONFIGURING = "configuring"            # Node being configured
    READY_FOR_CHAIN_TX = "ready_for_chain_tx"  # Awaiting on-chain registration
    READY = "ready"                        # Ready for operation
    ACTIVE = "active"                      # Fully operational validator
    COMPLETED = "completed"                # Successfully completed
    FAILED = "failed"                      # Error during setup
    CANCELLED = "cancelled"                # User cancelled


class NodeStatus(str, enum.Enum):
    """Validator node operational status."""
    STARTING = "starting"      # Container/VM starting
    RUNNING = "running"        # Node process running
    SYNCING = "syncing"        # Catching up with chain
    SYNCED = "synced"          # Fully synchronized
    STOPPED = "stopped"        # Intentionally stopped
    ERROR = "error"            # Error state
    TERMINATED = "terminated"  # Permanently removed
    MIGRATING = "migrating"    # Being moved to another region/server


# =============================================================================
# PROVIDER MARKETPLACE ENUMS
# =============================================================================

class ProviderType(str, enum.Enum):
    """Type of hosting provider."""
    OFFICIAL = "official"          # Omniphi Cloud (first-party)
    FIRST_PARTY = "first_party"    # Alias for official/first-party provider
    COMMUNITY = "community"        # Third-party verified providers
    DECENTRALIZED = "decentralized"  # Decentralized compute networks


class ProviderStatus(str, enum.Enum):
    """Provider availability status."""
    ACTIVE = "active"              # Available for new validators
    INACTIVE = "inactive"          # Not accepting new validators
    MAINTENANCE = "maintenance"    # Temporary maintenance
    SUSPENDED = "suspended"        # Suspended for issues


class ApplicationStatus(str, enum.Enum):
    """Provider application status (for joining marketplace)."""
    PENDING = "pending"            # Submitted, awaiting review
    UNDER_REVIEW = "under_review"  # Being reviewed
    APPROVED = "approved"          # Accepted
    REJECTED = "rejected"          # Declined
    SUSPENDED = "suspended"        # Access revoked


class VerificationCheckType(str, enum.Enum):
    """Types of verification checks for provider applications."""
    API_CONNECTIVITY = "api_connectivity"      # Can reach provider API
    PROVISION_TEST = "provision_test"          # Can provision test node
    HEALTH_CHECK = "health_check"              # Health endpoints work
    LATENCY_TEST = "latency_test"              # Latency acceptable
    SECURITY_AUDIT = "security_audit"          # Security requirements met
    UPTIME_VERIFICATION = "uptime_verification"  # Uptime SLA verified


# =============================================================================
# BILLING ENUMS
# =============================================================================

class BillingPlanType(str, enum.Enum):
    """Type of billing plan."""
    FREE = "free"              # Free tier
    STARTER = "starter"        # Entry-level paid
    PROFESSIONAL = "professional"  # Mid-tier
    BUSINESS = "business"      # Business tier
    ENTERPRISE = "enterprise"  # High-tier
    CUSTOM = "custom"          # Custom pricing


class BillingCycle(str, enum.Enum):
    """Billing cycle frequency."""
    HOURLY = "hourly"
    DAILY = "daily"
    WEEKLY = "weekly"
    MONTHLY = "monthly"
    YEARLY = "yearly"


class SubscriptionStatus(str, enum.Enum):
    """Billing subscription status."""
    ACTIVE = "active"              # Currently active
    TRIALING = "trialing"          # In trial period
    PAST_DUE = "past_due"          # Payment overdue
    CANCELLED = "cancelled"        # User cancelled
    UNPAID = "unpaid"              # Failed payment
    PAUSED = "paused"              # Temporarily paused


class PaymentMethod(str, enum.Enum):
    """Supported payment methods."""
    STRIPE = "stripe"              # Credit card via Stripe
    CRYPTO_ETH = "crypto_eth"      # Ethereum
    CRYPTO_BTC = "crypto_btc"      # Bitcoin
    CRYPTO_USDC = "crypto_usdc"    # USDC stablecoin
    CRYPTO_OMNI = "crypto_omni"    # Native OMNI token
    WIRE = "wire"                  # Bank wire transfer


class PaymentStatus(str, enum.Enum):
    """Payment transaction status."""
    PENDING = "pending"            # Awaiting processing
    PROCESSING = "processing"      # Being processed
    SUCCEEDED = "succeeded"        # Successfully completed
    FAILED = "failed"              # Payment failed
    REFUNDED = "refunded"          # Fully refunded
    PARTIALLY_REFUNDED = "partially_refunded"  # Partial refund


class InvoiceStatus(str, enum.Enum):
    """Invoice status."""
    DRAFT = "draft"                # Being prepared
    OPEN = "open"                  # Sent to customer
    PAID = "paid"                  # Fully paid
    VOID = "void"                  # Cancelled
    UNCOLLECTIBLE = "uncollectible"  # Cannot be collected


# =============================================================================
# UPGRADE ENUMS
# =============================================================================

class UpgradeStatus(str, enum.Enum):
    """Chain upgrade status."""
    SCHEDULED = "scheduled"        # Announced, not yet active
    PREPARING = "preparing"        # Nodes being prepared
    IN_PROGRESS = "in_progress"    # Upgrade happening
    COMPLETED = "completed"        # Successfully completed
    FAILED = "failed"              # Upgrade failed
    CANCELLED = "cancelled"        # Upgrade cancelled


class RolloutStatus(str, enum.Enum):
    """Per-region rollout status."""
    PENDING = "pending"            # Not started
    IN_PROGRESS = "in_progress"    # Rolling out
    COMPLETED = "completed"        # Done
    FAILED = "failed"              # Failed
    ROLLED_BACK = "rolled_back"    # Reverted


# =============================================================================
# MONITORING & SRE ENUMS
# =============================================================================

class IncidentSeverity(str, enum.Enum):
    """Incident severity levels (following industry standards)."""
    CRITICAL = "critical"      # P1 - Service down, immediate action
    HIGH = "high"              # P2 - Major impact, urgent
    MEDIUM = "medium"          # P3 - Moderate impact
    LOW = "low"                # P4 - Minor impact
    INFO = "info"              # Informational


class IncidentStatus(str, enum.Enum):
    """Incident lifecycle status."""
    OPEN = "open"                  # New incident
    ACKNOWLEDGED = "acknowledged"  # Someone is looking
    INVESTIGATING = "investigating"  # Under investigation
    IDENTIFIED = "identified"      # Root cause found
    MONITORING = "monitoring"      # Fix deployed, monitoring
    RESOLVED = "resolved"          # Fully resolved
    CLOSED = "closed"              # Closed permanently


class AlertType(str, enum.Enum):
    """Types of monitoring alerts."""
    NODE_DOWN = "node_down"            # Node not responding
    NODE_SYNCING = "node_syncing"      # Node behind chain
    HIGH_CPU = "high_cpu"              # CPU above threshold
    HIGH_MEMORY = "high_memory"        # Memory above threshold
    HIGH_DISK = "high_disk"            # Disk above threshold
    LOW_PEERS = "low_peers"            # Few peers connected
    MISSED_BLOCKS = "missed_blocks"    # Validator missing blocks
    JAILED = "jailed"                  # Validator jailed
    SLASHED = "slashed"                # Validator slashed
    CERTIFICATE_EXPIRY = "certificate_expiry"  # TLS cert expiring
    UPGRADE_REQUIRED = "upgrade_required"      # Chain upgrade needed
