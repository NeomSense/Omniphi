"""
Local Validator Heartbeat Model

Tracks heartbeats from locally-run validators using the desktop app.
Used for monitoring local validators that aren't provisioned by the orchestrator.

Table: local_validator_heartbeats
"""

import uuid
from datetime import datetime
from typing import Optional

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB

from app.db.database import Base


class LocalValidatorHeartbeat(Base):
    """
    Local validator heartbeat tracking table.

    Stores heartbeats from validators running on user's local machines
    via the Omniphi desktop validator application.
    """

    __tablename__ = "local_validator_heartbeats"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Validator identification
    wallet_address = Column(
        String(100),
        nullable=False,
        index=True,
        doc="Wallet address of the validator operator"
    )
    consensus_pubkey = Column(
        String(255),
        nullable=False,
        unique=True,
        index=True,
        doc="Consensus public key (base64)"
    )
    validator_operator_address = Column(
        String(100),
        nullable=True,
        index=True,
        doc="On-chain validator operator address"
    )

    # Chain status
    block_height = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Current block height"
    )
    is_synced = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether node is synced"
    )
    catching_up = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Whether node is catching up"
    )
    peer_count = Column(
        Integer,
        nullable=True,
        doc="Number of connected peers"
    )

    # Validator status
    is_active_validator = Column(
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

    # Node health
    uptime_seconds = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Node uptime in seconds"
    )
    cpu_percent = Column(
        Float,
        nullable=True,
        doc="CPU usage percentage"
    )
    memory_percent = Column(
        Float,
        nullable=True,
        doc="Memory usage percentage"
    )
    disk_percent = Column(
        Float,
        nullable=True,
        doc="Disk usage percentage"
    )

    # Network endpoints (local)
    local_rpc_port = Column(
        Integer,
        nullable=True,
        default=26657,
        doc="Local RPC port"
    )
    local_p2p_port = Column(
        Integer,
        nullable=True,
        default=26656,
        doc="Local P2P port"
    )
    local_grpc_port = Column(
        Integer,
        nullable=True,
        default=9090,
        doc="Local gRPC port"
    )

    # Version info
    node_version = Column(
        String(50),
        nullable=True,
        doc="Node software version"
    )
    app_version = Column(
        String(50),
        nullable=True,
        doc="Desktop app version"
    )
    chain_id = Column(
        String(100),
        nullable=True,
        doc="Chain ID"
    )

    # Client info
    os_type = Column(
        String(50),
        nullable=True,
        doc="Operating system type"
    )
    os_version = Column(
        String(50),
        nullable=True,
        doc="Operating system version"
    )
    machine_id = Column(
        String(255),
        nullable=True,
        doc="Unique machine identifier"
    )

    # Extra data
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional heartbeat data"
    )

    # Timestamps
    first_seen = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        doc="First heartbeat timestamp"
    )
    last_seen = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        doc="Last heartbeat timestamp"
    )
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
        Index("ix_local_heartbeat_wallet_seen", "wallet_address", "last_seen"),
        Index("ix_local_heartbeat_active", "is_active_validator", "last_seen"),
    )

    def __repr__(self) -> str:
        return f"<LocalValidatorHeartbeat {self.wallet_address} @ block {self.block_height}>"

    @property
    def is_online(self) -> bool:
        """Check if validator has sent heartbeat recently (within 5 minutes)."""
        if not self.last_seen:
            return False
        delta = datetime.utcnow() - self.last_seen
        return delta.total_seconds() < 300  # 5 minutes

    @property
    def is_healthy(self) -> bool:
        """Check if validator appears healthy."""
        return (
            self.is_online and
            self.is_synced and
            not self.catching_up and
            not self.is_jailed and
            (self.peer_count or 0) > 0
        )

    @property
    def uptime_hours(self) -> float:
        """Get uptime in hours."""
        return round(self.uptime_seconds / 3600, 2)

    @property
    def uptime_days(self) -> float:
        """Get uptime in days."""
        return round(self.uptime_seconds / 86400, 2)

    def update_heartbeat(
        self,
        block_height: int,
        uptime_seconds: int,
        peer_count: Optional[int] = None,
        cpu_percent: Optional[float] = None,
        memory_percent: Optional[float] = None,
        disk_percent: Optional[float] = None,
        is_synced: bool = False,
        catching_up: bool = True,
    ) -> None:
        """
        Update heartbeat data.

        Args:
            block_height: Current block height
            uptime_seconds: Node uptime in seconds
            peer_count: Number of connected peers
            cpu_percent: CPU usage percentage
            memory_percent: Memory usage percentage
            disk_percent: Disk usage percentage
            is_synced: Whether node is synced
            catching_up: Whether node is catching up
        """
        self.block_height = block_height
        self.uptime_seconds = uptime_seconds
        self.is_synced = is_synced
        self.catching_up = catching_up
        self.last_seen = datetime.utcnow()

        if peer_count is not None:
            self.peer_count = peer_count
        if cpu_percent is not None:
            self.cpu_percent = cpu_percent
        if memory_percent is not None:
            self.memory_percent = memory_percent
        if disk_percent is not None:
            self.disk_percent = disk_percent

    def update_validator_status(
        self,
        voting_power: str,
        is_jailed: bool,
        is_active: bool,
    ) -> None:
        """
        Update on-chain validator status.

        Args:
            voting_power: Current voting power
            is_jailed: Whether jailed
            is_active: Whether active validator
        """
        self.voting_power = voting_power
        self.is_jailed = is_jailed
        self.is_active_validator = is_active
