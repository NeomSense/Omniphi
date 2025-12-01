"""Background worker for provisioning validator nodes."""

import asyncio
from uuid import UUID
from datetime import datetime

from app.db.session import SessionLocal
from app.models import ValidatorSetupRequest, ValidatorNode
from app.models.validator_setup_request import SetupStatus
from app.models.validator_node import NodeStatus
from app.services.docker_manager import docker_manager


async def provision_cloud_validator(setup_request_id: UUID, redeploy: bool = False):
    """
    Provision a cloud validator node.

    This function runs in the background and:
    1. Creates a Docker container
    2. Initializes the validator node
    3. Retrieves the consensus pubkey
    4. Updates the database

    Args:
        setup_request_id: UUID of the setup request
        redeploy: If True, removes existing container first
    """
    db = SessionLocal()

    try:
        # Get setup request
        setup_request = db.query(ValidatorSetupRequest).filter(
            ValidatorSetupRequest.id == setup_request_id
        ).first()

        if not setup_request:
            print(f"Setup request {setup_request_id} not found")
            return

        # Update status to provisioning
        setup_request.status = SetupStatus.PROVISIONING
        db.commit()

        # If redeploying, remove existing node
        if redeploy:
            existing_node = db.query(ValidatorNode).filter(
                ValidatorNode.setup_request_id == setup_request_id
            ).first()

            if existing_node:
                await docker_manager.remove_container(existing_node.node_internal_id)
                db.delete(existing_node)
                db.commit()

        # Create Docker container
        try:
            container_info = await docker_manager.create_validator_container(
                validator_name=f"{setup_request.wallet_address[:8]}-{setup_request.id}",
                moniker=setup_request.validator_name,
                chain_id="omniphi-testnet-1"  # Use testnet for now
            )

            # Update setup request status to configuring
            setup_request.status = SetupStatus.CONFIGURING
            setup_request.consensus_pubkey = container_info.get("consensus_pubkey")
            db.commit()

            # Create validator node record
            validator_node = ValidatorNode(
                setup_request_id=setup_request.id,
                provider=setup_request.provider,
                node_internal_id=container_info["container_id"],
                rpc_endpoint=container_info.get("rpc_endpoint"),
                p2p_endpoint=container_info.get("p2p_endpoint"),
                grpc_endpoint=container_info.get("grpc_endpoint"),
                status=NodeStatus.STARTING
            )

            db.add(validator_node)
            db.commit()

            # Wait for node to start
            await asyncio.sleep(5)

            # Check node status
            status = await docker_manager.get_container_status(container_info["container_id"])

            if status.get("status") == "running":
                validator_node.status = NodeStatus.RUNNING
                setup_request.status = SetupStatus.READY
                setup_request.completed_at = datetime.utcnow()
            else:
                validator_node.status = NodeStatus.ERROR
                setup_request.status = SetupStatus.FAILED
                setup_request.error_message = "Container failed to start"

            db.commit()

            print(f"Successfully provisioned validator for {setup_request.wallet_address}")

        except Exception as e:
            # Mark as failed
            setup_request.status = SetupStatus.FAILED
            setup_request.error_message = str(e)
            db.commit()

            print(f"Error provisioning validator: {e}")

    except Exception as e:
        print(f"Fatal error in provision_cloud_validator: {e}")

    finally:
        db.close()


async def health_check_worker():
    """
    Background worker to periodically check validator node health.

    This should run continuously to monitor all running nodes.
    """
    db = SessionLocal()

    try:
        while True:
            # Get all running nodes
            nodes = db.query(ValidatorNode).filter(
                ValidatorNode.status.in_([NodeStatus.RUNNING, NodeStatus.SYNCING])
            ).all()

            for node in nodes:
                try:
                    # Check container status
                    status = await docker_manager.get_container_status(node.node_internal_id)

                    if status.get("status") != "running":
                        node.status = NodeStatus.STOPPED
                    else:
                        # TODO: Query RPC endpoint for block height
                        node.status = NodeStatus.RUNNING

                    node.last_health_check = datetime.utcnow()
                    db.commit()

                except Exception as e:
                    print(f"Error checking health for node {node.id}: {e}")
                    continue

            # Sleep for 30 seconds before next check
            await asyncio.sleep(30)

    except Exception as e:
        print(f"Fatal error in health_check_worker: {e}")

    finally:
        db.close()
