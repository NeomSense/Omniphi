"""Docker container management for validator nodes."""

import docker
import logging
import os
import tempfile
from typing import Optional, Dict, Any
import json

from app.core.config import settings

logger = logging.getLogger(__name__)


class DockerManager:
    """Manages Docker containers for validator nodes."""

    def __init__(self):
        self.client = docker.from_env()
        self.network_name = settings.DOCKER_NETWORK

        # Ensure network exists
        self._ensure_network()

    def _ensure_network(self):
        """Ensure Docker network exists for validators."""
        try:
            self.client.networks.get(self.network_name)
        except docker.errors.NotFound:
            self.client.networks.create(
                self.network_name,
                driver="bridge"
            )

    async def create_validator_container(
        self,
        validator_name: str,
        moniker: str,
        chain_id: str = None
    ) -> Dict[str, Any]:
        """
        Create and start a new validator container.

        Args:
            validator_name: Unique name for the container
            moniker: Validator moniker
            chain_id: Chain ID (defaults to settings)

        Returns:
            Dict containing container_id, rpc_endpoint, p2p_endpoint, consensus_pubkey
        """
        chain_id = chain_id or settings.OMNIPHI_CHAIN_ID

        # Container name
        container_name = f"omniphi-validator-{validator_name}"

        # Create temp directory for validator config
        temp_dir = tempfile.mkdtemp(prefix=f"validator-{validator_name}-")

        # SECURITY: Enforce checksum verification in production
        binary_sha256 = getattr(settings, 'OMNIPHI_BINARY_SHA256', None) or ""
        genesis_sha256 = getattr(settings, 'OMNIPHI_GENESIS_SHA256', None) or ""
        keyring_backend = getattr(settings, 'KEYRING_BACKEND', 'file')

        # Validate required checksums
        if not binary_sha256:
            raise ValueError(
                "SECURITY ERROR: OMNIPHI_BINARY_SHA256 not set. "
                "Binary checksum verification is REQUIRED. "
                "Set OMNIPHI_BINARY_SHA256 in environment or .env file."
            )

        if not genesis_sha256:
            raise ValueError(
                "SECURITY ERROR: OMNIPHI_GENESIS_SHA256 not set. "
                "Genesis checksum verification is REQUIRED. "
                "Set OMNIPHI_GENESIS_SHA256 in environment or .env file."
            )

        # Build container configuration
        container_config = {
            "image": settings.DOCKER_IMAGE,
            "name": container_name,
            "detach": True,
            "network": self.network_name,
            "environment": {
                "CHAIN_ID": chain_id,
                "MONIKER": moniker,
                "OMNIPHI_BINARY_URL": settings.OMNIPHI_BINARY_URL,
                "OMNIPHI_BINARY_SHA256": binary_sha256,
                "OMNIPHI_GENESIS_URL": settings.OMNIPHI_GENESIS_URL,
                "OMNIPHI_GENESIS_SHA256": genesis_sha256,
                "KEYRING_BACKEND": keyring_backend
            },
            "ports": {
                "26656/tcp": None,  # P2P - random host port
                "26657/tcp": None,  # RPC - random host port
                "9090/tcp": None,   # gRPC - random host port
                "1317/tcp": None    # REST - random host port
            },
            "volumes": {
                temp_dir: {"bind": "/root/.omniphi", "mode": "rw"}
            },
            "command": [
                "/bin/sh",
                "-c",
                f"""
                set -e  # Exit on any error

                # Security helper function
                verify_checksum() {{
                    local file="$1"
                    local expected="$2"
                    local actual

                    actual=$(sha256sum "$file" | awk '{{print $1}}')

                    if [ "$actual" != "$expected" ]; then
                        echo "SECURITY ERROR: Checksum mismatch for $file"
                        echo "Expected: $expected"
                        echo "Got:      $actual"
                        echo "File may have been tampered with!"
                        rm -f "$file"
                        exit 1
                    fi
                    echo "Checksum verified for $file"
                }}

                # Download and verify binary
                if [ ! -f /usr/local/bin/posd ]; then
                    echo "Downloading posd binary..."
                    wget -O /tmp/posd "$OMNIPHI_BINARY_URL"

                    # MANDATORY checksum verification
                    verify_checksum /tmp/posd "$OMNIPHI_BINARY_SHA256"

                    mv /tmp/posd /usr/local/bin/posd
                    chmod +x /usr/local/bin/posd

                    # Verify binary is executable
                    if ! /usr/local/bin/posd version; then
                        echo "ERROR: Binary verification failed"
                        exit 1
                    fi
                fi

                # Initialize node
                posd init "$MONIKER" --chain-id "$CHAIN_ID" --home /root/.omniphi

                # Download and verify genesis
                echo "Downloading genesis file..."
                wget -O /root/.omniphi/config/genesis.json "$OMNIPHI_GENESIS_URL"

                # MANDATORY checksum verification
                verify_checksum /root/.omniphi/config/genesis.json "$OMNIPHI_GENESIS_SHA256"

                # Configure node
                posd config set client chain-id "$CHAIN_ID" --home /root/.omniphi

                # Configure keyring backend
                # 'file' = encrypted keys (production)
                # 'test' = unencrypted keys (development only)
                posd config set client keyring-backend "$KEYRING_BACKEND" --home /root/.omniphi

                if [ "$KEYRING_BACKEND" = "test" ]; then
                    echo "=========================================="
                    echo "WARNING: Using 'test' keyring backend"
                    echo "Keys are stored UNENCRYPTED!"
                    echo "NOT SUITABLE FOR PRODUCTION VALIDATORS"
                    echo "=========================================="
                else
                    echo "Using '$KEYRING_BACKEND' keyring backend"
                fi

                # Start node
                echo "Starting validator node..."
                posd start --home /root/.omniphi
                """
            ]
        }

        try:
            # Create and start container
            container = self.client.containers.run(**container_config)

            # Get container details
            container.reload()

            # Extract port mappings
            ports = container.attrs["NetworkSettings"]["Ports"]
            p2p_port = ports.get("26656/tcp", [{}])[0].get("HostPort")
            rpc_port = ports.get("26657/tcp", [{}])[0].get("HostPort")
            grpc_port = ports.get("9090/tcp", [{}])[0].get("HostPort")
            api_port = ports.get("1317/tcp", [{}])[0].get("HostPort")

            # Get consensus pubkey (wait for node to initialize)
            import asyncio
            await asyncio.sleep(10)  # Wait for initialization

            consensus_pubkey = await self._get_consensus_pubkey(container.id)

            return {
                "container_id": container.id,
                "rpc_endpoint": f"http://localhost:{rpc_port}" if rpc_port else None,
                "p2p_endpoint": f"tcp://localhost:{p2p_port}" if p2p_port else None,
                "grpc_endpoint": f"localhost:{grpc_port}" if grpc_port else None,
                "api_endpoint": f"http://localhost:{api_port}" if api_port else None,
                "consensus_pubkey": consensus_pubkey,
                "config_dir": temp_dir
            }

        except Exception as e:
            logger.error(f"Error creating validator container: {e}")
            raise

    async def _get_consensus_pubkey(self, container_id: str) -> Optional[str]:
        """
        Extract consensus public key from container.

        Args:
            container_id: Docker container ID

        Returns:
            Consensus public key or None
        """
        try:
            container = self.client.containers.get(container_id)

            # Execute command to get consensus pubkey
            result = container.exec_run(
                "posd comet show-validator --home /root/.omniphi"
            )

            if result.exit_code == 0:
                output = result.output.decode("utf-8").strip()
                # Parse JSON output
                pubkey_data = json.loads(output)
                return pubkey_data.get("key")

            return None

        except Exception as e:
            logger.error(f"Error getting consensus pubkey: {e}")
            return None

    async def stop_container(self, container_id: str) -> bool:
        """
        Gracefully stop a validator container.

        Args:
            container_id: Docker container ID

        Returns:
            True if successful, False otherwise
        """
        try:
            container = self.client.containers.get(container_id)
            container.stop(timeout=30)
            return True
        except Exception as e:
            logger.error(f"Error stopping container: {e}")
            return False

    async def remove_container(self, container_id: str) -> bool:
        """
        Remove a validator container.

        Args:
            container_id: Docker container ID

        Returns:
            True if successful, False otherwise
        """
        try:
            container = self.client.containers.get(container_id)
            container.remove(force=True)
            return True
        except Exception as e:
            logger.error(f"Error removing container: {e}")
            return False

    async def restart_container(self, container_id: str) -> bool:
        """
        Restart a validator container.

        Args:
            container_id: Docker container ID

        Returns:
            True if successful, False otherwise
        """
        try:
            container = self.client.containers.get(container_id)
            container.restart(timeout=30)
            return True
        except Exception as e:
            logger.error(f"Error restarting container: {e}")
            return False

    async def get_container_logs(self, container_id: str, lines: int = 100) -> str:
        """
        Get logs from a validator container.

        Args:
            container_id: Docker container ID
            lines: Number of lines to retrieve

        Returns:
            Container logs
        """
        try:
            container = self.client.containers.get(container_id)
            logs = container.logs(tail=lines).decode("utf-8")
            return logs
        except Exception as e:
            logger.error(f"Error getting container logs: {e}")
            return f"Error: {str(e)}"

    async def get_container_status(self, container_id: str) -> Dict[str, Any]:
        """
        Get status of a validator container.

        Args:
            container_id: Docker container ID

        Returns:
            Status information
        """
        try:
            container = self.client.containers.get(container_id)
            container.reload()

            return {
                "id": container.id,
                "name": container.name,
                "status": container.status,
                "state": container.attrs["State"],
                "started_at": container.attrs["State"].get("StartedAt"),
                "ports": container.attrs["NetworkSettings"]["Ports"]
            }
        except Exception as e:
            logger.error(f"Error getting container status: {e}")
            return {"error": str(e)}


# Global instance
docker_manager = DockerManager()
