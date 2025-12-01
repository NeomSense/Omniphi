"""
Incident Model

SRE incident tracking and alerting.
Records issues, their severity, and resolution status.

Table: incidents
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
from app.db.models.enums import IncidentSeverity, IncidentStatus, AlertType

if TYPE_CHECKING:
    from app.db.models.validator_node import ValidatorNode


class Incident(Base):
    """
    SRE incident record.

    Tracks operational incidents including detection, investigation,
    resolution, and post-mortem analysis.
    """

    __tablename__ = "incidents"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Related validator node (optional)
    validator_node_id = Column(
        UUID(as_uuid=True),
        ForeignKey("validator_nodes.id", ondelete="SET NULL"),
        nullable=True,
        index=True
    )

    # Related region
    region_id = Column(
        UUID(as_uuid=True),
        ForeignKey("regions.id", ondelete="SET NULL"),
        nullable=True,
        index=True
    )
    region_code = Column(
        String(50),
        nullable=True,
        index=True
    )

    # Incident identification
    incident_number = Column(
        String(50),
        nullable=False,
        unique=True,
        index=True,
        doc="Human-readable incident number (INC-001)"
    )
    title = Column(
        String(255),
        nullable=False,
        doc="Incident title"
    )

    # Classification
    severity = Column(
        String(20),
        nullable=False,
        default=IncidentSeverity.MEDIUM.value,
        index=True
    )
    status = Column(
        String(50),
        nullable=False,
        default=IncidentStatus.OPEN.value,
        index=True
    )
    alert_type = Column(
        String(50),
        nullable=True,
        index=True,
        doc="Type of alert that triggered incident"
    )
    category = Column(
        String(50),
        nullable=True,
        doc="Incident category"
    )

    # Description
    description = Column(
        Text,
        nullable=True,
        doc="Detailed incident description"
    )
    impact = Column(
        Text,
        nullable=True,
        doc="Impact description"
    )
    affected_validators = Column(
        Integer,
        nullable=False,
        default=1,
        doc="Number of affected validators"
    )
    affected_customers = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of affected customers"
    )

    # Detection
    detected_by = Column(
        String(100),
        nullable=True,
        doc="How incident was detected (monitoring, customer, manual)"
    )
    detected_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        index=True
    )
    alert_id = Column(
        String(255),
        nullable=True,
        doc="Original alert ID"
    )

    # Response
    acknowledged_by = Column(
        String(100),
        nullable=True
    )
    acknowledged_at = Column(
        DateTime,
        nullable=True
    )
    assigned_to = Column(
        String(100),
        nullable=True
    )
    escalated = Column(
        Boolean,
        nullable=False,
        default=False
    )
    escalated_at = Column(
        DateTime,
        nullable=True
    )

    # Investigation
    root_cause = Column(
        Text,
        nullable=True,
        doc="Root cause analysis"
    )
    root_cause_category = Column(
        String(50),
        nullable=True
    )
    contributing_factors = Column(
        JSONB,
        nullable=False,
        default=list
    )

    # Resolution
    resolution = Column(
        Text,
        nullable=True,
        doc="How incident was resolved"
    )
    resolution_type = Column(
        String(50),
        nullable=True,
        doc="Resolution type (fixed, workaround, cannot_reproduce)"
    )
    resolved_by = Column(
        String(100),
        nullable=True
    )
    resolved_at = Column(
        DateTime,
        nullable=True
    )

    # Timing metrics
    time_to_acknowledge_minutes = Column(
        Float,
        nullable=True
    )
    time_to_resolve_minutes = Column(
        Float,
        nullable=True
    )
    downtime_minutes = Column(
        Float,
        nullable=True,
        doc="Total downtime caused"
    )

    # Post-mortem
    post_mortem_completed = Column(
        Boolean,
        nullable=False,
        default=False
    )
    post_mortem_url = Column(
        String(500),
        nullable=True
    )
    lessons_learned = Column(
        Text,
        nullable=True
    )
    action_items = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Follow-up action items"
    )

    # Communication
    public_message = Column(
        Text,
        nullable=True,
        doc="Public status page message"
    )
    status_page_updated = Column(
        Boolean,
        nullable=False,
        default=False
    )
    customers_notified = Column(
        Boolean,
        nullable=False,
        default=False
    )

    # Related data
    related_incidents = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Related incident IDs"
    )
    timeline = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Incident timeline events"
    )
    attachments = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Attached files/logs"
    )

    # Metadata
    tags = Column(
        JSONB,
        nullable=False,
        default=list
    )
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
    closed_at = Column(
        DateTime,
        nullable=True
    )

    # Relationships
    node: Mapped[Optional["ValidatorNode"]] = relationship(
        "ValidatorNode",
        back_populates="incidents"
    )

    # Indexes
    __table_args__ = (
        Index("ix_incidents_severity_status", "severity", "status"),
        Index("ix_incidents_node", "validator_node_id"),
        Index("ix_incidents_detected", "detected_at"),
        Index("ix_incidents_open", "status", "severity", "detected_at"),
    )

    def __repr__(self) -> str:
        return f"<Incident {self.incident_number}: {self.title}>"

    @property
    def is_open(self) -> bool:
        """Check if incident is open."""
        return self.status not in [
            IncidentStatus.RESOLVED.value,
            IncidentStatus.CLOSED.value,
        ]

    @property
    def is_resolved(self) -> bool:
        """Check if incident is resolved."""
        return self.status in [
            IncidentStatus.RESOLVED.value,
            IncidentStatus.CLOSED.value,
        ]

    @property
    def is_critical(self) -> bool:
        """Check if incident is critical."""
        return self.severity == IncidentSeverity.CRITICAL.value

    @property
    def is_acknowledged(self) -> bool:
        """Check if incident is acknowledged."""
        return self.acknowledged_at is not None

    @property
    def age_hours(self) -> float:
        """Get incident age in hours."""
        end_time = self.resolved_at or datetime.utcnow()
        delta = end_time - self.detected_at
        return round(delta.total_seconds() / 3600, 2)

    @property
    def meets_sla(self) -> bool:
        """
        Check if response meets SLA targets.

        SLA targets by severity:
        - Critical: Ack <15min, Resolve <4hr
        - High: Ack <30min, Resolve <8hr
        - Medium: Ack <2hr, Resolve <24hr
        - Low: Ack <8hr, Resolve <72hr
        """
        sla_targets = {
            IncidentSeverity.CRITICAL.value: {"ack": 15, "resolve": 240},
            IncidentSeverity.HIGH.value: {"ack": 30, "resolve": 480},
            IncidentSeverity.MEDIUM.value: {"ack": 120, "resolve": 1440},
            IncidentSeverity.LOW.value: {"ack": 480, "resolve": 4320},
        }

        targets = sla_targets.get(self.severity, {"ack": 120, "resolve": 1440})

        ack_ok = True
        if self.time_to_acknowledge_minutes:
            ack_ok = self.time_to_acknowledge_minutes <= targets["ack"]

        resolve_ok = True
        if self.time_to_resolve_minutes:
            resolve_ok = self.time_to_resolve_minutes <= targets["resolve"]

        return ack_ok and resolve_ok

    def set_status(self, status: IncidentStatus, by: str = None) -> None:
        """
        Update incident status.

        Args:
            status: New status
            by: User making the change
        """
        now = datetime.utcnow()
        self.status = status.value

        if status == IncidentStatus.ACKNOWLEDGED:
            self.acknowledged_at = now
            self.acknowledged_by = by
            if self.detected_at:
                self.time_to_acknowledge_minutes = (now - self.detected_at).total_seconds() / 60

        elif status == IncidentStatus.RESOLVED:
            self.resolved_at = now
            self.resolved_by = by
            if self.detected_at:
                self.time_to_resolve_minutes = (now - self.detected_at).total_seconds() / 60

        elif status == IncidentStatus.CLOSED:
            self.closed_at = now

        # Add to timeline
        self.add_timeline_event(f"Status changed to {status.value}", by)

    def add_timeline_event(self, message: str, by: str = None) -> None:
        """
        Add event to incident timeline.

        Args:
            message: Event message
            by: User who performed action
        """
        event = {
            "timestamp": datetime.utcnow().isoformat(),
            "message": message,
            "by": by,
        }
        self.timeline = [*self.timeline, event]

    def acknowledge(self, by: str) -> None:
        """
        Acknowledge the incident.

        Args:
            by: User acknowledging
        """
        self.set_status(IncidentStatus.ACKNOWLEDGED, by)

    def resolve(self, by: str, resolution: str, resolution_type: str = "fixed") -> None:
        """
        Resolve the incident.

        Args:
            by: User resolving
            resolution: Resolution description
            resolution_type: Type of resolution
        """
        self.resolution = resolution
        self.resolution_type = resolution_type
        self.set_status(IncidentStatus.RESOLVED, by)

    def escalate(self, to: str, by: str = None) -> None:
        """
        Escalate the incident.

        Args:
            to: Escalation target
            by: User escalating
        """
        self.escalated = True
        self.escalated_at = datetime.utcnow()
        self.assigned_to = to
        self.add_timeline_event(f"Escalated to {to}", by)
