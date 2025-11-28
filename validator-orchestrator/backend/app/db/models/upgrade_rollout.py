"""
Upgrade Rollout Model

Per-region rollout tracking for upgrades.
Tracks the rollout status in each region.

Table: upgrade_rollouts
"""

import uuid
from datetime import datetime
from typing import TYPE_CHECKING

from sqlalchemy import (
    Column,
    String,
    Integer,
    Boolean,
    DateTime,
    ForeignKey,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base
from app.db.models.enums import RolloutStatus

if TYPE_CHECKING:
    from app.db.models.upgrade import Upgrade


class UpgradeRollout(Base):
    """
    Per-region upgrade rollout tracking.

    Tracks the rollout status of an upgrade in each region,
    allowing for controlled regional rollouts.
    """

    __tablename__ = "upgrade_rollouts"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Foreign keys
    upgrade_id = Column(
        UUID(as_uuid=True),
        ForeignKey("upgrades.id", ondelete="CASCADE"),
        nullable=False,
        index=True
    )
    region_id = Column(
        UUID(as_uuid=True),
        ForeignKey("regions.id", ondelete="SET NULL"),
        nullable=True,
        index=True
    )
    region_code = Column(
        String(50),
        nullable=False,
        index=True
    )

    # Status
    status = Column(
        String(50),
        nullable=False,
        default=RolloutStatus.PENDING.value,
        index=True
    )

    # Rollout order
    rollout_order = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Order in rollout sequence"
    )
    is_canary = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether this is a canary region"
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
    skipped_nodes = Column(
        Integer,
        nullable=False,
        default=0
    )
    in_progress_nodes = Column(
        Integer,
        nullable=False,
        default=0
    )

    # Batching
    current_batch = Column(
        Integer,
        nullable=False,
        default=0
    )
    total_batches = Column(
        Integer,
        nullable=False,
        default=1
    )
    batch_size = Column(
        Integer,
        nullable=False,
        default=10
    )

    # Health checks
    pre_upgrade_health_passed = Column(
        Boolean,
        nullable=True
    )
    post_upgrade_health_passed = Column(
        Boolean,
        nullable=True
    )
    health_check_results = Column(
        JSONB,
        nullable=False,
        default=dict
    )

    # Timing
    scheduled_start = Column(
        DateTime,
        nullable=True
    )
    actual_start = Column(
        DateTime,
        nullable=True
    )
    estimated_completion = Column(
        DateTime,
        nullable=True
    )
    actual_completion = Column(
        DateTime,
        nullable=True
    )

    # Rollback
    rolled_back = Column(
        Boolean,
        nullable=False,
        default=False
    )
    rollback_reason = Column(
        Text,
        nullable=True
    )
    rolled_back_at = Column(
        DateTime,
        nullable=True
    )
    rollback_nodes = Column(
        Integer,
        nullable=False,
        default=0
    )

    # Error tracking
    error_message = Column(
        Text,
        nullable=True
    )
    error_details = Column(
        JSONB,
        nullable=True
    )
    last_error_at = Column(
        DateTime,
        nullable=True
    )

    # Extra data
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict
    )
    notes = Column(
        Text,
        nullable=True
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
    upgrade: Mapped["Upgrade"] = relationship(
        "Upgrade",
        back_populates="rollouts"
    )

    # Indexes
    __table_args__ = (
        Index("ix_upgrade_rollouts_upgrade_status", "upgrade_id", "status"),
        Index("ix_upgrade_rollouts_region", "region_code", "status"),
        Index("ix_upgrade_rollouts_upgrade_region", "upgrade_id", "region_code", unique=True),
    )

    def __repr__(self) -> str:
        return f"<UpgradeRollout {self.region_code} ({self.status})>"

    @property
    def is_pending(self) -> bool:
        """Check if rollout is pending."""
        return self.status == RolloutStatus.PENDING.value

    @property
    def is_in_progress(self) -> bool:
        """Check if rollout is in progress."""
        return self.status == RolloutStatus.IN_PROGRESS.value

    @property
    def is_completed(self) -> bool:
        """Check if rollout is completed."""
        return self.status == RolloutStatus.COMPLETED.value

    @property
    def is_failed(self) -> bool:
        """Check if rollout failed."""
        return self.status == RolloutStatus.FAILED.value

    @property
    def progress_percent(self) -> float:
        """Calculate rollout progress percentage."""
        if self.total_nodes == 0:
            return 0.0
        completed = self.upgraded_nodes + self.failed_nodes + self.skipped_nodes
        return round((completed / self.total_nodes) * 100, 2)

    @property
    def success_rate(self) -> float:
        """Calculate success rate."""
        attempted = self.upgraded_nodes + self.failed_nodes
        if attempted == 0:
            return 100.0
        return round((self.upgraded_nodes / attempted) * 100, 2)

    @property
    def pending_nodes(self) -> int:
        """Get pending node count."""
        return max(0, self.total_nodes - self.upgraded_nodes - self.failed_nodes - self.skipped_nodes)

    def set_status(self, status: RolloutStatus, error: str = None) -> None:
        """
        Update rollout status.

        Args:
            status: New status
            error: Error message if failed
        """
        self.status = status.value

        now = datetime.utcnow()
        if status == RolloutStatus.IN_PROGRESS:
            self.actual_start = self.actual_start or now
        elif status == RolloutStatus.COMPLETED:
            self.actual_completion = now
        elif status == RolloutStatus.FAILED:
            self.error_message = error
            self.last_error_at = now
        elif status == RolloutStatus.ROLLED_BACK:
            self.rolled_back = True
            self.rolled_back_at = now
            self.rollback_reason = error

    def update_progress(
        self,
        upgraded: int = 0,
        failed: int = 0,
        skipped: int = 0,
        in_progress: int = 0,
    ) -> None:
        """
        Update progress counters.

        Args:
            upgraded: Additional upgraded nodes
            failed: Additional failed nodes
            skipped: Additional skipped nodes
            in_progress: Current in-progress count
        """
        self.upgraded_nodes += upgraded
        self.failed_nodes += failed
        self.skipped_nodes += skipped
        self.in_progress_nodes = in_progress

        # Check for completion
        if self.pending_nodes == 0 and self.in_progress_nodes == 0:
            if self.failed_nodes == 0:
                self.set_status(RolloutStatus.COMPLETED)
            elif self.success_rate < 50:
                self.set_status(RolloutStatus.FAILED, "Too many nodes failed")

    def advance_batch(self) -> bool:
        """
        Advance to next batch.

        Returns:
            True if there are more batches
        """
        if self.current_batch >= self.total_batches:
            return False
        self.current_batch += 1
        return True

    def rollback(self, reason: str) -> None:
        """
        Initiate rollback.

        Args:
            reason: Rollback reason
        """
        self.set_status(RolloutStatus.ROLLED_BACK, reason)
