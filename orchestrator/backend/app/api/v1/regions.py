"""
Multi-Region Infrastructure API Endpoints

Endpoints for managing regions, server pools, and regional health monitoring.
"""

from datetime import datetime
from typing import List, Optional
from uuid import UUID

from fastapi import APIRouter, HTTPException, Query
from pydantic import BaseModel, Field

router = APIRouter(prefix="/regions", tags=["regions"])


# ============================================
# SCHEMAS
# ============================================

class RegionResponse(BaseModel):
    """Region response schema"""
    id: str
    code: str
    name: str
    display_name: str
    status: str
    max_validators: int
    active_validators: int
    capacity_percent: float
    cpu_utilization: float
    memory_utilization: float
    is_accepting_new: bool
    base_monthly_cost: float
    created_at: str


class RegionCapacityResponse(BaseModel):
    """Region capacity response schema"""
    region_id: str
    region_code: str
    max_validators: int
    active_validators: int
    available_slots: int
    capacity_percent: float
    max_cpu_cores: int
    used_cpu_cores: int
    cpu_utilization: float
    max_memory_gb: int
    used_memory_gb: int
    memory_utilization: float
    max_disk_gb: int
    used_disk_gb: int
    disk_utilization: float
    server_pools: List[dict]


class RegionHealthResponse(BaseModel):
    """Region health response schema"""
    region_id: str
    region_code: str
    is_healthy: bool
    health_score: float
    latency_ms: float
    success_rate: float
    error_rate: float
    p2p_connectivity: float
    rpc_availability: float
    total_nodes: int
    healthy_nodes: int
    warning_nodes: int
    error_nodes: int
    avg_block_height: int
    active_incidents: int
    checked_at: str


class RegisterServerRequest(BaseModel):
    """Request to register a new server"""
    region_code: str
    hostname: str
    ip_address: str
    internal_ip: Optional[str] = None
    provider: str = "omniphi-cloud"
    provider_instance_id: Optional[str] = None
    availability_zone: Optional[str] = None
    machine_type: str
    cpu_cores: int
    memory_gb: int
    disk_gb: int
    max_validators: int = 10
    tags: Optional[dict] = None


class ServerResponse(BaseModel):
    """Server response schema"""
    id: str
    hostname: str
    ip_address: str
    provider: str
    machine_type: str
    cpu_cores: int
    memory_gb: int
    disk_gb: int
    validators_hosted: int
    max_validators: int
    is_active: bool
    is_available: bool
    last_heartbeat: Optional[str]


# ============================================
# MOCK DATA
# ============================================

def get_mock_regions():
    """Generate mock region data."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440100",
            "code": "us-east",
            "name": "US East",
            "display_name": "US East (N. Virginia)",
            "status": "active",
            "max_validators": 500,
            "active_validators": 350,
            "capacity_percent": 70.0,
            "cpu_utilization": 55.0,
            "memory_utilization": 62.0,
            "is_accepting_new": True,
            "base_monthly_cost": 89.0,
            "created_at": "2024-01-01T00:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440101",
            "code": "us-west",
            "name": "US West",
            "display_name": "US West (Oregon)",
            "status": "active",
            "max_validators": 400,
            "active_validators": 280,
            "capacity_percent": 70.0,
            "cpu_utilization": 58.0,
            "memory_utilization": 65.0,
            "is_accepting_new": True,
            "base_monthly_cost": 89.0,
            "created_at": "2024-01-01T00:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440102",
            "code": "eu-central",
            "name": "EU Central",
            "display_name": "EU Central (Frankfurt)",
            "status": "active",
            "max_validators": 350,
            "active_validators": 220,
            "capacity_percent": 62.9,
            "cpu_utilization": 52.0,
            "memory_utilization": 58.0,
            "is_accepting_new": True,
            "base_monthly_cost": 94.0,
            "created_at": "2024-01-01T00:00:00Z"
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440103",
            "code": "asia-pacific",
            "name": "Asia Pacific",
            "display_name": "Asia Pacific (Singapore)",
            "status": "active",
            "max_validators": 250,
            "active_validators": 150,
            "capacity_percent": 60.0,
            "cpu_utilization": 48.0,
            "memory_utilization": 55.0,
            "is_accepting_new": True,
            "base_monthly_cost": 95.0,
            "created_at": "2024-01-01T00:00:00Z"
        }
    ]


# ============================================
# ENDPOINTS
# ============================================

@router.get("", response_model=List[RegionResponse])
async def list_regions(
    status: Optional[str] = Query(None, description="Filter by status"),
    accepting_new: Optional[bool] = Query(None, description="Filter by accepting new validators"),
):
    """
    List all regions with their current status.

    Returns regions with capacity and health metrics.
    """
    regions = get_mock_regions()

    if status:
        regions = [r for r in regions if r["status"] == status]
    if accepting_new is not None:
        regions = [r for r in regions if r["is_accepting_new"] == accepting_new]

    return regions


@router.get("/{region_id}", response_model=RegionResponse)
async def get_region(region_id: str):
    """Get details for a specific region."""
    regions = get_mock_regions()
    region = next((r for r in regions if r["id"] == region_id), None)

    if not region:
        raise HTTPException(status_code=404, detail="Region not found")

    return region


@router.get("/{region_id}/capacity", response_model=RegionCapacityResponse)
async def get_region_capacity(region_id: str):
    """
    Get detailed capacity information for a region.

    Includes server pool details and resource utilization.
    """
    regions = get_mock_regions()
    region = next((r for r in regions if r["id"] == region_id), None)

    if not region:
        raise HTTPException(status_code=404, detail="Region not found")

    return {
        "region_id": region["id"],
        "region_code": region["code"],
        "max_validators": region["max_validators"],
        "active_validators": region["active_validators"],
        "available_slots": region["max_validators"] - region["active_validators"],
        "capacity_percent": region["capacity_percent"],
        "max_cpu_cores": 2000,
        "used_cpu_cores": int(2000 * region["cpu_utilization"] / 100),
        "cpu_utilization": region["cpu_utilization"],
        "max_memory_gb": 8000,
        "used_memory_gb": int(8000 * region["memory_utilization"] / 100),
        "memory_utilization": region["memory_utilization"],
        "max_disk_gb": 100000,
        "used_disk_gb": 45000,
        "disk_utilization": 45.0,
        "server_pools": [
            {
                "id": "pool-1",
                "name": "c5.2xlarge Pool",
                "machine_type": "large",
                "total_machines": 50,
                "available_machines": 15,
                "monthly_cost": 150.0,
                "utilization_percent": 70.0
            },
            {
                "id": "pool-2",
                "name": "c5.xlarge Pool",
                "machine_type": "medium",
                "total_machines": 30,
                "available_machines": 8,
                "monthly_cost": 89.0,
                "utilization_percent": 73.3
            }
        ]
    }


@router.get("/{region_id}/health", response_model=RegionHealthResponse)
async def get_region_health(region_id: str):
    """
    Get health status for a region.

    Returns latest health metrics and node statistics.
    """
    regions = get_mock_regions()
    region = next((r for r in regions if r["id"] == region_id), None)

    if not region:
        raise HTTPException(status_code=404, detail="Region not found")

    return {
        "region_id": region["id"],
        "region_code": region["code"],
        "is_healthy": True,
        "health_score": 98.5,
        "latency_ms": 12.5,
        "success_rate": 99.9,
        "error_rate": 0.1,
        "p2p_connectivity": 97.8,
        "rpc_availability": 99.99,
        "total_nodes": region["active_validators"],
        "healthy_nodes": int(region["active_validators"] * 0.95),
        "warning_nodes": int(region["active_validators"] * 0.04),
        "error_nodes": int(region["active_validators"] * 0.01),
        "avg_block_height": 1567890,
        "active_incidents": 2,
        "checked_at": datetime.utcnow().isoformat()
    }


@router.post("/register-server", response_model=ServerResponse)
async def register_server(request: RegisterServerRequest):
    """
    Register a new server in a region.

    Adds a server to the regional server pool for validator hosting.
    """
    valid_regions = ["us-east", "us-west", "eu-central", "asia-pacific"]
    if request.region_code not in valid_regions:
        raise HTTPException(status_code=404, detail=f"Region '{request.region_code}' not found")

    return {
        "id": "550e8400-e29b-41d4-a716-446655440200",
        "hostname": request.hostname,
        "ip_address": request.ip_address,
        "provider": request.provider,
        "machine_type": request.machine_type,
        "cpu_cores": request.cpu_cores,
        "memory_gb": request.memory_gb,
        "disk_gb": request.disk_gb,
        "validators_hosted": 0,
        "max_validators": request.max_validators,
        "is_active": True,
        "is_available": True,
        "last_heartbeat": datetime.utcnow().isoformat()
    }


@router.get("/{region_id}/servers", response_model=List[ServerResponse])
async def list_region_servers(
    region_id: str,
    available_only: bool = Query(False, description="Only show available servers"),
):
    """List all servers in a region."""
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440201",
            "hostname": f"srv-{region_id[:8]}-001",
            "ip_address": "10.0.1.10",
            "provider": "omniphi-cloud",
            "machine_type": "large",
            "cpu_cores": 8,
            "memory_gb": 32,
            "disk_gb": 500,
            "validators_hosted": 5,
            "max_validators": 10,
            "is_active": True,
            "is_available": True,
            "last_heartbeat": datetime.utcnow().isoformat()
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440202",
            "hostname": f"srv-{region_id[:8]}-002",
            "ip_address": "10.0.1.11",
            "provider": "omniphi-cloud",
            "machine_type": "medium",
            "cpu_cores": 4,
            "memory_gb": 16,
            "disk_gb": 250,
            "validators_hosted": 3,
            "max_validators": 5,
            "is_active": True,
            "is_available": True,
            "last_heartbeat": datetime.utcnow().isoformat()
        }
    ]


@router.post("/{region_id}/servers/{server_id}/heartbeat")
async def server_heartbeat(region_id: str, server_id: str):
    """Update server heartbeat timestamp."""
    return {"status": "ok", "last_heartbeat": datetime.utcnow().isoformat()}
