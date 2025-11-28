"""
Billing Plan Model

Defines available billing plans and their features.
Plans determine pricing, limits, and features for subscribers.

Table: billing_plans
"""

import uuid
from datetime import datetime
from typing import List, TYPE_CHECKING

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
from app.db.models.enums import BillingPlanType, BillingCycle

if TYPE_CHECKING:
    from app.db.models.billing_subscription import BillingSubscription


class BillingPlan(Base):
    """
    Billing plan definition.

    Defines the available subscription plans with pricing,
    features, and resource limits.
    """

    __tablename__ = "billing_plans"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Plan identification
    code = Column(
        String(50),
        nullable=False,
        unique=True,
        index=True,
        doc="Plan code (e.g., starter, professional)"
    )
    name = Column(
        String(100),
        nullable=False,
        doc="Plan display name"
    )
    description = Column(
        Text,
        nullable=True,
        doc="Plan description"
    )
    tagline = Column(
        String(200),
        nullable=True,
        doc="Marketing tagline"
    )

    # Plan type and tier
    plan_type = Column(
        String(50),
        nullable=False,
        default=BillingPlanType.STARTER.value,
        index=True,
        doc="Plan type"
    )
    tier_level = Column(
        Integer,
        nullable=False,
        default=1,
        doc="Tier level for sorting (higher = better)"
    )
    display_order = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Display order in UI"
    )

    # Pricing
    price_monthly = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Monthly price"
    )
    price_yearly = Column(
        Float,
        nullable=True,
        doc="Yearly price (discounted)"
    )
    price_hourly = Column(
        Float,
        nullable=True,
        doc="Hourly price for usage-based"
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD"
    )
    setup_fee = Column(
        Float,
        nullable=False,
        default=0.0
    )

    # Crypto pricing
    price_monthly_crypto = Column(
        Float,
        nullable=True,
        doc="Monthly price in crypto"
    )
    crypto_currency = Column(
        String(20),
        nullable=True,
        doc="Crypto currency"
    )

    # Billing cycle options
    billing_cycles = Column(
        JSONB,
        nullable=False,
        default=["monthly", "yearly"],
        doc="Available billing cycles"
    )
    default_cycle = Column(
        String(20),
        nullable=False,
        default=BillingCycle.MONTHLY.value
    )

    # Resource limits
    max_validators = Column(
        Integer,
        nullable=True,
        doc="Maximum validators (null = unlimited)"
    )
    max_regions = Column(
        Integer,
        nullable=True,
        doc="Maximum regions"
    )
    max_team_members = Column(
        Integer,
        nullable=True,
        doc="Maximum team members"
    )
    max_api_requests_per_day = Column(
        Integer,
        nullable=True,
        doc="API request limit"
    )

    # Resource allocations (per validator)
    cpu_cores_per_validator = Column(
        Integer,
        nullable=False,
        default=2
    )
    memory_gb_per_validator = Column(
        Integer,
        nullable=False,
        default=4
    )
    disk_gb_per_validator = Column(
        Integer,
        nullable=False,
        default=100
    )
    bandwidth_tb_per_month = Column(
        Float,
        nullable=True
    )

    # Included resources
    included_validators = Column(
        Integer,
        nullable=False,
        default=1,
        doc="Validators included in base price"
    )
    price_per_additional_validator = Column(
        Float,
        nullable=True,
        doc="Price for additional validators"
    )

    # Features
    features = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Feature flags"
    )
    # Example: {"monitoring": true, "alerts": true, "api_access": true, "priority_support": false}

    feature_list = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Marketing feature list"
    )
    # Example: ["24/7 monitoring", "Email alerts", "API access"]

    # Support level
    support_level = Column(
        String(50),
        nullable=False,
        default="standard",
        doc="Support level (standard, priority, dedicated)"
    )
    support_response_hours = Column(
        Integer,
        nullable=False,
        default=24
    )

    # Trial settings
    trial_days = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Free trial days"
    )
    trial_validators = Column(
        Integer,
        nullable=True,
        doc="Validators allowed during trial"
    )

    # Status
    is_active = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True
    )
    is_public = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Visible in public pricing"
    )
    is_featured = Column(
        Boolean,
        nullable=False,
        default=False
    )
    is_legacy = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Legacy plan (not for new signups)"
    )

    # Stripe integration
    stripe_product_id = Column(
        String(100),
        nullable=True
    )
    stripe_price_id_monthly = Column(
        String(100),
        nullable=True
    )
    stripe_price_id_yearly = Column(
        String(100),
        nullable=True
    )

    # Extra data
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict
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
    subscriptions: Mapped[List["BillingSubscription"]] = relationship(
        "BillingSubscription",
        back_populates="plan",
        lazy="dynamic"
    )

    # Indexes
    __table_args__ = (
        Index("ix_billing_plans_type_active", "plan_type", "is_active"),
        Index("ix_billing_plans_public", "is_public", "is_active"),
    )

    def __repr__(self) -> str:
        return f"<BillingPlan {self.code}: ${self.price_monthly}/mo>"

    @property
    def is_free(self) -> bool:
        """Check if plan is free."""
        return self.price_monthly == 0.0

    @property
    def yearly_savings(self) -> float:
        """Calculate yearly savings vs monthly."""
        if not self.price_yearly or not self.price_monthly:
            return 0.0
        monthly_annual = self.price_monthly * 12
        return monthly_annual - self.price_yearly

    @property
    def yearly_savings_percent(self) -> float:
        """Calculate yearly savings percentage."""
        if not self.price_yearly or not self.price_monthly:
            return 0.0
        monthly_annual = self.price_monthly * 12
        return round((1 - self.price_yearly / monthly_annual) * 100, 0)

    @property
    def has_trial(self) -> bool:
        """Check if plan has trial."""
        return self.trial_days > 0

    def has_feature(self, feature: str) -> bool:
        """Check if plan has a feature."""
        return self.features.get(feature, False)

    def get_price_for_cycle(self, cycle: str) -> float:
        """
        Get price for billing cycle.

        Args:
            cycle: Billing cycle (monthly, yearly)

        Returns:
            Price for the cycle
        """
        if cycle == BillingCycle.YEARLY.value and self.price_yearly:
            return self.price_yearly
        return self.price_monthly

    def calculate_cost(
        self,
        validators: int,
        cycle: str = "monthly"
    ) -> float:
        """
        Calculate total cost for given validators.

        Args:
            validators: Number of validators
            cycle: Billing cycle

        Returns:
            Total cost
        """
        base_price = self.get_price_for_cycle(cycle)

        if cycle == BillingCycle.YEARLY.value:
            base_price = base_price / 12  # Monthly equivalent

        additional = max(0, validators - self.included_validators)
        additional_cost = additional * (self.price_per_additional_validator or 0)

        return base_price + additional_cost
