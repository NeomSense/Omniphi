"""
Snapshot Model

Chain state snapshots for fast validator sync.
Allows new validators to quickly catch up without full sync.

Table: snapshots
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
    ForeignKey,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB

from app.db.database import Base


class Snapshot(Base):
    """
    Chain state snapshot record.

    Represents a snapshot of chain state at a specific height
    that can be used for fast sync of new validator nodes.
    """

    __tablename__ = "snapshots"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Region
    region_id = Column(
        UUID(as_uuid=True),
        ForeignKey("regions.id", ondelete="SET NULL"),
        nullable=True,
        index=True,
        doc="Region where snapshot is stored"
    )
    region_code = Column(
        String(50),
        nullable=True,
        index=True,
        doc="Region code (for fast lookup)"
    )

    # Chain info
    chain_id = Column(
        String(100),
        nullable=False,
        index=True,
        doc="Chain identifier"
    )
    network_type = Column(
        String(20),
        nullable=False,
        default="mainnet",
        doc="Network type (mainnet, testnet)"
    )

    # Snapshot details
    height = Column(
        Integer,
        nullable=False,
        index=True,
        doc="Block height at snapshot"
    )
    block_hash = Column(
        String(100),
        nullable=True,
        doc="Block hash at snapshot height"
    )
    app_hash = Column(
        String(100),
        nullable=True,
        doc="Application state hash"
    )
    snapshot_time = Column(
        DateTime,
        nullable=False,
        doc="Timestamp of the block"
    )

    # Snapshot files
    snapshot_url = Column(
        String(1000),
        nullable=False,
        doc="Primary download URL"
    )
    mirror_urls = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="Mirror download URLs"
    )

    # File info
    file_size_bytes = Column(
        Integer,
        nullable=False,
        doc="Snapshot file size in bytes"
    )
    file_size_compressed = Column(
        Integer,
        nullable=True,
        doc="Compressed size if applicable"
    )
    compression_type = Column(
        String(20),
        nullable=True,
        doc="Compression type (lz4, zstd, gzip)"
    )
    format_type = Column(
        String(20),
        nullable=False,
        default="tar",
        doc="Archive format"
    )

    # Verification
    checksum = Column(
        String(128),
        nullable=False,
        doc="File checksum"
    )
    checksum_type = Column(
        String(20),
        nullable=False,
        default="sha256",
        doc="Checksum algorithm"
    )
    verified = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Whether snapshot has been verified"
    )
    verified_at = Column(
        DateTime,
        nullable=True
    )

    # Node version
    node_version = Column(
        String(50),
        nullable=True,
        doc="Node version used to create snapshot"
    )
    state_sync_compatible = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Compatible with state sync"
    )

    # Status
    is_active = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True,
        doc="Whether snapshot is available"
    )
    is_latest = Column(
        Boolean,
        nullable=False,
        default=False,
        index=True,
        doc="Whether this is the latest snapshot"
    )
    is_recommended = Column(
        Boolean,
        nullable=False,
        default=False,
        doc="Recommended for new validators"
    )

    # Usage stats
    download_count = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of downloads"
    )
    restore_count = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Successful restores"
    )
    failure_count = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Failed restores"
    )

    # Performance metrics
    avg_download_speed_mbps = Column(
        Float,
        nullable=True,
        doc="Average download speed"
    )
    avg_restore_time_seconds = Column(
        Float,
        nullable=True,
        doc="Average restore time"
    )

    # Metadata
    description = Column(
        Text,
        nullable=True
    )
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
    expires_at = Column(
        DateTime,
        nullable=True,
        doc="When snapshot expires"
    )

    # Indexes
    __table_args__ = (
        Index("ix_snapshots_chain_height", "chain_id", "height"),
        Index("ix_snapshots_region_active", "region_code", "is_active"),
        Index("ix_snapshots_latest", "chain_id", "is_latest"),
        Index("ix_snapshots_created", "created_at"),
    )

    def __repr__(self) -> str:
        return f"<Snapshot {self.chain_id} @ height {self.height}>"

    @property
    def file_size_gb(self) -> float:
        """Get file size in GB."""
        return round(self.file_size_bytes / (1024 ** 3), 2)

    @property
    def file_size_human(self) -> str:
        """Get human-readable file size."""
        size = self.file_size_bytes
        for unit in ["B", "KB", "MB", "GB", "TB"]:
            if size < 1024:
                return f"{size:.1f} {unit}"
            size /= 1024
        return f"{size:.1f} PB"

    @property
    def age_hours(self) -> float:
        """Get snapshot age in hours."""
        delta = datetime.utcnow() - self.snapshot_time
        return delta.total_seconds() / 3600

    @property
    def age_blocks(self) -> Optional[int]:
        """Get approximate age in blocks (assuming ~6s block time)."""
        age_seconds = (datetime.utcnow() - self.snapshot_time).total_seconds()
        return int(age_seconds / 6)

    @property
    def is_fresh(self) -> bool:
        """Check if snapshot is fresh (< 24 hours old)."""
        return self.age_hours < 24

    @property
    def success_rate(self) -> float:
        """Calculate restore success rate."""
        total = self.restore_count + self.failure_count
        if total == 0:
            return 100.0
        return round((self.restore_count / total) * 100, 2)

    @property
    def is_expired(self) -> bool:
        """Check if snapshot has expired."""
        if not self.expires_at:
            return False
        return datetime.utcnow() > self.expires_at

    def record_download(self) -> None:
        """Record a download."""
        self.download_count += 1

    def record_restore(self, success: bool, duration_seconds: float = None) -> None:
        """
        Record a restore attempt.

        Args:
            success: Whether restore succeeded
            duration_seconds: Restore duration
        """
        if success:
            self.restore_count += 1
            if duration_seconds and self.avg_restore_time_seconds:
                # Running average
                total = self.restore_count * self.avg_restore_time_seconds
                self.avg_restore_time_seconds = (total + duration_seconds) / (self.restore_count + 1)
            elif duration_seconds:
                self.avg_restore_time_seconds = duration_seconds
        else:
            self.failure_count += 1

    def mark_as_latest(self) -> None:
        """Mark this snapshot as the latest."""
        self.is_latest = True
        self.is_recommended = True

    def deactivate(self) -> None:
        """Deactivate snapshot."""
        self.is_active = False
        self.is_latest = False
        self.is_recommended = False
