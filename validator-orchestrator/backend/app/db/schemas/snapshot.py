"""
Snapshot Pydantic Schemas

Schemas for chain state snapshots.
"""

from datetime import datetime
from typing import Any, Dict, List, Optional
from uuid import UUID

from pydantic import Field

from app.db.schemas.base import BaseSchema, PaginatedResponse


# =============================================================================
# SNAPSHOT SCHEMAS
# =============================================================================

class SnapshotBase(BaseSchema):
    """Base schema for snapshot."""

    chain_id: str = Field(..., min_length=1, max_length=100, description="Chain ID")
    height: int = Field(..., ge=0, description="Block height")
    snapshot_url: str = Field(..., max_length=1000, description="Download URL")
    checksum: str = Field(..., max_length=128, description="File checksum")


class SnapshotCreate(SnapshotBase):
    """Schema for creating a snapshot."""

    region_id: Optional[UUID] = None
    region_code: Optional[str] = Field(None, max_length=50)
    network_type: str = Field("mainnet")
    block_hash: Optional[str] = Field(None, max_length=100)
    app_hash: Optional[str] = Field(None, max_length=100)
    snapshot_time: datetime
    mirror_urls: List[str] = Field(default_factory=list)
    file_size_bytes: int = Field(..., ge=0)
    file_size_compressed: Optional[int] = Field(None, ge=0)
    compression_type: Optional[str] = Field(None, max_length=20)
    format_type: str = Field("tar")
    checksum_type: str = Field("sha256")
    node_version: Optional[str] = Field(None, max_length=50)
    state_sync_compatible: bool = Field(True)
    is_active: bool = Field(True)
    is_latest: bool = Field(False)
    is_recommended: bool = Field(False)
    description: Optional[str] = None
    tags: List[str] = Field(default_factory=list)
    expires_at: Optional[datetime] = None


class SnapshotUpdate(BaseSchema):
    """Schema for updating a snapshot."""

    snapshot_url: Optional[str] = Field(None, max_length=1000)
    mirror_urls: Optional[List[str]] = None
    is_active: Optional[bool] = None
    is_latest: Optional[bool] = None
    is_recommended: Optional[bool] = None
    verified: Optional[bool] = None
    description: Optional[str] = None
    tags: Optional[List[str]] = None
    expires_at: Optional[datetime] = None


class SnapshotResponse(SnapshotBase):
    """Schema for snapshot response."""

    id: UUID
    region_id: Optional[UUID]
    region_code: Optional[str]
    network_type: str
    block_hash: Optional[str]
    app_hash: Optional[str]
    snapshot_time: datetime
    mirror_urls: List[str]
    file_size_bytes: int
    file_size_compressed: Optional[int]
    compression_type: Optional[str]
    format_type: str
    checksum_type: str
    verified: bool
    verified_at: Optional[datetime]
    node_version: Optional[str]
    state_sync_compatible: bool
    is_active: bool
    is_latest: bool
    is_recommended: bool
    download_count: int
    restore_count: int
    failure_count: int
    avg_download_speed_mbps: Optional[float]
    avg_restore_time_seconds: Optional[float]
    description: Optional[str]
    tags: List[str]
    created_at: datetime
    updated_at: datetime
    expires_at: Optional[datetime]

    # Computed
    file_size_gb: float
    file_size_human: str
    age_hours: float
    is_fresh: bool
    success_rate: float
    is_expired: bool


class SnapshotSummary(BaseSchema):
    """Compact snapshot summary."""

    id: UUID
    chain_id: str
    height: int
    network_type: str
    region_code: Optional[str]
    file_size_human: str
    is_latest: bool
    is_recommended: bool
    snapshot_time: datetime
    success_rate: float


class SnapshotListResponse(PaginatedResponse[SnapshotResponse]):
    """Paginated snapshot list."""
    pass


class SnapshotDownloadRequest(BaseSchema):
    """Request to download a snapshot."""

    snapshot_id: UUID
    preferred_mirror: Optional[str] = None


class SnapshotRestoreResult(BaseSchema):
    """Result of a snapshot restore operation."""

    success: bool
    snapshot_id: UUID
    duration_seconds: float
    height_restored: int
    error_message: Optional[str] = None
