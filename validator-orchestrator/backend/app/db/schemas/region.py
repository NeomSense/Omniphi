"""
Region Pydantic Schemas

Schemas for region, server pool, and region server operations.
"""

from datetime import datetime
from typing import Any, Dict, List, Optional
from uuid import UUID

from pydantic import Field, field_validator

from app.db.schemas.base import BaseSchema, PaginatedResponse


# =============================================================================
# REGION SCHEMAS
# =============================================================================

class RegionBase(BaseSchema):
    """Base schema for region data."""

    code: str = Field(..., min_length=2, max_length=50, description="Unique region code")
    name: str = Field(..., min_length=2, max_length=100, description="Region name")
    display_name: str = Field(..., min_length=2, max_length=100, description="Display name")
    location: Optional[str] = Field(None, max_length=200, description="Physical location")
    description: Optional[str] = Field(None, description="Region description")


class RegionCreate(RegionBase):
    """Schema for creating a region."""

    cloud_zones: Dict[str, str] = Field(default_factory=dict, description="Cloud provider zones")
    max_validators: int = Field(1000, ge=0, description="Maximum validators")
    max_cpu_cores: int = Field(5000, ge=0, description="Maximum CPU cores")
    max_memory_gb: int = Field(10000, ge=0, description="Maximum memory GB")
    max_disk_gb: int = Field(50000, ge=0, description="Maximum disk GB")
    base_monthly_cost: float = Field(50.0, ge=0, description="Base monthly cost")
    is_active: bool = Field(True, description="Whether region is active")
    is_accepting_new: bool = Field(True, description="Accepting new validators")


class RegionUpdate(BaseSchema):
    """Schema for updating a region."""

    name: Optional[str] = Field(None, min_length=2, max_length=100)
    display_name: Optional[str] = Field(None, min_length=2, max_length=100)
    location: Optional[str] = Field(None, max_length=200)
    description: Optional[str] = None
    cloud_zones: Optional[Dict[str, str]] = None
    max_validators: Optional[int] = Field(None, ge=0)
    max_cpu_cores: Optional[int] = Field(None, ge=0)
    max_memory_gb: Optional[int] = Field(None, ge=0)
    max_disk_gb: Optional[int] = Field(None, ge=0)
    base_monthly_cost: Optional[float] = Field(None, ge=0)
    is_active: Optional[bool] = None
    is_accepting_new: Optional[bool] = None
    status: Optional[str] = None


class RegionResponse(RegionBase):
    """Schema for region response."""

    id: UUID
    cloud_zones: Dict[str, str]
    max_validators: int
    max_cpu_cores: int
    max_memory_gb: int
    max_disk_gb: int
    active_validators: int
    used_cpu_cores: int
    used_memory_gb: int
    used_disk_gb: int
    status: str
    is_active: bool
    is_accepting_new: bool
    base_monthly_cost: float
    currency: str
    features: Dict[str, Any]
    created_at: datetime
    updated_at: datetime

    # Computed properties
    capacity_percent: float = Field(..., description="Capacity utilization %")
    cpu_utilization: float = Field(..., description="CPU utilization %")
    memory_utilization: float = Field(..., description="Memory utilization %")
    available_validators: int = Field(..., description="Available validator slots")


class RegionSummary(BaseSchema):
    """Compact region summary for lists."""

    id: UUID
    code: str
    display_name: str
    status: str
    is_active: bool
    active_validators: int
    max_validators: int
    capacity_percent: float


class RegionListResponse(PaginatedResponse[RegionResponse]):
    """Paginated region list response."""
    pass


# =============================================================================
# SERVER POOL SCHEMAS
# =============================================================================

class ServerPoolBase(BaseSchema):
    """Base schema for server pool."""

    name: str = Field(..., min_length=2, max_length=100)
    code: str = Field(..., min_length=2, max_length=50)
    machine_type: str = Field(..., description="Machine type")
    cpu_cores: int = Field(..., ge=1, description="CPU cores per machine")
    memory_gb: int = Field(..., ge=1, description="Memory GB per machine")
    disk_gb: int = Field(..., ge=10, description="Disk GB per machine")


class ServerPoolCreate(ServerPoolBase):
    """Schema for creating a server pool."""

    region_id: UUID = Field(..., description="Parent region ID")
    provider: str = Field("omniphi-cloud", description="Provider")
    bandwidth_gbps: float = Field(1.0, ge=0, description="Network bandwidth")
    hourly_cost: float = Field(0.10, ge=0, description="Hourly cost")
    monthly_cost: float = Field(50.0, ge=0, description="Monthly cost")
    total_machines: int = Field(0, ge=0)
    available_machines: int = Field(0, ge=0)
    total_validators: int = Field(0, ge=0)
    is_active: bool = Field(True)


class ServerPoolUpdate(BaseSchema):
    """Schema for updating a server pool."""

    name: Optional[str] = Field(None, min_length=2, max_length=100)
    hourly_cost: Optional[float] = Field(None, ge=0)
    monthly_cost: Optional[float] = Field(None, ge=0)
    total_machines: Optional[int] = Field(None, ge=0)
    available_machines: Optional[int] = Field(None, ge=0)
    total_validators: Optional[int] = Field(None, ge=0)
    is_active: Optional[bool] = None
    is_available: Optional[bool] = None


class ServerPoolResponse(ServerPoolBase):
    """Schema for server pool response."""

    id: UUID
    region_id: UUID
    provider: str
    bandwidth_gbps: float
    total_machines: int
    available_machines: int
    reserved_machines: int
    total_validators: int
    used_validators: int
    hourly_cost: float
    monthly_cost: float
    setup_fee: float
    currency: str
    is_active: bool
    is_available: bool
    avg_latency_ms: Optional[float]
    uptime_percent: float
    created_at: datetime
    updated_at: datetime

    utilization_percent: float


# =============================================================================
# REGION SERVER SCHEMAS
# =============================================================================

class RegionServerBase(BaseSchema):
    """Base schema for region server."""

    hostname: str = Field(..., min_length=2, max_length=255)
    ip_address: str = Field(..., description="Public IP address")
    cpu_cores: int = Field(..., ge=1)
    memory_gb: int = Field(..., ge=1)
    disk_gb: int = Field(..., ge=10)


class RegionServerCreate(RegionServerBase):
    """Schema for creating a region server."""

    region_id: UUID
    pool_id: Optional[UUID] = None
    internal_ip: Optional[str] = None
    provider: str = Field("omniphi-cloud")
    provider_instance_id: Optional[str] = None
    availability_zone: Optional[str] = None
    machine_type: str = Field("medium")
    disk_type: str = Field("ssd")
    max_validators: int = Field(10, ge=1)
    is_active: bool = Field(True)
    is_available: bool = Field(True)
    labels: Dict[str, str] = Field(default_factory=dict)


class RegionServerUpdate(BaseSchema):
    """Schema for updating a region server."""

    pool_id: Optional[UUID] = None
    is_active: Optional[bool] = None
    is_available: Optional[bool] = None
    status: Optional[str] = None
    max_validators: Optional[int] = Field(None, ge=1)
    labels: Optional[Dict[str, str]] = None


class RegionServerResponse(RegionServerBase):
    """Schema for region server response."""

    id: UUID
    region_id: UUID
    pool_id: Optional[UUID]
    internal_ip: Optional[str]
    provider: str
    provider_instance_id: Optional[str]
    availability_zone: Optional[str]
    machine_type: str
    disk_type: str
    used_cpu_cores: int
    used_memory_gb: int
    used_disk_gb: int
    validators_hosted: int
    max_validators: int
    status: str
    is_active: bool
    is_available: bool
    last_heartbeat: Optional[datetime]
    health_score: float
    labels: Dict[str, str]
    created_at: datetime
    updated_at: datetime

    # Computed
    available_cpu: int
    available_memory: int
    available_disk: int
    cpu_utilization: float
    memory_utilization: float
