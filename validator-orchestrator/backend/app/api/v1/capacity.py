"""Autoscaling and Capacity Management API endpoints for Module 7."""

from datetime import datetime, timedelta
from typing import List, Optional
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Query
from pydantic import BaseModel, Field

router = APIRouter()


# ============================================================================
# Schemas
# ============================================================================


class ScalingPolicyBase(BaseModel):
    """Base scaling policy schema."""
    name: str
    description: Optional[str] = None
    policy_type: str = "target_utilization"
    region_code: Optional[str] = None
    provider: Optional[str] = None


class ScalingPolicyCreate(ScalingPolicyBase):
    """Create scaling policy request."""
    min_capacity: int = 1
    max_capacity: int = 100
    desired_capacity: Optional[int] = None
    target_cpu_utilization: Optional[float] = 70.0
    target_memory_utilization: Optional[float] = 75.0
    scale_up_threshold: Optional[float] = 80.0
    scale_down_threshold: Optional[float] = 40.0
    scale_up_increment: int = 2
    scale_down_increment: int = 1
    scale_up_cooldown_seconds: int = 300
    scale_down_cooldown_seconds: int = 600
    evaluation_period_seconds: int = 300
    consecutive_breaches_required: int = 2
    enabled: bool = True


class ScalingPolicyResponse(ScalingPolicyCreate):
    """Scaling policy response."""
    id: UUID
    region_id: Optional[UUID]
    last_scale_up: Optional[datetime]
    last_scale_down: Optional[datetime]
    created_at: datetime
    updated_at: datetime

    class Config:
        from_attributes = True


class ScalingEventResponse(BaseModel):
    """Scaling event response."""
    id: UUID
    policy_id: UUID
    action: str
    status: str
    previous_capacity: int
    target_capacity: int
    actual_capacity: Optional[int]
    trigger_metric: Optional[str]
    trigger_value: Optional[float]
    trigger_threshold: Optional[float]
    reason: Optional[str]
    region_code: Optional[str]
    servers_added: int
    servers_removed: int
    started_at: Optional[datetime]
    completed_at: Optional[datetime]
    duration_seconds: Optional[int]
    error_message: Optional[str]
    created_at: datetime

    class Config:
        from_attributes = True


class CapacityOverview(BaseModel):
    """Current capacity overview."""
    total_servers: int
    available_servers: int
    reserved_servers: int
    total_validators: int
    active_validators: int
    pending_validators: int
    overall_utilization: float
    regions: List[dict]


class RegionCapacity(BaseModel):
    """Regional capacity details."""
    region_code: str
    region_name: str
    total_servers: int
    available_servers: int
    reserved_servers: int
    total_validators: int
    active_validators: int
    utilization_percent: float
    avg_cpu_percent: float
    avg_memory_percent: float
    status: str


class CapacityForecastResponse(BaseModel):
    """Capacity forecast response."""
    id: UUID
    region_code: Optional[str]
    forecast_date: datetime
    forecast_horizon_hours: int
    current_capacity: int
    current_usage: int
    current_utilization: float
    predicted_usage: int
    predicted_utilization: float
    confidence_score: Optional[float]
    recommended_capacity: int
    recommended_action: str
    capacity_delta: int
    created_at: datetime

    class Config:
        from_attributes = True


class CapacityReservationCreate(BaseModel):
    """Create capacity reservation request."""
    region_code: str
    tier: str
    quantity: int
    starts_at: datetime
    expires_at: datetime


class CapacityReservationResponse(BaseModel):
    """Capacity reservation response."""
    id: UUID
    user_id: UUID
    subscription_id: Optional[UUID]
    region_code: str
    tier: str
    quantity: int
    starts_at: datetime
    expires_at: datetime
    status: str
    fulfilled: bool
    fulfilled_at: Optional[datetime]
    created_at: datetime
    updated_at: datetime

    class Config:
        from_attributes = True


class CleanupJobResponse(BaseModel):
    """Cleanup job response."""
    id: UUID
    job_type: str
    status: str
    region_code: Optional[str]
    resources_found: int
    resources_cleaned: int
    resources_failed: int
    estimated_savings_usd: Optional[float]
    actual_savings_usd: Optional[float]
    started_at: Optional[datetime]
    completed_at: Optional[datetime]
    dry_run: bool
    error_message: Optional[str]
    created_at: datetime

    class Config:
        from_attributes = True


class CleanupJobCreate(BaseModel):
    """Create cleanup job request."""
    job_type: str  # idle_servers, orphaned_vms, old_snapshots
    region_code: Optional[str] = None
    dry_run: bool = True


# ============================================================================
# Capacity Overview Endpoints
# ============================================================================


@router.get("/capacity", response_model=CapacityOverview)
async def get_capacity_overview():
    """
    Get current capacity overview across all regions.

    Returns:
    - Total and available servers
    - Validator counts
    - Overall utilization
    - Per-region breakdown
    """
    # Mock data
    return {
        "total_servers": 150,
        "available_servers": 45,
        "reserved_servers": 10,
        "total_validators": 520,
        "active_validators": 485,
        "pending_validators": 35,
        "overall_utilization": 70.0,
        "regions": [
            {
                "region_code": "us-east",
                "region_name": "US East",
                "total_servers": 50,
                "available_servers": 15,
                "reserved_servers": 5,
                "total_validators": 175,
                "active_validators": 160,
                "utilization_percent": 70.0,
                "status": "healthy"
            },
            {
                "region_code": "us-west",
                "region_name": "US West",
                "total_servers": 40,
                "available_servers": 12,
                "reserved_servers": 2,
                "total_validators": 130,
                "active_validators": 125,
                "utilization_percent": 70.0,
                "status": "healthy"
            },
            {
                "region_code": "eu-central",
                "region_name": "EU Central",
                "total_servers": 35,
                "available_servers": 10,
                "reserved_servers": 2,
                "total_validators": 115,
                "active_validators": 110,
                "utilization_percent": 71.4,
                "status": "healthy"
            },
            {
                "region_code": "asia-pacific",
                "region_name": "Asia Pacific",
                "total_servers": 25,
                "available_servers": 8,
                "reserved_servers": 1,
                "total_validators": 100,
                "active_validators": 90,
                "utilization_percent": 68.0,
                "status": "healthy"
            }
        ]
    }


@router.get("/capacity/regions/{region_code}", response_model=RegionCapacity)
async def get_region_capacity(region_code: str):
    """Get detailed capacity info for a specific region."""
    valid_regions = ["us-east", "us-west", "eu-central", "asia-pacific"]
    if region_code not in valid_regions:
        raise HTTPException(status_code=404, detail=f"Region not found: {region_code}")

    return {
        "region_code": region_code,
        "region_name": region_code.replace("-", " ").title(),
        "total_servers": 50,
        "available_servers": 15,
        "reserved_servers": 5,
        "total_validators": 175,
        "active_validators": 160,
        "utilization_percent": 70.0,
        "avg_cpu_percent": 55.5,
        "avg_memory_percent": 62.3,
        "status": "healthy"
    }


# ============================================================================
# Capacity Forecast Endpoints
# ============================================================================


@router.get("/capacity/forecast", response_model=List[CapacityForecastResponse])
async def get_capacity_forecast(
    region_code: Optional[str] = None,
    horizon_hours: int = Query(default=24, ge=1, le=168)
):
    """
    Get capacity demand forecast.

    Uses ML-based prediction to forecast:
    - Expected validator demand
    - Resource utilization trends
    - Recommended capacity adjustments
    """
    forecasts = [
        {
            "id": "550e8400-e29b-41d4-a716-446655440700",
            "region_code": "us-east",
            "forecast_date": (datetime.utcnow() + timedelta(hours=6)).isoformat(),
            "forecast_horizon_hours": horizon_hours,
            "current_capacity": 50,
            "current_usage": 35,
            "current_utilization": 70.0,
            "predicted_usage": 40,
            "predicted_utilization": 80.0,
            "confidence_score": 0.85,
            "recommended_capacity": 55,
            "recommended_action": "scale_up",
            "capacity_delta": 5,
            "created_at": datetime.utcnow().isoformat()
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440701",
            "region_code": "us-west",
            "forecast_date": (datetime.utcnow() + timedelta(hours=6)).isoformat(),
            "forecast_horizon_hours": horizon_hours,
            "current_capacity": 40,
            "current_usage": 28,
            "current_utilization": 70.0,
            "predicted_usage": 30,
            "predicted_utilization": 75.0,
            "confidence_score": 0.82,
            "recommended_capacity": 40,
            "recommended_action": "no_action",
            "capacity_delta": 0,
            "created_at": datetime.utcnow().isoformat()
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440702",
            "region_code": "eu-central",
            "forecast_date": (datetime.utcnow() + timedelta(hours=6)).isoformat(),
            "forecast_horizon_hours": horizon_hours,
            "current_capacity": 35,
            "current_usage": 25,
            "current_utilization": 71.4,
            "predicted_usage": 22,
            "predicted_utilization": 62.9,
            "confidence_score": 0.78,
            "recommended_capacity": 32,
            "recommended_action": "scale_down",
            "capacity_delta": -3,
            "created_at": datetime.utcnow().isoformat()
        }
    ]

    if region_code:
        forecasts = [f for f in forecasts if f["region_code"] == region_code]

    return forecasts


# ============================================================================
# Scaling Policy Endpoints
# ============================================================================


@router.get("/capacity/policies", response_model=List[ScalingPolicyResponse])
async def list_scaling_policies(
    region_code: Optional[str] = None,
    enabled: Optional[bool] = None
):
    """List all scaling policies."""
    mock_policies = [
        {
            "id": "550e8400-e29b-41d4-a716-446655440800",
            "name": "Default Global Policy",
            "description": "Default autoscaling policy for all regions",
            "policy_type": "target_utilization",
            "region_id": None,
            "region_code": None,
            "provider": None,
            "min_capacity": 10,
            "max_capacity": 200,
            "desired_capacity": None,
            "target_cpu_utilization": 70.0,
            "target_memory_utilization": 75.0,
            "scale_up_threshold": 80.0,
            "scale_down_threshold": 40.0,
            "scale_up_increment": 5,
            "scale_down_increment": 2,
            "scale_up_cooldown_seconds": 300,
            "scale_down_cooldown_seconds": 600,
            "evaluation_period_seconds": 300,
            "consecutive_breaches_required": 2,
            "enabled": True,
            "last_scale_up": "2024-11-22T14:00:00Z",
            "last_scale_down": "2024-11-20T02:00:00Z",
            "created_at": "2024-11-01T00:00:00Z",
            "updated_at": "2024-11-22T14:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440801",
            "name": "US East High Availability",
            "description": "Higher capacity for US East region",
            "policy_type": "target_utilization",
            "region_id": "550e8400-e29b-41d4-a716-446655440100",
            "region_code": "us-east",
            "provider": None,
            "min_capacity": 20,
            "max_capacity": 100,
            "desired_capacity": 50,
            "target_cpu_utilization": 60.0,
            "target_memory_utilization": 65.0,
            "scale_up_threshold": 70.0,
            "scale_down_threshold": 35.0,
            "scale_up_increment": 10,
            "scale_down_increment": 5,
            "scale_up_cooldown_seconds": 180,
            "scale_down_cooldown_seconds": 900,
            "evaluation_period_seconds": 180,
            "consecutive_breaches_required": 2,
            "enabled": True,
            "last_scale_up": "2024-11-23T06:00:00Z",
            "last_scale_down": None,
            "created_at": "2024-11-01T00:00:00Z",
            "updated_at": "2024-11-23T06:00:00Z"
        }
    ]

    if region_code:
        mock_policies = [p for p in mock_policies if p["region_code"] == region_code or p["region_code"] is None]
    if enabled is not None:
        mock_policies = [p for p in mock_policies if p["enabled"] == enabled]

    return mock_policies


@router.post("/capacity/policies", response_model=ScalingPolicyResponse)
async def create_scaling_policy(policy: ScalingPolicyCreate):
    """Create a new scaling policy."""
    return {
        "id": "550e8400-e29b-41d4-a716-446655440899",
        "region_id": None,
        "last_scale_up": None,
        "last_scale_down": None,
        **policy.model_dump(),
        "created_at": datetime.utcnow().isoformat(),
        "updated_at": datetime.utcnow().isoformat()
    }


@router.put("/capacity/policies/{policy_id}", response_model=ScalingPolicyResponse)
async def update_scaling_policy(policy_id: UUID, policy: ScalingPolicyCreate):
    """Update an existing scaling policy."""
    return {
        "id": str(policy_id),
        "region_id": None,
        "last_scale_up": None,
        "last_scale_down": None,
        **policy.model_dump(),
        "created_at": "2024-11-01T00:00:00Z",
        "updated_at": datetime.utcnow().isoformat()
    }


@router.delete("/capacity/policies/{policy_id}")
async def delete_scaling_policy(policy_id: UUID):
    """Delete a scaling policy."""
    return {"status": "deleted", "policy_id": str(policy_id)}


# ============================================================================
# Scaling Event Endpoints
# ============================================================================


@router.get("/capacity/events", response_model=List[ScalingEventResponse])
async def list_scaling_events(
    policy_id: Optional[UUID] = None,
    region_code: Optional[str] = None,
    action: Optional[str] = None,
    limit: int = Query(default=50, ge=1, le=200),
    offset: int = Query(default=0, ge=0)
):
    """List scaling events with optional filtering."""
    mock_events = [
        {
            "id": "550e8400-e29b-41d4-a716-446655440900",
            "policy_id": "550e8400-e29b-41d4-a716-446655440801",
            "action": "scale_up",
            "status": "completed",
            "previous_capacity": 45,
            "target_capacity": 55,
            "actual_capacity": 55,
            "trigger_metric": "cpu",
            "trigger_value": 82.5,
            "trigger_threshold": 70.0,
            "reason": "CPU utilization exceeded threshold for 2 consecutive periods",
            "region_code": "us-east",
            "servers_added": 10,
            "servers_removed": 0,
            "started_at": "2024-11-23T06:00:00Z",
            "completed_at": "2024-11-23T06:05:00Z",
            "duration_seconds": 300,
            "error_message": None,
            "created_at": "2024-11-23T06:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440901",
            "policy_id": "550e8400-e29b-41d4-a716-446655440800",
            "action": "scale_down",
            "status": "completed",
            "previous_capacity": 32,
            "target_capacity": 30,
            "actual_capacity": 30,
            "trigger_metric": "cpu",
            "trigger_value": 35.2,
            "trigger_threshold": 40.0,
            "reason": "CPU utilization below threshold for 2 consecutive periods",
            "region_code": "eu-central",
            "servers_added": 0,
            "servers_removed": 2,
            "started_at": "2024-11-22T02:00:00Z",
            "completed_at": "2024-11-22T02:10:00Z",
            "duration_seconds": 600,
            "error_message": None,
            "created_at": "2024-11-22T02:00:00Z"
        }
    ]

    return mock_events


# ============================================================================
# Capacity Reservation Endpoints
# ============================================================================


@router.get("/capacity/reservations", response_model=List[CapacityReservationResponse])
async def list_capacity_reservations(
    user_id: Optional[UUID] = None,
    region_code: Optional[str] = None,
    status: Optional[str] = None
):
    """List capacity reservations."""
    mock_reservations = [
        {
            "id": "550e8400-e29b-41d4-a716-446655441000",
            "user_id": "550e8400-e29b-41d4-a716-446655440010",
            "subscription_id": "550e8400-e29b-41d4-a716-446655440020",
            "region_code": "us-east",
            "tier": "professional",
            "quantity": 5,
            "starts_at": "2024-11-01T00:00:00Z",
            "expires_at": "2024-12-01T00:00:00Z",
            "status": "active",
            "fulfilled": True,
            "fulfilled_at": "2024-11-01T00:05:00Z",
            "created_at": "2024-10-30T00:00:00Z",
            "updated_at": "2024-11-01T00:05:00Z"
        }
    ]

    return mock_reservations


@router.post("/capacity/reservations", response_model=CapacityReservationResponse)
async def create_capacity_reservation(reservation: CapacityReservationCreate):
    """Create a capacity reservation."""
    return {
        "id": "550e8400-e29b-41d4-a716-446655441099",
        "user_id": "550e8400-e29b-41d4-a716-446655440010",
        "subscription_id": None,
        "region_code": reservation.region_code,
        "tier": reservation.tier,
        "quantity": reservation.quantity,
        "starts_at": reservation.starts_at.isoformat(),
        "expires_at": reservation.expires_at.isoformat(),
        "status": "pending",
        "fulfilled": False,
        "fulfilled_at": None,
        "created_at": datetime.utcnow().isoformat(),
        "updated_at": datetime.utcnow().isoformat()
    }


@router.delete("/capacity/reservations/{reservation_id}")
async def cancel_capacity_reservation(reservation_id: UUID):
    """Cancel a capacity reservation."""
    return {"status": "cancelled", "reservation_id": str(reservation_id)}


# ============================================================================
# Cleanup Job Endpoints
# ============================================================================


@router.get("/capacity/cleanup-jobs", response_model=List[CleanupJobResponse])
async def list_cleanup_jobs(
    job_type: Optional[str] = None,
    status: Optional[str] = None,
    limit: int = Query(default=20, ge=1, le=100)
):
    """List cleanup jobs."""
    mock_jobs = [
        {
            "id": "550e8400-e29b-41d4-a716-446655441100",
            "job_type": "idle_servers",
            "status": "completed",
            "region_code": None,
            "resources_found": 5,
            "resources_cleaned": 5,
            "resources_failed": 0,
            "estimated_savings_usd": 150.0,
            "actual_savings_usd": 150.0,
            "started_at": "2024-11-22T00:00:00Z",
            "completed_at": "2024-11-22T00:15:00Z",
            "dry_run": False,
            "error_message": None,
            "created_at": "2024-11-22T00:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655441101",
            "job_type": "old_snapshots",
            "status": "completed",
            "region_code": "us-east",
            "resources_found": 12,
            "resources_cleaned": 12,
            "resources_failed": 0,
            "estimated_savings_usd": 50.0,
            "actual_savings_usd": 48.5,
            "started_at": "2024-11-21T00:00:00Z",
            "completed_at": "2024-11-21T00:05:00Z",
            "dry_run": False,
            "error_message": None,
            "created_at": "2024-11-21T00:00:00Z"
        }
    ]

    return mock_jobs


@router.post("/capacity/cleanup-jobs", response_model=CleanupJobResponse)
async def create_cleanup_job(job: CleanupJobCreate):
    """
    Create a cleanup job for unused resources.

    Supported job types:
    - **idle_servers**: Remove servers that have been idle for extended period
    - **orphaned_vms**: Clean up VMs without associated validators
    - **old_snapshots**: Delete snapshots older than retention policy
    """
    valid_types = ["idle_servers", "orphaned_vms", "old_snapshots"]
    if job.job_type not in valid_types:
        raise HTTPException(status_code=400, detail=f"Invalid job type. Must be one of: {valid_types}")

    return {
        "id": "550e8400-e29b-41d4-a716-446655441199",
        "job_type": job.job_type,
        "status": "pending",
        "region_code": job.region_code,
        "resources_found": 0,
        "resources_cleaned": 0,
        "resources_failed": 0,
        "estimated_savings_usd": None,
        "actual_savings_usd": None,
        "started_at": None,
        "completed_at": None,
        "dry_run": job.dry_run,
        "error_message": None,
        "created_at": datetime.utcnow().isoformat()
    }
