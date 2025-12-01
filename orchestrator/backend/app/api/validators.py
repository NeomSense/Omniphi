"""Validator API endpoints."""

from datetime import datetime
from typing import List
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import ValidatorSetupRequest, ValidatorNode, LocalValidatorHeartbeat
from app.models.validator_setup_request import SetupStatus, RunMode
from app.schemas.validator import (
    ValidatorSetupRequestCreate,
    ValidatorSetupRequestResponse,
    ValidatorCompleteInfo,
    ValidatorNodeResponse,
    LocalValidatorHeartbeatCreate,
    LocalValidatorHeartbeatResponse,
    ValidatorStopRequest,
    ValidatorRedeployRequest
)
from app.services.chain_client import chain_client
from app.workers.provisioner import provision_cloud_validator

router = APIRouter()


# ==================== Setup Requests ====================

@router.post("/setup-requests", response_model=ValidatorSetupRequestResponse, status_code=201)
async def create_setup_request(
    request: ValidatorSetupRequestCreate,
    background_tasks: BackgroundTasks,
    db: Session = Depends(get_db)
):
    """
    Create a new validator setup request.

    For cloud mode, this triggers automatic provisioning.
    For local mode, waits for desktop app to provide consensus pubkey.
    """
    # Create database record
    db_request = ValidatorSetupRequest(
        wallet_address=request.wallet_address,
        validator_name=request.validator_name,
        website=request.website,
        description=request.description,
        commission_rate=request.commission_rate,
        run_mode=RunMode(request.run_mode),
        provider=request.provider,
        status=SetupStatus.PENDING
    )

    db.add(db_request)
    db.commit()
    db.refresh(db_request)

    # If cloud mode, trigger provisioning in background
    if request.run_mode == "cloud":
        background_tasks.add_task(provision_cloud_validator, db_request.id)

    return db_request


@router.get("/setup-requests/{request_id}", response_model=ValidatorSetupRequestResponse)
async def get_setup_request(request_id: UUID, db: Session = Depends(get_db)):
    """Get setup request status and details."""
    db_request = db.query(ValidatorSetupRequest).filter(ValidatorSetupRequest.id == request_id).first()

    if not db_request:
        raise HTTPException(status_code=404, detail="Setup request not found")

    return db_request


@router.get("/by-wallet/{wallet_address}", response_model=List[ValidatorCompleteInfo])
async def get_validators_by_wallet(wallet_address: str, db: Session = Depends(get_db)):
    """
    Get all validators associated with a wallet address.

    Returns combined information from:
    - Setup requests
    - Cloud nodes
    - Chain validator status
    - Local heartbeats
    """
    # Get all setup requests for this wallet
    requests = db.query(ValidatorSetupRequest).filter(
        ValidatorSetupRequest.wallet_address == wallet_address
    ).all()

    results = []

    for req in requests:
        # Get associated node
        node = db.query(ValidatorNode).filter(
            ValidatorNode.setup_request_id == req.id
        ).first()

        # Get chain info if consensus pubkey available
        chain_info = None
        if req.consensus_pubkey:
            chain_info = await chain_client.get_validator_by_consensus_pubkey(req.consensus_pubkey)

        # Get heartbeat if local mode
        heartbeat = None
        if req.run_mode == RunMode.LOCAL and req.consensus_pubkey:
            heartbeat = db.query(LocalValidatorHeartbeat).filter(
                LocalValidatorHeartbeat.consensus_pubkey == req.consensus_pubkey
            ).first()

        results.append({
            "setup_request": req,
            "node": node,
            "chain_info": chain_info,
            "heartbeat": heartbeat
        })

    return results


# ==================== Node Control ====================

@router.post("/stop")
async def stop_validator(request: ValidatorStopRequest, db: Session = Depends(get_db)):
    """Gracefully stop a validator node."""
    setup_request = db.query(ValidatorSetupRequest).filter(
        ValidatorSetupRequest.id == request.setup_request_id
    ).first()

    if not setup_request:
        raise HTTPException(status_code=404, detail="Setup request not found")

    if setup_request.run_mode != RunMode.CLOUD:
        raise HTTPException(status_code=400, detail="Can only stop cloud validators via API")

    # Get node
    node = db.query(ValidatorNode).filter(
        ValidatorNode.setup_request_id == setup_request.id
    ).first()

    if not node:
        raise HTTPException(status_code=404, detail="Node not found")

    # Import docker service (will be created in Phase 2)
    from app.services.docker_manager import docker_manager

    # Stop container
    success = await docker_manager.stop_container(node.node_internal_id)

    if success:
        from app.models.validator_node import NodeStatus
        node.status = NodeStatus.STOPPED
        db.commit()
        return {"message": "Validator stopped successfully"}
    else:
        raise HTTPException(status_code=500, detail="Failed to stop validator")


@router.post("/redeploy")
async def redeploy_validator(
    request: ValidatorRedeployRequest,
    background_tasks: BackgroundTasks,
    db: Session = Depends(get_db)
):
    """Redeploy a validator node (stop and restart with latest config)."""
    setup_request = db.query(ValidatorSetupRequest).filter(
        ValidatorSetupRequest.id == request.setup_request_id
    ).first()

    if not setup_request:
        raise HTTPException(status_code=404, detail="Setup request not found")

    if setup_request.run_mode != RunMode.CLOUD:
        raise HTTPException(status_code=400, detail="Can only redeploy cloud validators via API")

    # Trigger redeployment in background
    background_tasks.add_task(provision_cloud_validator, setup_request.id, redeploy=True)

    return {"message": "Redeployment initiated"}


# ==================== Local Validator Heartbeat ====================

@router.post("/heartbeat", response_model=LocalValidatorHeartbeatResponse)
async def submit_heartbeat(heartbeat: LocalValidatorHeartbeatCreate, db: Session = Depends(get_db)):
    """
    Submit heartbeat from local validator desktop app.

    This endpoint is called by the desktop app to report status.
    """
    # Check if heartbeat already exists
    existing = db.query(LocalValidatorHeartbeat).filter(
        LocalValidatorHeartbeat.consensus_pubkey == heartbeat.consensus_pubkey
    ).first()

    if existing:
        # Update existing
        existing.block_height = heartbeat.block_height
        existing.uptime_seconds = heartbeat.uptime_seconds
        existing.local_rpc_port = heartbeat.local_rpc_port
        existing.local_p2p_port = heartbeat.local_p2p_port
        existing.last_seen = datetime.utcnow()
        db.commit()
        db.refresh(existing)
        return existing
    else:
        # Create new
        new_heartbeat = LocalValidatorHeartbeat(
            wallet_address=heartbeat.wallet_address,
            consensus_pubkey=heartbeat.consensus_pubkey,
            block_height=heartbeat.block_height,
            uptime_seconds=heartbeat.uptime_seconds,
            local_rpc_port=heartbeat.local_rpc_port,
            local_p2p_port=heartbeat.local_p2p_port
        )
        db.add(new_heartbeat)
        db.commit()
        db.refresh(new_heartbeat)
        return new_heartbeat


@router.get("/heartbeat/{consensus_pubkey}", response_model=LocalValidatorHeartbeatResponse)
async def get_heartbeat(consensus_pubkey: str, db: Session = Depends(get_db)):
    """Get latest heartbeat for a local validator."""
    heartbeat = db.query(LocalValidatorHeartbeat).filter(
        LocalValidatorHeartbeat.consensus_pubkey == consensus_pubkey
    ).first()

    if not heartbeat:
        raise HTTPException(status_code=404, detail="Heartbeat not found")

    return heartbeat
