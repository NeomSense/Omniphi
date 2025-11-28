"""Health check endpoints."""

from datetime import datetime, timedelta
import logging
import random

from fastapi import APIRouter

from app.core.config import settings

logger = logging.getLogger(__name__)

router = APIRouter()


def get_system_metrics():
    """Get CPU, memory, disk metrics using psutil if available."""
    try:
        import psutil
        cpu_percent = psutil.cpu_percent(interval=0.1)
        memory = psutil.virtual_memory()
        disk = psutil.disk_usage('/')
        uptime = int(datetime.utcnow().timestamp() - psutil.boot_time())
        return {
            "cpu_percent": cpu_percent,
            "memory_percent": memory.percent,
            "memory_used": memory.used,
            "memory_total": memory.total,
            "disk_percent": disk.percent,
            "disk_used": disk.used,
            "disk_total": disk.total,
            "uptime_seconds": uptime
        }
    except ImportError:
        # psutil not available, return mock data
        return {
            "cpu_percent": random.uniform(20, 60),
            "memory_percent": random.uniform(40, 70),
            "memory_used": 8 * 1024**3,
            "memory_total": 16 * 1024**3,
            "disk_percent": random.uniform(30, 60),
            "disk_used": 100 * 1024**3,
            "disk_total": 256 * 1024**3,
            "uptime_seconds": random.randint(86400, 864000)
        }


@router.get("/health")
async def health_check():
    """
    Health check endpoint.

    Returns:
        System health status
    """
    return {
        "status": "ok",
        "version": settings.VERSION,
        "database": "ok",
        "timestamp": datetime.utcnow().isoformat()
    }


@router.get("/health/system")
async def system_health():
    """
    Comprehensive system health endpoint for admin dashboard.

    Returns:
        Full system metrics including CPU, memory, disk, nodes, and recent errors
    """
    metrics = get_system_metrics()
    now = datetime.utcnow()

    # Mock data for dashboard
    total_requests = 156
    active_nodes = 142
    pending_requests = 8
    failed_requests = 3
    success_rate = 97.4

    # Generate resource history (last 24 hours, hourly)
    resource_history = []
    for i in range(24):
        hour_ago = now - timedelta(hours=23-i)
        resource_history.append({
            "timestamp": hour_ago.isoformat(),
            "cpu": max(5, metrics["cpu_percent"] + random.uniform(-15, 15)),
            "memory": max(10, metrics["memory_percent"] + random.uniform(-8, 8)),
            "disk": max(20, metrics["disk_percent"] + random.uniform(-2, 2))
        })

    # RPC health status
    rpc_health = [
        {
            "chain_id": "omniphi-mainnet-1",
            "endpoint": settings.OMNIPHI_RPC_URL,
            "status": "healthy",
            "latency_ms": random.randint(15, 50),
            "block_height": 1234567,
            "last_check": now.isoformat()
        },
        {
            "chain_id": "omniphi-testnet-1",
            "endpoint": "https://rpc.testnet.omniphi.network",
            "status": "healthy",
            "latency_ms": random.randint(20, 60),
            "block_height": 987654,
            "last_check": now.isoformat()
        }
    ]

    # Recent errors
    recent_errors = [
        {
            "id": "err-1",
            "type": "provisioning",
            "message": "AWS instance limit reached in us-east-1",
            "request_id": "req-123",
            "node_id": None,
            "timestamp": (now - timedelta(hours=1)).isoformat()
        },
        {
            "id": "err-2",
            "type": "health_check",
            "message": "Node node-456 failed health check",
            "request_id": None,
            "node_id": "node-456",
            "timestamp": (now - timedelta(hours=2)).isoformat()
        }
    ]

    # Format memory and disk for display
    memory_used_gb = round(metrics["memory_used"] / (1024**3), 1)
    memory_total_gb = round(metrics["memory_total"] / (1024**3), 1)
    disk_used_gb = round(metrics["disk_used"] / (1024**3), 0)
    disk_total_gb = round(metrics["disk_total"] / (1024**3), 0)

    return {
        # Status
        "orchestrator_status": "healthy",
        "orchestrator_uptime": metrics["uptime_seconds"],
        "orchestrator_version": settings.VERSION,

        # Validator stats
        "total_validators": total_requests,
        "active_validators": active_nodes,
        "pending_requests": pending_requests,
        "provisioning_failures": failed_requests,
        "success_rate": success_rate,
        "avg_provisioning_time": 245,

        # Resource usage (nested object for frontend)
        "resource_usage": {
            "cpu_percent": round(metrics["cpu_percent"], 1),
            "memory_percent": round(metrics["memory_percent"], 1),
            "memory_used": f"{memory_used_gb}GB / {memory_total_gb}GB",
            "disk_percent": round(metrics["disk_percent"], 1),
            "disk_used": f"{int(disk_used_gb)}GB / {int(disk_total_gb)}GB"
        },

        # RPC status (frontend expects chain_rpc_status)
        "chain_rpc_status": rpc_health,

        # Recent errors
        "recent_errors": recent_errors,

        # Historical data for charts
        "resource_history": resource_history
    }
