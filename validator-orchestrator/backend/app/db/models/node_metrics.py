"""
Node Metrics Model

Real-time and historical metrics for validator nodes.
Used for monitoring, alerting, and performance analysis.

Table: node_metrics
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
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base

if TYPE_CHECKING:
    from app.db.models.validator_node import ValidatorNode


class NodeMetrics(Base):
    """
    Validator node metrics record.

    Stores time-series metrics data for validator nodes including
    resource usage, chain status, and network performance.
    """

    __tablename__ = "node_metrics"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Foreign key
    validator_node_id = Column(
        UUID(as_uuid=True),
        ForeignKey("validator_nodes.id", ondelete="CASCADE"),
        nullable=False,
        index=True
    )

    # Timestamp
    recorded_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        index=True
    )
    period_type = Column(
        String(20),
        nullable=False,
        default="minute",
        doc="Aggregation period (minute, hour, day)"
    )

    # Resource usage - CPU
    cpu_percent = Column(
        Float,
        nullable=True,
        doc="CPU usage percentage"
    )
    cpu_cores_used = Column(
        Float,
        nullable=True,
        doc="CPU cores actively used"
    )
    load_average_1m = Column(
        Float,
        nullable=True,
        doc="1-minute load average"
    )
    load_average_5m = Column(
        Float,
        nullable=True,
        doc="5-minute load average"
    )
    load_average_15m = Column(
        Float,
        nullable=True,
        doc="15-minute load average"
    )

    # Resource usage - Memory
    memory_percent = Column(
        Float,
        nullable=True,
        doc="Memory usage percentage"
    )
    memory_used_gb = Column(
        Float,
        nullable=True,
        doc="Memory used in GB"
    )
    memory_available_gb = Column(
        Float,
        nullable=True,
        doc="Memory available in GB"
    )
    swap_percent = Column(
        Float,
        nullable=True,
        doc="Swap usage percentage"
    )

    # Resource usage - Disk
    disk_percent = Column(
        Float,
        nullable=True,
        doc="Disk usage percentage"
    )
    disk_used_gb = Column(
        Float,
        nullable=True,
        doc="Disk used in GB"
    )
    disk_available_gb = Column(
        Float,
        nullable=True,
        doc="Disk available in GB"
    )
    disk_read_mb_s = Column(
        Float,
        nullable=True,
        doc="Disk read speed MB/s"
    )
    disk_write_mb_s = Column(
        Float,
        nullable=True,
        doc="Disk write speed MB/s"
    )
    disk_iops = Column(
        Integer,
        nullable=True,
        doc="Disk IOPS"
    )

    # Network metrics
    network_rx_mb_s = Column(
        Float,
        nullable=True,
        doc="Network receive MB/s"
    )
    network_tx_mb_s = Column(
        Float,
        nullable=True,
        doc="Network transmit MB/s"
    )
    network_connections = Column(
        Integer,
        nullable=True,
        doc="Active network connections"
    )

    # Chain metrics
    block_height = Column(
        Integer,
        nullable=True,
        index=True,
        doc="Current block height"
    )
    blocks_behind = Column(
        Integer,
        nullable=True,
        doc="Blocks behind chain tip"
    )
    is_syncing = Column(
        Boolean,
        nullable=True,
        doc="Whether node is syncing"
    )
    sync_speed_blocks_per_sec = Column(
        Float,
        nullable=True,
        doc="Sync speed"
    )

    # P2P metrics
    peer_count = Column(
        Integer,
        nullable=True,
        doc="Connected peers"
    )
    inbound_peers = Column(
        Integer,
        nullable=True
    )
    outbound_peers = Column(
        Integer,
        nullable=True
    )
    peer_latency_avg_ms = Column(
        Float,
        nullable=True,
        doc="Average peer latency"
    )

    # Validator metrics
    voting_power = Column(
        String(50),
        nullable=True
    )
    missed_blocks = Column(
        Integer,
        nullable=True,
        doc="Missed blocks in current window"
    )
    missed_blocks_window = Column(
        Integer,
        nullable=True,
        doc="Missed blocks window size"
    )
    uptime_percent = Column(
        Float,
        nullable=True,
        doc="Uptime in current window"
    )
    is_jailed = Column(
        Boolean,
        nullable=True
    )
    commission_earned = Column(
        Float,
        nullable=True,
        doc="Commission earned (period)"
    )

    # RPC metrics
    rpc_requests_per_sec = Column(
        Float,
        nullable=True
    )
    rpc_latency_avg_ms = Column(
        Float,
        nullable=True
    )
    rpc_error_rate = Column(
        Float,
        nullable=True,
        doc="RPC error rate percentage"
    )

    # Process metrics
    process_cpu_percent = Column(
        Float,
        nullable=True,
        doc="Node process CPU"
    )
    process_memory_mb = Column(
        Float,
        nullable=True,
        doc="Node process memory"
    )
    goroutines = Column(
        Integer,
        nullable=True,
        doc="Go routines count"
    )
    open_files = Column(
        Integer,
        nullable=True
    )

    # Health score
    health_score = Column(
        Float,
        nullable=True,
        doc="Computed health score (0-100)"
    )
    health_status = Column(
        String(20),
        nullable=True,
        doc="Health status (healthy, warning, critical)"
    )

    # Additional data
    extra_metrics = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional metrics data"
    )

    # Relationships
    node: Mapped["ValidatorNode"] = relationship(
        "ValidatorNode",
        back_populates="metrics"
    )

    # Indexes
    __table_args__ = (
        Index("ix_node_metrics_node_time", "validator_node_id", "recorded_at"),
        Index("ix_node_metrics_time", "recorded_at"),
        Index("ix_node_metrics_period", "period_type", "recorded_at"),
    )

    def __repr__(self) -> str:
        return f"<NodeMetrics {self.validator_node_id} @ {self.recorded_at}>"

    @property
    def is_healthy(self) -> bool:
        """Check if metrics indicate healthy node."""
        return (
            (self.cpu_percent or 0) < 90 and
            (self.memory_percent or 0) < 90 and
            (self.disk_percent or 0) < 90 and
            (self.blocks_behind or 0) < 100 and
            not self.is_jailed
        )

    @property
    def has_resource_warning(self) -> bool:
        """Check if any resource is at warning level (>80%)."""
        return (
            (self.cpu_percent or 0) > 80 or
            (self.memory_percent or 0) > 80 or
            (self.disk_percent or 0) > 80
        )

    @property
    def has_resource_critical(self) -> bool:
        """Check if any resource is at critical level (>95%)."""
        return (
            (self.cpu_percent or 0) > 95 or
            (self.memory_percent or 0) > 95 or
            (self.disk_percent or 0) > 95
        )

    def calculate_health_score(self) -> float:
        """
        Calculate overall health score (0-100).

        Weights:
        - Resource usage: 30%
        - Chain sync: 30%
        - Network: 20%
        - Validator status: 20%
        """
        scores = []

        # Resource score (lower usage = higher score)
        if self.cpu_percent is not None:
            scores.append(max(0, 100 - self.cpu_percent))
        if self.memory_percent is not None:
            scores.append(max(0, 100 - self.memory_percent))
        if self.disk_percent is not None:
            scores.append(max(0, 100 - self.disk_percent))

        resource_score = sum(scores) / len(scores) if scores else 100

        # Sync score
        sync_score = 100
        if self.is_syncing:
            sync_score = 50
        if self.blocks_behind:
            sync_score = max(0, 100 - self.blocks_behind)

        # Network score
        network_score = 100
        if self.peer_count is not None:
            if self.peer_count < 5:
                network_score = 50
            elif self.peer_count < 10:
                network_score = 75

        # Validator score
        validator_score = 100
        if self.is_jailed:
            validator_score = 0
        elif self.missed_blocks and self.missed_blocks_window:
            miss_rate = self.missed_blocks / self.missed_blocks_window
            validator_score = max(0, 100 - (miss_rate * 200))

        # Weighted average
        self.health_score = round(
            (resource_score * 0.30) +
            (sync_score * 0.30) +
            (network_score * 0.20) +
            (validator_score * 0.20),
            2
        )

        # Set status
        if self.health_score >= 80:
            self.health_status = "healthy"
        elif self.health_score >= 50:
            self.health_status = "warning"
        else:
            self.health_status = "critical"

        return self.health_score
