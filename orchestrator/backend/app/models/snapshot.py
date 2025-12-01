"""
Snapshot Server Models

Database models for blockchain snapshots and fast sync.
"""

import enum
from datetime import datetime
from uuid import uuid4

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    ForeignKey,
    Enum,
    JSON,
    Text,
    Index,
    BigInteger,
)
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship

from app.database import Base


class SnapshotStatus(str, enum.Enum):
    """Snapshot status"""
    CREATING = "creating"
    UPLOADING = "uploading"
    AVAILABLE = "available"
    VERIFYING = "verifying"
    FAILED = "failed"
    EXPIRED = "expired"
    DELETED = "deleted"


class SnapshotType(str, enum.Enum):
    """Snapshot type"""
    FULL = "full"           # Complete state
    PRUNED = "pruned"       # Pruned state (smaller)
    ARCHIVE = "archive"     # Archive node snapshot


class DownloadStatus(str, enum.Enum):
    """Download status"""
    PENDING = "pending"
    DOWNLOADING = "downloading"
    EXTRACTING = "extracting"
    VERIFYING = "verifying"
    COMPLETED = "completed"
    FAILED = "failed"


class Snapshot(Base):
    """
    Blockchain snapshot record.

    Stores information about available snapshots for fast sync.
    """
    __tablename__ = "snapshots"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Chain identification
    chain_id = Column(String(50), nullable=False, index=True)
    network = Column(String(50), nullable=False, default="mainnet")

    # Snapshot details
    snapshot_type = Column(Enum(SnapshotType), nullable=False, default=SnapshotType.PRUNED)
    block_height = Column(BigInteger, nullable=False)
    block_time = Column(DateTime, nullable=True)

    # File information
    file_name = Column(String(255), nullable=False)
    file_url = Column(String(1000), nullable=False)
    file_size_bytes = Column(BigInteger, nullable=False)

    # Verification
    checksum_sha256 = Column(String(64), nullable=False)
    checksum_md5 = Column(String(32), nullable=True)

    # Storage location
    storage_provider = Column(String(50), nullable=False, default="s3")
    storage_bucket = Column(String(100), nullable=True)
    storage_region = Column(String(50), nullable=True)

    # Chunk information (for large snapshots)
    is_chunked = Column(Boolean, nullable=False, default=False)
    chunk_count = Column(Integer, nullable=True)
    chunk_size_bytes = Column(BigInteger, nullable=True)

    # Status
    status = Column(Enum(SnapshotStatus), nullable=False, default=SnapshotStatus.AVAILABLE)

    # Flags
    is_latest = Column(Boolean, nullable=False, default=False)
    is_verified = Column(Boolean, nullable=False, default=True)

    # Retention
    expires_at = Column(DateTime, nullable=True)

    # Metadata
    metadata = Column(JSON, nullable=True, default=dict)
    # e.g., {"cosmos_sdk_version": "0.47.0", "app_version": "1.0.0"}

    # Timestamps
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    downloads = relationship("SnapshotDownload", back_populates="snapshot")
    chunks = relationship("SnapshotChunk", back_populates="snapshot", cascade="all, delete-orphan")

    __table_args__ = (
        Index("ix_snapshots_chain_latest", "chain_id", "is_latest"),
        Index("ix_snapshots_chain_height", "chain_id", "block_height"),
        Index("ix_snapshots_status", "status"),
    )

    def __repr__(self):
        return f"<Snapshot {self.chain_id} @ {self.block_height}>"

    @property
    def file_size_gb(self) -> float:
        """Get file size in GB"""
        return self.file_size_bytes / (1024 ** 3)

    @property
    def is_expired(self) -> bool:
        """Check if snapshot is expired"""
        if self.expires_at is None:
            return False
        return datetime.utcnow() > self.expires_at


class SnapshotChunk(Base):
    """
    Snapshot chunk for large snapshots.

    Enables parallel downloading and resumable transfers.
    """
    __tablename__ = "snapshot_chunks"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    snapshot_id = Column(UUID(as_uuid=True), ForeignKey("snapshots.id", ondelete="CASCADE"), nullable=False)

    # Chunk details
    chunk_index = Column(Integer, nullable=False)
    file_name = Column(String(255), nullable=False)
    file_url = Column(String(1000), nullable=False)
    file_size_bytes = Column(BigInteger, nullable=False)

    # Verification
    checksum_sha256 = Column(String(64), nullable=False)

    # Range (byte offset in original file)
    byte_start = Column(BigInteger, nullable=False)
    byte_end = Column(BigInteger, nullable=False)

    # Timestamps
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)

    # Relationships
    snapshot = relationship("Snapshot", back_populates="chunks")

    __table_args__ = (
        Index("ix_snapshot_chunks_snapshot_index", "snapshot_id", "chunk_index"),
    )

    def __repr__(self):
        return f"<SnapshotChunk {self.snapshot_id} #{self.chunk_index}>"


class SnapshotDownload(Base):
    """
    Snapshot download tracking.

    Records download attempts and progress.
    """
    __tablename__ = "snapshot_downloads"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    snapshot_id = Column(UUID(as_uuid=True), ForeignKey("snapshots.id", ondelete="SET NULL"), nullable=True)
    node_id = Column(UUID(as_uuid=True), nullable=False, index=True)

    # Download status
    status = Column(Enum(DownloadStatus), nullable=False, default=DownloadStatus.PENDING)

    # Progress
    bytes_downloaded = Column(BigInteger, nullable=False, default=0)
    total_bytes = Column(BigInteger, nullable=False)
    progress_percent = Column(Float, nullable=False, default=0.0)

    # Chunk tracking (for chunked downloads)
    chunks_completed = Column(Integer, nullable=False, default=0)
    total_chunks = Column(Integer, nullable=False, default=1)
    current_chunk = Column(Integer, nullable=True)

    # Performance
    download_speed_mbps = Column(Float, nullable=True)
    estimated_remaining_seconds = Column(Integer, nullable=True)

    # Verification
    checksum_verified = Column(Boolean, nullable=False, default=False)

    # Error tracking
    error_message = Column(Text, nullable=True)
    retry_count = Column(Integer, nullable=False, default=0)
    max_retries = Column(Integer, nullable=False, default=3)

    # Timestamps
    started_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    completed_at = Column(DateTime, nullable=True)

    # Relationships
    snapshot = relationship("Snapshot", back_populates="downloads")

    __table_args__ = (
        Index("ix_snapshot_downloads_node", "node_id"),
        Index("ix_snapshot_downloads_status", "status"),
    )

    def __repr__(self):
        return f"<SnapshotDownload {self.node_id} ({self.status.value})>"


class SnapshotSchedule(Base):
    """
    Snapshot generation schedule.

    Configures automatic snapshot creation.
    """
    __tablename__ = "snapshot_schedules"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Target
    chain_id = Column(String(50), nullable=False, index=True)
    network = Column(String(50), nullable=False, default="mainnet")
    snapshot_type = Column(Enum(SnapshotType), nullable=False, default=SnapshotType.PRUNED)

    # Schedule (cron-like)
    schedule_cron = Column(String(100), nullable=False, default="0 0 * * *")  # Daily at midnight
    timezone = Column(String(50), nullable=False, default="UTC")

    # Storage configuration
    storage_provider = Column(String(50), nullable=False, default="s3")
    storage_bucket = Column(String(100), nullable=False)
    storage_path_prefix = Column(String(255), nullable=True)
    storage_region = Column(String(50), nullable=True)

    # Retention
    retention_days = Column(Integer, nullable=False, default=7)
    keep_latest_count = Column(Integer, nullable=False, default=3)

    # Chunking configuration
    enable_chunking = Column(Boolean, nullable=False, default=True)
    chunk_size_mb = Column(Integer, nullable=False, default=1024)  # 1GB chunks

    # Status
    is_active = Column(Boolean, nullable=False, default=True)
    last_run_at = Column(DateTime, nullable=True)
    last_success_at = Column(DateTime, nullable=True)
    last_failure_at = Column(DateTime, nullable=True)
    last_error = Column(Text, nullable=True)

    # Timestamps
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    __table_args__ = (
        Index("ix_snapshot_schedules_chain", "chain_id"),
    )

    def __repr__(self):
        return f"<SnapshotSchedule {self.chain_id} ({self.schedule_cron})>"


class SnapshotGeneration(Base):
    """
    Snapshot generation job record.

    Tracks individual snapshot generation runs.
    """
    __tablename__ = "snapshot_generations"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    schedule_id = Column(UUID(as_uuid=True), ForeignKey("snapshot_schedules.id", ondelete="SET NULL"), nullable=True)

    # Target
    chain_id = Column(String(50), nullable=False)
    snapshot_type = Column(Enum(SnapshotType), nullable=False)

    # Source node
    source_node_id = Column(UUID(as_uuid=True), nullable=True)
    source_block_height = Column(BigInteger, nullable=False)

    # Status
    status = Column(String(20), nullable=False, default="pending")
    # pending, creating, compressing, uploading, verifying, completed, failed

    # Progress
    progress_percent = Column(Float, nullable=False, default=0.0)
    current_step = Column(String(50), nullable=True)

    # Results
    snapshot_id = Column(UUID(as_uuid=True), ForeignKey("snapshots.id"), nullable=True)
    file_size_bytes = Column(BigInteger, nullable=True)

    # Error tracking
    error_message = Column(Text, nullable=True)

    # Timing
    started_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    completed_at = Column(DateTime, nullable=True)
    duration_seconds = Column(Integer, nullable=True)

    __table_args__ = (
        Index("ix_snapshot_generations_chain", "chain_id"),
        Index("ix_snapshot_generations_status", "status"),
    )

    def __repr__(self):
        return f"<SnapshotGeneration {self.chain_id} @ {self.source_block_height}>"
