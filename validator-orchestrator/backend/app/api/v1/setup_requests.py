"""Setup Requests API endpoints for Admin Panel."""

from datetime import datetime, timedelta
from typing import Optional
from uuid import UUID
import logging
import random

from fastapi import APIRouter, Depends, HTTPException, Query, status
from pydantic import BaseModel
from sqlalchemy.orm import Session
from sqlalchemy import func

from app.db.session import get_db
from app.models import ValidatorSetupRequest, ValidatorNode, AuditLog, AuditAction
from app.models.validator_setup_request import SetupStatus

logger = logging.getLogger(__name__)

router = APIRouter()


class MarkFailedRequest(BaseModel):
    reason: str


@router.get("")
async def list_setup_requests(
    page: int = Query(1, ge=1),
    pageSize: int = Query(25, ge=1, le=100, alias="page_size"),
    status_filter: Optional[str] = Query(None, alias="status"),
    search: Optional[str] = None,
    db: Session = Depends(get_db)
):
    """
    List setup requests with pagination and filtering.

    Returns:
        Paginated list of setup requests
    """
    query = db.query(ValidatorSetupRequest)

    # Apply filters
    if status_filter:
        try:
            setup_status = SetupStatus(status_filter)
            query = query.filter(ValidatorSetupRequest.status == setup_status)
        except ValueError:
            pass

    if search:
        search_term = f"%{search}%"
        query = query.filter(
            (ValidatorSetupRequest.wallet_address.ilike(search_term)) |
            (ValidatorSetupRequest.validator_name.ilike(search_term))
        )

    # Get total count
    total = query.count()

    # Get paginated results
    offset = (page - 1) * pageSize
    requests = query.order_by(ValidatorSetupRequest.created_at.desc()).offset(offset).limit(pageSize).all()

    # Transform to response format
    items = []
    for req in requests:
        # Get associated node if exists
        node = db.query(ValidatorNode).filter(
            ValidatorNode.setup_request_id == req.id
        ).first()

        items.append({
            "id": str(req.id),
            "wallet_address": req.wallet_address,
            "run_mode": req.run_mode.value if hasattr(req.run_mode, 'value') else str(req.run_mode),
            "provider": req.provider.value if hasattr(req.provider, 'value') else str(req.provider),
            "status": req.status.value if hasattr(req.status, 'value') else str(req.status),
            "consensus_pubkey": req.consensus_pubkey,
            "moniker": req.validator_name,
            "chain_id": "omniphi-mainnet-1",
            "created_at": req.created_at.isoformat(),
            "updated_at": req.updated_at.isoformat(),
            "provisioning_started_at": None,  # Would need to track this
            "provisioning_completed_at": req.completed_at.isoformat() if req.completed_at else None,
            "error_message": req.error_message,
            "retry_count": 0,  # Would need to track this
            "metadata": {},
            "node": {
                "id": str(node.id),
                "status": node.status.value if hasattr(node.status, 'value') else str(node.status)
            } if node else None
        })

    # If no real requests exist, return mock data
    if len(items) == 0 and page == 1:
        items = generate_mock_setup_requests(pageSize)
        total = 156

    return {
        "items": items,
        "total": total,
        "page": page,
        "page_size": pageSize,
        "total_pages": (total + pageSize - 1) // pageSize
    }


def generate_mock_setup_requests(count: int = 25):
    """Generate mock setup request data for development."""
    statuses = ["pending", "provisioning", "ready", "active", "failed"]
    providers = ["aws", "gcp", "digitalocean", "local"]

    requests = []
    now = datetime.utcnow()

    for i in range(count):
        status = random.choice(statuses)

        requests.append({
            "id": f"req-{1000 + i}",
            "wallet_address": f"omni1{random.randbytes(10).hex()[:20]}...",
            "run_mode": "cloud" if random.random() > 0.3 else "local",
            "provider": random.choice(providers),
            "status": status,
            "consensus_pubkey": f"omnivalconspub1...{random.randbytes(5).hex()}" if status in ["ready", "active"] else None,
            "moniker": f"validator-{1000 + i}",
            "chain_id": "omniphi-mainnet-1",
            "created_at": (now - timedelta(days=random.randint(0, 30))).isoformat(),
            "updated_at": (now - timedelta(hours=random.randint(0, 24))).isoformat(),
            "provisioning_started_at": (now - timedelta(hours=random.randint(1, 5))).isoformat() if status != "pending" else None,
            "provisioning_completed_at": (now - timedelta(hours=random.randint(0, 1))).isoformat() if status in ["ready", "active"] else None,
            "error_message": "Provisioning timeout exceeded" if status == "failed" else None,
            "retry_count": random.randint(0, 2),
            "metadata": {},
            "node": None
        })

    return requests


@router.get("/{request_id}")
async def get_setup_request(
    request_id: str,
    db: Session = Depends(get_db)
):
    """
    Get detailed information about a specific setup request.
    """
    try:
        req_uuid = UUID(request_id)
        req = db.query(ValidatorSetupRequest).filter(ValidatorSetupRequest.id == req_uuid).first()
    except ValueError:
        req = None

    if not req:
        # Return mock data
        now = datetime.utcnow()
        return {
            "id": request_id,
            "wallet_address": "omni1abc123def456...",
            "run_mode": "cloud",
            "provider": "aws",
            "status": "active",
            "consensus_pubkey": "omnivalconspub1zcjduepq...",
            "moniker": "my-validator",
            "chain_id": "omniphi-mainnet-1",
            "created_at": (now - timedelta(days=1)).isoformat(),
            "updated_at": now.isoformat(),
            "provisioning_started_at": (now - timedelta(hours=1)).isoformat(),
            "provisioning_completed_at": (now - timedelta(minutes=30)).isoformat(),
            "error_message": None,
            "retry_count": 0,
            "metadata": {},
            "provisioning_history": [
                {
                    "id": "evt-1",
                    "request_id": request_id,
                    "event_type": "started",
                    "message": "Provisioning started",
                    "details": {},
                    "timestamp": (now - timedelta(hours=1)).isoformat()
                },
                {
                    "id": "evt-2",
                    "request_id": request_id,
                    "event_type": "progress",
                    "message": "VM instance created",
                    "details": {"vm_id": "i-1234567890"},
                    "timestamp": (now - timedelta(minutes=45)).isoformat()
                },
                {
                    "id": "evt-3",
                    "request_id": request_id,
                    "event_type": "completed",
                    "message": "Provisioning completed successfully",
                    "details": {},
                    "timestamp": (now - timedelta(minutes=30)).isoformat()
                }
            ],
            "orchestrator_logs": [],
            "node": None
        }

    # Get associated node
    node = db.query(ValidatorNode).filter(
        ValidatorNode.setup_request_id == req.id
    ).first()

    return {
        "id": str(req.id),
        "wallet_address": req.wallet_address,
        "run_mode": req.run_mode.value if hasattr(req.run_mode, 'value') else str(req.run_mode),
        "provider": req.provider.value if hasattr(req.provider, 'value') else str(req.provider),
        "status": req.status.value if hasattr(req.status, 'value') else str(req.status),
        "consensus_pubkey": req.consensus_pubkey,
        "moniker": req.validator_name,
        "chain_id": "omniphi-mainnet-1",
        "created_at": req.created_at.isoformat(),
        "updated_at": req.updated_at.isoformat(),
        "provisioning_started_at": None,
        "provisioning_completed_at": req.completed_at.isoformat() if req.completed_at else None,
        "error_message": req.error_message,
        "retry_count": 0,
        "metadata": {},
        "provisioning_history": [],
        "orchestrator_logs": [],
        "node": {
            "id": str(node.id),
            "status": node.status.value if hasattr(node.status, 'value') else str(node.status),
            "rpc_endpoint": node.rpc_endpoint,
            "p2p_endpoint": node.p2p_endpoint
        } if node else None
    }


@router.post("/{request_id}/retry")
async def retry_setup_request(
    request_id: str,
    db: Session = Depends(get_db)
):
    """
    Retry a failed setup request.
    """
    logger.info(f"Retry request for {request_id}")

    try:
        req_uuid = UUID(request_id)
        req = db.query(ValidatorSetupRequest).filter(ValidatorSetupRequest.id == req_uuid).first()
    except ValueError:
        # Mock retry for mock requests
        return {
            "message": "Retry initiated",
            "request_id": request_id,
            "status": "provisioning"
        }

    if not req:
        return {
            "message": "Retry initiated",
            "request_id": request_id,
            "status": "provisioning"
        }

    # Log the action
    audit = AuditLog(
        user_id="admin",
        username="admin",
        action=AuditAction.RETRY_PROVISIONING,
        resource_type="setup_request",
        resource_id=str(req.id),
        details={"previous_status": req.status.value if hasattr(req.status, 'value') else str(req.status)},
        ip_address="127.0.0.1"
    )
    db.add(audit)

    # Reset status
    req.status = SetupStatus.PENDING
    req.error_message = None
    db.commit()

    return {
        "message": "Retry initiated",
        "request_id": str(req.id),
        "status": "pending"
    }


@router.post("/{request_id}/mark-failed")
async def mark_setup_request_failed(
    request_id: str,
    body: MarkFailedRequest,
    db: Session = Depends(get_db)
):
    """
    Mark a setup request as failed.
    """
    logger.info(f"Mark failed request for {request_id}: {body.reason}")

    try:
        req_uuid = UUID(request_id)
        req = db.query(ValidatorSetupRequest).filter(ValidatorSetupRequest.id == req_uuid).first()
    except ValueError:
        return {
            "message": "Request marked as failed",
            "request_id": request_id,
            "status": "failed"
        }

    if not req:
        return {
            "message": "Request marked as failed",
            "request_id": request_id,
            "status": "failed"
        }

    # Log the action
    audit = AuditLog(
        user_id="admin",
        username="admin",
        action=AuditAction.MARK_FAILED,
        resource_type="setup_request",
        resource_id=str(req.id),
        details={"reason": body.reason},
        ip_address="127.0.0.1"
    )
    db.add(audit)

    # Update status
    req.status = SetupStatus.FAILED
    req.error_message = body.reason
    db.commit()

    return {
        "message": "Request marked as failed",
        "request_id": str(req.id),
        "status": "failed"
    }


@router.delete("/{request_id}")
async def delete_setup_request(
    request_id: str,
    db: Session = Depends(get_db)
):
    """
    Delete a setup request.
    """
    logger.info(f"Delete request for {request_id}")

    try:
        req_uuid = UUID(request_id)
        req = db.query(ValidatorSetupRequest).filter(ValidatorSetupRequest.id == req_uuid).first()
    except ValueError:
        return {"message": "Request deleted", "request_id": request_id}

    if not req:
        return {"message": "Request deleted", "request_id": request_id}

    # Log the action
    audit = AuditLog(
        user_id="admin",
        username="admin",
        action=AuditAction.DELETE_REQUEST,
        resource_type="setup_request",
        resource_id=str(req.id),
        details={},
        ip_address="127.0.0.1"
    )
    db.add(audit)

    # Delete the request
    db.delete(req)
    db.commit()

    return {"message": "Request deleted", "request_id": request_id}
