"""
Snapshot Server API Endpoints

Endpoints for snapshot management and fast sync.
"""

from datetime import datetime, timedelta
from typing import List, Optional
from uuid import UUID, uuid4

from fastapi import APIRouter, HTTPException, Query, BackgroundTasks
from pydantic import BaseModel

router = APIRouter(prefix="/snapshots", tags=["snapshots"])


# ============================================
# SCHEMAS
# ============================================

class SnapshotResponse(BaseModel):
    """Snapshot response"""
    id: str
    chain_id: str
    network: str
    snapshot_type: str
    block_height: int
    block_time: Optional[str]
    file_name: str
    file_url: str
    file_size_bytes: int
    file_size_gb: float
    checksum_sha256: str
    is_chunked: bool
    chunk_count: Optional[int]
    is_latest: bool
    is_verified: bool
    status: str
    created_at: str


class SnapshotChunkResponse(BaseModel):
    """Snapshot chunk response"""
    chunk_index: int
    file_name: str
    file_url: str
    file_size_bytes: int
    checksum_sha256: str
    byte_start: int
    byte_end: int


class DownloadResponse(BaseModel):
    """Download response"""
    id: str
    snapshot_id: str
    node_id: str
    status: str
    bytes_downloaded: int
    total_bytes: int
    progress_percent: float
    chunks_completed: int
    total_chunks: int
    download_speed_mbps: Optional[float]
    estimated_remaining_seconds: Optional[int]
    started_at: str


class ScheduleResponse(BaseModel):
    """Schedule response"""
    id: str
    chain_id: str
    network: str
    snapshot_type: str
    schedule_cron: str
    storage_bucket: str
    retention_days: int
    is_active: bool
    last_success_at: Optional[str]


class CreateScheduleRequest(BaseModel):
    """Create schedule request"""
    chain_id: str
    network: str = "mainnet"
    snapshot_type: str = "pruned"
    schedule_cron: str = "0 0 * * *"
    storage_provider: str = "s3"
    storage_bucket: str
    storage_path_prefix: Optional[str] = None
    storage_region: Optional[str] = None
    retention_days: int = 7
    enable_chunking: bool = True
    chunk_size_mb: int = 1024


class GenerateSnapshotRequest(BaseModel):
    """Generate snapshot request"""
    chain_id: str
    network: str = "mainnet"
    snapshot_type: str = "pruned"
    source_node_id: Optional[str] = None


# ============================================
# MOCK DATA
# ============================================

def get_mock_snapshots():
    """Generate mock snapshot data."""
    now = datetime.utcnow()
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440001",
            "chain_id": "omniphi-1",
            "network": "mainnet",
            "snapshot_type": "pruned",
            "block_height": 4990000,
            "block_time": (now - timedelta(hours=2)).isoformat(),
            "file_name": "omniphi-1_4990000_pruned.tar.lz4",
            "file_url": "https://snapshots.omniphi.io/mainnet/omniphi-1_4990000_pruned.tar.lz4",
            "file_size_bytes": 52428800000,  # ~50GB
            "file_size_gb": 48.8,
            "checksum_sha256": "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
            "is_chunked": True,
            "chunk_count": 50,
            "is_latest": True,
            "is_verified": True,
            "status": "available",
            "created_at": (now - timedelta(hours=2)).isoformat()
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440002",
            "chain_id": "omniphi-1",
            "network": "mainnet",
            "snapshot_type": "archive",
            "block_height": 4980000,
            "block_time": (now - timedelta(hours=26)).isoformat(),
            "file_name": "omniphi-1_4980000_archive.tar.lz4",
            "file_url": "https://snapshots.omniphi.io/mainnet/omniphi-1_4980000_archive.tar.lz4",
            "file_size_bytes": 209715200000,  # ~200GB
            "file_size_gb": 195.3,
            "checksum_sha256": "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3",
            "is_chunked": True,
            "chunk_count": 200,
            "is_latest": True,
            "is_verified": True,
            "status": "available",
            "created_at": (now - timedelta(hours=26)).isoformat()
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440003",
            "chain_id": "omniphi-1",
            "network": "mainnet",
            "snapshot_type": "pruned",
            "block_height": 4950000,
            "block_time": (now - timedelta(days=2)).isoformat(),
            "file_name": "omniphi-1_4950000_pruned.tar.lz4",
            "file_url": "https://snapshots.omniphi.io/mainnet/omniphi-1_4950000_pruned.tar.lz4",
            "file_size_bytes": 51539607552,  # ~48GB
            "file_size_gb": 48.0,
            "checksum_sha256": "c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
            "is_chunked": True,
            "chunk_count": 48,
            "is_latest": False,
            "is_verified": True,
            "status": "available",
            "created_at": (now - timedelta(days=2)).isoformat()
        }
    ]


def get_mock_chunks(snapshot_id: str):
    """Generate mock snapshot chunks."""
    return [
        {
            "chunk_index": 0,
            "file_name": f"snapshot_{snapshot_id}_chunk_000.tar.lz4",
            "file_url": f"https://snapshots.omniphi.io/chunks/snapshot_{snapshot_id}_chunk_000.tar.lz4",
            "file_size_bytes": 1073741824,  # 1GB
            "checksum_sha256": "d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5",
            "byte_start": 0,
            "byte_end": 1073741823
        },
        {
            "chunk_index": 1,
            "file_name": f"snapshot_{snapshot_id}_chunk_001.tar.lz4",
            "file_url": f"https://snapshots.omniphi.io/chunks/snapshot_{snapshot_id}_chunk_001.tar.lz4",
            "file_size_bytes": 1073741824,
            "checksum_sha256": "e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6",
            "byte_start": 1073741824,
            "byte_end": 2147483647
        },
        {
            "chunk_index": 2,
            "file_name": f"snapshot_{snapshot_id}_chunk_002.tar.lz4",
            "file_url": f"https://snapshots.omniphi.io/chunks/snapshot_{snapshot_id}_chunk_002.tar.lz4",
            "file_size_bytes": 1073741824,
            "checksum_sha256": "f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1",
            "byte_start": 2147483648,
            "byte_end": 3221225471
        }
    ]


def get_mock_schedules():
    """Generate mock snapshot schedules."""
    now = datetime.utcnow()
    return [
        {
            "id": "550e8400-e29b-41d4-a716-446655440010",
            "chain_id": "omniphi-1",
            "network": "mainnet",
            "snapshot_type": "pruned",
            "schedule_cron": "0 0 * * *",  # Daily at midnight
            "storage_bucket": "omniphi-snapshots-prod",
            "retention_days": 7,
            "is_active": True,
            "last_success_at": (now - timedelta(hours=2)).isoformat()
        },
        {
            "id": "550e8400-e29b-41d4-a716-446655440011",
            "chain_id": "omniphi-1",
            "network": "mainnet",
            "snapshot_type": "archive",
            "schedule_cron": "0 0 * * 0",  # Weekly on Sunday
            "storage_bucket": "omniphi-snapshots-prod",
            "retention_days": 30,
            "is_active": True,
            "last_success_at": (now - timedelta(days=3)).isoformat()
        }
    ]


# ============================================
# ENDPOINTS
# ============================================

@router.get("", response_model=List[SnapshotResponse])
async def list_snapshots(
    chain_id: Optional[str] = Query(None, description="Filter by chain ID"),
    network: str = Query("mainnet", description="Filter by network"),
    snapshot_type: Optional[str] = Query(None, description="Filter by type"),
    latest_only: bool = Query(False, description="Only return latest snapshots"),
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
):
    """
    List available snapshots.

    Returns snapshots sorted by block height (newest first).
    """
    snapshots = get_mock_snapshots()

    if chain_id:
        snapshots = [s for s in snapshots if s["chain_id"] == chain_id]
    if network:
        snapshots = [s for s in snapshots if s["network"] == network]
    if snapshot_type:
        snapshots = [s for s in snapshots if s["snapshot_type"] == snapshot_type]
    if latest_only:
        snapshots = [s for s in snapshots if s["is_latest"]]

    return snapshots[offset:offset + limit]


@router.get("/latest", response_model=SnapshotResponse)
async def get_latest_snapshot(
    chain_id: str = Query(..., description="Chain ID"),
    network: str = Query("mainnet", description="Network"),
    snapshot_type: str = Query("pruned", description="Snapshot type"),
):
    """
    Get the latest snapshot for a chain.

    Returns the most recent available snapshot.
    """
    snapshots = get_mock_snapshots()

    matching = [
        s for s in snapshots
        if s["chain_id"] == chain_id
        and s["network"] == network
        and s["snapshot_type"] == snapshot_type
        and s["status"] == "available"
    ]

    if not matching:
        raise HTTPException(
            status_code=404,
            detail=f"No snapshot found for {chain_id}/{network}/{snapshot_type}"
        )

    # Return the one with highest block height
    return max(matching, key=lambda s: s["block_height"])


@router.get("/{snapshot_id}", response_model=SnapshotResponse)
async def get_snapshot(snapshot_id: str):
    """Get snapshot details."""
    snapshots = get_mock_snapshots()
    snapshot = next((s for s in snapshots if s["id"] == snapshot_id), None)

    if not snapshot:
        raise HTTPException(status_code=404, detail="Snapshot not found")

    return snapshot


@router.get("/{snapshot_id}/chunks", response_model=List[SnapshotChunkResponse])
async def get_snapshot_chunks(snapshot_id: str):
    """
    Get chunks for a chunked snapshot.

    Returns ordered list of chunks for parallel downloading.
    """
    snapshots = get_mock_snapshots()
    snapshot = next((s for s in snapshots if s["id"] == snapshot_id), None)

    if not snapshot:
        raise HTTPException(status_code=404, detail="Snapshot not found")

    if not snapshot["is_chunked"]:
        raise HTTPException(status_code=400, detail="Snapshot is not chunked")

    return get_mock_chunks(snapshot_id)


@router.get("/{snapshot_id}/download")
async def get_download_url(snapshot_id: str):
    """
    Get download URL for a snapshot.

    Returns pre-signed URL(s) for downloading.
    """
    snapshots = get_mock_snapshots()
    snapshot = next((s for s in snapshots if s["id"] == snapshot_id), None)

    if not snapshot:
        raise HTTPException(status_code=404, detail="Snapshot not found")

    if snapshot["status"] != "available":
        raise HTTPException(
            status_code=400,
            detail=f"Snapshot is not available (status: {snapshot['status']})"
        )

    response = {
        "snapshot_id": snapshot["id"],
        "file_name": snapshot["file_name"],
        "file_size_bytes": snapshot["file_size_bytes"],
        "file_size_gb": snapshot["file_size_gb"],
        "checksum_sha256": snapshot["checksum_sha256"],
        "is_chunked": snapshot["is_chunked"],
    }

    if snapshot["is_chunked"]:
        chunks = get_mock_chunks(snapshot_id)
        response["chunk_count"] = len(chunks)
        response["chunks"] = [
            {
                "index": c["chunk_index"],
                "url": c["file_url"],
                "size": c["file_size_bytes"],
                "checksum": c["checksum_sha256"],
            }
            for c in chunks
        ]
    else:
        response["download_url"] = snapshot["file_url"]

    return response


@router.post("/{snapshot_id}/start-download", response_model=DownloadResponse)
async def start_download(
    snapshot_id: str,
    node_id: str = Query(..., description="Node ID"),
):
    """
    Start a snapshot download.

    Creates a download tracking record for a node.
    """
    snapshots = get_mock_snapshots()
    snapshot = next((s for s in snapshots if s["id"] == snapshot_id), None)

    if not snapshot:
        raise HTTPException(status_code=404, detail="Snapshot not found")

    return {
        "id": f"550e8400-e29b-41d4-a716-{uuid4().hex[:12]}",
        "snapshot_id": snapshot_id,
        "node_id": node_id,
        "status": "pending",
        "bytes_downloaded": 0,
        "total_bytes": snapshot["file_size_bytes"],
        "progress_percent": 0.0,
        "chunks_completed": 0,
        "total_chunks": snapshot["chunk_count"] or 1,
        "download_speed_mbps": None,
        "estimated_remaining_seconds": None,
        "started_at": datetime.utcnow().isoformat()
    }


@router.put("/{snapshot_id}/downloads/{download_id}/progress")
async def update_download_progress(
    snapshot_id: str,
    download_id: str,
    bytes_downloaded: int = Query(...),
    chunks_completed: int = Query(0),
    download_speed_mbps: Optional[float] = Query(None),
):
    """Update download progress."""
    snapshots = get_mock_snapshots()
    snapshot = next((s for s in snapshots if s["id"] == snapshot_id), None)

    if not snapshot:
        raise HTTPException(status_code=404, detail="Snapshot not found")

    total_bytes = snapshot["file_size_bytes"]
    progress_percent = (bytes_downloaded / total_bytes) * 100 if total_bytes > 0 else 0

    return {"status": "updated", "progress_percent": progress_percent}


@router.post("/{snapshot_id}/downloads/{download_id}/complete")
async def complete_download(
    snapshot_id: str,
    download_id: str,
    checksum_verified: bool = Query(...),
):
    """Mark download as complete."""
    return {"status": "completed", "checksum_verified": checksum_verified}


@router.post("/generate")
async def generate_snapshot(
    request: GenerateSnapshotRequest,
    background_tasks: BackgroundTasks,
):
    """
    Trigger snapshot generation.

    Creates a new snapshot from the current chain state.
    """
    valid_types = ["pruned", "archive", "full"]
    if request.snapshot_type not in valid_types:
        raise HTTPException(
            status_code=400,
            detail=f"Invalid snapshot type. Must be one of: {valid_types}"
        )

    generation_id = str(uuid4())

    return {
        "status": "scheduled",
        "generation_id": generation_id,
        "chain_id": request.chain_id,
        "snapshot_type": request.snapshot_type,
    }


@router.get("/schedules", response_model=List[ScheduleResponse])
async def list_schedules(
    chain_id: Optional[str] = Query(None),
    active_only: bool = Query(True),
):
    """List snapshot schedules."""
    schedules = get_mock_schedules()

    if chain_id:
        schedules = [s for s in schedules if s["chain_id"] == chain_id]
    if active_only:
        schedules = [s for s in schedules if s["is_active"]]

    return schedules


@router.post("/schedules", response_model=ScheduleResponse)
async def create_schedule(request: CreateScheduleRequest):
    """Create a new snapshot schedule."""
    valid_types = ["pruned", "archive", "full"]
    if request.snapshot_type not in valid_types:
        raise HTTPException(
            status_code=400,
            detail=f"Invalid snapshot type. Must be one of: {valid_types}"
        )

    return {
        "id": f"550e8400-e29b-41d4-a716-{uuid4().hex[:12]}",
        "chain_id": request.chain_id,
        "network": request.network,
        "snapshot_type": request.snapshot_type,
        "schedule_cron": request.schedule_cron,
        "storage_bucket": request.storage_bucket,
        "retention_days": request.retention_days,
        "is_active": True,
        "last_success_at": None,
    }


@router.delete("/schedules/{schedule_id}")
async def delete_schedule(schedule_id: str):
    """Deactivate a snapshot schedule."""
    schedules = get_mock_schedules()
    schedule = next((s for s in schedules if s["id"] == schedule_id), None)

    if not schedule:
        raise HTTPException(status_code=404, detail="Schedule not found")

    return {"status": "deactivated", "schedule_id": schedule_id}
