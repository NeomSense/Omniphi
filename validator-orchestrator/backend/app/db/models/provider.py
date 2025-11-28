"""
Provider Model

Represents a hosting provider in the validator marketplace.
Supports official (Omniphi Cloud), community, and decentralized providers.

Table: providers
"""

import uuid
from datetime import datetime
from typing import List, Optional, TYPE_CHECKING

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base
from app.db.models.enums import ProviderType, ProviderStatus

if TYPE_CHECKING:
    from app.db.models.provider_pricing_tier import ProviderPricingTier
    from app.db.models.provider_metrics import ProviderMetrics
    from app.db.models.provider_sla import ProviderSLA
    from app.db.models.provider_review import ProviderReview


class Provider(Base):
    """
    Cloud provider registration for the validator marketplace.

    Represents a hosting provider that can provision validator nodes.
    Types include:
    - Official: Omniphi Cloud (first-party)
    - Community: Third-party verified providers
    - Decentralized: Decentralized compute networks (Akash, etc.)
    """

    __tablename__ = "providers"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Provider identification
    code = Column(
        String(50),
        nullable=False,
        unique=True,
        index=True,
        doc="Unique provider code (e.g., omniphi-cloud, aws)"
    )
    name = Column(
        String(100),
        nullable=False,
        doc="Internal provider name"
    )
    display_name = Column(
        String(100),
        nullable=False,
        doc="Human-readable display name"
    )
    description = Column(
        Text,
        nullable=True,
        doc="Provider description"
    )
    tagline = Column(
        String(200),
        nullable=True,
        doc="Short tagline"
    )

    # Branding
    logo_url = Column(
        String(500),
        nullable=True,
        doc="Logo image URL"
    )
    website_url = Column(
        String(500),
        nullable=True,
        doc="Provider website"
    )
    documentation_url = Column(
        String(500),
        nullable=True,
        doc="Documentation URL"
    )

    # Provider type and classification
    provider_type = Column(
        String(50),
        nullable=False,
        default=ProviderType.COMMUNITY.value,
        index=True,
        doc="Provider type"
    )
    is_official = Column(
        Boolean,
        nullable=False,
        default=False,
        index=True,
        doc="True for Omniphi Cloud (first-party)"
    )
    is_verified = Column(
        Boolean,
        nullable=False,
        default=False,
        index=True,
        doc="Whether provider is verified"
    )
    is_featured = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Featured in marketplace"
    )

    # API configuration
    api_endpoint = Column(
        String(500),
        nullable=True,
        doc="Provider API endpoint"
    )
    api_version = Column(
        String(20),
        nullable=True,
        doc="API version"
    )
    api_auth_type = Column(
        String(50),
        nullable=True,
        doc="API authentication type"
    )
    webhook_url = Column(
        String(500),
        nullable=True,
        doc="Webhook URL for events"
    )

    # Capabilities
    supported_regions = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="List of supported region codes"
    )
    supported_machine_types = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="List of supported machine types"
    )
    features = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Provider features/capabilities"
    )
    # Example: {"auto_scaling": true, "snapshot_restore": true, "monitoring": true}

    # Status
    status = Column(
        String(50),
        nullable=False,
        default=ProviderStatus.ACTIVE.value,
        index=True,
        doc="Provider availability status"
    )
    is_active = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True,
        doc="Whether provider is active"
    )
    is_accepting_new = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Whether accepting new validators"
    )

    # Pricing (base/promotional)
    price_monthly_min = Column(
        Float,
        nullable=True,
        doc="Minimum monthly price (USD)"
    )
    price_monthly_max = Column(
        Float,
        nullable=True,
        doc="Maximum monthly price (USD)"
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD",
        doc="Primary currency"
    )
    accepts_crypto = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether accepts crypto payments"
    )
    supported_crypto = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Supported cryptocurrencies"
    )

    # Performance metrics (cached)
    avg_provision_time_seconds = Column(
        Float,
        nullable=True,
        doc="Average provisioning time"
    )
    uptime_percent = Column(
        Float,
        nullable=False,
        default=99.9,
        doc="Overall uptime percentage"
    )
    avg_latency_ms = Column(
        Float,
        nullable=True,
        doc="Average network latency"
    )

    # Ratings and reviews
    rating = Column(
        Float,
        nullable=False,
        default=5.0,
        doc="Average rating (1-5)"
    )
    rating_count = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of ratings"
    )
    review_count = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of reviews"
    )

    # Usage statistics
    total_validators = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Total validators ever provisioned"
    )
    active_validators = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Currently active validators"
    )
    total_customers = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Total unique customers"
    )

    # Contact info
    support_email = Column(
        String(255),
        nullable=True,
        doc="Support email"
    )
    support_url = Column(
        String(500),
        nullable=True,
        doc="Support portal URL"
    )

    # Extra data
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional metadata"
    )

    # Timestamps
    created_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow
    )
    updated_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        onupdate=datetime.utcnow
    )
    verified_at = Column(
        DateTime,
        nullable=True,
        doc="Verification timestamp"
    )

    # Relationships
    pricing_tiers: Mapped[List["ProviderPricingTier"]] = relationship(
        "ProviderPricingTier",
        back_populates="provider",
        cascade="all, delete-orphan",
        lazy="selectin"
    )
    metrics: Mapped[List["ProviderMetrics"]] = relationship(
        "ProviderMetrics",
        back_populates="provider",
        cascade="all, delete-orphan",
        lazy="dynamic"
    )
    slas: Mapped[List["ProviderSLA"]] = relationship(
        "ProviderSLA",
        back_populates="provider",
        cascade="all, delete-orphan",
        lazy="selectin"
    )
    reviews: Mapped[List["ProviderReview"]] = relationship(
        "ProviderReview",
        back_populates="provider",
        cascade="all, delete-orphan",
        lazy="dynamic"
    )

    # Indexes
    __table_args__ = (
        Index("ix_providers_type_status", "provider_type", "status"),
        Index("ix_providers_active_rating", "is_active", "rating"),
        Index("ix_providers_official", "is_official", "is_active"),
    )

    def __repr__(self) -> str:
        return f"<Provider {self.code}: {self.display_name}>"

    @property
    def is_omniphi_cloud(self) -> bool:
        """Check if this is Omniphi Cloud (official provider)."""
        return self.is_official and self.code == "omniphi-cloud"

    @property
    def is_available(self) -> bool:
        """Check if provider is available for new validators."""
        return (
            self.is_active and
            self.is_accepting_new and
            self.status == ProviderStatus.ACTIVE.value
        )

    @property
    def price_range(self) -> str:
        """Get formatted price range string."""
        if self.price_monthly_min and self.price_monthly_max:
            if self.price_monthly_min == self.price_monthly_max:
                return f"${self.price_monthly_min}/mo"
            return f"${self.price_monthly_min}-${self.price_monthly_max}/mo"
        elif self.price_monthly_min:
            return f"From ${self.price_monthly_min}/mo"
        elif self.price_monthly_max:
            return f"Up to ${self.price_monthly_max}/mo"
        return "Contact for pricing"

    def supports_region(self, region_code: str) -> bool:
        """Check if provider supports a region."""
        return region_code in self.supported_regions

    def supports_machine_type(self, machine_type: str) -> bool:
        """Check if provider supports a machine type."""
        return machine_type in self.supported_machine_types

    def has_feature(self, feature: str) -> bool:
        """Check if provider has a specific feature."""
        return self.features.get(feature, False)

    def update_stats(
        self,
        total_validators: Optional[int] = None,
        active_validators: Optional[int] = None,
        uptime_percent: Optional[float] = None,
    ) -> None:
        """
        Update provider statistics.

        Args:
            total_validators: New total validators count
            active_validators: New active validators count
            uptime_percent: New uptime percentage
        """
        if total_validators is not None:
            self.total_validators = total_validators
        if active_validators is not None:
            self.active_validators = active_validators
        if uptime_percent is not None:
            self.uptime_percent = uptime_percent

    def update_rating(self, new_rating: float) -> None:
        """
        Update provider rating with new review.

        Args:
            new_rating: New rating value (1-5)
        """
        # Calculate new average
        total_rating = self.rating * self.rating_count
        self.rating_count += 1
        self.rating = (total_rating + new_rating) / self.rating_count
        self.review_count += 1

    @property
    def active_sla(self) -> Optional["ProviderSLA"]:
        """Get current active SLA."""
        if not self.slas:
            return None
        for sla in self.slas:
            if sla.is_active:
                return sla
        return None
