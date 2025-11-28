"""Autoscaling and Capacity Management models for Module 7."""

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


class ScalingAction(str, PyEnum):
    """Scaling action types."""
    SCALE_UP = "scale_up"
    SCALE_DOWN = "scale_down"
    NO_ACTION = "no_action"


class ScalingStatus(str, PyEnum):
    """Scaling event status."""
    PENDING = "pending"
    IN_PROGRESS = "in_progress"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


class PolicyType(str, PyEnum):
    """Scaling policy type."""
    TARGET_UTILIZATION = "target_utilization"
    SCHEDULE_BASED = "schedule_based"
    PREDICTIVE = "predictive"
    STEP = "step"


class ScalingPolicy(Base):
    """Scaling policy configuration."""

    __tablename__ = "scaling_policies"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Policy identification
    name = Column(String(200), nullable=False)
    description = Column(Text, nullable=True)
    policy_type = Column(Enum(PolicyType), nullable=False, default=PolicyType.TARGET_UTILIZATION)

    # Scope
    region_id = Column(UUID(as_uuid=True), nullable=True)  # null = global
    region_code = Column(String(50), nullable=True)
    provider = Column(String(50), nullable=True)  # null = all providers

    # Capacity limits
    min_capacity = Column(Integer, nullable=False, default=1)
    max_capacity = Column(Integer, nullable=False, default=100)
    desired_capacity = Column(Integer, nullable=True)

    # Target utilization settings
    target_cpu_utilization = Column(Float, nullable=True, default=70.0)
    target_memory_utilization = Column(Float, nullable=True, default=75.0)
    target_validator_per_server = Column(Integer, nullable=True, default=5)

    # Scaling thresholds
    scale_up_threshold = Column(Float, nullable=True, default=80.0)
    scale_down_threshold = Column(Float, nullable=True, default=40.0)
    scale_up_increment = Column(Integer, nullable=False, default=2)
    scale_down_increment = Column(Integer, nullable=False, default=1)

    # Cooldown settings
    scale_up_cooldown_seconds = Column(Integer, nullable=False, default=300)
    scale_down_cooldown_seconds = Column(Integer, nullable=False, default=600)
    last_scale_up = Column(DateTime, nullable=True)
    last_scale_down = Column(DateTime, nullable=True)

    # Evaluation settings
    evaluation_period_seconds = Column(Integer, nullable=False, default=300)
    consecutive_breaches_required = Column(Integer, nullable=False, default=2)

    # Status
    enabled = Column(Boolean, nullable=False, default=True)

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    events = relationship("ScalingEvent", back_populates="policy", cascade="all, delete-orphan")


class ScalingEvent(Base):
    """Scaling event history."""

    __tablename__ = "scaling_events"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    policy_id = Column(UUID(as_uuid=True), ForeignKey("scaling_policies.id", ondelete="CASCADE"), nullable=False)

    # Event details
    action = Column(Enum(ScalingAction), nullable=False)
    status = Column(Enum(ScalingStatus), nullable=False, default=ScalingStatus.PENDING)

    # Capacity changes
    previous_capacity = Column(Integer, nullable=False)
    target_capacity = Column(Integer, nullable=False)
    actual_capacity = Column(Integer, nullable=True)

    # Trigger info
    trigger_metric = Column(String(50), nullable=True)  # cpu, memory, validators
    trigger_value = Column(Float, nullable=True)
    trigger_threshold = Column(Float, nullable=True)
    reason = Column(Text, nullable=True)

    # Region info
    region_code = Column(String(50), nullable=True)

    # Resources affected
    servers_added = Column(Integer, nullable=False, default=0)
    servers_removed = Column(Integer, nullable=False, default=0)
    server_ids = Column(JSON, nullable=True)

    # Timing
    started_at = Column(DateTime, nullable=True)
    completed_at = Column(DateTime, nullable=True)
    duration_seconds = Column(Integer, nullable=True)

    # Error handling
    error_message = Column(Text, nullable=True)

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)

    # Relationships
    policy = relationship("ScalingPolicy", back_populates="events")


class CapacityForecast(Base):
    """Capacity demand forecasting."""

    __tablename__ = "capacity_forecasts"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Scope
    region_id = Column(UUID(as_uuid=True), nullable=True)
    region_code = Column(String(50), nullable=True)

    # Forecast period
    forecast_date = Column(DateTime, nullable=False)
    forecast_horizon_hours = Column(Integer, nullable=False, default=24)

    # Current state
    current_capacity = Column(Integer, nullable=False)
    current_usage = Column(Integer, nullable=False)
    current_utilization = Column(Float, nullable=False)

    # Predictions
    predicted_usage = Column(Integer, nullable=False)
    predicted_utilization = Column(Float, nullable=False)
    confidence_score = Column(Float, nullable=True)

    # Recommendations
    recommended_capacity = Column(Integer, nullable=False)
    recommended_action = Column(Enum(ScalingAction), nullable=False, default=ScalingAction.NO_ACTION)
    capacity_delta = Column(Integer, nullable=False, default=0)

    # Model info
    model_version = Column(String(50), nullable=True)
    features_used = Column(JSON, nullable=True)

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)


class CapacityReservation(Base):
    """Capacity reservations for guaranteed resources."""

    __tablename__ = "capacity_reservations"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Owner
    user_id = Column(UUID(as_uuid=True), nullable=False, index=True)
    subscription_id = Column(UUID(as_uuid=True), nullable=True)

    # Reservation details
    region_code = Column(String(50), nullable=False)
    tier = Column(String(50), nullable=False)
    quantity = Column(Integer, nullable=False)

    # Timing
    starts_at = Column(DateTime, nullable=False)
    expires_at = Column(DateTime, nullable=False)

    # Status
    status = Column(String(50), nullable=False, default="active")  # active, expired, cancelled

    # Fulfillment
    fulfilled = Column(Boolean, nullable=False, default=False)
    fulfilled_at = Column(DateTime, nullable=True)
    server_ids = Column(JSON, nullable=True)

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)


class ResourceUsageMetric(Base):
    """Resource usage metrics for capacity planning."""

    __tablename__ = "resource_usage_metrics"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Scope
    region_code = Column(String(50), nullable=False, index=True)
    server_pool_id = Column(UUID(as_uuid=True), nullable=True)

    # Timestamp
    timestamp = Column(DateTime, nullable=False, index=True)
    granularity = Column(String(20), nullable=False, default="5m")  # 1m, 5m, 15m, 1h

    # Capacity metrics
    total_servers = Column(Integer, nullable=False)
    available_servers = Column(Integer, nullable=False)
    reserved_servers = Column(Integer, nullable=False)

    # Validator metrics
    total_validators = Column(Integer, nullable=False)
    active_validators = Column(Integer, nullable=False)
    pending_validators = Column(Integer, nullable=False)

    # Resource utilization
    avg_cpu_percent = Column(Float, nullable=False)
    avg_memory_percent = Column(Float, nullable=False)
    avg_disk_percent = Column(Float, nullable=False)
    avg_network_mbps = Column(Float, nullable=True)

    # Peak values
    peak_cpu_percent = Column(Float, nullable=True)
    peak_memory_percent = Column(Float, nullable=True)

    # Cost metrics
    estimated_cost_usd = Column(Float, nullable=True)

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)


class CleanupJob(Base):
    """Cleanup jobs for unused resources."""

    __tablename__ = "cleanup_jobs"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)

    # Job details
    job_type = Column(String(50), nullable=False)  # idle_servers, orphaned_vms, old_snapshots
    status = Column(String(50), nullable=False, default="pending")  # pending, running, completed, failed

    # Scope
    region_code = Column(String(50), nullable=True)

    # Resources identified
    resources_found = Column(Integer, nullable=False, default=0)
    resources_cleaned = Column(Integer, nullable=False, default=0)
    resources_failed = Column(Integer, nullable=False, default=0)
    resource_ids = Column(JSON, nullable=True)

    # Cost savings
    estimated_savings_usd = Column(Float, nullable=True)
    actual_savings_usd = Column(Float, nullable=True)

    # Timing
    started_at = Column(DateTime, nullable=True)
    completed_at = Column(DateTime, nullable=True)

    # Error handling
    error_message = Column(Text, nullable=True)

    # Dry run
    dry_run = Column(Boolean, nullable=False, default=False)

    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)
