"""
Provider Pydantic Schemas

Schemas for provider marketplace operations.
"""

from datetime import datetime
from typing import Any, Dict, List, Optional
from uuid import UUID

from pydantic import Field, field_validator

from app.db.schemas.base import BaseSchema, PaginatedResponse


# =============================================================================
# PROVIDER SCHEMAS
# =============================================================================

class ProviderBase(BaseSchema):
    """Base schema for provider."""

    code: str = Field(..., min_length=2, max_length=50, description="Provider code")
    name: str = Field(..., min_length=2, max_length=100, description="Provider name")
    display_name: str = Field(..., min_length=2, max_length=100, description="Display name")
    description: Optional[str] = Field(None, description="Provider description")


class ProviderCreate(ProviderBase):
    """Schema for creating a provider."""

    tagline: Optional[str] = Field(None, max_length=200)
    logo_url: Optional[str] = Field(None, max_length=500)
    website_url: Optional[str] = Field(None, max_length=500)
    documentation_url: Optional[str] = Field(None, max_length=500)
    provider_type: str = Field("community", description="Provider type")
    is_official: bool = Field(False)
    is_verified: bool = Field(False)
    api_endpoint: Optional[str] = Field(None, max_length=500)
    api_version: Optional[str] = Field(None, max_length=20)
    supported_regions: List[str] = Field(default_factory=list)
    supported_machine_types: List[str] = Field(default_factory=list)
    features: Dict[str, Any] = Field(default_factory=dict)
    price_monthly_min: Optional[float] = Field(None, ge=0)
    price_monthly_max: Optional[float] = Field(None, ge=0)
    accepts_crypto: bool = Field(False)
    supported_crypto: List[str] = Field(default_factory=list)
    support_email: Optional[str] = Field(None, max_length=255)


class ProviderUpdate(BaseSchema):
    """Schema for updating a provider."""

    name: Optional[str] = Field(None, min_length=2, max_length=100)
    display_name: Optional[str] = Field(None, min_length=2, max_length=100)
    description: Optional[str] = None
    tagline: Optional[str] = Field(None, max_length=200)
    logo_url: Optional[str] = Field(None, max_length=500)
    website_url: Optional[str] = Field(None, max_length=500)
    status: Optional[str] = None
    is_active: Optional[bool] = None
    is_accepting_new: Optional[bool] = None
    is_verified: Optional[bool] = None
    is_featured: Optional[bool] = None
    supported_regions: Optional[List[str]] = None
    supported_machine_types: Optional[List[str]] = None
    features: Optional[Dict[str, Any]] = None
    price_monthly_min: Optional[float] = Field(None, ge=0)
    price_monthly_max: Optional[float] = Field(None, ge=0)


class ProviderResponse(ProviderBase):
    """Schema for provider response."""

    id: UUID
    tagline: Optional[str]
    logo_url: Optional[str]
    website_url: Optional[str]
    documentation_url: Optional[str]
    provider_type: str
    is_official: bool
    is_verified: bool
    is_featured: bool
    api_endpoint: Optional[str]
    api_version: Optional[str]
    webhook_url: Optional[str]
    supported_regions: List[str]
    supported_machine_types: List[str]
    features: Dict[str, Any]
    status: str
    is_active: bool
    is_accepting_new: bool
    price_monthly_min: Optional[float]
    price_monthly_max: Optional[float]
    currency: str
    accepts_crypto: bool
    supported_crypto: List[str]
    avg_provision_time_seconds: Optional[float]
    uptime_percent: float
    avg_latency_ms: Optional[float]
    rating: float
    rating_count: int
    review_count: int
    total_validators: int
    active_validators: int
    total_customers: int
    support_email: Optional[str]
    support_url: Optional[str]
    created_at: datetime
    updated_at: datetime
    verified_at: Optional[datetime]

    # Computed
    is_available: bool
    price_range: str


class ProviderSummary(BaseSchema):
    """Compact provider summary."""

    id: UUID
    code: str
    display_name: str
    logo_url: Optional[str]
    provider_type: str
    is_official: bool
    is_verified: bool
    status: str
    rating: float
    active_validators: int
    price_range: str


class ProviderListResponse(PaginatedResponse[ProviderResponse]):
    """Paginated provider list."""
    pass


# =============================================================================
# PROVIDER PRICING TIER SCHEMAS
# =============================================================================

class ProviderPricingTierCreate(BaseSchema):
    """Schema for creating a pricing tier."""

    provider_id: UUID
    tier_code: str = Field(..., min_length=2, max_length=50)
    name: str = Field(..., min_length=2, max_length=100)
    description: Optional[str] = None
    cpu_cores: int = Field(..., ge=1)
    memory_gb: int = Field(..., ge=1)
    disk_gb: int = Field(..., ge=10)
    disk_type: str = Field("ssd")
    bandwidth_gbps: float = Field(1.0, ge=0)
    hourly_price: float = Field(..., ge=0)
    monthly_price: float = Field(..., ge=0)
    yearly_price: Optional[float] = Field(None, ge=0)
    setup_fee: float = Field(0.0, ge=0)
    currency: str = Field("USD")
    is_available: bool = Field(True)
    available_in_regions: List[str] = Field(default_factory=list)
    is_recommended: bool = Field(False)
    display_order: int = Field(0)


class ProviderPricingTierResponse(BaseSchema):
    """Schema for pricing tier response."""

    id: UUID
    provider_id: UUID
    tier_code: str
    name: str
    description: Optional[str]
    display_order: int
    cpu_cores: int
    memory_gb: int
    disk_gb: int
    disk_type: str
    bandwidth_gbps: float
    bandwidth_tb_month: Optional[float]
    hourly_price: float
    monthly_price: float
    yearly_price: Optional[float]
    setup_fee: float
    currency: str
    is_available: bool
    available_in_regions: List[str]
    max_instances: Optional[int]
    current_instances: int
    is_promotional: bool
    promotional_price: Optional[float]
    promotional_ends_at: Optional[datetime]
    is_recommended: bool
    recommended_for: List[str]
    features: Dict[str, Any]
    specs: Dict[str, Any]
    created_at: datetime
    updated_at: datetime

    # Computed
    specs_summary: str
    effective_monthly_price: float
    has_capacity: bool


# =============================================================================
# PROVIDER SLA SCHEMAS
# =============================================================================

class ProviderSLACreate(BaseSchema):
    """Schema for creating a provider SLA."""

    provider_id: UUID
    name: str = Field(..., min_length=2, max_length=100)
    version: str = Field("1.0")
    description: Optional[str] = None
    uptime_guarantee: float = Field(99.9, ge=90, le=100)
    response_time_hours: int = Field(4, ge=1)
    resolution_time_hours: int = Field(24, ge=1)
    critical_response_time_hours: int = Field(1, ge=1)
    critical_resolution_time_hours: int = Field(4, ge=1)
    max_latency_ms: Optional[float] = Field(None, ge=0)
    penalty_per_hour_down: float = Field(0.0, ge=0)
    max_monthly_penalty: float = Field(0.0, ge=0)
    credit_tiers: List[Dict[str, float]] = Field(default_factory=list)
    exclusions: List[str] = Field(default_factory=list)
    support_hours: str = Field("24x7")
    support_channels: List[str] = Field(default_factory=list)
    effective_from: Optional[datetime] = None


class ProviderSLAResponse(BaseSchema):
    """Schema for provider SLA response."""

    id: UUID
    provider_id: UUID
    name: str
    version: str
    description: Optional[str]
    uptime_guarantee: float
    availability_calculation_method: str
    excluded_maintenance_windows: bool
    max_scheduled_maintenance_hours: int
    response_time_hours: int
    resolution_time_hours: int
    critical_response_time_hours: int
    critical_resolution_time_hours: int
    max_latency_ms: Optional[float]
    max_provision_time_minutes: Optional[int]
    penalty_per_hour_down: float
    max_monthly_penalty: float
    credit_tiers: List[Dict[str, float]]
    exclusions: List[str]
    support_hours: str
    support_channels: List[str]
    reporting_frequency: str
    is_active: bool
    is_default: bool
    effective_from: datetime
    effective_until: Optional[datetime]
    terms_url: Optional[str]
    created_at: datetime
    updated_at: datetime

    # Computed
    monthly_downtime_budget_minutes: float


# =============================================================================
# PROVIDER REVIEW SCHEMAS
# =============================================================================

class ProviderReviewCreate(BaseSchema):
    """Schema for creating a provider review."""

    provider_id: UUID
    user_id: str = Field(..., min_length=1, max_length=100)
    rating: float = Field(..., ge=1, le=5, description="Rating (1-5)")
    rating_reliability: Optional[float] = Field(None, ge=1, le=5)
    rating_performance: Optional[float] = Field(None, ge=1, le=5)
    rating_support: Optional[float] = Field(None, ge=1, le=5)
    rating_value: Optional[float] = Field(None, ge=1, le=5)
    title: Optional[str] = Field(None, max_length=200)
    comment: Optional[str] = None
    pros: List[str] = Field(default_factory=list)
    cons: List[str] = Field(default_factory=list)
    validators_hosted: Optional[int] = Field(None, ge=0)
    months_used: Optional[int] = Field(None, ge=0)
    region_used: Optional[str] = None
    plan_used: Optional[str] = None


class ProviderReviewResponse(BaseSchema):
    """Schema for provider review response."""

    id: UUID
    provider_id: UUID
    user_id: str
    wallet_address: Optional[str]
    reviewer_name: Optional[str]
    rating: float
    rating_reliability: Optional[float]
    rating_performance: Optional[float]
    rating_support: Optional[float]
    rating_value: Optional[float]
    title: Optional[str]
    comment: Optional[str]
    pros: List[str]
    cons: List[str]
    validators_hosted: Optional[int]
    months_used: Optional[int]
    region_used: Optional[str]
    plan_used: Optional[str]
    use_case: Optional[str]
    is_verified: bool
    verified_at: Optional[datetime]
    is_visible: bool
    is_featured: bool
    provider_response: Optional[str]
    provider_responded_at: Optional[datetime]
    helpful_count: int
    not_helpful_count: int
    created_at: datetime
    updated_at: datetime

    # Computed
    helpfulness_score: float
    is_long_term_user: bool


# =============================================================================
# PROVIDER APPLICATION SCHEMAS
# =============================================================================

class ProviderApplicationCreate(BaseSchema):
    """Schema for creating a provider application."""

    company_name: str = Field(..., min_length=2, max_length=200)
    company_website: Optional[str] = Field(None, max_length=500)
    company_description: Optional[str] = None
    contact_name: str = Field(..., min_length=2, max_length=100)
    contact_email: str = Field(..., max_length=255)
    contact_phone: Optional[str] = Field(None, max_length=50)
    contact_role: Optional[str] = Field(None, max_length=100)
    proposed_code: str = Field(..., min_length=2, max_length=50)
    proposed_name: str = Field(..., min_length=2, max_length=100)
    description: str = Field(..., min_length=10)
    logo_url: Optional[str] = Field(None, max_length=500)
    api_endpoint: str = Field(..., max_length=500)
    api_documentation_url: Optional[str] = Field(None, max_length=500)
    supported_regions: List[str] = Field(default_factory=list)
    proposed_pricing: List[Dict[str, Any]] = Field(default_factory=list)
    uptime_guarantee: float = Field(99.9, ge=90, le=100)
    response_time_sla_hours: int = Field(24, ge=1)
    data_retention_days: int = Field(30, ge=1)
    security_certifications: List[str] = Field(default_factory=list)


class ProviderApplicationResponse(BaseSchema):
    """Schema for provider application response."""

    id: UUID
    company_name: str
    company_website: Optional[str]
    company_description: Optional[str]
    company_size: Optional[str]
    contact_name: str
    contact_email: str
    contact_phone: Optional[str]
    contact_role: Optional[str]
    proposed_code: str
    proposed_name: str
    description: str
    logo_url: Optional[str]
    api_endpoint: str
    api_documentation_url: Optional[str]
    api_auth_type: Optional[str]
    supported_regions: List[str]
    proposed_pricing: List[Dict[str, Any]]
    infrastructure_type: Optional[str]
    uptime_guarantee: float
    response_time_sla_hours: int
    resolution_time_sla_hours: int
    data_retention_days: int
    security_certifications: List[str]
    status: str
    status_reason: Optional[str]
    reviewed_by: Optional[str]
    reviewed_at: Optional[datetime]
    verification_results: Dict[str, Any]
    verification_score: Optional[float]
    provider_id: Optional[UUID]
    submitted_at: datetime
    approved_at: Optional[datetime]
    rejected_at: Optional[datetime]
    created_at: datetime
    updated_at: datetime

    # Computed
    is_pending: bool
    is_approved: bool
    is_rejected: bool
