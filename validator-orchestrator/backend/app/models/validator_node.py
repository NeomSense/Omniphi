"""Validator Node Model."""

import uuid
from datetime import datetime
from typing import Optional

from sqlalchemy import Column, String, DateTime, Enum as SQLEnum, ForeignKey
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship
import enum

from app.db.base_class import Base


class NodeStatus(str, enum.Enum):
    """Node operational status."""
    STARTING = "starting"
    RUNNING = "running"
    SYNCING = "syncing"
    SYNCED = "synced"
    STOPPED = "stopped"
    ERROR = "error"
    TERMINATED = "terminated"


class ValidatorNode(Base):
    """Validator node instance table."""

    __tablename__ = "validator_nodes"

    # Primary key
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4, index=True)

    # Foreign key to setup request
    setup_request_id = Column(
        UUID(as_uuid=True),
        ForeignKey("validator_setup_requests.id", ondelete="CASCADE"),
        nullable=False,
        index=True
    )

    # Provider details
    provider = Column(String, nullable=False)  # omniphi_cloud, aws, gcp, etc.
    node_internal_id = Column(String, nullable=False, unique=True)  # Docker container ID / VM ID

    # Network endpoints
    rpc_endpoint = Column(String, nullable=True)  # http://ip:26657
    p2p_endpoint = Column(String, nullable=True)  # tcp://ip:26656
    grpc_endpoint = Column(String, nullable=True)  # ip:9090

    # Operational status
    status = Column(SQLEnum(NodeStatus), nullable=False, default=NodeStatus.STARTING)

    # Monitoring
    logs_url = Column(String, nullable=True)
    last_block_height = Column(String, nullable=True)
    last_health_check = Column(DateTime, nullable=True)

    # Resource info
    cpu_cores = Column(String, nullable=True)
    memory_gb = Column(String, nullable=True)
    disk_gb = Column(String, nullable=True)

    # Timestamps
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)
    terminated_at = Column(DateTime, nullable=True)

    # Relationship
    # setup_request = relationship("ValidatorSetupRequest", back_populates="nodes")

    def __repr__(self):
        return f"<ValidatorNode {self.node_internal_id} ({self.status})>"
