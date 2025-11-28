"""
CRUD Repository Layer for Omniphi Cloud Database

This module provides repository classes for database operations following
the Repository pattern. Each repository provides:
- create: Create new records
- get: Get single record by ID
- get_by_*: Get records by specific fields
- list: List records with filtering and pagination
- update: Update existing records
- delete: Delete records

All repositories support both sync and async operations.
"""

from app.db.crud.base import BaseRepository
from app.db.crud.region import RegionRepository, ServerPoolRepository, RegionServerRepository
from app.db.crud.validator import (
    ValidatorSetupRequestRepository,
    ValidatorNodeRepository,
    LocalValidatorHeartbeatRepository,
)
from app.db.crud.provider import (
    ProviderRepository,
    ProviderPricingTierRepository,
    ProviderMetricsRepository,
    ProviderApplicationRepository,
    ProviderVerificationRepository,
    ProviderSLARepository,
    ProviderReviewRepository,
)
from app.db.crud.billing import (
    BillingAccountRepository,
    BillingPlanRepository,
    BillingSubscriptionRepository,
    BillingInvoiceRepository,
    BillingPaymentRepository,
    BillingUsageRepository,
)
from app.db.crud.snapshot import SnapshotRepository
from app.db.crud.upgrade import UpgradeRepository, UpgradeRolloutRepository
from app.db.crud.monitoring import NodeMetricsRepository, IncidentRepository

__all__ = [
    # Base
    "BaseRepository",
    # Region
    "RegionRepository",
    "ServerPoolRepository",
    "RegionServerRepository",
    # Validator
    "ValidatorSetupRequestRepository",
    "ValidatorNodeRepository",
    "LocalValidatorHeartbeatRepository",
    # Provider
    "ProviderRepository",
    "ProviderPricingTierRepository",
    "ProviderMetricsRepository",
    "ProviderApplicationRepository",
    "ProviderVerificationRepository",
    "ProviderSLARepository",
    "ProviderReviewRepository",
    # Billing
    "BillingAccountRepository",
    "BillingPlanRepository",
    "BillingSubscriptionRepository",
    "BillingInvoiceRepository",
    "BillingPaymentRepository",
    "BillingUsageRepository",
    # Snapshot
    "SnapshotRepository",
    # Upgrade
    "UpgradeRepository",
    "UpgradeRolloutRepository",
    # Monitoring
    "NodeMetricsRepository",
    "IncidentRepository",
]
