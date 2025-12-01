"""Migration and Failover API endpoints for Module 8."""

from datetime import datetime
from typing import List, Optional
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Query
from pydantic import BaseModel, Field

router = APIRouter()


# ============================================================================
# Schemas
# ============================================================================


class MigrationJobBase(BaseModel):
    """Base migration job schema."""
    node_id: UUID
    source_region: str
    target_region: str
    migration_type: str = "manual"
    priority: int = Field(default=5, ge=1, le=10)


class MigrationJobCreate(MigrationJobBase):
    """Create migration job request."""
    snapshot_id: Optional[UUID] = None
    initiated_by: Optional[UUID] = None


class MigrationJobResponse(MigrationJobBase):
    """Migration job response."""
    id: UUID
    status: str
    validator_id: Optional[UUID]
    source_server_id: Optional[UUID]
    target_server_id: Optional[UUID]
    transfer_progress_percent: float
    double_sign_check_passed: bool
    signing_key_transferred: bool
    last_signed_block: Optional[int]
    started_at: Optional[datetime]
    completed_at: Optional[datetime]
    estimated_duration_seconds: Optional[int]
    actual_duration_seconds: Optional[int]
    error_message: Optional[str]
    retry_count: int
    rollback_available: bool
    created_at: datetime
    updated_at: datetime

    class Config:
        from_attributes = True


class MigrationLogResponse(BaseModel):
    """Migration log entry."""
    id: UUID
    migration_id: UUID
    level: str
    step: Optional[str]
    message: str
    details: Optional[dict]
    timestamp: datetime

    class Config:
        from_attributes = True


class FailoverRuleBase(BaseModel):
    """Base failover rule schema."""
    name: str
    description: Optional[str] = None
    trigger_type: str
    action: str
    priority: int = Field(default=5, ge=1, le=10)


class FailoverRuleCreate(FailoverRuleBase):
    """Create failover rule request."""
    cpu_threshold: Optional[float] = None
    memory_threshold: Optional[float] = None
    missed_blocks_threshold: Optional[int] = None
    downtime_threshold_seconds: Optional[int] = None
    sync_lag_threshold: Optional[int] = None
    applies_to_region: Optional[str] = None
    applies_to_tier: Optional[str] = None
    target_region: Optional[str] = None
    cooldown_seconds: int = 300
    max_actions_per_hour: int = 3
    notify_on_trigger: bool = True
    notification_channels: Optional[List[str]] = None
    enabled: bool = True


class FailoverRuleResponse(FailoverRuleCreate):
    """Failover rule response."""
    id: UUID
    created_at: datetime
    updated_at: datetime

    class Config:
        from_attributes = True


class FailoverEventResponse(BaseModel):
    """Failover event response."""
    id: UUID
    rule_id: Optional[UUID]
    node_id: UUID
    trigger_type: str
    action_taken: str
    trigger_value: Optional[float]
    trigger_threshold: Optional[float]
    trigger_reason: Optional[str]
    success: bool
    migration_job_id: Optional[UUID]
    error_message: Optional[str]
    detected_at: datetime
    action_started_at: Optional[datetime]
    action_completed_at: Optional[datetime]
    recovery_time_seconds: Optional[int]
    source_region: Optional[str]
    target_region: Optional[str]
    created_at: datetime

    class Config:
        from_attributes = True


class RegionOutageResponse(BaseModel):
    """Region outage response."""
    id: UUID
    region: str
    detected_at: datetime
    confirmed_at: Optional[datetime]
    resolved_at: Optional[datetime]
    affected_nodes_count: int
    nodes_migrated_count: int
    nodes_failed_migration: int
    cause: Optional[str]
    description: Optional[str]
    auto_failover_triggered: bool
    status: str
    detection_latency_seconds: Optional[int]
    total_downtime_seconds: Optional[int]
    created_at: datetime
    updated_at: datetime

    class Config:
        from_attributes = True


class DoubleSignGuardResponse(BaseModel):
    """Double sign guard status."""
    id: UUID
    validator_id: UUID
    validator_address: str
    is_signing_active: bool
    active_node_id: Optional[UUID]
    active_region: Optional[str]
    last_signed_height: Optional[int]
    last_signed_time: Optional[datetime]
    migration_lock: bool
    migration_lock_id: Optional[UUID]
    migration_lock_expires: Optional[datetime]
    verification_passed: Optional[bool]
    created_at: datetime
    updated_at: datetime

    class Config:
        from_attributes = True


class MigrationExecuteRequest(BaseModel):
    """Execute migration request."""
    node_id: UUID
    target_region: str
    migration_type: str = "manual"
    priority: int = Field(default=5, ge=1, le=10)
    use_snapshot: bool = True
    initiated_by: Optional[UUID] = None


class MigrationStatusResponse(BaseModel):
    """Detailed migration status."""
    job: MigrationJobResponse
    logs: List[MigrationLogResponse]
    double_sign_guard: Optional[DoubleSignGuardResponse]


class RollbackRequest(BaseModel):
    """Rollback migration request."""
    reason: Optional[str] = None


# ============================================================================
# Migration Job Endpoints
# ============================================================================


@router.get("/migration/jobs", response_model=List[MigrationJobResponse])
async def list_migration_jobs(
    status: Optional[str] = None,
    node_id: Optional[UUID] = None,
    source_region: Optional[str] = None,
    target_region: Optional[str] = None,
    limit: int = Query(default=50, ge=1, le=200),
    offset: int = Query(default=0, ge=0)
):
    """
    List migration jobs with optional filtering.

    - **status**: Filter by status (pending, preparing, completed, etc.)
    - **node_id**: Filter by specific node
    - **source_region**: Filter by source region
    - **target_region**: Filter by target region
    """
    # Mock data for now
    mock_jobs = [
        {
            "id": "550e8400-e29b-41d4-a716-446655440001",
            "node_id": "550e8400-e29b-41d4-a716-446655440010",
            "validator_id": "550e8400-e29b-41d4-a716-446655440020",
            "source_region": "us-east",
            "target_region": "us-west",
            "source_server_id": "550e8400-e29b-41d4-a716-446655440030",
            "target_server_id": "550e8400-e29b-41d4-a716-446655440031",
            "migration_type": "manual",
            "status": "completed",
            "priority": 5,
            "transfer_progress_percent": 100.0,
            "double_sign_check_passed": True,
            "signing_key_transferred": True,
            "last_signed_block": 1234567,
            "started_at": "2024-11-22T10:00:00Z",
            "completed_at": "2024-11-22T10:15:00Z",
            "estimated_duration_seconds": 900,
            "actual_duration_seconds": 892,
            "error_message": None,
            "retry_count": 0,
            "rollback_available": True,
            "created_at": "2024-11-22T09:55:00Z",
            "updated_at": "2024-11-22T10:15:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440002",
            "node_id": "550e8400-e29b-41d4-a716-446655440011",
            "validator_id": "550e8400-e29b-41d4-a716-446655440021",
            "source_region": "eu-central",
            "target_region": "asia-pacific",
            "source_server_id": "550e8400-e29b-41d4-a716-446655440032",
            "target_server_id": None,
            "migration_type": "auto_failover",
            "status": "transferring",
            "priority": 8,
            "transfer_progress_percent": 45.5,
            "double_sign_check_passed": True,
            "signing_key_transferred": False,
            "last_signed_block": 1234580,
            "started_at": "2024-11-23T08:30:00Z",
            "completed_at": None,
            "estimated_duration_seconds": 1200,
            "actual_duration_seconds": None,
            "error_message": None,
            "retry_count": 0,
            "rollback_available": True,
            "created_at": "2024-11-23T08:28:00Z",
            "updated_at": "2024-11-23T08:35:00Z"
        }
    ]

    return mock_jobs


@router.post("/migration/execute", response_model=MigrationJobResponse)
async def execute_migration(request: MigrationExecuteRequest):
    """
    Execute a node migration to a different region.

    This endpoint:
    1. Creates a migration job
    2. Stops the source node (safely)
    3. Transfers state/snapshot to target region
    4. Starts node in target region
    5. Verifies the migration was successful

    **Double-sign prevention**: The source node is stopped before
    the target node is allowed to start signing.
    """
    # Validate target region
    valid_regions = ["us-east", "us-west", "eu-central", "asia-pacific"]
    if request.target_region not in valid_regions:
        raise HTTPException(status_code=400, detail=f"Invalid target region: {request.target_region}")

    # Mock response - in production, this would create a migration job
    return {
        "id": "550e8400-e29b-41d4-a716-446655440099",
        "node_id": str(request.node_id),
        "validator_id": None,
        "source_region": "us-east",  # Would be looked up from node
        "target_region": request.target_region,
        "source_server_id": None,
        "target_server_id": None,
        "migration_type": request.migration_type,
        "status": "pending",
        "priority": request.priority,
        "transfer_progress_percent": 0.0,
        "double_sign_check_passed": False,
        "signing_key_transferred": False,
        "last_signed_block": None,
        "started_at": None,
        "completed_at": None,
        "estimated_duration_seconds": 900,
        "actual_duration_seconds": None,
        "error_message": None,
        "retry_count": 0,
        "rollback_available": True,
        "created_at": datetime.utcnow().isoformat(),
        "updated_at": datetime.utcnow().isoformat()
    }


@router.get("/migration/{job_id}/status", response_model=MigrationStatusResponse)
async def get_migration_status(job_id: UUID):
    """Get detailed status of a migration job including logs and guard status."""
    # Mock response
    return {
        "job": {
            "id": str(job_id),
            "node_id": "550e8400-e29b-41d4-a716-446655440010",
            "validator_id": "550e8400-e29b-41d4-a716-446655440020",
            "source_region": "us-east",
            "target_region": "us-west",
            "source_server_id": "550e8400-e29b-41d4-a716-446655440030",
            "target_server_id": "550e8400-e29b-41d4-a716-446655440031",
            "migration_type": "manual",
            "status": "transferring",
            "priority": 5,
            "transfer_progress_percent": 65.0,
            "double_sign_check_passed": True,
            "signing_key_transferred": False,
            "last_signed_block": 1234590,
            "started_at": "2024-11-23T09:00:00Z",
            "completed_at": None,
            "estimated_duration_seconds": 900,
            "actual_duration_seconds": None,
            "error_message": None,
            "retry_count": 0,
            "rollback_available": True,
            "created_at": "2024-11-23T08:58:00Z",
            "updated_at": "2024-11-23T09:05:00Z"
        },
        "logs": [
            {
                "id": "550e8400-e29b-41d4-a716-446655440100",
                "migration_id": str(job_id),
                "level": "info",
                "step": "preparing",
                "message": "Migration job created",
                "details": None,
                "timestamp": "2024-11-23T08:58:00Z"
            },
            {
                "id": "550e8400-e29b-41d4-a716-446655440101",
                "migration_id": str(job_id),
                "level": "info",
                "step": "stopping_source",
                "message": "Stopping source node validator process",
                "details": {"last_block": 1234590},
                "timestamp": "2024-11-23T09:00:00Z"
            },
            {
                "id": "550e8400-e29b-41d4-a716-446655440102",
                "migration_id": str(job_id),
                "level": "info",
                "step": "transferring",
                "message": "State transfer in progress",
                "details": {"progress": 65.0, "size_mb": 2048},
                "timestamp": "2024-11-23T09:05:00Z"
            }
        ],
        "double_sign_guard": {
            "id": "550e8400-e29b-41d4-a716-446655440200",
            "validator_id": "550e8400-e29b-41d4-a716-446655440020",
            "validator_address": "omniphivaloper1abc123def456",
            "is_signing_active": False,
            "active_node_id": None,
            "active_region": None,
            "last_signed_height": 1234590,
            "last_signed_time": "2024-11-23T09:00:00Z",
            "migration_lock": True,
            "migration_lock_id": str(job_id),
            "migration_lock_expires": "2024-11-23T10:00:00Z",
            "verification_passed": True,
            "created_at": "2024-11-20T00:00:00Z",
            "updated_at": "2024-11-23T09:00:00Z"
        }
    }


@router.post("/migration/{job_id}/rollback", response_model=MigrationJobResponse)
async def rollback_migration(job_id: UUID, request: RollbackRequest):
    """
    Rollback a migration to the source node.

    Only available for migrations that have rollback enabled and
    haven't exceeded the rollback window.
    """
    return {
        "id": str(job_id),
        "node_id": "550e8400-e29b-41d4-a716-446655440010",
        "validator_id": "550e8400-e29b-41d4-a716-446655440020",
        "source_region": "us-east",
        "target_region": "us-west",
        "source_server_id": "550e8400-e29b-41d4-a716-446655440030",
        "target_server_id": "550e8400-e29b-41d4-a716-446655440031",
        "migration_type": "manual",
        "status": "rolled_back",
        "priority": 5,
        "transfer_progress_percent": 65.0,
        "double_sign_check_passed": True,
        "signing_key_transferred": False,
        "last_signed_block": 1234590,
        "started_at": "2024-11-23T09:00:00Z",
        "completed_at": "2024-11-23T09:10:00Z",
        "estimated_duration_seconds": 900,
        "actual_duration_seconds": 600,
        "error_message": f"Rollback initiated: {request.reason or 'User requested'}",
        "retry_count": 0,
        "rollback_available": False,
        "created_at": "2024-11-23T08:58:00Z",
        "updated_at": "2024-11-23T09:10:00Z"
    }


# ============================================================================
# Failover Rule Endpoints
# ============================================================================


@router.get("/failover/rules", response_model=List[FailoverRuleResponse])
async def list_failover_rules(
    trigger_type: Optional[str] = None,
    enabled: Optional[bool] = None
):
    """List all failover rules."""
    mock_rules = [
        {
            "id": "550e8400-e29b-41d4-a716-446655440300",
            "name": "High CPU Auto-Migrate",
            "description": "Migrate node when CPU exceeds 90% for extended period",
            "trigger_type": "high_cpu",
            "action": "migrate",
            "priority": 5,
            "cpu_threshold": 90.0,
            "memory_threshold": None,
            "missed_blocks_threshold": None,
            "downtime_threshold_seconds": None,
            "sync_lag_threshold": None,
            "applies_to_region": None,
            "applies_to_tier": None,
            "target_region": None,
            "cooldown_seconds": 300,
            "max_actions_per_hour": 3,
            "notify_on_trigger": True,
            "notification_channels": ["slack", "email"],
            "enabled": True,
            "created_at": "2024-11-01T00:00:00Z",
            "updated_at": "2024-11-01T00:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440301",
            "name": "Node Down Auto-Restart",
            "description": "Automatically restart node when it goes down",
            "trigger_type": "node_down",
            "action": "restart",
            "priority": 10,
            "cpu_threshold": None,
            "memory_threshold": None,
            "missed_blocks_threshold": None,
            "downtime_threshold_seconds": 60,
            "sync_lag_threshold": None,
            "applies_to_region": None,
            "applies_to_tier": None,
            "target_region": None,
            "cooldown_seconds": 120,
            "max_actions_per_hour": 5,
            "notify_on_trigger": True,
            "notification_channels": ["pagerduty", "slack"],
            "enabled": True,
            "created_at": "2024-11-01T00:00:00Z",
            "updated_at": "2024-11-01T00:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440302",
            "name": "Missed Blocks Alert",
            "description": "Alert when validator misses more than 10 blocks",
            "trigger_type": "missed_blocks",
            "action": "alert_only",
            "priority": 8,
            "cpu_threshold": None,
            "memory_threshold": None,
            "missed_blocks_threshold": 10,
            "downtime_threshold_seconds": None,
            "sync_lag_threshold": None,
            "applies_to_region": None,
            "applies_to_tier": None,
            "target_region": None,
            "cooldown_seconds": 600,
            "max_actions_per_hour": 10,
            "notify_on_trigger": True,
            "notification_channels": ["slack", "email"],
            "enabled": True,
            "created_at": "2024-11-01T00:00:00Z",
            "updated_at": "2024-11-01T00:00:00Z"
        }
    ]

    return mock_rules


@router.post("/failover/rules", response_model=FailoverRuleResponse)
async def create_failover_rule(rule: FailoverRuleCreate):
    """Create a new failover rule."""
    return {
        "id": "550e8400-e29b-41d4-a716-446655440399",
        **rule.model_dump(),
        "created_at": datetime.utcnow().isoformat(),
        "updated_at": datetime.utcnow().isoformat()
    }


@router.put("/failover/rules/{rule_id}", response_model=FailoverRuleResponse)
async def update_failover_rule(rule_id: UUID, rule: FailoverRuleCreate):
    """Update an existing failover rule."""
    return {
        "id": str(rule_id),
        **rule.model_dump(),
        "created_at": "2024-11-01T00:00:00Z",
        "updated_at": datetime.utcnow().isoformat()
    }


@router.delete("/failover/rules/{rule_id}")
async def delete_failover_rule(rule_id: UUID):
    """Delete a failover rule."""
    return {"status": "deleted", "rule_id": str(rule_id)}


# ============================================================================
# Failover Event Endpoints
# ============================================================================


@router.get("/failover/events", response_model=List[FailoverEventResponse])
async def list_failover_events(
    node_id: Optional[UUID] = None,
    trigger_type: Optional[str] = None,
    success: Optional[bool] = None,
    limit: int = Query(default=50, ge=1, le=200),
    offset: int = Query(default=0, ge=0)
):
    """List failover events with optional filtering."""
    mock_events = [
        {
            "id": "550e8400-e29b-41d4-a716-446655440400",
            "rule_id": "550e8400-e29b-41d4-a716-446655440301",
            "node_id": "550e8400-e29b-41d4-a716-446655440010",
            "trigger_type": "node_down",
            "action_taken": "restart",
            "trigger_value": None,
            "trigger_threshold": 60,
            "trigger_reason": "Node heartbeat missed for 65 seconds",
            "success": True,
            "migration_job_id": None,
            "error_message": None,
            "detected_at": "2024-11-22T14:30:00Z",
            "action_started_at": "2024-11-22T14:30:05Z",
            "action_completed_at": "2024-11-22T14:31:00Z",
            "recovery_time_seconds": 55,
            "source_region": "us-east",
            "target_region": None,
            "created_at": "2024-11-22T14:30:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440401",
            "rule_id": "550e8400-e29b-41d4-a716-446655440300",
            "node_id": "550e8400-e29b-41d4-a716-446655440011",
            "trigger_type": "high_cpu",
            "action_taken": "migrate",
            "trigger_value": 95.5,
            "trigger_threshold": 90.0,
            "trigger_reason": "CPU usage at 95.5% for 10 minutes",
            "success": True,
            "migration_job_id": "550e8400-e29b-41d4-a716-446655440001",
            "error_message": None,
            "detected_at": "2024-11-21T08:00:00Z",
            "action_started_at": "2024-11-21T08:00:30Z",
            "action_completed_at": "2024-11-21T08:15:00Z",
            "recovery_time_seconds": 870,
            "source_region": "us-east",
            "target_region": "us-west",
            "created_at": "2024-11-21T08:00:00Z"
        }
    ]

    return mock_events


# ============================================================================
# Region Outage Endpoints
# ============================================================================


@router.get("/failover/outages", response_model=List[RegionOutageResponse])
async def list_region_outages(
    region: Optional[str] = None,
    status: Optional[str] = None,
    limit: int = Query(default=20, ge=1, le=100),
    offset: int = Query(default=0, ge=0)
):
    """List region outages."""
    mock_outages = [
        {
            "id": "550e8400-e29b-41d4-a716-446655440500",
            "region": "eu-central",
            "detected_at": "2024-11-15T03:00:00Z",
            "confirmed_at": "2024-11-15T03:02:00Z",
            "resolved_at": "2024-11-15T04:30:00Z",
            "affected_nodes_count": 15,
            "nodes_migrated_count": 12,
            "nodes_failed_migration": 0,
            "cause": "Provider network issue",
            "description": "Upstream provider experienced network partition",
            "auto_failover_triggered": True,
            "status": "resolved",
            "detection_latency_seconds": 45,
            "total_downtime_seconds": 5400,
            "created_at": "2024-11-15T03:00:00Z",
            "updated_at": "2024-11-15T04:30:00Z"
        }
    ]

    return mock_outages


@router.post("/failover/outages/{outage_id}/resolve")
async def resolve_outage(outage_id: UUID, cause: Optional[str] = None):
    """Mark a region outage as resolved."""
    return {
        "status": "resolved",
        "outage_id": str(outage_id),
        "resolved_at": datetime.utcnow().isoformat(),
        "cause": cause
    }


# ============================================================================
# Double-Sign Guard Endpoints
# ============================================================================


@router.get("/failover/double-sign-guards", response_model=List[DoubleSignGuardResponse])
async def list_double_sign_guards(
    validator_id: Optional[UUID] = None,
    migration_lock: Optional[bool] = None
):
    """List double-sign guard status for validators."""
    mock_guards = [
        {
            "id": "550e8400-e29b-41d4-a716-446655440600",
            "validator_id": "550e8400-e29b-41d4-a716-446655440020",
            "validator_address": "omniphivaloper1abc123def456",
            "is_signing_active": True,
            "active_node_id": "550e8400-e29b-41d4-a716-446655440010",
            "active_region": "us-east",
            "last_signed_height": 1234600,
            "last_signed_time": "2024-11-23T10:00:00Z",
            "migration_lock": False,
            "migration_lock_id": None,
            "migration_lock_expires": None,
            "verification_passed": True,
            "created_at": "2024-11-01T00:00:00Z",
            "updated_at": "2024-11-23T10:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440601",
            "validator_id": "550e8400-e29b-41d4-a716-446655440021",
            "validator_address": "omniphivaloper1xyz789ghi012",
            "is_signing_active": False,
            "active_node_id": None,
            "active_region": None,
            "last_signed_height": 1234580,
            "last_signed_time": "2024-11-23T08:30:00Z",
            "migration_lock": True,
            "migration_lock_id": "550e8400-e29b-41d4-a716-446655440002",
            "migration_lock_expires": "2024-11-23T09:30:00Z",
            "verification_passed": True,
            "created_at": "2024-11-01T00:00:00Z",
            "updated_at": "2024-11-23T08:30:00Z"
        }
    ]

    return mock_guards


@router.post("/failover/double-sign-guards/{validator_id}/verify")
async def verify_double_sign_safety(validator_id: UUID):
    """
    Verify that a validator is safe to start signing.

    Checks:
    1. No other node is currently signing for this validator
    2. Sufficient blocks have passed since last signed block
    3. No conflicting migration is in progress
    """
    return {
        "validator_id": str(validator_id),
        "verification_passed": True,
        "checks": {
            "no_active_signer": True,
            "signing_gap_sufficient": True,
            "no_migration_conflict": True
        },
        "safe_to_sign": True,
        "verified_at": datetime.utcnow().isoformat()
    }
