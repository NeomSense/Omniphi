"""Migration and Failover models for Module 8."""

from datetime import datetime
from uuid import uuid4
from enum import Enum as PyEnum

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    Text,
    ForeignKey,
    Enum,
    JSON,
)
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship

from app.core.database import Base


class MigrationStatus(str, PyEnum):
    """Migration job status."""
    PENDING = "pending"
    PREPARING = "preparing"
    STOPPING_SOURCE = "stopping_source"
    TRANSFERRING = "transferring"
    STARTING_TARGET = "starting_target"
    VERIFYING = "verifying"
    COMPLETED = "completed"
    FAILED = "failed"
    ROLLED_BACK = "rolled_back"


class MigrationType(str, PyEnum):
    """Type of migration."""
    MANUAL = "manual"
    AUTO_FAILOVER = "auto_failover"
    REGION_OUTAGE = "region_outage"
    LOAD_BALANCING = "load_balancing"
    UPGRADE = "upgrade"


class FailoverTriggerType(str, PyEnum):
    """Types of failover triggers."""
    NODE_DOWN = "node_down"
    HIGH_CPU = "high_cpu"
    HIGH_MEMORY = "high_memory"
    SYNC_ISSUES = "sync_issues"
    MISSED_BLOCKS = "missed_blocks"
    REGION_OUTAGE = "region_outage"
    MANUAL = "manual"


class FailoverAction(str, PyEnum):
    """Failover actions."""
    RESTART = "restart"
    MIGRATE = "migrate"
    REPROVISION = "reprovision"
    ALERT_ONLY = "alert_only"


class MigrationJob(Base):
    """Migration job tracking."""

    __tablename__ = "migration_jobs"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Node being migrated
    node_id = Column(UUID(as_uuid=True), nullable=False, index=True)
    validator_id = Column(UUID(as_uuid=True), nullable=True)

    # Source and target
    source_region = Column(String(50), nullable=False)
    target_region = Column(String(50), nullable=False)
    source_server_id = Column(UUID(as_uuid=True), nullable=True)
    target_server_id = Column(UUID(as_uuid=True), nullable=True)

    # Migration details
    migration_type = Column(Enum(MigrationType), nullable=False, default=MigrationType.MANUAL)
    status = Column(Enum(MigrationStatus), nullable=False, default=MigrationStatus.PENDING)
    priority = Column(Integer, nullable=False, default=5)  # 1-10, higher = more urgent

    # State transfer
    snapshot_id = Column(UUID(as_uuid=True), nullable=True)
    state_data_size_mb = Column(Float, nullable=True)
    transfer_progress_percent = Column(Float, nullable=False, default=0.0)

    # Double-sign prevention
    source_stopped_at = Column(DateTime, nullable=True)
    double_sign_check_passed = Column(Boolean, nullable=False, default=False)
    signing_key_transferred = Column(Boolean, nullable=False, default=False)

    # Block tracking
    last_signed_block = Column(Integer, nullable=True)
    target_start_block = Column(Integer, nullable=True)

    # Timing
    started_at = Column(DateTime, nullable=True)
    completed_at = Column(DateTime, nullable=True)
    estimated_duration_seconds = Column(Integer, nullable=True)
    actual_duration_seconds = Column(Integer, nullable=True)

    # Error handling
    error_message = Column(Text, nullable=True)
    retry_count = Column(Integer, nullable=False, default=0)
    max_retries = Column(Integer, nullable=False, default=3)

    # Rollback info
    rollback_available = Column(Boolean, nullable=False, default=True)
    rollback_snapshot_id = Column(UUID(as_uuid=True), nullable=True)

    # Metadata
    initiated_by = Column(UUID(as_uuid=True), nullable=True)  # User ID if manual
    metadata = Column(JSON, nullable=True)

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    logs = relationship("MigrationLog", back_populates="migration", cascade="all, delete-orphan")


class MigrationLog(Base):
    """Migration job logs."""

    __tablename__ = "migration_logs"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    migration_id = Column(UUID(as_uuid=True), ForeignKey("migration_jobs.id", ondelete="CASCADE"), nullable=False)

    level = Column(String(20), nullable=False, default="info")  # debug, info, warning, error
    step = Column(String(100), nullable=True)  # Current migration step
    message = Column(Text, nullable=False)
    details = Column(JSON, nullable=True)

    timestamp = Column(DateTime, nullable=False, default=datetime.utcnow)

    # Relationships
    migration = relationship("MigrationJob", back_populates="logs")


class FailoverRule(Base):
    """Failover rule definitions."""

    __tablename__ = "failover_rules"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Rule identification
    name = Column(String(200), nullable=False)
    description = Column(Text, nullable=True)

    # Trigger conditions
    trigger_type = Column(Enum(FailoverTriggerType), nullable=False)
    action = Column(Enum(FailoverAction), nullable=False)
    priority = Column(Integer, nullable=False, default=5)  # Execution order

    # Thresholds
    cpu_threshold = Column(Float, nullable=True)  # Percent
    memory_threshold = Column(Float, nullable=True)  # Percent
    missed_blocks_threshold = Column(Integer, nullable=True)
    downtime_threshold_seconds = Column(Integer, nullable=True)
    sync_lag_threshold = Column(Integer, nullable=True)  # Blocks behind

    # Scope
    applies_to_region = Column(String(50), nullable=True)  # null = all regions
    applies_to_tier = Column(String(50), nullable=True)  # null = all tiers

    # Action configuration
    target_region = Column(String(50), nullable=True)  # For migration action
    cooldown_seconds = Column(Integer, nullable=False, default=300)
    max_actions_per_hour = Column(Integer, nullable=False, default=3)

    # Notifications
    notify_on_trigger = Column(Boolean, nullable=False, default=True)
    notification_channels = Column(JSON, nullable=True)  # email, slack, pagerduty

    # Status
    enabled = Column(Boolean, nullable=False, default=True)

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)


class FailoverEvent(Base):
    """Failover event history."""

    __tablename__ = "failover_events"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Event details
    rule_id = Column(UUID(as_uuid=True), ForeignKey("failover_rules.id", ondelete="SET NULL"), nullable=True)
    node_id = Column(UUID(as_uuid=True), nullable=False, index=True)

    trigger_type = Column(Enum(FailoverTriggerType), nullable=False)
    action_taken = Column(Enum(FailoverAction), nullable=False)

    # Trigger context
    trigger_value = Column(Float, nullable=True)  # CPU %, memory %, etc.
    trigger_threshold = Column(Float, nullable=True)
    trigger_reason = Column(Text, nullable=True)

    # Result
    success = Column(Boolean, nullable=False, default=False)
    migration_job_id = Column(UUID(as_uuid=True), nullable=True)  # If migration was triggered
    error_message = Column(Text, nullable=True)

    # Timing
    detected_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    action_started_at = Column(DateTime, nullable=True)
    action_completed_at = Column(DateTime, nullable=True)
    recovery_time_seconds = Column(Integer, nullable=True)

    # Metadata
    source_region = Column(String(50), nullable=True)
    target_region = Column(String(50), nullable=True)
    metadata = Column(JSON, nullable=True)

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)


class RegionOutage(Base):
    """Region outage tracking."""

    __tablename__ = "region_outages"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    region = Column(String(50), nullable=False, index=True)

    # Outage detection
    detected_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    confirmed_at = Column(DateTime, nullable=True)
    resolved_at = Column(DateTime, nullable=True)

    # Impact
    affected_nodes_count = Column(Integer, nullable=False, default=0)
    nodes_migrated_count = Column(Integer, nullable=False, default=0)
    nodes_failed_migration = Column(Integer, nullable=False, default=0)

    # Details
    cause = Column(String(200), nullable=True)
    description = Column(Text, nullable=True)

    # Auto-response
    auto_failover_triggered = Column(Boolean, nullable=False, default=False)
    migration_jobs = Column(JSON, nullable=True)  # List of migration job IDs

    # Metrics
    detection_latency_seconds = Column(Integer, nullable=True)
    total_downtime_seconds = Column(Integer, nullable=True)

    # Status
    status = Column(String(50), nullable=False, default="active")  # active, recovering, resolved

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)


class DoubleSignGuard(Base):
    """Double-sign prevention tracking."""

    __tablename__ = "double_sign_guards"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Validator identification
    validator_id = Column(UUID(as_uuid=True), nullable=False, unique=True, index=True)
    validator_address = Column(String(100), nullable=False)

    # Current signing status
    is_signing_active = Column(Boolean, nullable=False, default=False)
    active_node_id = Column(UUID(as_uuid=True), nullable=True)
    active_region = Column(String(50), nullable=True)

    # Last signing info
    last_signed_height = Column(Integer, nullable=True)
    last_signed_time = Column(DateTime, nullable=True)
    last_signed_hash = Column(String(100), nullable=True)

    # Lock status for migrations
    migration_lock = Column(Boolean, nullable=False, default=False)
    migration_lock_id = Column(UUID(as_uuid=True), nullable=True)
    migration_lock_expires = Column(DateTime, nullable=True)

    # Safety checks
    signing_gap_required = Column(Integer, nullable=False, default=2)  # Blocks
    last_verification = Column(DateTime, nullable=True)
    verification_passed = Column(Boolean, nullable=True)

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)
