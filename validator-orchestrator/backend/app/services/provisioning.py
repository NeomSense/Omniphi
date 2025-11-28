"""Cloud validator provisioning service."""

import asyncio
import logging
from uuid import UUID
from datetime import datetime
import base64
import secrets

from app.db.session import SessionLocal
from app.models import ValidatorSetupRequest, ValidatorNode
from app.models.validator_setup_request import SetupStatus
from app.models.validator_node import NodeStatus

logger = logging.getLogger(__name__)


async def provision_cloud_validator(setup_request_id: UUID, redeploy: bool = False):
    """
    Provision a cloud validator node.

    This function runs in the background and:
    1. Creates a Docker container (placeholder in MVP)
    2. Initializes the validator node
    3. Generates a fake but valid-looking consensus pubkey (MVP)
    4. Updates the database with status and pubkey

    Args:
        setup_request_id: UUID of the setup request
        redeploy: If True, removes existing container first

    Note:
        In production, this would:
        - Use real Omniphi node Docker image
        - Inject genesis and seeds
        - Extract real consensus pubkey from container
        - Configure persistent storage
    """
    db = SessionLocal()

    try:
        # Get setup request
        setup_request = db.query(ValidatorSetupRequest).filter(
            ValidatorSetupRequest.id == setup_request_id
        ).first()

        if not setup_request:
            logger.error(f"Setup request {setup_request_id} not found")
            return

        logger.info(f"Starting provisioning for setup request {setup_request_id}")

        # Update status to provisioning
        setup_request.status = SetupStatus.PROVISIONING
        db.commit()

        # If redeploying, remove existing node
        if redeploy:
            existing_node = db.query(ValidatorNode).filter(
                ValidatorNode.setup_request_id == setup_request_id
            ).first()

            if existing_node:
                logger.info(f"Removing existing node {existing_node.id} for redeployment")
                from app.services.docker_manager import docker_manager
                await docker_manager.remove_container(existing_node.node_internal_id)
                db.delete(existing_node)
                db.commit()

        # ========== MVP: Simulated Provisioning ==========
        # In production, this would create real Docker container
        # For MVP, we simulate with placeholder values

        try:
            # Simulate container creation delay
            await asyncio.sleep(2)

            # Generate placeholder container ID
            container_id = f"placeholder-{secrets.token_hex(8)}"

            # Generate fake but valid-looking consensus pubkey
            # Real format: base64-encoded Ed25519 public key
            random_bytes = secrets.token_bytes(32)
            consensus_pubkey = base64.b64encode(random_bytes).decode('utf-8')

            logger.info(f"Generated placeholder container {container_id}")
            logger.info(f"Generated consensus pubkey: {consensus_pubkey[:16]}...")

            # Create validator node record
            validator_node = ValidatorNode(
                setup_request_id=setup_request.id,
                provider=setup_request.provider,
                node_internal_id=container_id,
                rpc_endpoint=f"http://placeholder-{setup_request.id}:26657",
                p2p_endpoint=f"tcp://placeholder-{setup_request.id}:26656",
                grpc_endpoint=f"placeholder-{setup_request.id}:9090",
                status=NodeStatus.RUNNING,
                logs_url=f"http://placeholder/logs/{container_id}"
            )

            db.add(validator_node)

            # Update setup request
            setup_request.consensus_pubkey = consensus_pubkey
            setup_request.status = SetupStatus.READY_FOR_CHAIN_TX
            setup_request.completed_at = datetime.utcnow()

            db.commit()

            logger.info(f"Successfully provisioned validator for {setup_request.wallet_address}")
            logger.info(f"Status: {setup_request.status.value}")

            # ========== Production Implementation Would Be: ==========
            #
            # from app.services.docker_manager import docker_manager
            #
            # container_info = await docker_manager.create_validator_container(
            #     validator_name=f"{setup_request.wallet_address[:8]}-{setup_request.id}",
            #     moniker=setup_request.validator_name,
            #     chain_id=settings.OMNIPHI_CHAIN_ID
            # )
            #
            # validator_node = ValidatorNode(
            #     setup_request_id=setup_request.id,
            #     provider=setup_request.provider,
            #     node_internal_id=container_info["container_id"],
            #     rpc_endpoint=container_info.get("rpc_endpoint"),
            #     p2p_endpoint=container_info.get("p2p_endpoint"),
            #     grpc_endpoint=container_info.get("grpc_endpoint"),
            #     status=NodeStatus.RUNNING
            # )
            #
            # setup_request.consensus_pubkey = container_info.get("consensus_pubkey")
            # setup_request.status = SetupStatus.READY_FOR_CHAIN_TX
            #
            # ========================================================

        except Exception as e:
            # Mark as failed
            logger.error(f"Error provisioning validator: {e}", exc_info=True)

            setup_request.status = SetupStatus.FAILED
            setup_request.error_message = str(e)
            db.commit()

    except Exception as e:
        logger.error(f"Fatal error in provision_cloud_validator: {e}", exc_info=True)

    finally:
        db.close()


async def health_check_worker():
    """
    Background worker to periodically check validator node health.

    This should run continuously to monitor all running nodes.

    **Actions:**
    - Query all running nodes
    - Check Docker container status
    - Query RPC endpoint for block height
    - Update database with current status
    - Alert if node goes offline
    """
    db = SessionLocal()

    logger.info("Starting health check worker")

    try:
        while True:
            try:
                # Get all running or syncing nodes
                nodes = db.query(ValidatorNode).filter(
                    ValidatorNode.status.in_([NodeStatus.RUNNING, NodeStatus.SYNCING])
                ).all()

                logger.debug(f"Checking health for {len(nodes)} nodes")

                for node in nodes:
                    try:
                        # ========== MVP: Skip actual health checks ==========
                        # In production, would check:
                        # - Docker container status
                        # - RPC endpoint reachability
                        # - Current block height
                        # - Sync status

                        node.last_health_check = datetime.utcnow()

                        # ========== Production Implementation: ==========
                        #
                        # from app.services.docker_manager import docker_manager
                        # import httpx
                        #
                        # # Check container status
                        # status = await docker_manager.get_container_status(node.node_internal_id)
                        #
                        # if status.get("status") != "running":
                        #     node.status = NodeStatus.STOPPED
                        #     logger.warning(f"Node {node.id} container stopped")
                        # else:
                        #     # Query RPC for block height
                        #     async with httpx.AsyncClient() as client:
                        #         try:
                        #             response = await client.get(
                        #                 f"{node.rpc_endpoint}/status",
                        #                 timeout=5.0
                        #             )
                        #             if response.status_code == 200:
                        #                 data = response.json()
                        #                 node.last_block_height = data["result"]["sync_info"]["latest_block_height"]
                        #                 logger.debug(f"Node {node.id} at height {node.last_block_height}")
                        #         except Exception as e:
                        #             logger.warning(f"Failed to query RPC for node {node.id}: {e}")
                        #
                        # node.last_health_check = datetime.utcnow()
                        #
                        # ====================================================

                    except Exception as e:
                        logger.error(f"Error checking health for node {node.id}: {e}")
                        continue

                db.commit()

            except Exception as e:
                logger.error(f"Error in health check iteration: {e}")

            # Sleep for 30 seconds before next check
            await asyncio.sleep(30)

    except Exception as e:
        logger.error(f"Fatal error in health_check_worker: {e}", exc_info=True)

    finally:
        db.close()
