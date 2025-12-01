"""
Validator Upgrade Pipeline Models

Database models for chain upgrades, canary rollouts, and upgrade tracking.
Supports zero-downtime upgrades with automatic rollback.
"""

import enum
from datetime import datetime
from typing import Optional
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
)
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship

from app.database import Base


class UpgradeStatus(str, enum.Enum):
    """Status of an upgrade"""
    SCHEDULED = "scheduled"
    CANARY = "canary"           # Canary rollout (1%)
    ROLLING_OUT = "rolling_out"  # Regional rollout in progress
    PAUSED = "paused"           # Rollout paused
    COMPLETED = "completed"
    FAILED = "failed"
    ROLLED_BACK = "rolled_back"


class NodeUpgradeStatusEnum(str, enum.Enum):
    """Status of a node's upgrade"""
    PENDING = "pending"
    DOWNLOADING = "downloading"
    INSTALLING = "installing"
    RESTARTING = "restarting"
    VERIFYING = "verifying"
    COMPLETED = "completed"
    FAILED = "failed"
    SKIPPED = "skipped"
    ROLLED_BACK = "rolled_back"


class UpgradeType(str, enum.Enum):
    """Type of upgrade"""
    CHAIN_UPGRADE = "chain_upgrade"      # Cosmos SDK upgrade at specific height
    BINARY_UPDATE = "binary_update"      # Regular binary update
    CONFIG_CHANGE = "config_change"      # Configuration only
    HOTFIX = "hotfix"                    # Emergency hotfix


class ChainUpgrade(Base):
    """
    Chain upgrade definition and rollout tracking.

    Manages the upgrade process across the entire fleet with canary support.
    """
    __tablename__ = "chain_upgrades"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Upgrade identification
    name = Column(String(100), nullable=False)
    version = Column(String(50), nullable=False)
    upgrade_type = Column(Enum(UpgradeType), nullable=False, default=UpgradeType.CHAIN_UPGRADE)

    # Binary information
    current_version = Column(String(50), nullable=False)
    new_binary_version = Column(String(50), nullable=False)
    binary_url = Column(String(500), nullable=True)
    binary_checksum = Column(String(128), nullable=True)  # SHA256

    # Chain upgrade specifics
    upgrade_height = Column(Integer, nullable=True)  # Block height for chain upgrades
    current_height = Column(Integer, nullable=False, default=0)

    # Scheduling
    scheduled_time = Column(DateTime, nullable=True)
    started_at = Column(DateTime, nullable=True)
    completed_at = Column(DateTime, nullable=True)

    # Status and progress
    status = Column(Enum(UpgradeStatus), nullable=False, default=UpgradeStatus.SCHEDULED)
    total_nodes = Column(Integer, nullable=False, default=0)
    updated_nodes = Column(Integer, nullable=False, default=0)
    failed_nodes = Column(Integer, nullable=False, default=0)
    pending_nodes = Column(Integer, nullable=False, default=0)

    # Canary configuration
    canary_enabled = Column(Boolean, nullable=False, default=True)
    canary_percent = Column(Float, nullable=False, default=1.0)  # 1% canary
    canary_nodes = Column(JSON, nullable=False, default=list)  # List of canary node IDs
    canary_completed = Column(Boolean, nullable=False, default=False)
    canary_success = Column(Boolean, nullable=False, default=False)
    canary_wait_minutes = Column(Integer, nullable=False, default=30)

    # Regional rollout order
    region_order = Column(JSON, nullable=False, default=list)  # ["us-east", "us-west", ...]
    current_region = Column(String(50), nullable=True)

    # Rollback configuration
    rollback_available = Column(Boolean, nullable=False, default=True)
    rollback_version = Column(String(50), nullable=True)
    rollback_binary_url = Column(String(500), nullable=True)

    # Upgrade details
    release_notes = Column(Text, nullable=True)
    breaking_changes = Column(JSON, nullable=True, default=list)
    required_actions = Column(JSON, nullable=True, default=list)

    # Metadata
    created_by = Column(String(100), nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    node_statuses = relationship("NodeUpgradeStatus", back_populates="upgrade", cascade="all, delete-orphan")
    logs = relationship("UpgradeLog", back_populates="upgrade", cascade="all, delete-orphan")

    __table_args__ = (
        Index("ix_chain_upgrades_status", "status"),
        Index("ix_chain_upgrades_scheduled", "scheduled_time"),
    )

    def __repr__(self):
        return f"<ChainUpgrade {self.name} v{self.version} ({self.status.value})>"

    @property
    def completion_percent(self) -> float:
        """Calculate upgrade completion percentage"""
        if self.total_nodes == 0:
            return 0.0
        return (self.updated_nodes / self.total_nodes) * 100

    @property
    def success_rate(self) -> float:
        """Calculate success rate of completed upgrades"""
        completed = self.updated_nodes + self.failed_nodes
        if completed == 0:
            return 0.0
        return (self.updated_nodes / completed) * 100

    @property
    def is_in_progress(self) -> bool:
        """Check if upgrade is actively in progress"""
        return self.status in [UpgradeStatus.CANARY, UpgradeStatus.ROLLING_OUT]


class NodeUpgradeStatus(Base):
    """
    Individual node upgrade status tracking.

    Tracks the upgrade progress for each validator node.
    """
    __tablename__ = "node_upgrade_statuses"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    upgrade_id = Column(UUID(as_uuid=True), ForeignKey("chain_upgrades.id", ondelete="CASCADE"), nullable=False)
    node_id = Column(UUID(as_uuid=True), nullable=False, index=True)

    # Node identification
    moniker = Column(String(100), nullable=False)
    region = Column(String(50), nullable=False)

    # Version tracking
    current_version = Column(String(50), nullable=False)
    target_version = Column(String(50), nullable=False)

    # Status
    status = Column(Enum(NodeUpgradeStatusEnum), nullable=False, default=NodeUpgradeStatusEnum.PENDING)
    is_canary = Column(Boolean, nullable=False, default=False)

    # Progress tracking
    download_percent = Column(Float, nullable=False, default=0.0)
    install_percent = Column(Float, nullable=False, default=0.0)

    # Timing
    scheduled_at = Column(DateTime, nullable=True)
    started_at = Column(DateTime, nullable=True)
    completed_at = Column(DateTime, nullable=True)
    duration_seconds = Column(Integer, nullable=True)

    # Error tracking
    error_message = Column(Text, nullable=True)
    retry_count = Column(Integer, nullable=False, default=0)
    max_retries = Column(Integer, nullable=False, default=3)

    # Health after upgrade
    post_upgrade_healthy = Column(Boolean, nullable=True)
    post_upgrade_block_height = Column(Integer, nullable=True)
    post_upgrade_peers = Column(Integer, nullable=True)

    # Relationships
    upgrade = relationship("ChainUpgrade", back_populates="node_statuses")

    __table_args__ = (
        Index("ix_node_upgrade_status_upgrade_status", "upgrade_id", "status"),
        Index("ix_node_upgrade_status_node", "node_id"),
    )

    def __repr__(self):
        return f"<NodeUpgradeStatus {self.moniker} -> {self.target_version} ({self.status.value})>"

    @property
    def can_retry(self) -> bool:
        """Check if upgrade can be retried"""
        return self.status == NodeUpgradeStatusEnum.FAILED and self.retry_count < self.max_retries


class UpgradeLog(Base):
    """
    Upgrade event logs for debugging and auditing.

    Records all events during the upgrade process.
    """
    __tablename__ = "upgrade_logs"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    upgrade_id = Column(UUID(as_uuid=True), ForeignKey("chain_upgrades.id", ondelete="CASCADE"), nullable=False)
    node_id = Column(UUID(as_uuid=True), nullable=True)  # NULL for upgrade-level logs

    # Log content
    level = Column(String(10), nullable=False, default="info")  # debug, info, warn, error
    source = Column(String(50), nullable=False, default="upgrade_manager")
    message = Column(Text, nullable=False)
    context = Column(JSON, nullable=True)  # Additional context data

    # Timing
    timestamp = Column(DateTime, nullable=False, default=datetime.utcnow, index=True)

    # Relationships
    upgrade = relationship("ChainUpgrade", back_populates="logs")

    __table_args__ = (
        Index("ix_upgrade_logs_upgrade_time", "upgrade_id", "timestamp"),
    )

    def __repr__(self):
        return f"<UpgradeLog [{self.level}] {self.message[:50]}>"


class BinaryVersion(Base):
    """
    Binary version registry for tracking available versions.

    Used for upgrade detection and version management.
    """
    __tablename__ = "binary_versions"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Version information
    version = Column(String(50), nullable=False, unique=True)
    chain_id = Column(String(50), nullable=False, default="omniphi-1")

    # Binary details
    binary_url = Column(String(500), nullable=False)
    checksum = Column(String(128), nullable=False)  # SHA256
    size_bytes = Column(Integer, nullable=True)

    # Compatibility
    min_upgrade_height = Column(Integer, nullable=True)
    max_upgrade_height = Column(Integer, nullable=True)
    compatible_from = Column(String(50), nullable=True)  # Minimum version to upgrade from

    # Status
    is_latest = Column(Boolean, nullable=False, default=False)
    is_stable = Column(Boolean, nullable=False, default=True)
    is_deprecated = Column(Boolean, nullable=False, default=False)

    # Release information
    release_date = Column(DateTime, nullable=True)
    release_notes_url = Column(String(500), nullable=True)
    changelog = Column(Text, nullable=True)

    # Metadata
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)

    __table_args__ = (
        Index("ix_binary_versions_chain_latest", "chain_id", "is_latest"),
    )

    def __repr__(self):
        return f"<BinaryVersion {self.version} (latest={self.is_latest})>"
