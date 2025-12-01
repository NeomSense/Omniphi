"""
Provider Metrics Model

Tracks performance data for providers over time.
Used for monitoring, SLA compliance, and marketplace rankings.

Table: provider_metrics
"""

import uuid
from datetime import datetime
from typing import TYPE_CHECKING

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    DateTime,
    ForeignKey,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base

if TYPE_CHECKING:
    from app.db.models.provider import Provider


class ProviderMetrics(Base):
    """
    Provider performance metrics tracking.

    Records performance data at regular intervals for:
    - Latency measurements
    - Uptime/availability
    - Provisioning success rates
    - Resource usage statistics
    """

    __tablename__ = "provider_metrics"

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

    # Region (optional - for per-region metrics)
    region_code = Column(
        String(50),
        nullable=True,
        index=True,
        doc="Region code (null for aggregate)"
    )

    # Time period
    period_start = Column(
        DateTime,
        nullable=False,
        doc="Metrics period start"
    )
    period_end = Column(
        DateTime,
        nullable=False,
        doc="Metrics period end"
    )
    period_type = Column(
        String(20),
        nullable=False,
        default="hourly",
        doc="Period type (hourly, daily, weekly, monthly)"
    )

    # Latency metrics
    avg_latency_ms = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Average latency in milliseconds"
    )
    p50_latency_ms = Column(
        Float,
        nullable=True,
        doc="50th percentile latency"
    )
    p95_latency_ms = Column(
        Float,
        nullable=True,
        doc="95th percentile latency"
    )
    p99_latency_ms = Column(
        Float,
        nullable=True,
        doc="99th percentile latency"
    )
    max_latency_ms = Column(
        Float,
        nullable=True,
        doc="Maximum latency"
    )

    # Availability metrics
    uptime_percent = Column(
        Float,
        nullable=False,
        default=100.0,
        doc="Uptime percentage"
    )
    downtime_minutes = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Total downtime in minutes"
    )
    success_rate = Column(
        Float,
        nullable=False,
        default=100.0,
        doc="Request success rate percentage"
    )
    error_rate = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="Error rate percentage"
    )

    # Provisioning stats
    provision_requests = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of provision requests"
    )
    provision_success = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Successful provisions"
    )
    provision_failed = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Failed provisions"
    )
    provision_success_rate = Column(
        Float,
        nullable=False,
        default=100.0,
        doc="Provisioning success rate"
    )
    avg_provision_time_seconds = Column(
        Float,
        nullable=True,
        doc="Average provision time"
    )

    # Resource metrics
    total_validators = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Total validators in period"
    )
    active_validators = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Active validators"
    )
    failed_validators = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Failed/error validators"
    )
    terminated_validators = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Terminated validators"
    )

    # Incident tracking
    incident_count = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of incidents"
    )
    critical_incidents = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Critical incidents"
    )
    mean_time_to_resolve_minutes = Column(
        Float,
        nullable=True,
        doc="Average incident resolution time"
    )

    # Customer metrics
    active_customers = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Active customers"
    )
    new_customers = Column(
        Integer,
        nullable=False,
        default=0,
        doc="New customers in period"
    )
    churned_customers = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Churned customers"
    )

    # Revenue metrics (optional)
    revenue = Column(
        Float,
        nullable=True,
        doc="Revenue in period"
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD",
        doc="Revenue currency"
    )

    # Additional data
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional metrics data"
    )

    # Timestamp
    recorded_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        index=True,
        doc="When metrics were recorded"
    )

    # Relationships
    provider: Mapped["Provider"] = relationship(
        "Provider",
        back_populates="metrics"
    )

    # Indexes
    __table_args__ = (
        Index("ix_provider_metrics_provider_period", "provider_id", "period_start", "period_type"),
        Index("ix_provider_metrics_provider_region", "provider_id", "region_code", "recorded_at"),
        Index("ix_provider_metrics_recorded", "recorded_at"),
    )

    def __repr__(self) -> str:
        region = f" in {self.region_code}" if self.region_code else ""
        return f"<ProviderMetrics {self.provider_id}{region} @ {self.period_start}>"

    @property
    def provision_rate(self) -> float:
        """Calculate provisioning success rate."""
        if self.provision_requests == 0:
            return 100.0
        return round((self.provision_success / self.provision_requests) * 100, 2)

    @property
    def is_healthy(self) -> bool:
        """Check if metrics indicate healthy performance."""
        return (
            self.uptime_percent >= 99.0 and
            self.success_rate >= 95.0 and
            self.provision_success_rate >= 90.0 and
            self.critical_incidents == 0
        )

    @property
    def health_score(self) -> float:
        """
        Calculate overall health score (0-100).

        Weights:
        - Uptime: 30%
        - Success rate: 25%
        - Latency: 20%
        - Provision success: 15%
        - Incidents: 10%
        """
        # Normalize latency (target: <100ms = 100, >500ms = 0)
        latency_score = max(0, min(100, 100 - ((self.avg_latency_ms - 100) / 4)))

        # Incident penalty (each critical = -10, each normal = -2)
        incident_penalty = (self.critical_incidents * 10) + ((self.incident_count - self.critical_incidents) * 2)
        incident_score = max(0, 100 - incident_penalty)

        return round(
            (self.uptime_percent * 0.30) +
            (self.success_rate * 0.25) +
            (latency_score * 0.20) +
            (self.provision_success_rate * 0.15) +
            (incident_score * 0.10),
            2
        )
