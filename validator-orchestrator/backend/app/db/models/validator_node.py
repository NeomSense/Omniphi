"""
Validator Node Model

Represents deployed validator containers or VMs.
This is the actual running validator instance.

Table: validator_nodes
"""

import uuid
from datetime import datetime
from typing import Optional, List, TYPE_CHECKING

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
from app.db.models.enums import NodeStatus

if TYPE_CHECKING:
    from app.db.models.validator_setup_request import ValidatorSetupRequest
    from app.db.models.provider import Provider
    from app.db.models.region import Region
    from app.db.models.region_server import RegionServer
    from app.db.models.node_metrics import NodeMetrics
    from app.db.models.incident import Incident


class ValidatorNode(Base):
    """
    Validator node instance table.

    Represents a deployed validator including its infrastructure details,
    network endpoints, operational status, and resource allocation.
    """

    __tablename__ = "validator_nodes"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Foreign keys
    setup_request_id = Column(
        UUID(as_uuid=True),
        ForeignKey("validator_setup_requests.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
        doc="Parent setup request"
    )
    provider_id = Column(
        UUID(as_uuid=True),
        ForeignKey("providers.id", ondelete="SET NULL"),
        nullable=True,
        index=True,
        doc="Hosting provider"
    )
    region_id = Column(
        UUID(as_uuid=True),
        ForeignKey("regions.id", ondelete="SET NULL"),
        nullable=True,
        index=True,
        doc="Deployment region"
    )
    server_id = Column(
        UUID(as_uuid=True),
        ForeignKey("region_servers.id", ondelete="SET NULL"),
        nullable=True,
        index=True,
        doc="Host server"
    )

    # Container/VM identification
    container_id = Column(
        String(255),
        nullable=True,
        unique=True,
        index=True,
        doc="Docker container ID"
    )
    vm_instance_id = Column(
        String(255),
        nullable=True,
        doc="Cloud VM instance ID"
    )
    kubernetes_pod = Column(
        String(255),
        nullable=True,
        doc="Kubernetes pod name"
    )

    # Network endpoints
    rpc_endpoint = Column(
        String(255),
        nullable=True,
        doc="RPC endpoint URL (http://ip:26657)"
    )
    p2p_endpoint = Column(
        String(255),
        nullable=True,
        doc="P2P endpoint (tcp://ip:26656)"
    )
    grpc_endpoint = Column(
        String(255),
        nullable=True,
        doc="gRPC endpoint (ip:9090)"
    )
    rest_endpoint = Column(
        String(255),
        nullable=True,
        doc="REST API endpoint (http://ip:1317)"
    )
    metrics_endpoint = Column(
        String(255),
        nullable=True,
        doc="Prometheus metrics endpoint"
    )

    # Internal networking
    internal_ip = Column(
        String(45),
        nullable=True,
        doc="Internal/private IP"
    )
    external_ip = Column(
        String(45),
        nullable=True,
        doc="External/public IP"
    )
    p2p_port = Column(
        Integer,
        nullable=False,
        default=26656,
        doc="P2P port"
    )
    rpc_port = Column(
        Integer,
        nullable=False,
        default=26657,
        doc="RPC port"
    )

    # Operational status
    status = Column(
        String(50),
        nullable=False,
        default=NodeStatus.STARTING.value,
        index=True,
        doc="Node operational status"
    )
    is_active = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True,
        doc="Whether node is the active instance"
    )

    # Chain status
    last_block_height = Column(
        Integer,
        nullable=True,
        doc="Last known block height"
    )
    is_synced = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether node is synced with chain"
    )
    peer_count = Column(
        Integer,
        nullable=True,
        doc="Number of connected peers"
    )
    catching_up = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Whether node is catching up"
    )

    # Validator status (on-chain)
    is_validator = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether active as validator on chain"
    )
    voting_power = Column(
        String(50),
        nullable=True,
        doc="Current voting power"
    )
    is_jailed = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether validator is jailed"
    )
    jailed_until = Column(
        DateTime,
        nullable=True,
        doc="Jail release time"
    )
    missed_blocks = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Missed blocks counter"
    )

    # Resource allocation
    cpu_cores = Column(
        Integer,
        nullable=True,
        doc="Allocated CPU cores"
    )
    memory_gb = Column(
        Integer,
        nullable=True,
        doc="Allocated memory in GB"
    )
    disk_gb = Column(
        Integer,
        nullable=True,
        doc="Allocated disk in GB"
    )
    bandwidth_gbps = Column(
        Float,
        nullable=True,
        doc="Network bandwidth in Gbps"
    )

    # Version info
    node_version = Column(
        String(50),
        nullable=True,
        doc="Node software version"
    )
    chain_id = Column(
        String(100),
        nullable=True,
        doc="Chain ID"
    )

    # Monitoring
    last_heartbeat = Column(
        DateTime,
        nullable=True,
        doc="Last health check timestamp"
    )
    last_health_check = Column(
        DateTime,
        nullable=True,
        doc="Last comprehensive health check"
    )
    health_score = Column(
        Float,
        nullable=False,
        default=100.0,
        doc="Health score (0-100)"
    )
    uptime_percent = Column(
        Float,
        nullable=False,
        default=100.0,
        doc="Uptime percentage"
    )

    # Logs and debugging
    logs_url = Column(
        String(500),
        nullable=True,
        doc="URL to access logs"
    )
    logs_container_path = Column(
        String(500),
        nullable=True,
        doc="Path to logs in container"
    )

    # Migration tracking
    previous_region_id = Column(
        UUID(as_uuid=True),
        nullable=True,
        doc="Previous region if migrated"
    )
    migration_id = Column(
        UUID(as_uuid=True),
        nullable=True,
        doc="Associated migration record"
    )

    # Metadata
    labels = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Custom labels"
    )
    annotations = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional annotations"
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
    started_at = Column(
        DateTime,
        nullable=True,
        doc="When node was started"
    )
    stopped_at = Column(
        DateTime,
        nullable=True,
        doc="When node was stopped"
    )
    terminated_at = Column(
        DateTime,
        nullable=True,
        doc="When node was terminated"
    )

    # Relationships
    setup_request: Mapped["ValidatorSetupRequest"] = relationship(
        "ValidatorSetupRequest",
        back_populates="nodes"
    )
    provider: Mapped[Optional["Provider"]] = relationship(
        "Provider",
        foreign_keys=[provider_id]
    )
    region: Mapped[Optional["Region"]] = relationship(
        "Region",
        foreign_keys=[region_id]
    )
    server: Mapped[Optional["RegionServer"]] = relationship(
        "RegionServer",
        foreign_keys=[server_id]
    )
    metrics: Mapped[List["NodeMetrics"]] = relationship(
        "NodeMetrics",
        back_populates="node",
        cascade="all, delete-orphan",
        lazy="selectin"
    )
    incidents: Mapped[List["Incident"]] = relationship(
        "Incident",
        back_populates="node",
        cascade="all, delete-orphan",
        lazy="selectin"
    )

    # Indexes
    __table_args__ = (
        Index("ix_validator_nodes_provider_status", "provider_id", "status"),
        Index("ix_validator_nodes_region_status", "region_id", "status"),
        Index("ix_validator_nodes_server", "server_id"),
        Index("ix_validator_nodes_active", "is_active", "status"),
    )

    def __repr__(self) -> str:
        return f"<ValidatorNode {self.container_id or self.id} ({self.status})>"

    @property
    def is_running(self) -> bool:
        """Check if node is in running state."""
        return self.status in [
            NodeStatus.RUNNING.value,
            NodeStatus.SYNCING.value,
            NodeStatus.SYNCED.value,
        ]

    @property
    def is_healthy(self) -> bool:
        """Check if node is healthy."""
        return (
            self.is_running and
            self.health_score >= 80 and
            not self.is_jailed
        )

    @property
    def is_terminated(self) -> bool:
        """Check if node is terminated."""
        return self.status == NodeStatus.TERMINATED.value

    @property
    def needs_attention(self) -> bool:
        """Check if node needs attention."""
        return (
            self.status == NodeStatus.ERROR.value or
            self.is_jailed or
            self.health_score < 50 or
            (self.catching_up and self.is_validator)
        )

    def set_status(self, status: NodeStatus) -> None:
        """
        Update node status with appropriate timestamps.

        Args:
            status: New node status
        """
        self.status = status.value

        now = datetime.utcnow()
        if status == NodeStatus.RUNNING:
            self.started_at = self.started_at or now
            self.stopped_at = None
        elif status == NodeStatus.STOPPED:
            self.stopped_at = now
        elif status == NodeStatus.TERMINATED:
            self.terminated_at = now
            self.is_active = False

    def update_heartbeat(self) -> None:
        """Update heartbeat timestamp."""
        self.last_heartbeat = datetime.utcnow()

    def update_chain_status(
        self,
        block_height: int,
        peer_count: int,
        catching_up: bool,
        synced: bool,
    ) -> None:
        """
        Update chain sync status.

        Args:
            block_height: Current block height
            peer_count: Number of connected peers
            catching_up: Whether still catching up
            synced: Whether fully synced
        """
        self.last_block_height = block_height
        self.peer_count = peer_count
        self.catching_up = catching_up
        self.is_synced = synced
        self.last_health_check = datetime.utcnow()

        # Update status based on sync state
        if not catching_up and synced:
            if self.status == NodeStatus.SYNCING.value:
                self.status = NodeStatus.SYNCED.value

    def update_validator_status(
        self,
        voting_power: str,
        jailed: bool,
        jailed_until: Optional[datetime] = None,
    ) -> None:
        """
        Update on-chain validator status.

        Args:
            voting_power: Current voting power
            jailed: Whether jailed
            jailed_until: Jail release time
        """
        self.voting_power = voting_power
        self.is_jailed = jailed
        self.jailed_until = jailed_until
        self.is_validator = int(voting_power or "0") > 0
