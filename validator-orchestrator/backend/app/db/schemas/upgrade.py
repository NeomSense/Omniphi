"""
Upgrade Pydantic Schemas

Schemas for chain upgrades and rollouts.
"""

from datetime import datetime
from typing import Any, Dict, List, Optional
from uuid import UUID

from pydantic import Field

from app.db.schemas.base import BaseSchema, PaginatedResponse


# =============================================================================
# UPGRADE SCHEMAS
# =============================================================================

class UpgradeBase(BaseSchema):
    """Base schema for upgrade."""

    name: str = Field(..., min_length=1, max_length=100, description="Upgrade name")
    version: str = Field(..., min_length=1, max_length=50, description="Target version")
    chain_id: str = Field(..., min_length=1, max_length=100, description="Chain ID")
    upgrade_height: int = Field(..., ge=0, description="Upgrade height")


class UpgradeCreate(UpgradeBase):
    """Schema for creating an upgrade."""

    version_tag: Optional[str] = Field(None, max_length=100)
    estimated_time: Optional[datetime] = None
    description: Optional[str] = None
    release_notes: Optional[str] = None
    release_url: Optional[str] = Field(None, max_length=500)
    changelog_url: Optional[str] = Field(None, max_length=500)
    binary_url: Optional[str] = Field(None, max_length=500)
    binary_checksum: Optional[str] = Field(None, max_length=128)
    docker_image: Optional[str] = Field(None, max_length=255)
    cosmovisor_compatible: bool = Field(True)
    upgrade_info: Optional[Dict[str, Any]] = None
    is_mandatory: bool = Field(True)
    is_breaking: bool = Field(False)
    rollout_strategy: str = Field("sequential")
    rollout_percent_per_batch: int = Field(25, ge=1, le=100)
    min_healthy_percent: int = Field(90, ge=0, le=100)
    auto_rollback_enabled: bool = Field(True)
    tags: List[str] = Field(default_factory=list)


class UpgradeUpdate(BaseSchema):
    """Schema for updating an upgrade."""

    status: Optional[str] = None
    estimated_time: Optional[datetime] = None
    description: Optional[str] = None
    release_notes: Optional[str] = None
    release_url: Optional[str] = Field(None, max_length=500)
    binary_url: Optional[str] = Field(None, max_length=500)
    binary_checksum: Optional[str] = Field(None, max_length=128)
    docker_image: Optional[str] = Field(None, max_length=255)
    is_mandatory: Optional[bool] = None
    rollout_strategy: Optional[str] = None
    rollout_percent_per_batch: Optional[int] = Field(None, ge=1, le=100)
    notification_sent: Optional[bool] = None
    reminder_sent: Optional[bool] = None
    tags: Optional[List[str]] = None


class UpgradeResponse(UpgradeBase):
    """Schema for upgrade response."""

    id: UUID
    version_tag: Optional[str]
    estimated_time: Optional[datetime]
    actual_time: Optional[datetime]
    description: Optional[str]
    release_notes: Optional[str]
    release_url: Optional[str]
    changelog_url: Optional[str]
    binary_url: Optional[str]
    binary_checksum: Optional[str]
    docker_image: Optional[str]
    cosmovisor_compatible: bool
    upgrade_info: Optional[Dict[str, Any]]
    status: str
    is_mandatory: bool
    is_breaking: bool
    rollout_strategy: str
    rollout_percent_per_batch: int
    min_healthy_percent: int
    auto_rollback_enabled: bool
    total_nodes: int
    upgraded_nodes: int
    failed_nodes: int
    pending_nodes: int
    notification_sent: bool
    reminder_sent: bool
    completion_notified: bool
    tags: List[str]
    created_at: datetime
    updated_at: datetime
    started_at: Optional[datetime]
    completed_at: Optional[datetime]
    cancelled_at: Optional[datetime]

    # Computed
    progress_percent: float
    success_rate: float
    is_upcoming: bool
    is_imminent: bool


class UpgradeSummary(BaseSchema):
    """Compact upgrade summary."""

    id: UUID
    name: str
    version: str
    chain_id: str
    upgrade_height: int
    status: str
    is_mandatory: bool
    estimated_time: Optional[datetime]
    progress_percent: float


# =============================================================================
# UPGRADE ROLLOUT SCHEMAS
# =============================================================================

class UpgradeRolloutCreate(BaseSchema):
    """Schema for creating an upgrade rollout."""

    upgrade_id: UUID
    region_id: Optional[UUID] = None
    region_code: str = Field(..., min_length=1, max_length=50)
    rollout_order: int = Field(0, ge=0)
    is_canary: bool = Field(False)
    batch_size: int = Field(10, ge=1)
    scheduled_start: Optional[datetime] = None


class UpgradeRolloutUpdate(BaseSchema):
    """Schema for updating an upgrade rollout."""

    status: Optional[str] = None
    scheduled_start: Optional[datetime] = None
    batch_size: Optional[int] = Field(None, ge=1)
    notes: Optional[str] = None


class UpgradeRolloutResponse(BaseSchema):
    """Schema for upgrade rollout response."""

    id: UUID
    upgrade_id: UUID
    region_id: Optional[UUID]
    region_code: str
    status: str
    rollout_order: int
    is_canary: bool
    total_nodes: int
    upgraded_nodes: int
    failed_nodes: int
    skipped_nodes: int
    in_progress_nodes: int
    current_batch: int
    total_batches: int
    batch_size: int
    pre_upgrade_health_passed: Optional[bool]
    post_upgrade_health_passed: Optional[bool]
    health_check_results: Dict[str, Any]
    scheduled_start: Optional[datetime]
    actual_start: Optional[datetime]
    estimated_completion: Optional[datetime]
    actual_completion: Optional[datetime]
    rolled_back: bool
    rollback_reason: Optional[str]
    rolled_back_at: Optional[datetime]
    rollback_nodes: int
    error_message: Optional[str]
    error_details: Optional[Dict[str, Any]]
    notes: Optional[str]
    created_at: datetime
    updated_at: datetime

    # Computed
    progress_percent: float
    success_rate: float
    pending_nodes: int


class RolloutProgress(BaseSchema):
    """Rollout progress summary."""

    upgrade_id: UUID
    total_regions: int
    completed_regions: int
    in_progress_regions: int
    failed_regions: int
    total_nodes: int
    upgraded_nodes: int
    failed_nodes: int
    overall_progress: float
    overall_success_rate: float
