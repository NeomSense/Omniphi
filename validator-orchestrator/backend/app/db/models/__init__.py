"""
Omniphi Cloud Database Models

This module exports all SQLAlchemy models for the Omniphi Cloud + Validator Orchestrator ecosystem.
These models power the production-grade database layer including:

- Multi-region infrastructure (regions, servers, pools)
- Validator domain (setup requests, nodes, heartbeats)
- Provider marketplace
- Billing system
- Snapshots
- Upgrades & rollouts
- Monitoring & SRE (metrics, incidents)

All models use UUIDs as primary keys and follow SQLAlchemy 2.0 patterns.
"""

# Enums - Export all enums for use throughout the application
from app.db.models.enums import (
    # Region enums
    RegionCode,
    RegionStatus,
    MachineType,
    ServerStatus,
    # Validator enums
    RunMode,
    SetupStatus,
    NodeStatus,
    # Provider enums
    ProviderType,
    ProviderStatus,
    ApplicationStatus,
    VerificationCheckType,
    # Billing enums
    BillingPlanType,
    BillingCycle,
    SubscriptionStatus,
    PaymentMethod,
    PaymentStatus,
    InvoiceStatus,
    # Upgrade enums
    RolloutStatus,
    UpgradeStatus,
    # Monitoring enums
    IncidentSeverity,
    IncidentStatus,
    AlertType,
)

# Region & Infrastructure models
from app.db.models.region import Region
from app.db.models.region_server import RegionServer
from app.db.models.server_pool import ServerPool

# Validator domain models
from app.db.models.validator_setup_request import ValidatorSetupRequest
from app.db.models.validator_node import ValidatorNode
from app.db.models.local_validator_heartbeat import LocalValidatorHeartbeat

# Provider marketplace models
from app.db.models.provider import Provider
from app.db.models.provider_pricing_tier import ProviderPricingTier
from app.db.models.provider_metrics import ProviderMetrics
from app.db.models.provider_application import ProviderApplication
from app.db.models.provider_verification import ProviderVerification
from app.db.models.provider_sla import ProviderSLA
from app.db.models.provider_review import ProviderReview

# Billing models
from app.db.models.billing_account import BillingAccount
from app.db.models.billing_plan import BillingPlan
from app.db.models.billing_subscription import BillingSubscription
from app.db.models.billing_invoice import BillingInvoice
from app.db.models.billing_payment import BillingPayment
from app.db.models.billing_usage import BillingUsage

# Snapshot models
from app.db.models.snapshot import Snapshot

# Upgrade models
from app.db.models.upgrade import Upgrade
from app.db.models.upgrade_rollout import UpgradeRollout

# Monitoring & SRE models
from app.db.models.node_metrics import NodeMetrics
from app.db.models.incident import Incident

__all__ = [
    # Enums
    "RegionCode",
    "RegionStatus",
    "MachineType",
    "ServerStatus",
    "RunMode",
    "SetupStatus",
    "NodeStatus",
    "ProviderType",
    "ProviderStatus",
    "ApplicationStatus",
    "VerificationCheckType",
    "BillingPlanType",
    "BillingCycle",
    "SubscriptionStatus",
    "PaymentMethod",
    "PaymentStatus",
    "InvoiceStatus",
    "RolloutStatus",
    "UpgradeStatus",
    "IncidentSeverity",
    "IncidentStatus",
    "AlertType",
    # Region & Infrastructure
    "Region",
    "RegionServer",
    "ServerPool",
    # Validator Domain
    "ValidatorSetupRequest",
    "ValidatorNode",
    "LocalValidatorHeartbeat",
    # Provider Marketplace
    "Provider",
    "ProviderPricingTier",
    "ProviderMetrics",
    "ProviderApplication",
    "ProviderVerification",
    "ProviderSLA",
    "ProviderReview",
    # Billing
    "BillingAccount",
    "BillingPlan",
    "BillingSubscription",
    "BillingInvoice",
    "BillingPayment",
    "BillingUsage",
    # Snapshots
    "Snapshot",
    # Upgrades
    "Upgrade",
    "UpgradeRollout",
    # Monitoring & SRE
    "NodeMetrics",
    "Incident",
]
