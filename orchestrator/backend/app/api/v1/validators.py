"""Validator API endpoints - v1."""

from datetime import datetime
from typing import List, Optional
from uuid import UUID
import logging

from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks, status
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import ValidatorSetupRequest, ValidatorNode, LocalValidatorHeartbeat
from app.models.validator_setup_request import SetupStatus, RunMode
from app.schemas.validators import (
    ValidatorSetupRequestCreate,
    ValidatorSetupRequestResponse,
    ValidatorByWalletResponse,
    ValidatorStopRequest,
    ValidatorRedeployRequest,
    LocalValidatorHeartbeatCreate,
    LocalValidatorHeartbeatResponse
)
from app.services.provisioning import provision_cloud_validator
from app.services.chain_client import chain_client

logger = logging.getLogger(__name__)

router = APIRouter()


# ==================== Setup Requests ====================

@router.post(
    "/setup-requests",
    response_model=ValidatorSetupRequestResponse,
    status_code=status.HTTP_201_CREATED,
    summary="Create validator setup request",
    description="Create a new validator setup request. For cloud mode, triggers automatic provisioning."
)
async def create_setup_request(
    request: ValidatorSetupRequestCreate,
    background_tasks: BackgroundTasks,
    db: Session = Depends(get_db)
):
    """
    Create a new validator setup request.

    **Flow:**
    1. Creates database record with status='pending'
    2. If runMode='cloud', triggers background provisioning job
    3. Returns setup request ID for polling

    **Cloud Mode:**
    - Provisions Docker container
    - Generates consensus keypair inside container
    - Returns consensus pubkey when ready

    **Local Mode:**
    - Waits for desktop app to submit consensus pubkey
    - Desktop app calls heartbeat endpoint
    """
    logger.info(f"Creating setup request for wallet {request.walletAddress}")

    # Create database record
    db_request = ValidatorSetupRequest(
        wallet_address=request.walletAddress,
        validator_name=request.validatorName,
        website=request.website,
        description=request.description,
        commission_rate=request.commissionRate,
        run_mode=RunMode(request.runMode),
        provider=request.provider,
        status=SetupStatus.PENDING
    )

    db.add(db_request)
    db.commit()
    db.refresh(db_request)

    logger.info(f"Created setup request {db_request.id} with status {db_request.status}")

    # If cloud mode, trigger provisioning in background
    if request.runMode == "cloud":
        logger.info(f"Triggering cloud provisioning for request {db_request.id}")
        background_tasks.add_task(provision_cloud_validator, db_request.id)

    return {
        "setupRequest": {
            "id": str(db_request.id),
            "status": db_request.status.value,
            "walletAddress": db_request.wallet_address,
            "validatorName": db_request.validator_name,
            "runMode": db_request.run_mode.value,
            "consensusPubkey": db_request.consensus_pubkey,
            "createdAt": db_request.created_at.isoformat(),
            "updatedAt": db_request.updated_at.isoformat()
        }
    }


@router.get(
    "/setup-requests/{request_id}",
    response_model=ValidatorSetupRequestResponse,
    summary="Get setup request status",
    description="Poll setup request status and get consensus pubkey when ready."
)
async def get_setup_request(request_id: UUID, db: Session = Depends(get_db)):
    """
    Get setup request status.

    **Polling:**
    Frontend should poll this endpoint every 2-3 seconds while status is 'pending' or 'provisioning'.

    **Status Flow:**
    - pending → provisioning → ready_for_chain_tx
    - When status='ready_for_chain_tx', consensusPubkey is available

    **Next Steps:**
    Once consensusPubkey is available, user can call chain_client to build and sign MsgCreateValidator.
    """
    db_request = db.query(ValidatorSetupRequest).filter(
        ValidatorSetupRequest.id == request_id
    ).first()

    if not db_request:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Setup request {request_id} not found"
        )

    # Get associated node if exists
    node = db.query(ValidatorNode).filter(
        ValidatorNode.setup_request_id == db_request.id
    ).first()

    return {
        "setupRequest": {
            "id": str(db_request.id),
            "status": db_request.status.value,
            "walletAddress": db_request.wallet_address,
            "validatorName": db_request.validator_name,
            "runMode": db_request.run_mode.value,
            "consensusPubkey": db_request.consensus_pubkey,
            "createdAt": db_request.created_at.isoformat(),
            "updatedAt": db_request.updated_at.isoformat()
        },
        "node": {
            "id": str(node.id),
            "status": node.status.value,
            "rpcEndpoint": node.rpc_endpoint,
            "p2pEndpoint": node.p2p_endpoint,
            "logsUrl": node.logs_url
        } if node else None
    }


@router.get(
    "/by-wallet/{walletAddress}",
    response_model=List[ValidatorByWalletResponse],
    summary="Get validators by wallet",
    description="Get all validators associated with a wallet address, enriched with chain data."
)
async def get_validators_by_wallet(walletAddress: str, db: Session = Depends(get_db)):
    """
    Get all validators for a wallet address.

    **Returns:**
    - Setup requests
    - Associated nodes
    - Chain validator info (if exists)
    - Local heartbeats (for local mode)

    **Chain Integration:**
    Calls chain RPC to check if validator is active on-chain.
    """
    logger.info(f"Fetching validators for wallet {walletAddress}")

    # Get all setup requests for this wallet
    requests = db.query(ValidatorSetupRequest).filter(
        ValidatorSetupRequest.wallet_address == walletAddress
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
            try:
                chain_info = await chain_client.get_validator_info(walletAddress)
            except Exception as e:
                logger.warning(f"Failed to fetch chain info for {walletAddress}: {e}")

        # Get heartbeat if local mode
        heartbeat = None
        if req.run_mode == RunMode.LOCAL and req.consensus_pubkey:
            heartbeat = db.query(LocalValidatorHeartbeat).filter(
                LocalValidatorHeartbeat.consensus_pubkey == req.consensus_pubkey
            ).first()

        results.append({
            "setupRequest": {
                "id": str(req.id),
                "status": req.status.value,
                "validatorName": req.validator_name,
                "runMode": req.run_mode.value,
                "consensusPubkey": req.consensus_pubkey
            },
            "node": {
                "status": node.status.value,
                "rpcEndpoint": node.rpc_endpoint,
                "p2pEndpoint": node.p2p_endpoint
            } if node else None,
            "chainInfo": chain_info,
            "heartbeat": {
                "blockHeight": heartbeat.block_height,
                "lastSeen": heartbeat.last_seen.isoformat()
            } if heartbeat else None
        })

    return results


# ==================== Node Control ====================

@router.post(
    "/stop",
    summary="Stop validator node",
    description="Gracefully stop a cloud validator node."
)
async def stop_validator(request: ValidatorStopRequest, db: Session = Depends(get_db)):
    """
    Stop a cloud validator node.

    **Only for cloud mode validators.**

    **Actions:**
    1. Stops Docker container
    2. Updates node status to 'stopped'
    3. Does NOT delete on-chain validator
    """
    logger.info(f"Stop request for setup request {request.setupRequestId}")

    setup_request = db.query(ValidatorSetupRequest).filter(
        ValidatorSetupRequest.id == UUID(request.setupRequestId)
    ).first()

    if not setup_request:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Setup request not found"
        )

    if setup_request.run_mode != RunMode.CLOUD:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Can only stop cloud validators via API. Local validators must be stopped from desktop app."
        )

    # Get node
    node = db.query(ValidatorNode).filter(
        ValidatorNode.setup_request_id == setup_request.id
    ).first()

    if not node:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Node not found"
        )

    # Stop container
    from app.services.docker_manager import docker_manager

    try:
        success = await docker_manager.stop_container(node.node_internal_id)

        if success:
            from app.models.validator_node import NodeStatus
            node.status = NodeStatus.STOPPED
            db.commit()

            logger.info(f"Successfully stopped node {node.id}")

            return {
                "message": "Validator stopped successfully",
                "nodeId": str(node.id),
                "status": "stopped"
            }
        else:
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail="Failed to stop validator container"
            )

    except Exception as e:
        logger.error(f"Error stopping validator: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Error stopping validator: {str(e)}"
        )


@router.post(
    "/redeploy",
    summary="Redeploy validator node",
    description="Redeploy a cloud validator node with same configuration."
)
async def redeploy_validator(
    request: ValidatorRedeployRequest,
    background_tasks: BackgroundTasks,
    db: Session = Depends(get_db)
):
    """
    Redeploy a validator node.

    **Steps:**
    1. Stop existing container (if running)
    2. Remove old container
    3. Trigger new provisioning with same config
    4. Update database status

    **Use Cases:**
    - Node crashed and won't restart
    - Need to update node configuration
    - Recover from corruption
    - Migrate to new container

    **Note:**
    - Consensus keys are regenerated
    - User must submit new MsgEditValidator with new consensus pubkey
    - On-chain validator remains active (just changes consensus pubkey)
    """
    logger.info(f"Redeploy request for setup request {request.setupRequestId}")

    setup_request = db.query(ValidatorSetupRequest).filter(
        ValidatorSetupRequest.id == UUID(request.setupRequestId)
    ).first()

    if not setup_request:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Setup request not found"
        )

    if setup_request.run_mode != RunMode.CLOUD:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Can only redeploy cloud validators. Local validators must be managed via desktop app."
        )

    # Get existing node
    node = db.query(ValidatorNode).filter(
        ValidatorNode.setup_request_id == setup_request.id
    ).first()

    if node:
        # Stop and remove existing container
        from app.services.docker_manager import docker_manager

        try:
            logger.info(f"Stopping existing container {node.node_internal_id}")
            await docker_manager.stop_container(node.node_internal_id)

            logger.info(f"Removing existing container {node.node_internal_id}")
            await docker_manager.remove_container(node.node_internal_id)

            # Delete old node record
            db.delete(node)
            db.commit()

            logger.info(f"Removed old node {node.id}")

        except Exception as e:
            logger.warning(f"Error cleaning up old container: {e}. Proceeding with redeploy anyway.")

    # Reset setup request status
    setup_request.status = SetupStatus.PENDING
    setup_request.consensus_pubkey = None
    db.commit()

    logger.info(f"Reset setup request {setup_request.id} to PENDING")

    # Trigger new provisioning
    background_tasks.add_task(provision_cloud_validator, setup_request.id)

    logger.info(f"Triggered redeployment for {setup_request.id}")

    return {
        "message": "Validator redeployment initiated",
        "setupRequestId": str(setup_request.id),
        "status": "provisioning",
        "instructions": "Poll /api/v1/validators/setup-requests/{id} to monitor provisioning status. You will receive a new consensus pubkey when ready."
    }


# ==================== Local Validator Heartbeat ====================

@router.post(
    "/heartbeat",
    response_model=LocalValidatorHeartbeatResponse,
    summary="Submit local validator heartbeat",
    description="Called by desktop app to report local validator status."
)
async def submit_heartbeat(
    heartbeat: LocalValidatorHeartbeatCreate,
    db: Session = Depends(get_db)
):
    """
    Submit heartbeat from local validator desktop app.

    **Called by:** Desktop app every 30-60 seconds

    **Purpose:**
    - Track local validator status
    - Show in dashboard that local validator is running
    - Monitor block height and uptime
    """
    logger.info(f"Heartbeat from {heartbeat.walletAddress}, height {heartbeat.blockHeight}")

    # Check if heartbeat already exists
    existing = db.query(LocalValidatorHeartbeat).filter(
        LocalValidatorHeartbeat.consensus_pubkey == heartbeat.consensusPubkey
    ).first()

    if existing:
        # Update existing
        existing.block_height = heartbeat.blockHeight
        existing.uptime_seconds = heartbeat.uptimeSeconds
        existing.local_rpc_port = heartbeat.localRpcPort
        existing.local_p2p_port = heartbeat.localP2pPort
        existing.last_seen = datetime.utcnow()
        db.commit()
        db.refresh(existing)

        return {
            "id": str(existing.id),
            "blockHeight": existing.block_height,
            "lastSeen": existing.last_seen.isoformat(),
            "message": "Heartbeat updated"
        }
    else:
        # Create new
        new_heartbeat = LocalValidatorHeartbeat(
            wallet_address=heartbeat.walletAddress,
            consensus_pubkey=heartbeat.consensusPubkey,
            block_height=heartbeat.blockHeight,
            uptime_seconds=heartbeat.uptimeSeconds,
            local_rpc_port=heartbeat.localRpcPort,
            local_p2p_port=heartbeat.localP2pPort
        )
        db.add(new_heartbeat)
        db.commit()
        db.refresh(new_heartbeat)

        return {
            "id": str(new_heartbeat.id),
            "blockHeight": new_heartbeat.block_height,
            "lastSeen": new_heartbeat.last_seen.isoformat(),
            "message": "Heartbeat created"
        }
