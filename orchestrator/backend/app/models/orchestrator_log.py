"""Orchestrator Log Model."""

import uuid
from datetime import datetime
from typing import Optional
import enum

from sqlalchemy import Column, String, DateTime, Enum as SQLEnum, Text
from sqlalchemy.dialects.postgresql import UUID

from app.db.base_class import Base


class LogLevel(str, enum.Enum):
    """Log level types."""
    DEBUG = "debug"
    INFO = "info"
    WARN = "warn"
    ERROR = "error"


class LogSource(str, enum.Enum):
    """Log source types."""
    ORCHESTRATOR = "orchestrator"
    PROVISIONING = "provisioning"
    HEALTH = "health"
    DOCKER = "docker"
    CHAIN = "chain"


class OrchestratorLog(Base):
    """Orchestrator logs table."""

    __tablename__ = "orchestrator_logs"

    # Primary key
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4, index=True)

    # Log info
    level = Column(SQLEnum(LogLevel), nullable=False, default=LogLevel.INFO, index=True)
    source = Column(SQLEnum(LogSource), nullable=False, default=LogSource.ORCHESTRATOR, index=True)
    message = Column(Text, nullable=False)

    # Related resources (optional)
    request_id = Column(String, nullable=True, index=True)
    node_id = Column(String, nullable=True, index=True)

    # Timestamp
    timestamp = Column(DateTime, nullable=False, default=datetime.utcnow, index=True)

    def __repr__(self):
        return f"<OrchestratorLog [{self.level}] {self.source}: {self.message[:50]}>"
