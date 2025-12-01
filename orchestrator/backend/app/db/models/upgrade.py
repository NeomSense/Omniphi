"""
Upgrade Model

Chain upgrade management and tracking.
Handles scheduled upgrades and their rollout status.

Table: upgrades
"""

import uuid
from datetime import datetime
from typing import List, TYPE_CHECKING

from sqlalchemy import (
    Column,
    String,
    Integer,
    Boolean,
    DateTime,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base
from app.db.models.enums import UpgradeStatus

if TYPE_CHECKING:
    from app.db.models.upgrade_rollout import UpgradeRollout


class Upgrade(Base):
    """
    Chain upgrade definition.

    Tracks scheduled chain upgrades including version info,
    upgrade height, and overall rollout status.
    """

    __tablename__ = "upgrades"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Upgrade identification
    name = Column(
        String(100),
        nullable=False,
        index=True,
        doc="Upgrade name (e.g., v2.0.0)"
    )
    version = Column(
        String(50),
        nullable=False,
        index=True,
        doc="Target version"
    )
    version_tag = Column(
        String(100),
        nullable=True,
        doc="Git tag or release tag"
    )

    # Chain info
    chain_id = Column(
        String(100),
        nullable=False,
        index=True
    )

    # Upgrade height and timing
    upgrade_height = Column(
        Integer,
        nullable=False,
        index=True,
        doc="Block height at which upgrade activates"
    )
    estimated_time = Column(
        DateTime,
        nullable=True,
        doc="Estimated time of upgrade height"
    )
    actual_time = Column(
        DateTime,
        nullable=True,
        doc="Actual time upgrade completed"
    )

    # Upgrade details
    description = Column(
        Text,
        nullable=True
    )
    release_notes = Column(
        Text,
        nullable=True
    )
    release_url = Column(
        String(500),
        nullable=True,
        doc="URL to release page"
    )
    changelog_url = Column(
        String(500),
        nullable=True
    )

    # Binary info
    binary_url = Column(
        String(500),
        nullable=True,
        doc="Download URL for new binary"
    )
    binary_checksum = Column(
        String(128),
        nullable=True
    )
    docker_image = Column(
        String(255),
        nullable=True,
        doc="Docker image for upgrade"
    )

    # Cosmovisor support
    cosmovisor_compatible = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Whether cosmovisor auto-upgrade works"
    )
    upgrade_info = Column(
        JSONB,
        nullable=True,
        doc="Cosmovisor upgrade-info JSON"
    )

    # Status
    status = Column(
        String(50),
        nullable=False,
        default=UpgradeStatus.SCHEDULED.value,
        index=True
    )
    is_mandatory = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Whether upgrade is mandatory"
    )
    is_breaking = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether upgrade has breaking changes"
    )

    # Rollout configuration
    rollout_strategy = Column(
        String(50),
        nullable=False,
        default="sequential",
        doc="Rollout strategy (sequential, parallel, canary)"
    )
    rollout_percent_per_batch = Column(
        Integer,
        nullable=False,
        default=25,
        doc="Percentage of nodes per rollout batch"
    )
    min_healthy_percent = Column(
        Integer,
        nullable=False,
        default=90,
        doc="Minimum healthy nodes to proceed"
    )
    auto_rollback_enabled = Column(
        Boolean,
        nullable=False,
        default=True
    )

    # Progress tracking
    total_nodes = Column(
        Integer,
        nullable=False,
        default=0
    )
    upgraded_nodes = Column(
        Integer,
        nullable=False,
        default=0
    )
    failed_nodes = Column(
        Integer,
        nullable=False,
        default=0
    )
    pending_nodes = Column(
        Integer,
        nullable=False,
        default=0
    )

    # Notifications
    notification_sent = Column(
        Boolean,
        nullable=False,
        default=False
    )
    reminder_sent = Column(
        Boolean,
        nullable=False,
        default=False
    )
    completion_notified = Column(
        Boolean,
        nullable=False,
        default=False
    )

    # Extra data
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict
    )
    tags = Column(
        JSONB,
        nullable=False,
        default=list
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
        nullable=True
    )
    completed_at = Column(
        DateTime,
        nullable=True
    )
    cancelled_at = Column(
        DateTime,
        nullable=True
    )

    # Relationships
    rollouts: Mapped[List["UpgradeRollout"]] = relationship(
        "UpgradeRollout",
        back_populates="upgrade",
        cascade="all, delete-orphan",
        lazy="selectin"
    )

    # Indexes
    __table_args__ = (
        Index("ix_upgrades_chain_status", "chain_id", "status"),
        Index("ix_upgrades_chain_height", "chain_id", "upgrade_height"),
        Index("ix_upgrades_status_height", "status", "upgrade_height"),
    )

    def __repr__(self) -> str:
        return f"<Upgrade {self.name} @ height {self.upgrade_height}>"

    @property
    def is_scheduled(self) -> bool:
        """Check if upgrade is scheduled."""
        return self.status == UpgradeStatus.SCHEDULED.value

    @property
    def is_in_progress(self) -> bool:
        """Check if upgrade is in progress."""
        return self.status == UpgradeStatus.IN_PROGRESS.value

    @property
    def is_completed(self) -> bool:
        """Check if upgrade is completed."""
        return self.status == UpgradeStatus.COMPLETED.value

    @property
    def is_failed(self) -> bool:
        """Check if upgrade failed."""
        return self.status == UpgradeStatus.FAILED.value

    @property
    def progress_percent(self) -> float:
        """Calculate upgrade progress percentage."""
        if self.total_nodes == 0:
            return 0.0
        return round((self.upgraded_nodes / self.total_nodes) * 100, 2)

    @property
    def success_rate(self) -> float:
        """Calculate upgrade success rate."""
        attempted = self.upgraded_nodes + self.failed_nodes
        if attempted == 0:
            return 100.0
        return round((self.upgraded_nodes / attempted) * 100, 2)

    @property
    def is_upcoming(self) -> bool:
        """Check if upgrade is upcoming (within 7 days)."""
        if not self.estimated_time:
            return False
        delta = self.estimated_time - datetime.utcnow()
        return 0 < delta.days <= 7

    @property
    def is_imminent(self) -> bool:
        """Check if upgrade is imminent (within 24 hours)."""
        if not self.estimated_time:
            return False
        delta = self.estimated_time - datetime.utcnow()
        return 0 < delta.total_seconds() <= 86400

    def set_status(self, status: UpgradeStatus) -> None:
        """
        Update upgrade status with timestamps.

        Args:
            status: New status
        """
        self.status = status.value

        now = datetime.utcnow()
        if status == UpgradeStatus.IN_PROGRESS:
            self.started_at = self.started_at or now
        elif status == UpgradeStatus.COMPLETED:
            self.completed_at = now
            self.actual_time = now
        elif status == UpgradeStatus.CANCELLED:
            self.cancelled_at = now

    def update_progress(self, upgraded: int = 0, failed: int = 0, pending: int = 0) -> None:
        """
        Update progress counters.

        Args:
            upgraded: Additional upgraded nodes
            failed: Additional failed nodes
            pending: Total pending nodes
        """
        self.upgraded_nodes += upgraded
        self.failed_nodes += failed
        self.pending_nodes = pending

        # Auto-complete if all nodes upgraded
        if self.pending_nodes == 0 and self.upgraded_nodes > 0:
            if self.upgraded_nodes >= self.total_nodes * (self.min_healthy_percent / 100):
                self.set_status(UpgradeStatus.COMPLETED)
