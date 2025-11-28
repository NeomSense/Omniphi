"""
Validator Upgrade Pipeline API Endpoints

Endpoints for managing chain upgrades, canary rollouts, and upgrade status tracking.
"""

from datetime import datetime, timedelta
from typing import List, Optional
from uuid import UUID

from fastapi import APIRouter, HTTPException, Query, BackgroundTasks
from pydantic import BaseModel, Field

router = APIRouter(prefix="/upgrades", tags=["upgrades"])


# ============================================
# SCHEMAS
# ============================================

class UpgradeResponse(BaseModel):
    """Chain upgrade response schema"""
    id: str
    name: str
    version: str
    upgrade_type: str
    current_version: str
    new_binary_version: str
    upgrade_height: Optional[int]
    current_height: int
    scheduled_time: Optional[str]
    started_at: Optional[str]
    completed_at: Optional[str]
    status: str
    total_nodes: int
    updated_nodes: int
    failed_nodes: int
    pending_nodes: int
    completion_percent: float
    canary_enabled: bool
    canary_completed: bool
    canary_success: bool
    rollback_available: bool
    created_at: str


class UpgradeDetailResponse(UpgradeResponse):
    """Detailed upgrade response with additional fields"""
    binary_url: Optional[str]
    canary_nodes: List[str]
    canary_percent: float
    canary_wait_minutes: int
    region_order: List[str]
    current_region: Optional[str]
    rollback_version: Optional[str]
    release_notes: Optional[str]
    breaking_changes: Optional[List[str]]
    created_by: Optional[str]


class NodeUpgradeStatusResponse(BaseModel):
    """Node upgrade status response"""
    id: str
    node_id: str
    moniker: str
    region: str
    current_version: str
    target_version: str
    status: str
    is_canary: bool
    download_percent: float
    install_percent: float
    started_at: Optional[str]
    completed_at: Optional[str]
    duration_seconds: Optional[int]
    error_message: Optional[str]
    retry_count: int


class UpgradeLogResponse(BaseModel):
    """Upgrade log entry response"""
    id: str
    level: str
    source: str
    message: str
    node_id: Optional[str]
    timestamp: str


class CreateUpgradeRequest(BaseModel):
    """Request to create a new upgrade"""
    name: str
    version: str
    upgrade_type: str = "chain_upgrade"
    current_version: str
    new_binary_version: str
    binary_url: Optional[str] = None
    binary_checksum: Optional[str] = None
    upgrade_height: Optional[int] = None
    scheduled_time: Optional[datetime] = None
    canary_enabled: bool = True
    canary_percent: float = 1.0
    canary_wait_minutes: int = 30
    region_order: List[str] = []
    release_notes: Optional[str] = None
    breaking_changes: Optional[List[str]] = None


class StartRolloutRequest(BaseModel):
    """Request to start upgrade rollout"""
    skip_canary: bool = False
    target_regions: Optional[List[str]] = None
    batch_size: int = 10


class CheckVersionResponse(BaseModel):
    """Version check response"""
    current_version: str
    latest_version: str
    update_available: bool
    upgrade: Optional[UpgradeResponse]
    binary_url: Optional[str]
    release_notes: Optional[str]


class RolloutStatusResponse(BaseModel):
    """Rollout status response"""
    upgrade_id: str
    status: str
    phase: str  # canary, regional, complete
    current_region: Optional[str]
    progress: dict
    estimated_completion: Optional[str]
    nodes_by_status: dict


# ============================================
# MOCK DATA
# ============================================

def get_mock_upgrades():
    """Generate mock upgrade data."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440001",
            "name": "v1.2.0 Chain Upgrade",
            "version": "v1.2.0",
            "upgrade_type": "chain_upgrade",
            "current_version": "v1.1.0",
            "new_binary_version": "v1.2.0",
            "upgrade_height": 5000000,
            "current_height": 4990000,
            "scheduled_time": "2024-12-01T00:00:00Z",
            "started_at": None,
            "completed_at": None,
            "status": "scheduled",
            "total_nodes": 100,
            "updated_nodes": 0,
            "failed_nodes": 0,
            "pending_nodes": 100,
            "completion_percent": 0.0,
            "canary_enabled": True,
            "canary_completed": False,
            "canary_success": False,
            "rollback_available": True,
            "created_at": "2024-11-20T00:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440002",
            "name": "v1.1.0 Chain Upgrade",
            "version": "v1.1.0",
            "upgrade_type": "chain_upgrade",
            "current_version": "v1.0.0",
            "new_binary_version": "v1.1.0",
            "upgrade_height": 4500000,
            "current_height": 4990000,
            "scheduled_time": "2024-11-01T00:00:00Z",
            "started_at": "2024-11-01T00:00:00Z",
            "completed_at": "2024-11-01T02:00:00Z",
            "status": "completed",
            "total_nodes": 95,
            "updated_nodes": 95,
            "failed_nodes": 0,
            "pending_nodes": 0,
            "completion_percent": 100.0,
            "canary_enabled": True,
            "canary_completed": True,
            "canary_success": True,
            "rollback_available": False,
            "created_at": "2024-10-25T00:00:00Z"
        }
    ]


def get_mock_node_statuses(upgrade_id: str):
    """Generate mock node upgrade status data."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440010",
            "node_id": "550e8400-e29b-41d4-a716-446655440100",
            "moniker": "validator-us-east-01",
            "region": "us-east",
            "current_version": "v1.1.0",
            "target_version": "v1.2.0",
            "status": "pending",
            "is_canary": True,
            "download_percent": 0.0,
            "install_percent": 0.0,
            "started_at": None,
            "completed_at": None,
            "duration_seconds": None,
            "error_message": None,
            "retry_count": 0
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440011",
            "node_id": "550e8400-e29b-41d4-a716-446655440101",
            "moniker": "validator-us-west-01",
            "region": "us-west",
            "current_version": "v1.1.0",
            "target_version": "v1.2.0",
            "status": "pending",
            "is_canary": False,
            "download_percent": 0.0,
            "install_percent": 0.0,
            "started_at": None,
            "completed_at": None,
            "duration_seconds": None,
            "error_message": None,
            "retry_count": 0
        }
    ]


def get_mock_upgrade_logs(upgrade_id: str):
    """Generate mock upgrade logs."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440020",
            "level": "info",
            "source": "api",
            "message": "Upgrade scheduled successfully",
            "node_id": None,
            "timestamp": "2024-11-20T00:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440021",
            "level": "info",
            "source": "scheduler",
            "message": "Upgrade notification sent to all nodes",
            "node_id": None,
            "timestamp": "2024-11-20T00:00:30Z"
        }
    ]


# ============================================
# ENDPOINTS
# ============================================

@router.get("", response_model=List[UpgradeResponse])
async def list_upgrades(
    status: Optional[str] = Query(None, description="Filter by status"),
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
):
    """
    List all upgrades with optional filtering.

    Returns upgrades sorted by scheduled/created time.
    """
    upgrades = get_mock_upgrades()

    if status:
        upgrades = [u for u in upgrades if u["status"] == status]

    return upgrades[offset:offset + limit]


@router.get("/check", response_model=CheckVersionResponse)
async def check_for_updates(
    current_version: str = Query(..., description="Current binary version"),
    chain_id: str = Query("omniphi-1", description="Chain ID"),
):
    """
    Check for new binary versions.

    Returns the latest available version and upgrade info if available.
    """
    latest_version = "v1.2.0"
    update_available = current_version != latest_version

    upgrade = None
    if update_available:
        upgrades = get_mock_upgrades()
        scheduled = [u for u in upgrades if u["status"] == "scheduled"]
        if scheduled:
            upgrade = scheduled[0]

    return {
        "current_version": current_version,
        "latest_version": latest_version,
        "update_available": update_available,
        "upgrade": upgrade,
        "binary_url": f"https://releases.omniphi.io/pos/binaries/{latest_version}/posd" if update_available else None,
        "release_notes": "Performance improvements and bug fixes" if update_available else None,
    }


@router.post("", response_model=UpgradeResponse)
async def create_upgrade(request: CreateUpgradeRequest):
    """
    Create a new upgrade definition.

    The upgrade will be scheduled but not started until rollout is initiated.
    """
    valid_types = ["chain_upgrade", "binary_update", "config_change", "emergency"]
    if request.upgrade_type not in valid_types:
        raise HTTPException(
            status_code=400,
            detail=f"Invalid upgrade type. Must be one of: {valid_types}"
        )

    return {
        "id": "550e8400-e29b-41d4-a716-446655440099",
        "name": request.name,
        "version": request.version,
        "upgrade_type": request.upgrade_type,
        "current_version": request.current_version,
        "new_binary_version": request.new_binary_version,
        "upgrade_height": request.upgrade_height,
        "current_height": 4990000,
        "scheduled_time": request.scheduled_time.isoformat() if request.scheduled_time else None,
        "started_at": None,
        "completed_at": None,
        "status": "scheduled",
        "total_nodes": 100,
        "updated_nodes": 0,
        "failed_nodes": 0,
        "pending_nodes": 100,
        "completion_percent": 0.0,
        "canary_enabled": request.canary_enabled,
        "canary_completed": False,
        "canary_success": False,
        "rollback_available": True,
        "created_at": datetime.utcnow().isoformat()
    }


@router.post("/{upgrade_id}/rollout", response_model=RolloutStatusResponse)
async def start_rollout(
    upgrade_id: str,
    request: StartRolloutRequest,
    background_tasks: BackgroundTasks,
):
    """
    Start the upgrade rollout process.

    Initiates canary rollout (if enabled) followed by regional rollout.
    """
    upgrades = get_mock_upgrades()
    upgrade = next((u for u in upgrades if u["id"] == upgrade_id), None)

    if not upgrade:
        raise HTTPException(status_code=404, detail="Upgrade not found")

    if upgrade["status"] not in ["scheduled", "paused"]:
        raise HTTPException(
            status_code=400,
            detail=f"Cannot start rollout for upgrade in status: {upgrade['status']}"
        )

    phase = "regional" if request.skip_canary else "canary"

    return {
        "upgrade_id": upgrade_id,
        "status": "rolling_out" if request.skip_canary else "canary",
        "phase": phase,
        "current_region": "us-east",
        "progress": {
            "total": 100,
            "updated": 0,
            "failed": 0,
            "pending": 100,
            "percent": 0.0,
        },
        "estimated_completion": (datetime.utcnow() + timedelta(hours=2)).isoformat(),
        "nodes_by_status": {
            "pending": 100,
            "downloading": 0,
            "installing": 0,
            "completed": 0,
            "failed": 0,
        }
    }


@router.get("/{upgrade_id}/status", response_model=RolloutStatusResponse)
async def get_rollout_status(upgrade_id: str):
    """Get current rollout status for an upgrade."""
    upgrades = get_mock_upgrades()
    upgrade = next((u for u in upgrades if u["id"] == upgrade_id), None)

    if not upgrade:
        raise HTTPException(status_code=404, detail="Upgrade not found")

    # Determine phase
    if upgrade["status"] == "scheduled":
        phase = "pending"
    elif upgrade["status"] == "completed":
        phase = "complete"
    else:
        phase = "canary" if not upgrade["canary_completed"] else "regional"

    return {
        "upgrade_id": upgrade_id,
        "status": upgrade["status"],
        "phase": phase,
        "current_region": "us-east" if upgrade["status"] not in ["scheduled", "completed"] else None,
        "progress": {
            "total": upgrade["total_nodes"],
            "updated": upgrade["updated_nodes"],
            "failed": upgrade["failed_nodes"],
            "pending": upgrade["pending_nodes"],
            "percent": upgrade["completion_percent"],
        },
        "estimated_completion": None,
        "nodes_by_status": {
            "pending": upgrade["pending_nodes"],
            "downloading": 0,
            "installing": 0,
            "completed": upgrade["updated_nodes"],
            "failed": upgrade["failed_nodes"],
        }
    }


@router.post("/{upgrade_id}/rollback")
async def rollback_upgrade(upgrade_id: str):
    """
    Rollback an upgrade to the previous version.

    Only available for upgrades with rollback_available=True.
    """
    upgrades = get_mock_upgrades()
    upgrade = next((u for u in upgrades if u["id"] == upgrade_id), None)

    if not upgrade:
        raise HTTPException(status_code=404, detail="Upgrade not found")

    if not upgrade["rollback_available"]:
        raise HTTPException(status_code=400, detail="Rollback not available for this upgrade")

    if upgrade["status"] not in ["canary", "rolling_out", "failed"]:
        raise HTTPException(
            status_code=400,
            detail=f"Cannot rollback upgrade in status: {upgrade['status']}"
        )

    return {
        "status": "rollback_initiated",
        "upgrade_id": upgrade_id,
        "rollback_version": upgrade["current_version"],
        "affected_nodes": upgrade["updated_nodes"],
    }


@router.get("/{upgrade_id}/nodes", response_model=List[NodeUpgradeStatusResponse])
async def list_node_statuses(
    upgrade_id: str,
    status: Optional[str] = Query(None, description="Filter by status"),
    region: Optional[str] = Query(None, description="Filter by region"),
    is_canary: Optional[bool] = Query(None, description="Filter canary nodes"),
    limit: int = Query(100, ge=1, le=500),
    offset: int = Query(0, ge=0),
):
    """List node upgrade statuses for an upgrade."""
    statuses = get_mock_node_statuses(upgrade_id)

    if status:
        statuses = [s for s in statuses if s["status"] == status]
    if region:
        statuses = [s for s in statuses if s["region"] == region]
    if is_canary is not None:
        statuses = [s for s in statuses if s["is_canary"] == is_canary]

    return statuses[offset:offset + limit]


@router.get("/{upgrade_id}/logs", response_model=List[UpgradeLogResponse])
async def get_upgrade_logs(
    upgrade_id: str,
    level: Optional[str] = Query(None, description="Filter by log level"),
    node_id: Optional[str] = Query(None, description="Filter by node ID"),
    limit: int = Query(100, ge=1, le=500),
    offset: int = Query(0, ge=0),
):
    """Get logs for an upgrade."""
    logs = get_mock_upgrade_logs(upgrade_id)

    if level:
        logs = [l for l in logs if l["level"] == level]
    if node_id:
        logs = [l for l in logs if l["node_id"] == node_id]

    return logs[offset:offset + limit]


@router.post("/{upgrade_id}/pause")
async def pause_rollout(upgrade_id: str):
    """Pause an in-progress rollout."""
    upgrades = get_mock_upgrades()
    upgrade = next((u for u in upgrades if u["id"] == upgrade_id), None)

    if not upgrade:
        raise HTTPException(status_code=404, detail="Upgrade not found")

    if upgrade["status"] not in ["canary", "rolling_out"]:
        raise HTTPException(
            status_code=400,
            detail=f"Cannot pause upgrade in status: {upgrade['status']}"
        )

    return {"status": "paused", "upgrade_id": upgrade_id}


@router.post("/{upgrade_id}/resume")
async def resume_rollout(upgrade_id: str):
    """Resume a paused rollout."""
    upgrades = get_mock_upgrades()
    upgrade = next((u for u in upgrades if u["id"] == upgrade_id), None)

    if not upgrade:
        raise HTTPException(status_code=404, detail="Upgrade not found")

    # For mock, just return resumed
    return {"status": "resumed", "upgrade_id": upgrade_id}
