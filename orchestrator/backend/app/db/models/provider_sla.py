"""
Provider SLA Model

Defines Service Level Agreement terms for providers.
Includes uptime guarantees, response times, and penalty structures.

Table: provider_slas
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
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base

if TYPE_CHECKING:
    from app.db.models.provider import Provider


class ProviderSLA(Base):
    """
    Provider SLA (Service Level Agreement) configuration.

    Defines the contractual obligations and penalty structures
    for provider service levels.
    """

    __tablename__ = "provider_slas"

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

    # SLA identification
    name = Column(
        String(100),
        nullable=False,
        doc="SLA name"
    )
    version = Column(
        String(20),
        nullable=False,
        default="1.0",
        doc="SLA version"
    )
    description = Column(
        Text,
        nullable=True,
        doc="SLA description"
    )

    # Availability guarantees
    uptime_guarantee = Column(
        Float,
        nullable=False,
        default=99.9,
        doc="Uptime guarantee percentage"
    )
    availability_calculation_method = Column(
        String(50),
        nullable=False,
        default="monthly",
        doc="How availability is calculated (monthly, quarterly, yearly)"
    )
    excluded_maintenance_windows = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Whether maintenance windows excluded from uptime"
    )
    max_scheduled_maintenance_hours = Column(
        Integer,
        nullable=False,
        default=4,
        doc="Maximum monthly scheduled maintenance hours"
    )

    # Response time guarantees
    response_time_hours = Column(
        Integer,
        nullable=False,
        default=4,
        doc="Initial response time for issues (hours)"
    )
    resolution_time_hours = Column(
        Integer,
        nullable=False,
        default=24,
        doc="Target resolution time (hours)"
    )
    critical_response_time_hours = Column(
        Integer,
        nullable=False,
        default=1,
        doc="Response time for critical issues"
    )
    critical_resolution_time_hours = Column(
        Integer,
        nullable=False,
        default=4,
        doc="Resolution time for critical issues"
    )

    # Performance guarantees
    max_latency_ms = Column(
        Float,
        nullable=True,
        doc="Maximum acceptable latency (ms)"
    )
    max_provision_time_minutes = Column(
        Integer,
        nullable=True,
        doc="Maximum provisioning time (minutes)"
    )

    # Penalties and credits
    penalty_per_hour_down = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Penalty per hour of downtime (USD)"
    )
    max_monthly_penalty = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Maximum monthly penalty (USD)"
    )
    credit_tiers = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Credit tiers based on uptime"
    )
    # Example: [{"below_percent": 99.9, "credit_percent": 10}, {"below_percent": 99.0, "credit_percent": 25}]

    # Exclusions
    exclusions = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Events excluded from SLA calculations"
    )
    # Example: ["force_majeure", "customer_caused", "third_party_failure"]

    # Support levels
    support_hours = Column(
        String(50),
        nullable=False,
        default="24x7",
        doc="Support availability"
    )
    support_channels = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Available support channels"
    )
    # Example: ["email", "phone", "chat", "ticket"]

    # Reporting
    reporting_frequency = Column(
        String(50),
        nullable=False,
        default="monthly",
        doc="SLA report frequency"
    )
    reporting_metrics = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Metrics included in reports"
    )

    # Status
    is_active = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True,
        doc="Whether SLA is currently active"
    )
    is_default = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether this is the default SLA"
    )

    # Validity period
    effective_from = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        doc="SLA effective start date"
    )
    effective_until = Column(
        DateTime,
        nullable=True,
        doc="SLA effective end date"
    )

    # Legal
    terms_url = Column(
        String(500),
        nullable=True,
        doc="URL to full SLA terms"
    )
    legal_jurisdiction = Column(
        String(100),
        nullable=True,
        doc="Legal jurisdiction"
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
        back_populates="slas"
    )

    # Indexes
    __table_args__ = (
        Index("ix_provider_slas_provider_active", "provider_id", "is_active"),
        Index("ix_provider_slas_effective", "effective_from", "effective_until"),
    )

    def __repr__(self) -> str:
        return f"<ProviderSLA {self.name} ({self.uptime_guarantee}% uptime)>"

    @property
    def is_currently_effective(self) -> bool:
        """Check if SLA is currently in effect."""
        now = datetime.utcnow()
        if not self.is_active:
            return False
        if now < self.effective_from:
            return False
        if self.effective_until and now > self.effective_until:
            return False
        return True

    @property
    def monthly_downtime_budget_minutes(self) -> float:
        """
        Calculate allowed monthly downtime in minutes.

        Based on uptime guarantee percentage.
        30 days * 24 hours * 60 minutes = 43200 minutes
        """
        return 43200 * (1 - self.uptime_guarantee / 100)

    def calculate_credit(self, actual_uptime: float) -> float:
        """
        Calculate credit percentage based on actual uptime.

        Args:
            actual_uptime: Actual uptime percentage achieved

        Returns:
            Credit percentage to apply
        """
        if actual_uptime >= self.uptime_guarantee:
            return 0.0

        # Sort tiers by threshold descending
        sorted_tiers = sorted(
            self.credit_tiers,
            key=lambda t: t.get("below_percent", 0),
            reverse=True
        )

        for tier in sorted_tiers:
            threshold = tier.get("below_percent", 0)
            credit = tier.get("credit_percent", 0)
            if actual_uptime < threshold:
                return credit

        return 0.0

    def calculate_penalty(self, downtime_hours: float) -> float:
        """
        Calculate penalty for downtime.

        Args:
            downtime_hours: Total downtime hours

        Returns:
            Penalty amount in USD
        """
        penalty = downtime_hours * self.penalty_per_hour_down
        return min(penalty, self.max_monthly_penalty)
