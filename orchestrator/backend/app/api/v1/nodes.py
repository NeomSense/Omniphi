"""Validator Nodes API endpoints."""

from datetime import datetime, timedelta
from typing import Optional
from uuid import UUID
import logging
import random

from fastapi import APIRouter, Depends, HTTPException, Query, status
from sqlalchemy.orm import Session
from sqlalchemy import func

from app.db.session import get_db
from app.models import ValidatorNode, ValidatorSetupRequest, AuditLog, AuditAction
from app.models.validator_node import NodeStatus

logger = logging.getLogger(__name__)

router = APIRouter()


@router.get("")
async def list_nodes(
    page: int = Query(1, ge=1),
    pageSize: int = Query(12, ge=1, le=100),
    status_filter: Optional[str] = Query(None, alias="status"),
    provider: Optional[str] = None,
    db: Session = Depends(get_db)
):
    """
    List validator nodes with pagination and filtering.

    Returns:
        Paginated list of validator nodes with metrics
    """
    query = db.query(ValidatorNode)

    # Apply filters
    if status_filter:
        try:
            node_status = NodeStatus(status_filter)
            query = query.filter(ValidatorNode.status == node_status)
        except ValueError:
            pass  # Invalid status, ignore filter

    if provider:
        query = query.filter(ValidatorNode.provider == provider)

    # Get total count
    total = query.count()

    # Get paginated results
    offset = (page - 1) * pageSize
    nodes = query.order_by(ValidatorNode.created_at.desc()).offset(offset).limit(pageSize).all()

    # Transform to response format
    items = []
    for node in nodes:
        # Get associated setup request
        setup_request = db.query(ValidatorSetupRequest).filter(
            ValidatorSetupRequest.id == node.setup_request_id
        ).first()

        # Generate mock metrics (in production, these would come from actual monitoring)
        items.append({
            "id": str(node.id),
            "provider": node.provider,
            "status": node.status.value if hasattr(node.status, 'value') else str(node.status),
            "rpc_endpoint": node.rpc_endpoint or f"http://node-{str(node.id)[:8]}:26657",
            "p2p_endpoint": node.p2p_endpoint or f"tcp://node-{str(node.id)[:8]}:26656",
            "metrics_endpoint": f"http://node-{str(node.id)[:8]}:26660/metrics",
            "block_height": int(node.last_block_height) if node.last_block_height else random.randint(1000000, 2000000),
            "peers": random.randint(20, 50),
            "syncing": False,
            "uptime": random.randint(86400, 864000),  # 1-10 days in seconds
            "cpu_percent": round(random.uniform(10, 60), 1),
            "ram_percent": round(random.uniform(30, 70), 1),
            "ram_used": f"{random.randint(2, 8)}GB",
            "disk_percent": round(random.uniform(20, 50), 1),
            "disk_used": f"{random.randint(50, 150)}GB",
            "last_health_check": (node.last_health_check or datetime.utcnow()).isoformat(),
            "created_at": node.created_at.isoformat(),
            "validator_name": setup_request.validator_name if setup_request else "Unknown"
        })

    # If no real nodes exist, return mock data
    if len(items) == 0 and page == 1:
        items = generate_mock_nodes(pageSize)
        total = 24  # Mock total

    return {
        "items": items,
        "total": total,
        "page": page,
        "pageSize": pageSize,
        "totalPages": (total + pageSize - 1) // pageSize
    }


def generate_mock_nodes(count: int = 12):
    """Generate mock node data for development."""
    providers = ["aws", "gcp", "digitalocean", "local"]
    statuses = ["running", "running", "running", "stopped", "error"]

    nodes = []
    for i in range(count):
        node_id = f"node-{i+1:03d}"
        status = random.choice(statuses)

        nodes.append({
            "id": node_id,
            "provider": random.choice(providers),
            "status": status,
            "rpc_endpoint": f"http://{node_id}.omniphi.network:26657",
            "p2p_endpoint": f"tcp://{node_id}.omniphi.network:26656",
            "metrics_endpoint": f"http://{node_id}.omniphi.network:26660/metrics",
            "block_height": random.randint(1500000, 1600000),
            "peers": random.randint(20, 50),
            "syncing": status == "running" and random.random() < 0.1,
            "uptime": random.randint(86400, 864000),
            "cpu_percent": round(random.uniform(10, 60), 1),
            "ram_percent": round(random.uniform(30, 70), 1),
            "ram_used": f"{random.randint(2, 8)}GB",
            "disk_percent": round(random.uniform(20, 50), 1),
            "disk_used": f"{random.randint(50, 150)}GB",
            "last_health_check": (datetime.utcnow() - timedelta(minutes=random.randint(1, 5))).isoformat(),
            "created_at": (datetime.utcnow() - timedelta(days=random.randint(1, 30))).isoformat(),
            "validator_name": f"Validator {i+1}"
        })

    return nodes


@router.post("/{node_id}/restart")
async def restart_node(
    node_id: str,
    db: Session = Depends(get_db)
):
    """
    Restart a validator node.
    """
    logger.info(f"Restart request for node {node_id}")

    # Try to find the node
    try:
        node_uuid = UUID(node_id)
        node = db.query(ValidatorNode).filter(ValidatorNode.id == node_uuid).first()
    except ValueError:
        # Mock restart for mock nodes
        logger.info(f"Mock restart for node {node_id}")
        return {
            "message": "Node restart initiated",
            "node_id": node_id,
            "status": "starting"
        }

    if not node:
        # Allow mock restart
        return {
            "message": "Node restart initiated",
            "node_id": node_id,
            "status": "starting"
        }

    # Log the action
    audit = AuditLog(
        user_id="admin",
        username="admin",
        action=AuditAction.RESTART_NODE,
        resource_type="node",
        resource_id=str(node.id),
        details={"previous_status": node.status.value if hasattr(node.status, 'value') else str(node.status)},
        ip_address="127.0.0.1"
    )
    db.add(audit)

    # Update node status
    node.status = NodeStatus.STARTING
    db.commit()

    return {
        "message": "Node restart initiated",
        "node_id": str(node.id),
        "status": "starting"
    }


@router.post("/{node_id}/stop")
async def stop_node(
    node_id: str,
    db: Session = Depends(get_db)
):
    """
    Stop a validator node.
    """
    logger.info(f"Stop request for node {node_id}")

    # Try to find the node
    try:
        node_uuid = UUID(node_id)
        node = db.query(ValidatorNode).filter(ValidatorNode.id == node_uuid).first()
    except ValueError:
        # Mock stop for mock nodes
        logger.info(f"Mock stop for node {node_id}")
        return {
            "message": "Node stop initiated",
            "node_id": node_id,
            "status": "stopping"
        }

    if not node:
        # Allow mock stop
        return {
            "message": "Node stop initiated",
            "node_id": node_id,
            "status": "stopping"
        }

    # Log the action
    audit = AuditLog(
        user_id="admin",
        username="admin",
        action=AuditAction.STOP_NODE,
        resource_type="node",
        resource_id=str(node.id),
        details={"previous_status": node.status.value if hasattr(node.status, 'value') else str(node.status)},
        ip_address="127.0.0.1"
    )
    db.add(audit)

    # Update node status
    node.status = NodeStatus.STOPPED
    db.commit()

    return {
        "message": "Node stop initiated",
        "node_id": str(node.id),
        "status": "stopped"
    }


@router.get("/{node_id}")
async def get_node(
    node_id: str,
    db: Session = Depends(get_db)
):
    """
    Get detailed information about a specific node.
    """
    try:
        node_uuid = UUID(node_id)
        node = db.query(ValidatorNode).filter(ValidatorNode.id == node_uuid).first()
    except ValueError:
        node = None

    if not node:
        # Return mock data
        return {
            "id": node_id,
            "provider": "aws",
            "status": "running",
            "rpc_endpoint": f"http://{node_id}.omniphi.network:26657",
            "p2p_endpoint": f"tcp://{node_id}.omniphi.network:26656",
            "metrics_endpoint": f"http://{node_id}.omniphi.network:26660/metrics",
            "block_height": random.randint(1500000, 1600000),
            "peers": random.randint(20, 50),
            "syncing": False,
            "uptime": random.randint(86400, 864000),
            "cpu_percent": round(random.uniform(10, 60), 1),
            "ram_percent": round(random.uniform(30, 70), 1),
            "disk_percent": round(random.uniform(20, 50), 1),
            "last_health_check": datetime.utcnow().isoformat(),
            "created_at": (datetime.utcnow() - timedelta(days=7)).isoformat()
        }

    return {
        "id": str(node.id),
        "provider": node.provider,
        "status": node.status.value if hasattr(node.status, 'value') else str(node.status),
        "rpc_endpoint": node.rpc_endpoint,
        "p2p_endpoint": node.p2p_endpoint,
        "grpc_endpoint": node.grpc_endpoint,
        "logs_url": node.logs_url,
        "block_height": int(node.last_block_height) if node.last_block_height else 0,
        "cpu_cores": node.cpu_cores,
        "memory_gb": node.memory_gb,
        "disk_gb": node.disk_gb,
        "last_health_check": node.last_health_check.isoformat() if node.last_health_check else None,
        "created_at": node.created_at.isoformat(),
        "updated_at": node.updated_at.isoformat()
    }
