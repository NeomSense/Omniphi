"""
Provider Pricing Tier Model

Defines machine types and pricing for a provider.
Each tier has specific hardware specifications and associated costs.

Table: provider_pricing_tiers
"""

import uuid
from datetime import datetime
from typing import Optional, TYPE_CHECKING

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    ForeignKey,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base

if TYPE_CHECKING:
    from app.db.models.provider import Provider


class ProviderPricingTier(Base):
    """
    Provider pricing tier definition.

    Defines machine specifications and pricing for a specific tier
    offered by a provider in the marketplace.
    """

    __tablename__ = "provider_pricing_tiers"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Foreign key
    provider_id = Column(
        UUID(as_uuid=True),
        ForeignKey("providers.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
        doc="Parent provider"
    )

    # Tier identification
    tier_code = Column(
        String(50),
        nullable=False,
        doc="Tier code (e.g., small, medium, large)"
    )
    name = Column(
        String(100),
        nullable=False,
        doc="Tier display name"
    )
    description = Column(
        Text,
        nullable=True,
        doc="Tier description"
    )
    display_order = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Display order in UI"
    )

    # Hardware specifications
    cpu_cores = Column(
        Integer,
        nullable=False,
        doc="Number of CPU cores"
    )
    memory_gb = Column(
        Integer,
        nullable=False,
        doc="Memory in GB"
    )
    disk_gb = Column(
        Integer,
        nullable=False,
        doc="Disk space in GB"
    )
    disk_type = Column(
        String(50),
        nullable=False,
        default="ssd",
        doc="Disk type (ssd, nvme, hdd)"
    )
    bandwidth_gbps = Column(
        Float,
        nullable=False,
        default=1.0,
        doc="Network bandwidth in Gbps"
    )
    bandwidth_tb_month = Column(
        Float,
        nullable=True,
        doc="Monthly bandwidth allowance in TB"
    )

    # Pricing
    hourly_price = Column(
        Float,
        nullable=False,
        doc="Hourly price"
    )
    monthly_price = Column(
        Float,
        nullable=False,
        doc="Monthly price (usually discounted vs hourly)"
    )
    yearly_price = Column(
        Float,
        nullable=True,
        doc="Yearly price (usually discounted vs monthly)"
    )
    setup_fee = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="One-time setup fee"
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD",
        doc="Currency code"
    )

    # Crypto pricing
    hourly_price_crypto = Column(
        Float,
        nullable=True,
        doc="Hourly price in crypto"
    )
    monthly_price_crypto = Column(
        Float,
        nullable=True,
        doc="Monthly price in crypto"
    )
    crypto_currency = Column(
        String(20),
        nullable=True,
        doc="Crypto currency code (OMNI, USDC, etc.)"
    )

    # Availability
    is_available = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True,
        doc="Whether tier is available"
    )
    available_in_regions = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Regions where tier is available"
    )
    max_instances = Column(
        Integer,
        nullable=True,
        doc="Maximum instances available"
    )
    current_instances = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Currently provisioned instances"
    )

    # Promotions
    is_promotional = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether promotional pricing"
    )
    promotional_price = Column(
        Float,
        nullable=True,
        doc="Promotional monthly price"
    )
    promotional_ends_at = Column(
        DateTime,
        nullable=True,
        doc="Promotion end date"
    )

    # Recommendations
    is_recommended = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether tier is recommended"
    )
    recommended_for = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Use cases this tier is recommended for"
    )

    # Metadata
    features = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Tier-specific features"
    )
    specs = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional specifications"
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

    # Relationships
    provider: Mapped["Provider"] = relationship(
        "Provider",
        back_populates="pricing_tiers"
    )

    # Indexes
    __table_args__ = (
        Index("ix_pricing_tiers_provider_code", "provider_id", "tier_code", unique=True),
        Index("ix_pricing_tiers_provider_available", "provider_id", "is_available"),
        Index("ix_pricing_tiers_price", "monthly_price"),
    )

    def __repr__(self) -> str:
        return f"<ProviderPricingTier {self.tier_code} @ ${self.monthly_price}/mo>"

    @property
    def specs_summary(self) -> str:
        """Get human-readable specs summary."""
        return f"{self.cpu_cores} CPU, {self.memory_gb}GB RAM, {self.disk_gb}GB {self.disk_type.upper()}"

    @property
    def effective_monthly_price(self) -> float:
        """Get effective monthly price (promotional if active)."""
        if self.is_promotional and self.promotional_price:
            if not self.promotional_ends_at or self.promotional_ends_at > datetime.utcnow():
                return self.promotional_price
        return self.monthly_price

    @property
    def has_capacity(self) -> bool:
        """Check if tier has capacity."""
        if self.max_instances is None:
            return True
        return self.current_instances < self.max_instances

    @property
    def available_instances(self) -> Optional[int]:
        """Get number of available instances."""
        if self.max_instances is None:
            return None
        return max(0, self.max_instances - self.current_instances)

    def is_available_in_region(self, region_code: str) -> bool:
        """Check if tier is available in a specific region."""
        if not self.available_in_regions:
            return True  # Available in all regions if not specified
        return region_code in self.available_in_regions

    def allocate_instance(self) -> bool:
        """
        Allocate an instance of this tier.

        Returns:
            True if allocation successful
        """
        if not self.has_capacity:
            return False
        self.current_instances += 1
        return True

    def release_instance(self) -> None:
        """Release an instance of this tier."""
        self.current_instances = max(0, self.current_instances - 1)
