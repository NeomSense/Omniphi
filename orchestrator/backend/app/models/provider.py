"""
Provider Models

Database models for cloud providers, pricing tiers, and provider applications.
"""

import enum
from datetime import datetime
from typing import Optional
from uuid import uuid4

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    ForeignKey,
    Enum,
    JSON,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship

from app.database import Base


class ProviderStatus(str, enum.Enum):
    """Provider status"""
    ACTIVE = "active"
    INACTIVE = "inactive"
    MAINTENANCE = "maintenance"
    SUSPENDED = "suspended"


class ApplicationStatus(str, enum.Enum):
    """Provider application status"""
    PENDING = "pending"
    UNDER_REVIEW = "under_review"
    APPROVED = "approved"
    REJECTED = "rejected"
    SUSPENDED = "suspended"


class VerificationCheckType(str, enum.Enum):
    """Types of verification checks"""
    API_CONNECTIVITY = "api_connectivity"
    PROVISION_TEST = "provision_test"
    HEALTH_CHECK = "health_check"
    LATENCY_TEST = "latency_test"
    SECURITY_AUDIT = "security_audit"


class Provider(Base):
    """
    Cloud provider registration.

    Represents a hosting provider in the marketplace.
    """
    __tablename__ = "providers"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Provider identification
    code = Column(String(50), nullable=False, unique=True, index=True)
    name = Column(String(100), nullable=False)
    display_name = Column(String(100), nullable=False)
    description = Column(Text, nullable=True)
    logo_url = Column(String(500), nullable=True)
    website_url = Column(String(500), nullable=True)

    # Provider type
    is_first_party = Column(Boolean, nullable=False, default=False)  # True for Omniphi Cloud
    is_community = Column(Boolean, nullable=False, default=False)    # True for third-party

    # API configuration
    api_endpoint = Column(String(500), nullable=True)
    api_version = Column(String(20), nullable=True)
    auth_type = Column(String(50), nullable=True)  # api_key, oauth, etc.

    # Capabilities
    supported_regions = Column(JSON, nullable=False, default=list)
    supported_machine_types = Column(JSON, nullable=False, default=list)
    features = Column(JSON, nullable=False, default=dict)
    # e.g., {"auto_scaling": true, "snapshot_restore": true, "monitoring": true}

    # Status
    status = Column(Enum(ProviderStatus), nullable=False, default=ProviderStatus.ACTIVE)

    # Performance metrics (cached)
    avg_provision_time_seconds = Column(Float, nullable=True)
    uptime_percent = Column(Float, nullable=True, default=99.9)
    avg_latency_ms = Column(Float, nullable=True)

    # Rating
    rating = Column(Float, nullable=True, default=5.0)
    review_count = Column(Integer, nullable=False, default=0)

    # Usage stats
    total_validators = Column(Integer, nullable=False, default=0)
    active_validators = Column(Integer, nullable=False, default=0)

    # Metadata
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    pricing_tiers = relationship("ProviderPricingTier", back_populates="provider", cascade="all, delete-orphan")
    metrics = relationship("ProviderMetrics", back_populates="provider", cascade="all, delete-orphan")

    def __repr__(self):
        return f"<Provider {self.code}: {self.display_name}>"


class ProviderPricingTier(Base):
    """
    Provider pricing tier.

    Defines machine types and pricing for a provider.
    """
    __tablename__ = "provider_pricing_tiers"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    provider_id = Column(UUID(as_uuid=True), ForeignKey("providers.id", ondelete="CASCADE"), nullable=False)

    # Tier identification
    tier_code = Column(String(50), nullable=False)
    name = Column(String(100), nullable=False)
    description = Column(Text, nullable=True)

    # Specifications
    cpu_cores = Column(Integer, nullable=False)
    memory_gb = Column(Integer, nullable=False)
    disk_gb = Column(Integer, nullable=False)
    bandwidth_gbps = Column(Float, nullable=True, default=1.0)

    # Pricing
    hourly_price = Column(Float, nullable=False)
    monthly_price = Column(Float, nullable=False)
    setup_fee = Column(Float, nullable=False, default=0.0)
    currency = Column(String(3), nullable=False, default="USD")

    # Availability
    is_available = Column(Boolean, nullable=False, default=True)
    available_in_regions = Column(JSON, nullable=False, default=list)

    # Metadata
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    provider = relationship("Provider", back_populates="pricing_tiers")

    __table_args__ = (
        Index("ix_provider_pricing_provider_tier", "provider_id", "tier_code"),
    )

    def __repr__(self):
        return f"<ProviderPricingTier {self.tier_code} @ ${self.monthly_price}/mo>"


class ProviderMetrics(Base):
    """
    Provider performance metrics.

    Tracks performance data for providers over time.
    """
    __tablename__ = "provider_metrics"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    provider_id = Column(UUID(as_uuid=True), ForeignKey("providers.id", ondelete="CASCADE"), nullable=False)
    region = Column(String(50), nullable=False)

    # Performance metrics
    avg_latency_ms = Column(Float, nullable=False, default=0.0)
    p95_latency_ms = Column(Float, nullable=False, default=0.0)
    p99_latency_ms = Column(Float, nullable=False, default=0.0)

    # Availability
    uptime_percent = Column(Float, nullable=False, default=100.0)
    success_rate = Column(Float, nullable=False, default=100.0)

    # Provisioning stats
    avg_provision_time_seconds = Column(Float, nullable=False, default=0.0)
    provision_success_rate = Column(Float, nullable=False, default=100.0)

    # Resource metrics
    total_validators = Column(Integer, nullable=False, default=0)
    active_validators = Column(Integer, nullable=False, default=0)
    failed_validators = Column(Integer, nullable=False, default=0)

    # Timestamps
    recorded_at = Column(DateTime, nullable=False, default=datetime.utcnow, index=True)

    # Relationships
    provider = relationship("Provider", back_populates="metrics")

    __table_args__ = (
        Index("ix_provider_metrics_provider_region_time", "provider_id", "region", "recorded_at"),
    )

    def __repr__(self):
        return f"<ProviderMetrics {self.provider_id} in {self.region}>"


class ProviderApplication(Base):
    """
    Third-party provider application.

    For community providers wanting to join the marketplace.
    """
    __tablename__ = "provider_applications"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Applicant info
    company_name = Column(String(200), nullable=False)
    contact_name = Column(String(100), nullable=False)
    contact_email = Column(String(255), nullable=False)
    contact_phone = Column(String(50), nullable=True)
    website_url = Column(String(500), nullable=True)

    # Provider details
    proposed_code = Column(String(50), nullable=False)
    description = Column(Text, nullable=False)
    logo_url = Column(String(500), nullable=True)

    # Technical details
    api_endpoint = Column(String(500), nullable=False)
    api_documentation_url = Column(String(500), nullable=True)
    supported_regions = Column(JSON, nullable=False, default=list)
    proposed_pricing = Column(JSON, nullable=False, default=list)

    # SLA commitments
    uptime_guarantee = Column(Float, nullable=False, default=99.9)
    response_time_sla_hours = Column(Integer, nullable=False, default=24)
    data_retention_days = Column(Integer, nullable=False, default=30)

    # Status
    status = Column(Enum(ApplicationStatus), nullable=False, default=ApplicationStatus.PENDING)
    status_reason = Column(Text, nullable=True)

    # Review tracking
    submitted_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    reviewed_at = Column(DateTime, nullable=True)
    reviewed_by = Column(String(100), nullable=True)
    approved_at = Column(DateTime, nullable=True)

    # Verification results
    verification_results = Column(JSON, nullable=True, default=dict)

    # Created provider (if approved)
    provider_id = Column(UUID(as_uuid=True), ForeignKey("providers.id"), nullable=True)

    __table_args__ = (
        Index("ix_provider_applications_status", "status"),
        Index("ix_provider_applications_email", "contact_email"),
    )

    def __repr__(self):
        return f"<ProviderApplication {self.company_name} ({self.status.value})>"


class ProviderVerification(Base):
    """
    Provider verification check record.

    Tracks verification tests run against provider APIs.
    """
    __tablename__ = "provider_verifications"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    application_id = Column(UUID(as_uuid=True), ForeignKey("provider_applications.id", ondelete="CASCADE"), nullable=False)

    # Check details
    check_type = Column(Enum(VerificationCheckType), nullable=False)
    check_name = Column(String(100), nullable=False)

    # Results
    passed = Column(Boolean, nullable=False)
    result_message = Column(Text, nullable=True)
    result_data = Column(JSON, nullable=True)

    # Timing
    duration_ms = Column(Float, nullable=True)
    executed_at = Column(DateTime, nullable=False, default=datetime.utcnow)

    __table_args__ = (
        Index("ix_provider_verifications_application", "application_id"),
    )

    def __repr__(self):
        return f"<ProviderVerification {self.check_type.value}: {'PASS' if self.passed else 'FAIL'}>"


class ProviderSLA(Base):
    """
    Provider SLA configuration.

    Defines SLA terms and penalty structures.
    """
    __tablename__ = "provider_slas"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    provider_id = Column(UUID(as_uuid=True), ForeignKey("providers.id", ondelete="CASCADE"), nullable=False)

    # SLA terms
    uptime_guarantee = Column(Float, nullable=False, default=99.9)
    response_time_hours = Column(Integer, nullable=False, default=4)
    resolution_time_hours = Column(Integer, nullable=False, default=24)

    # Penalties
    penalty_per_hour_down = Column(Float, nullable=False, default=0.0)
    max_monthly_penalty = Column(Float, nullable=False, default=0.0)

    # Credits structure (JSON)
    credit_tiers = Column(JSON, nullable=False, default=list)
    # e.g., [{"below_percent": 99.9, "credit_percent": 10}, {"below_percent": 99.0, "credit_percent": 25}]

    # Status
    is_active = Column(Boolean, nullable=False, default=True)
    effective_from = Column(DateTime, nullable=False, default=datetime.utcnow)
    effective_until = Column(DateTime, nullable=True)

    # Metadata
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    def __repr__(self):
        return f"<ProviderSLA {self.provider_id} ({self.uptime_guarantee}% uptime)>"


class ProviderReview(Base):
    """
    Provider review from users.

    User ratings and reviews for providers.
    """
    __tablename__ = "provider_reviews"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    provider_id = Column(UUID(as_uuid=True), ForeignKey("providers.id", ondelete="CASCADE"), nullable=False)
    user_id = Column(String(100), nullable=False)

    # Rating (1-5)
    rating = Column(Float, nullable=False)

    # Review content
    title = Column(String(200), nullable=True)
    comment = Column(Text, nullable=True)

    # Usage context
    validators_hosted = Column(Integer, nullable=True)
    months_used = Column(Integer, nullable=True)

    # Status
    is_verified = Column(Boolean, nullable=False, default=False)
    is_visible = Column(Boolean, nullable=False, default=True)

    # Timestamps
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    __table_args__ = (
        Index("ix_provider_reviews_provider", "provider_id"),
        Index("ix_provider_reviews_user", "user_id"),
    )

    def __repr__(self):
        return f"<ProviderReview {self.provider_id} ({self.rating}/5)>"
