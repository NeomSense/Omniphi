"""
Billing Usage Model

Tracks resource usage for billing purposes.
Records validator hours, bandwidth, API calls, etc.

Table: billing_usage
"""

import uuid
from datetime import datetime
from typing import TYPE_CHECKING

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    ForeignKey,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB

from app.db.database import Base

if TYPE_CHECKING:
    pass


class BillingUsage(Base):
    """
    Billing usage record.

    Tracks resource consumption for usage-based billing
    including validator hours, bandwidth, and API usage.
    """

    __tablename__ = "billing_usage"

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
        index=True
    )
    subscription_id = Column(
        UUID(as_uuid=True),
        ForeignKey("billing_subscriptions.id", ondelete="SET NULL"),
        nullable=True,
        index=True
    )
    validator_node_id = Column(
        UUID(as_uuid=True),
        ForeignKey("validator_nodes.id", ondelete="SET NULL"),
        nullable=True,
        index=True
    )

    # Usage period
    period_start = Column(
        DateTime,
        nullable=False,
        index=True
    )
    period_end = Column(
        DateTime,
        nullable=False
    )
    period_type = Column(
        String(20),
        nullable=False,
        default="hourly",
        doc="Usage aggregation period (hourly, daily)"
    )

    # Usage type
    usage_type = Column(
        String(50),
        nullable=False,
        index=True,
        doc="Type of usage (validator_hours, bandwidth, api_calls)"
    )

    # Usage quantities
    quantity = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Usage quantity"
    )
    unit = Column(
        String(20),
        nullable=False,
        default="hours",
        doc="Unit of measurement"
    )

    # Pricing
    unit_price = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Price per unit"
    )
    total_cost = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Total cost for this usage"
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD"
    )

    # Included vs overage
    included_quantity = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Quantity included in plan"
    )
    overage_quantity = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Quantity over plan limit"
    )
    overage_cost = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Cost for overage"
    )

    # Resource details
    resource_type = Column(
        String(50),
        nullable=True,
        doc="Resource type (cpu, memory, disk, bandwidth)"
    )
    resource_tier = Column(
        String(50),
        nullable=True,
        doc="Resource tier/plan"
    )
    region_code = Column(
        String(50),
        nullable=True,
        doc="Region where usage occurred"
    )
    provider_code = Column(
        String(50),
        nullable=True,
        doc="Provider code"
    )

    # Detailed breakdown
    breakdown = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Detailed usage breakdown"
    )
    # Example: {"cpu_hours": 48, "memory_gb_hours": 192, "disk_gb_hours": 2400}

    # Billing status
    is_billed = Column(
        Boolean,
        nullable=False,
        default=False,
        index=True,
        doc="Whether usage has been billed"
    )
    invoice_id = Column(
        UUID(as_uuid=True),
        ForeignKey("billing_invoices.id", ondelete="SET NULL"),
        nullable=True,
        doc="Invoice this usage was billed on"
    )
    billed_at = Column(
        DateTime,
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

    # Indexes
    __table_args__ = (
        Index("ix_billing_usage_account_period", "account_id", "period_start", "period_end"),
        Index("ix_billing_usage_type_period", "usage_type", "period_start"),
        Index("ix_billing_usage_unbilled", "is_billed", "account_id"),
        Index("ix_billing_usage_validator", "validator_node_id", "period_start"),
    )

    def __repr__(self) -> str:
        return f"<BillingUsage {self.usage_type}: {self.quantity} {self.unit}>"

    @property
    def is_overage(self) -> bool:
        """Check if usage exceeded included amount."""
        return self.overage_quantity > 0

    @property
    def utilization_percent(self) -> float:
        """Calculate utilization of included quota."""
        if self.included_quantity == 0:
            return 100.0 if self.quantity > 0 else 0.0
        return min(100.0, (self.quantity / self.included_quantity) * 100)

    @property
    def period_hours(self) -> float:
        """Get period duration in hours."""
        delta = self.period_end - self.period_start
        return delta.total_seconds() / 3600

    def calculate_cost(self, included: float = 0, unit_price: float = None) -> float:
        """
        Calculate cost for this usage.

        Args:
            included: Included quantity (free tier)
            unit_price: Price per unit (overrides stored price)

        Returns:
            Total cost
        """
        price = unit_price or self.unit_price
        self.included_quantity = included

        if self.quantity <= included:
            self.overage_quantity = 0
            self.overage_cost = 0
            self.total_cost = 0
        else:
            self.overage_quantity = self.quantity - included
            self.overage_cost = self.overage_quantity * price
            self.total_cost = self.overage_cost

        return self.total_cost

    def mark_billed(self, invoice_id: uuid.UUID) -> None:
        """
        Mark usage as billed.

        Args:
            invoice_id: Invoice ID
        """
        self.is_billed = True
        self.invoice_id = invoice_id
        self.billed_at = datetime.utcnow()


# Import Boolean
from sqlalchemy import Boolean  # noqa: E402
