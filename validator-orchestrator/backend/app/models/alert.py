"""Alert Model."""

import uuid
from datetime import datetime
from typing import Optional
import enum

from sqlalchemy import Column, String, DateTime, Enum as SQLEnum, JSON, Boolean
from sqlalchemy.dialects.postgresql import UUID

from app.db.base_class import Base


class AlertSeverity(str, enum.Enum):
    """Alert severity levels."""
    INFO = "info"
    WARNING = "warning"
    ERROR = "error"
    CRITICAL = "critical"


class AlertStatus(str, enum.Enum):
    """Alert status."""
    ACTIVE = "active"
    ACKNOWLEDGED = "acknowledged"
    RESOLVED = "resolved"


class Alert(Base):
    """Alerts table for system incidents."""

    __tablename__ = "alerts"

    # Primary key
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4, index=True)

    # Alert info
    title = Column(String, nullable=False)
    message = Column(String, nullable=False)
    severity = Column(SQLEnum(AlertSeverity), nullable=False, default=AlertSeverity.WARNING, index=True)
    status = Column(SQLEnum(AlertStatus), nullable=False, default=AlertStatus.ACTIVE, index=True)

    # Related resources
    resource_type = Column(String, nullable=True)  # e.g., "node", "setup_request"
    resource_id = Column(String, nullable=True, index=True)

    # Additional context
    details = Column(JSON, nullable=False, default=dict)

    # Acknowledgment info
    acknowledged_by = Column(String, nullable=True)
    acknowledged_at = Column(DateTime, nullable=True)

    # Resolution info
    resolved_by = Column(String, nullable=True)
    resolved_at = Column(DateTime, nullable=True)
    resolution_notes = Column(String, nullable=True)

    # Timestamps
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow, index=True)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    def __repr__(self):
        return f"<Alert {self.title} ({self.severity})>"
