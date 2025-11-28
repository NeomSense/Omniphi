"""
Billing Subscription Model

Active subscriptions linking accounts to plans.
Tracks subscription lifecycle, billing periods, and usage.

Table: billing_subscriptions
"""

import uuid
from datetime import datetime, timedelta
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
from app.db.models.enums import SubscriptionStatus, BillingCycle

if TYPE_CHECKING:
    from app.db.models.billing_account import BillingAccount
    from app.db.models.billing_plan import BillingPlan


class BillingSubscription(Base):
    """
    Billing subscription record.

    Links a billing account to a plan and tracks the subscription
    lifecycle including billing periods and cancellation.
    """

    __tablename__ = "billing_subscriptions"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Foreign keys
    account_id = Column(
        UUID(as_uuid=True),
        ForeignKey("billing_accounts.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
        doc="Billing account"
    )
    plan_id = Column(
        UUID(as_uuid=True),
        ForeignKey("billing_plans.id", ondelete="RESTRICT"),
        nullable=False,
        index=True,
        doc="Subscribed plan"
    )

    # Subscription identification
    subscription_number = Column(
        String(50),
        nullable=False,
        unique=True,
        index=True,
        doc="Human-readable subscription number"
    )

    # Status
    status = Column(
        String(50),
        nullable=False,
        default=SubscriptionStatus.ACTIVE.value,
        index=True
    )
    is_active = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True
    )

    # Billing cycle
    billing_cycle = Column(
        String(20),
        nullable=False,
        default=BillingCycle.MONTHLY.value
    )
    billing_anchor_day = Column(
        Integer,
        nullable=False,
        default=1,
        doc="Day of month for billing"
    )

    # Pricing at time of subscription
    price_at_signup = Column(
        Float,
        nullable=False,
        doc="Price when subscription was created"
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD"
    )
    discount_percent = Column(
        Float,
        nullable=False,
        default=0.0
    )
    discount_amount = Column(
        Float,
        nullable=False,
        default=0.0
    )
    coupon_code = Column(
        String(50),
        nullable=True
    )

    # Resource quantities
    quantity = Column(
        Integer,
        nullable=False,
        default=1,
        doc="Number of units (validators)"
    )
    max_quantity = Column(
        Integer,
        nullable=True,
        doc="Maximum allowed quantity"
    )

    # Billing period
    current_period_start = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow
    )
    current_period_end = Column(
        DateTime,
        nullable=False
    )
    next_billing_date = Column(
        DateTime,
        nullable=True
    )

    # Trial
    trial_start = Column(
        DateTime,
        nullable=True
    )
    trial_end = Column(
        DateTime,
        nullable=True
    )
    is_trial = Column(
        Boolean,
        nullable=False,
        default=False
    )

    # Cancellation
    cancel_at_period_end = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Cancel at end of current period"
    )
    cancelled_at = Column(
        DateTime,
        nullable=True
    )
    cancellation_reason = Column(
        Text,
        nullable=True
    )
    cancellation_feedback = Column(
        Text,
        nullable=True
    )

    # Pause (if supported)
    is_paused = Column(
        Boolean,
        nullable=False,
        default=False
    )
    paused_at = Column(
        DateTime,
        nullable=True
    )
    resume_at = Column(
        DateTime,
        nullable=True
    )

    # Stripe integration
    stripe_subscription_id = Column(
        String(100),
        nullable=True,
        unique=True,
        index=True
    )
    stripe_schedule_id = Column(
        String(100),
        nullable=True,
        doc="Stripe subscription schedule ID"
    )

    # Usage tracking
    usage_this_period = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Usage metrics for current period"
    )

    # Extra data
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict
    )
    notes = Column(
        Text,
        nullable=True
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
    ended_at = Column(
        DateTime,
        nullable=True
    )

    # Relationships
    account: Mapped["BillingAccount"] = relationship(
        "BillingAccount",
        back_populates="subscriptions"
    )
    plan: Mapped["BillingPlan"] = relationship(
        "BillingPlan",
        back_populates="subscriptions"
    )

    # Indexes
    __table_args__ = (
        Index("ix_billing_subscriptions_account_status", "account_id", "status"),
        Index("ix_billing_subscriptions_next_billing", "next_billing_date"),
        Index("ix_billing_subscriptions_stripe", "stripe_subscription_id"),
    )

    def __repr__(self) -> str:
        return f"<BillingSubscription {self.subscription_number} ({self.status})>"

    @property
    def is_in_trial(self) -> bool:
        """Check if subscription is in trial period."""
        if not self.is_trial or not self.trial_end:
            return False
        return datetime.utcnow() < self.trial_end

    @property
    def trial_days_remaining(self) -> int:
        """Get remaining trial days."""
        if not self.is_in_trial or not self.trial_end:
            return 0
        delta = self.trial_end - datetime.utcnow()
        return max(0, delta.days)

    @property
    def days_until_renewal(self) -> int:
        """Get days until next billing."""
        if not self.current_period_end:
            return 0
        delta = self.current_period_end - datetime.utcnow()
        return max(0, delta.days)

    @property
    def is_cancelled(self) -> bool:
        """Check if subscription is cancelled."""
        return self.status == SubscriptionStatus.CANCELLED.value

    @property
    def will_cancel(self) -> bool:
        """Check if subscription will cancel at period end."""
        return self.cancel_at_period_end

    @property
    def effective_price(self) -> float:
        """Get effective price after discounts."""
        price = self.price_at_signup
        if self.discount_percent:
            price = price * (1 - self.discount_percent / 100)
        if self.discount_amount:
            price = price - self.discount_amount
        return max(0, price)

    @property
    def total_price(self) -> float:
        """Get total price for quantity."""
        return self.effective_price * self.quantity

    def set_status(self, status: SubscriptionStatus) -> None:
        """
        Update subscription status.

        Args:
            status: New status
        """
        self.status = status.value

        if status == SubscriptionStatus.ACTIVE:
            self.is_active = True
            self.ended_at = None
        elif status == SubscriptionStatus.CANCELLED:
            self.is_active = False
            self.ended_at = datetime.utcnow()
        elif status == SubscriptionStatus.PAUSED:
            self.is_paused = True
            self.paused_at = datetime.utcnow()

    def cancel(self, at_period_end: bool = True, reason: str = None) -> None:
        """
        Cancel subscription.

        Args:
            at_period_end: Whether to cancel at period end or immediately
            reason: Cancellation reason
        """
        self.cancelled_at = datetime.utcnow()
        self.cancellation_reason = reason

        if at_period_end:
            self.cancel_at_period_end = True
        else:
            self.set_status(SubscriptionStatus.CANCELLED)

    def reactivate(self) -> None:
        """Reactivate a cancelled subscription."""
        self.cancel_at_period_end = False
        self.cancelled_at = None
        self.cancellation_reason = None
        self.set_status(SubscriptionStatus.ACTIVE)

    def advance_period(self) -> None:
        """Advance to next billing period."""
        self.current_period_start = self.current_period_end

        if self.billing_cycle == BillingCycle.MONTHLY.value:
            self.current_period_end = self.current_period_start + timedelta(days=30)
        elif self.billing_cycle == BillingCycle.YEARLY.value:
            self.current_period_end = self.current_period_start + timedelta(days=365)
        else:
            self.current_period_end = self.current_period_start + timedelta(days=30)

        self.next_billing_date = self.current_period_end
        self.usage_this_period = {}  # Reset usage

    def update_quantity(self, new_quantity: int) -> None:
        """
        Update subscription quantity.

        Args:
            new_quantity: New quantity
        """
        if self.max_quantity and new_quantity > self.max_quantity:
            raise ValueError(f"Quantity cannot exceed {self.max_quantity}")
        self.quantity = new_quantity
