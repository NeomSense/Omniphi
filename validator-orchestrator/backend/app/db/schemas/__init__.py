"""
Pydantic Schemas for Omniphi Cloud Database

This module exports all Pydantic schemas for request/response handling
and data validation across the database layer.
"""

# Base schemas
from app.db.schemas.base import (
    BaseSchema,
    TimestampSchema,
    PaginatedResponse,
    SuccessResponse,
    ErrorResponse,
)

# Region schemas
from app.db.schemas.region import (
    RegionBase,
    RegionCreate,
    RegionUpdate,
    RegionResponse,
    RegionListResponse,
    ServerPoolBase,
    ServerPoolCreate,
    ServerPoolResponse,
    RegionServerBase,
    RegionServerCreate,
    RegionServerResponse,
)

# Validator schemas
from app.db.schemas.validator import (
    ValidatorSetupRequestBase,
    ValidatorSetupRequestCreate,
    ValidatorSetupRequestUpdate,
    ValidatorSetupRequestResponse,
    ValidatorNodeBase,
    ValidatorNodeCreate,
    ValidatorNodeUpdate,
    ValidatorNodeResponse,
    LocalValidatorHeartbeatCreate,
    LocalValidatorHeartbeatResponse,
)

# Provider schemas
from app.db.schemas.provider import (
    ProviderBase,
    ProviderCreate,
    ProviderUpdate,
    ProviderResponse,
    ProviderListResponse,
    ProviderPricingTierCreate,
    ProviderPricingTierResponse,
    ProviderSLACreate,
    ProviderSLAResponse,
    ProviderReviewCreate,
    ProviderReviewResponse,
    ProviderApplicationCreate,
    ProviderApplicationResponse,
)

# Billing schemas
from app.db.schemas.billing import (
    BillingAccountBase,
    BillingAccountCreate,
    BillingAccountUpdate,
    BillingAccountResponse,
    BillingPlanBase,
    BillingPlanCreate,
    BillingPlanResponse,
    BillingSubscriptionCreate,
    BillingSubscriptionResponse,
    BillingInvoiceResponse,
    BillingPaymentCreate,
    BillingPaymentResponse,
)

# Snapshot schemas
from app.db.schemas.snapshot import (
    SnapshotBase,
    SnapshotCreate,
    SnapshotResponse,
    SnapshotListResponse,
)

# Upgrade schemas
from app.db.schemas.upgrade import (
    UpgradeBase,
    UpgradeCreate,
    UpgradeUpdate,
    UpgradeResponse,
    UpgradeRolloutResponse,
)

# Monitoring schemas
from app.db.schemas.monitoring import (
    NodeMetricsCreate,
    NodeMetricsResponse,
    IncidentBase,
    IncidentCreate,
    IncidentUpdate,
    IncidentResponse,
)

__all__ = [
    # Base
    "BaseSchema",
    "TimestampSchema",
    "PaginatedResponse",
    "SuccessResponse",
    "ErrorResponse",
    # Region
    "RegionBase",
    "RegionCreate",
    "RegionUpdate",
    "RegionResponse",
    "RegionListResponse",
    "ServerPoolBase",
    "ServerPoolCreate",
    "ServerPoolResponse",
    "RegionServerBase",
    "RegionServerCreate",
    "RegionServerResponse",
    # Validator
    "ValidatorSetupRequestBase",
    "ValidatorSetupRequestCreate",
    "ValidatorSetupRequestUpdate",
    "ValidatorSetupRequestResponse",
    "ValidatorNodeBase",
    "ValidatorNodeCreate",
    "ValidatorNodeUpdate",
    "ValidatorNodeResponse",
    "LocalValidatorHeartbeatCreate",
    "LocalValidatorHeartbeatResponse",
    # Provider
    "ProviderBase",
    "ProviderCreate",
    "ProviderUpdate",
    "ProviderResponse",
    "ProviderListResponse",
    "ProviderPricingTierCreate",
    "ProviderPricingTierResponse",
    "ProviderSLACreate",
    "ProviderSLAResponse",
    "ProviderReviewCreate",
    "ProviderReviewResponse",
    "ProviderApplicationCreate",
    "ProviderApplicationResponse",
    # Billing
    "BillingAccountBase",
    "BillingAccountCreate",
    "BillingAccountUpdate",
    "BillingAccountResponse",
    "BillingPlanBase",
    "BillingPlanCreate",
    "BillingPlanResponse",
    "BillingSubscriptionCreate",
    "BillingSubscriptionResponse",
    "BillingInvoiceResponse",
    "BillingPaymentCreate",
    "BillingPaymentResponse",
    # Snapshot
    "SnapshotBase",
    "SnapshotCreate",
    "SnapshotResponse",
    "SnapshotListResponse",
    # Upgrade
    "UpgradeBase",
    "UpgradeCreate",
    "UpgradeUpdate",
    "UpgradeResponse",
    "UpgradeRolloutResponse",
    # Monitoring
    "NodeMetricsCreate",
    "NodeMetricsResponse",
    "IncidentBase",
    "IncidentCreate",
    "IncidentUpdate",
    "IncidentResponse",
]
