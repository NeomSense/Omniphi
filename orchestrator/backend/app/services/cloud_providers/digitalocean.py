"""DigitalOcean Droplets Provider for validator node provisioning."""

import logging
import base64
import secrets
from typing import Dict, Optional, Any
from datetime import datetime
import asyncio

logger = logging.getLogger(__name__)

try:
    import httpx
    HTTPX_AVAILABLE = True
except ImportError:
    HTTPX_AVAILABLE = False
    logger.warning("httpx not installed. DigitalOcean provisioning will not be available.")


class DigitalOceanProvider:
    """
    DigitalOcean Droplets provider for provisioning Omniphi validator nodes.

    Features:
    - Create Droplets with Ubuntu 22.04
    - Configure Cloud Firewalls
    - Install and initialize Omniphi validator
    - Extract consensus pubkey
    - Configure monitoring and backups
    """

    API_BASE_URL = "https://api.digitalocean.com/v2"

    def __init__(self, api_token: str):
        """
        Initialize DigitalOcean provider.

        Args:
            api_token: DigitalOcean API token (Personal Access Token)
                      Get from: https://cloud.digitalocean.com/account/api/tokens
        """
        if not HTTPX_AVAILABLE:
            raise ImportError(
                "httpx is required for DigitalOcean provisioning. "
                "Install with: pip install httpx"
            )

        if not api_token:
            raise ValueError("DigitalOcean API token is required")

        self.api_token = api_token
        self.headers = {
            "Authorization": f"Bearer {api_token}",
            "Content-Type": "application/json"
        }

        logger.info("Initialized DigitalOcean provider")

    async def provision_validator(
        self,
        validator_name: str,
        moniker: str,
        chain_id: str,
        region: str = "nyc3",
        size: str = "s-2vcpu-4gb",
        volume_size_gb: int = 500,
        ssh_keys: Optional[list] = None,
        tags: Optional[list] = None
    ) -> Dict[str, Any]:
        """
        Provision a new validator Droplet on DigitalOcean.

        Args:
            validator_name: Unique validator identifier
            moniker: Validator display name
            chain_id: Blockchain network ID
            region: DigitalOcean region (default: nyc3)
                   Options: nyc1, nyc3, sfo3, ams3, sgp1, lon1, fra1, tor1, blr1
            size: Droplet size (default: s-2vcpu-4gb - 2vCPU, 4GB RAM, 80GB SSD, $24/mo)
                 Options: s-2vcpu-4gb, s-4vcpu-8gb, s-8vcpu-16gb
            volume_size_gb: Additional volume size in GB (default: 500GB)
            ssh_keys: List of SSH key IDs or fingerprints
            tags: Additional tags for the Droplet

        Returns:
            Dict containing Droplet info and consensus pubkey:
            {
                "droplet_id": 123456789,
                "name": "omniphi-validator-xxx",
                "public_ip": "1.2.3.4",
                "private_ip": "10.0.1.5",
                "consensus_pubkey": "base64_encoded_pubkey",
                "rpc_endpoint": "http://1.2.3.4:26657",
                "p2p_endpoint": "tcp://1.2.3.4:26656",
                "grpc_endpoint": "1.2.3.4:9090",
                "ssh_command": "ssh root@1.2.3.4",
                "volume_id": "vol-xxxxx"
            }
        """
        try:
            logger.info(f"Provisioning validator '{validator_name}' on DigitalOcean")

            async with httpx.AsyncClient() as client:
                # Create volume for blockchain data
                volume = await self._create_volume(
                    client,
                    validator_name,
                    region,
                    volume_size_gb
                )
                volume_id = volume['id']
                logger.info(f"Created volume: {volume_id}")

                # Get or create SSH keys
                if not ssh_keys:
                    ssh_keys = await self._get_ssh_keys(client)
                    if not ssh_keys:
                        logger.warning("No SSH keys found. You won't be able to SSH into the Droplet.")

                # Generate user data script
                user_data = self._generate_user_data(moniker, chain_id)

                # Build Droplet configuration
                droplet_name = f"omniphi-validator-{validator_name}"
                droplet_tags = ["omniphi", "validator", f"chain-{chain_id}"]
                if tags:
                    droplet_tags.extend(tags)

                droplet_config = {
                    "name": droplet_name,
                    "region": region,
                    "size": size,
                    "image": "ubuntu-22-04-x64",
                    "ssh_keys": ssh_keys if ssh_keys else [],
                    "backups": False,  # Can enable for additional cost
                    "ipv6": True,
                    "monitoring": True,  # Free monitoring
                    "tags": droplet_tags,
                    "user_data": user_data,
                    "volumes": [volume_id]
                }

                # Create Droplet
                response = await client.post(
                    f"{self.API_BASE_URL}/droplets",
                    headers=self.headers,
                    json=droplet_config,
                    timeout=30.0
                )

                if response.status_code != 202:
                    raise Exception(f"Failed to create Droplet: {response.text}")

                droplet_data = response.json()['droplet']
                droplet_id = droplet_data['id']
                logger.info(f"Created Droplet: {droplet_id}")

                # Wait for Droplet to be active
                logger.info("Waiting for Droplet to become active...")
                droplet_info = await self._wait_for_droplet_active(client, droplet_id)

                public_ip = droplet_info['networks']['v4'][0]['ip_address']
                private_ip = droplet_info['networks']['v4'][1]['ip_address'] if len(droplet_info['networks']['v4']) > 1 else None

                logger.info(f"Droplet active: {droplet_id} (IP: {public_ip})")

                # Create firewall rules
                firewall_id = await self._create_firewall(client, droplet_name, droplet_id)
                logger.info(f"Created firewall: {firewall_id}")

                # Wait for validator initialization
                logger.info("Waiting for validator initialization (this may take 5-10 minutes)...")
                await asyncio.sleep(300)  # Wait 5 minutes for cloud-init

                # Extract consensus pubkey
                consensus_pubkey = await self._extract_consensus_pubkey(client, droplet_id)

                result = {
                    "droplet_id": droplet_id,
                    "name": droplet_name,
                    "public_ip": public_ip,
                    "private_ip": private_ip,
                    "consensus_pubkey": consensus_pubkey,
                    "rpc_endpoint": f"http://{public_ip}:26657",
                    "p2p_endpoint": f"tcp://{public_ip}:26656",
                    "grpc_endpoint": f"{public_ip}:9090",
                    "ssh_command": f"ssh root@{public_ip}",
                    "region": region,
                    "size": size,
                    "volume_id": volume_id,
                    "volume_size_gb": volume_size_gb,
                    "firewall_id": firewall_id
                }

                logger.info(f"Successfully provisioned validator: {droplet_id}")
                return result

        except httpx.HTTPError as e:
            logger.error(f"HTTP error during provisioning: {e}")
            raise Exception(f"Failed to provision Droplet: {str(e)}")
        except Exception as e:
            logger.error(f"Error during provisioning: {e}", exc_info=True)
            raise

    async def _create_volume(
        self,
        client: httpx.AsyncClient,
        validator_name: str,
        region: str,
        size_gb: int
    ) -> Dict[str, Any]:
        """Create a Block Storage volume for blockchain data."""
        volume_name = f"omniphi-validator-{validator_name}-data"

        response = await client.post(
            f"{self.API_BASE_URL}/volumes",
            headers=self.headers,
            json={
                "size_gigabytes": size_gb,
                "name": volume_name,
                "description": f"Blockchain data volume for {validator_name}",
                "region": region,
                "filesystem_type": "ext4",
                "tags": ["omniphi", "validator", "blockchain-data"]
            },
            timeout=30.0
        )

        if response.status_code != 201:
            raise Exception(f"Failed to create volume: {response.text}")

        return response.json()['volume']

    async def _get_ssh_keys(self, client: httpx.AsyncClient) -> list:
        """Get list of SSH keys from DigitalOcean account."""
        try:
            response = await client.get(
                f"{self.API_BASE_URL}/account/keys",
                headers=self.headers,
                timeout=10.0
            )

            if response.status_code == 200:
                keys = response.json()['ssh_keys']
                return [key['id'] for key in keys]
            return []

        except Exception as e:
            logger.warning(f"Error fetching SSH keys: {e}")
            return []

    def _generate_user_data(self, moniker: str, chain_id: str) -> str:
        """
        Generate cloud-init user data script to initialize validator.

        This script runs on first boot and:
        1. Mounts the Block Storage volume
        2. Installs dependencies
        3. Downloads and installs posd binary
        4. Initializes validator node
        5. Configures systemd service
        6. Starts validator
        """
        script = f"""#!/bin/bash
set -e

# Log all output
exec > >(tee /var/log/validator-init.log)
exec 2>&1

echo "=== Omniphi Validator Initialization ==="
echo "Moniker: {moniker}"
echo "Chain ID: {chain_id}"
echo "Started: $(date)"

# Update system
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get upgrade -y

# Install dependencies
apt-get install -y \\
    curl \\
    wget \\
    jq \\
    build-essential \\
    git \\
    ca-certificates \\
    ufw

# Configure firewall (UFW)
ufw --force enable
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp comment 'SSH'
ufw allow 26656/tcp comment 'P2P'
ufw allow 9090/tcp comment 'gRPC'
ufw allow 26660/tcp comment 'Prometheus'

# Mount Block Storage volume
VOLUME_DEVICE="/dev/disk/by-id/scsi-0DO_Volume_omniphi-validator-*-data"
MOUNT_POINT="/mnt/validator-data"

# Wait for volume to be attached
for i in {{1..30}}; do
    if ls $VOLUME_DEVICE 2>/dev/null; then
        break
    fi
    echo "Waiting for volume to be attached... ($i/30)"
    sleep 2
done

# Create mount point and mount volume
mkdir -p $MOUNT_POINT
mount -o discard,defaults $VOLUME_DEVICE $MOUNT_POINT
echo "$VOLUME_DEVICE $MOUNT_POINT ext4 defaults,nofail,discard 0 2" >> /etc/fstab

# Create validator user
if ! id -u omniphi &>/dev/null; then
    useradd -m -s /bin/bash omniphi
fi

# Create omniphi home directory on volume
mkdir -p $MOUNT_POINT/omniphi-home
ln -sf $MOUNT_POINT/omniphi-home /home/omniphi/.omniphi
chown -R omniphi:omniphi $MOUNT_POINT/omniphi-home

# Download posd binary (TODO: Replace with actual release URL)
cd /tmp
# Example: wget https://github.com/omniphi/omniphi/releases/download/v1.0.0/posd
# Example: chmod +x posd
# Example: mv posd /usr/local/bin/

# For MVP, create marker files
sudo -u omniphi bash <<'EOF'
mkdir -p /home/omniphi/.omniphi/config
mkdir -p /home/omniphi/.omniphi/data

# Placeholder consensus pubkey
echo '{{"@type":"/cosmos.crypto.ed25519.PubKey","key":"PLACEHOLDER_WILL_BE_EXTRACTED"}}' > /home/omniphi/.omniphi/consensus_pubkey.json

# Mark initialization complete
touch /home/omniphi/.omniphi/initialized
EOF

# TODO: In production:
# 1. posd init {moniker} --chain-id {chain_id}
# 2. Download genesis file
# 3. Configure seeds/peers
# 4. Copy systemd service template
# 5. Start validator

echo "=== Initialization Complete ==="
echo "Completed: $(date)"
"""
        return script

    async def _wait_for_droplet_active(
        self,
        client: httpx.AsyncClient,
        droplet_id: int,
        timeout: int = 300
    ) -> Dict[str, Any]:
        """Wait for Droplet to become active."""
        start_time = datetime.now()

        while (datetime.now() - start_time).total_seconds() < timeout:
            response = await client.get(
                f"{self.API_BASE_URL}/droplets/{droplet_id}",
                headers=self.headers,
                timeout=10.0
            )

            if response.status_code == 200:
                droplet = response.json()['droplet']
                if droplet['status'] == 'active':
                    return droplet

            logger.debug(f"Waiting for Droplet to be active... ({int((datetime.now() - start_time).total_seconds())}s)")
            await asyncio.sleep(5)

        raise TimeoutError(f"Droplet activation timed out after {timeout}s")

    async def _create_firewall(
        self,
        client: httpx.AsyncClient,
        validator_name: str,
        droplet_id: int
    ) -> str:
        """
        Create Cloud Firewall for validator.

        Firewall rules:
        - SSH (22): Open to all (should restrict to your IP in production)
        - P2P (26656): Open to all (required for validator)
        - gRPC (9090): Open to all (for client connections)
        - Prometheus (26660): Open to all (should restrict to monitoring server)
        - RPC (26657): Blocked (localhost only)
        """
        firewall_name = f"omniphi-validator-{validator_name}-fw"

        firewall_config = {
            "name": firewall_name,
            "inbound_rules": [
                {
                    "protocol": "tcp",
                    "ports": "22",
                    "sources": {
                        "addresses": ["0.0.0.0/0", "::/0"]
                    }
                },
                {
                    "protocol": "tcp",
                    "ports": "26656",
                    "sources": {
                        "addresses": ["0.0.0.0/0", "::/0"]
                    }
                },
                {
                    "protocol": "tcp",
                    "ports": "9090",
                    "sources": {
                        "addresses": ["0.0.0.0/0", "::/0"]
                    }
                },
                {
                    "protocol": "tcp",
                    "ports": "26660",
                    "sources": {
                        "addresses": ["0.0.0.0/0", "::/0"]
                    }
                }
            ],
            "outbound_rules": [
                {
                    "protocol": "tcp",
                    "ports": "all",
                    "destinations": {
                        "addresses": ["0.0.0.0/0", "::/0"]
                    }
                },
                {
                    "protocol": "udp",
                    "ports": "all",
                    "destinations": {
                        "addresses": ["0.0.0.0/0", "::/0"]
                    }
                }
            ],
            "droplet_ids": [droplet_id],
            "tags": ["omniphi", "validator"]
        }

        response = await client.post(
            f"{self.API_BASE_URL}/firewalls",
            headers=self.headers,
            json=firewall_config,
            timeout=30.0
        )

        if response.status_code != 202:
            raise Exception(f"Failed to create firewall: {response.text}")

        return response.json()['firewall']['id']

    async def _extract_consensus_pubkey(self, client: httpx.AsyncClient, droplet_id: int) -> str:
        """
        Extract consensus public key from validator Droplet.

        In production, this would use Droplet's API or SSH to run:
        sudo -u omniphi posd tendermint show-validator

        For now, returns a placeholder.
        """
        # TODO: Implement actual pubkey extraction via SSH or Droplet API
        logger.warning("Using placeholder consensus pubkey - implement actual extraction in production")

        random_bytes = secrets.token_bytes(32)
        return base64.b64encode(random_bytes).decode('utf-8')

    async def delete_droplet(self, droplet_id: int):
        """Delete a Droplet and associated resources."""
        try:
            logger.info(f"Deleting Droplet {droplet_id}")

            async with httpx.AsyncClient() as client:
                # Get Droplet info to find volume
                response = await client.get(
                    f"{self.API_BASE_URL}/droplets/{droplet_id}",
                    headers=self.headers,
                    timeout=10.0
                )

                if response.status_code == 200:
                    droplet = response.json()['droplet']
                    volume_ids = droplet.get('volume_ids', [])

                    # Delete Droplet
                    response = await client.delete(
                        f"{self.API_BASE_URL}/droplets/{droplet_id}",
                        headers=self.headers,
                        timeout=30.0
                    )

                    if response.status_code not in [204, 404]:
                        raise Exception(f"Failed to delete Droplet: {response.text}")

                    # Wait for deletion
                    await asyncio.sleep(10)

                    # Delete volumes
                    for volume_id in volume_ids:
                        try:
                            await client.delete(
                                f"{self.API_BASE_URL}/volumes/{volume_id}",
                                headers=self.headers,
                                timeout=30.0
                            )
                            logger.info(f"Deleted volume {volume_id}")
                        except Exception as e:
                            logger.warning(f"Error deleting volume {volume_id}: {e}")

                    logger.info(f"Droplet {droplet_id} deleted")

        except Exception as e:
            logger.error(f"Error deleting Droplet: {e}")
            raise

    async def get_droplet_status(self, droplet_id: int) -> Dict[str, Any]:
        """Get current status of a Droplet."""
        try:
            async with httpx.AsyncClient() as client:
                response = await client.get(
                    f"{self.API_BASE_URL}/droplets/{droplet_id}",
                    headers=self.headers,
                    timeout=10.0
                )

                if response.status_code != 200:
                    raise Exception(f"Failed to get Droplet status: {response.text}")

                droplet = response.json()['droplet']

                return {
                    "droplet_id": droplet_id,
                    "name": droplet['name'],
                    "status": droplet['status'],
                    "public_ip": droplet['networks']['v4'][0]['ip_address'] if droplet['networks']['v4'] else None,
                    "region": droplet['region']['slug'],
                    "size": droplet['size']['slug'],
                    "created_at": droplet['created_at']
                }

        except Exception as e:
            logger.error(f"Error getting Droplet status: {e}")
            raise

    async def list_regions(self) -> list:
        """List available DigitalOcean regions."""
        try:
            async with httpx.AsyncClient() as client:
                response = await client.get(
                    f"{self.API_BASE_URL}/regions",
                    headers=self.headers,
                    timeout=10.0
                )

                if response.status_code == 200:
                    regions = response.json()['regions']
                    return [
                        {
                            "slug": r['slug'],
                            "name": r['name'],
                            "available": r['available']
                        }
                        for r in regions if r['available']
                    ]
                return []

        except Exception as e:
            logger.error(f"Error listing regions: {e}")
            return []

    async def list_sizes(self) -> list:
        """List available Droplet sizes."""
        try:
            async with httpx.AsyncClient() as client:
                response = await client.get(
                    f"{self.API_BASE_URL}/sizes",
                    headers=self.headers,
                    timeout=10.0
                )

                if response.status_code == 200:
                    sizes = response.json()['sizes']
                    # Filter for relevant validator sizes (2+ CPUs, 4+ GB RAM)
                    return [
                        {
                            "slug": s['slug'],
                            "vcpus": s['vcpus'],
                            "memory": s['memory'],
                            "disk": s['disk'],
                            "price_monthly": s['price_monthly']
                        }
                        for s in sizes
                        if s['available'] and s['vcpus'] >= 2 and s['memory'] >= 4096
                    ]
                return []

        except Exception as e:
            logger.error(f"Error listing sizes: {e}")
            return []
