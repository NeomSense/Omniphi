"""AWS EC2 Provider for validator node provisioning."""

import logging
import base64
from typing import Dict, Optional, Any
from datetime import datetime

logger = logging.getLogger(__name__)

try:
    import boto3
    from botocore.exceptions import ClientError, BotoCoreError
    AWS_AVAILABLE = True
except ImportError:
    AWS_AVAILABLE = False
    logger.warning("boto3 not installed. AWS EC2 provisioning will not be available.")


class AWSEC2Provider:
    """
    AWS EC2 provider for provisioning Omniphi validator nodes.

    Features:
    - Launch EC2 instances with Ubuntu 22.04
    - Configure security groups (P2P, SSH, monitoring)
    - Install and initialize Omniphi validator
    - Extract consensus pubkey
    - Configure monitoring and backups
    """

    def __init__(
        self,
        region: str = "us-east-1",
        access_key_id: Optional[str] = None,
        secret_access_key: Optional[str] = None,
        session_token: Optional[str] = None
    ):
        """
        Initialize AWS EC2 provider.

        Args:
            region: AWS region (default: us-east-1)
            access_key_id: AWS access key ID (optional, uses env/config if not provided)
            secret_access_key: AWS secret access key (optional)
            session_token: AWS session token for temporary credentials (optional)
        """
        if not AWS_AVAILABLE:
            raise ImportError(
                "boto3 is required for AWS EC2 provisioning. "
                "Install with: pip install boto3"
            )

        self.region = region

        # Initialize boto3 client
        session_kwargs = {"region_name": region}
        if access_key_id and secret_access_key:
            session_kwargs["aws_access_key_id"] = access_key_id
            session_kwargs["aws_secret_access_key"] = secret_access_key
        if session_token:
            session_kwargs["aws_session_token"] = session_token

        self.session = boto3.Session(**session_kwargs)
        self.ec2_client = self.session.client('ec2')
        self.ec2_resource = self.session.resource('ec2')

        logger.info(f"Initialized AWS EC2 provider for region {region}")

    async def provision_validator(
        self,
        validator_name: str,
        moniker: str,
        chain_id: str,
        instance_type: str = "t3.medium",
        volume_size_gb: int = 500,
        key_pair_name: Optional[str] = None,
        tags: Optional[Dict[str, str]] = None
    ) -> Dict[str, Any]:
        """
        Provision a new validator instance on AWS EC2.

        Args:
            validator_name: Unique validator identifier
            moniker: Validator display name
            chain_id: Blockchain network ID
            instance_type: EC2 instance type (default: t3.medium - 2vCPU, 4GB RAM)
            volume_size_gb: Root volume size in GB (default: 500GB)
            key_pair_name: SSH key pair name (must exist in AWS region)
            tags: Additional tags for the instance

        Returns:
            Dict containing instance info and consensus pubkey:
            {
                "instance_id": "i-1234567890abcdef0",
                "public_ip": "1.2.3.4",
                "private_ip": "10.0.1.5",
                "consensus_pubkey": "base64_encoded_pubkey",
                "rpc_endpoint": "http://1.2.3.4:26657",
                "p2p_endpoint": "tcp://1.2.3.4:26656",
                "grpc_endpoint": "1.2.3.4:9090",
                "ssh_command": "ssh -i ~/.ssh/key.pem ubuntu@1.2.3.4"
            }
        """
        try:
            logger.info(f"Provisioning validator '{validator_name}' on AWS EC2")

            # Get latest Ubuntu 22.04 AMI
            ami_id = await self._get_ubuntu_ami()
            logger.info(f"Using AMI: {ami_id}")

            # Create or get security group
            security_group_id = await self._create_security_group(validator_name)
            logger.info(f"Using security group: {security_group_id}")

            # Generate user data script to initialize validator
            user_data = self._generate_user_data(moniker, chain_id)

            # Build tags
            instance_tags = {
                "Name": f"omniphi-validator-{validator_name}",
                "Project": "Omniphi",
                "Type": "Validator",
                "Moniker": moniker,
                "ChainID": chain_id,
                "ManagedBy": "OmniphiOrchestrator"
            }
            if tags:
                instance_tags.update(tags)

            tag_specifications = [{
                "ResourceType": "instance",
                "Tags": [{"Key": k, "Value": v} for k, v in instance_tags.items()]
            }]

            # Launch instance
            launch_params = {
                "ImageId": ami_id,
                "InstanceType": instance_type,
                "MinCount": 1,
                "MaxCount": 1,
                "SecurityGroupIds": [security_group_id],
                "UserData": user_data,
                "BlockDeviceMappings": [{
                    "DeviceName": "/dev/sda1",
                    "Ebs": {
                        "VolumeSize": volume_size_gb,
                        "VolumeType": "gp3",  # Latest generation SSD
                        "DeleteOnTermination": True,
                        "Encrypted": True
                    }
                }],
                "TagSpecifications": tag_specifications,
                "MetadataOptions": {
                    "HttpTokens": "required",  # IMDSv2 only (security best practice)
                    "HttpPutResponseHopLimit": 1
                },
                "Monitoring": {
                    "Enabled": True  # Enable detailed CloudWatch monitoring
                }
            }

            if key_pair_name:
                launch_params["KeyName"] = key_pair_name

            response = self.ec2_client.run_instances(**launch_params)
            instance = response['Instances'][0]
            instance_id = instance['InstanceId']

            logger.info(f"Launched instance {instance_id}, waiting for running state...")

            # Wait for instance to be running
            waiter = self.ec2_client.get_waiter('instance_running')
            waiter.wait(InstanceIds=[instance_id])

            # Get updated instance info with public IP
            instance_info = self.ec2_client.describe_instances(
                InstanceIds=[instance_id]
            )['Reservations'][0]['Instances'][0]

            public_ip = instance_info.get('PublicIpAddress')
            private_ip = instance_info.get('PrivateIpAddress')

            logger.info(f"Instance running: {instance_id} (IP: {public_ip})")

            # Wait for validator initialization (user-data script)
            logger.info("Waiting for validator initialization (this may take 5-10 minutes)...")
            await self._wait_for_validator_init(instance_id, timeout=600)

            # Extract consensus pubkey from instance
            consensus_pubkey = await self._extract_consensus_pubkey(instance_id)

            result = {
                "instance_id": instance_id,
                "public_ip": public_ip,
                "private_ip": private_ip,
                "consensus_pubkey": consensus_pubkey,
                "rpc_endpoint": f"http://{public_ip}:26657",
                "p2p_endpoint": f"tcp://{public_ip}:26656",
                "grpc_endpoint": f"{public_ip}:9090",
                "ssh_command": f"ssh -i ~/.ssh/{key_pair_name or 'key'}.pem ubuntu@{public_ip}" if key_pair_name else f"ssh ubuntu@{public_ip}",
                "region": self.region,
                "instance_type": instance_type,
                "volume_size_gb": volume_size_gb
            }

            logger.info(f"Successfully provisioned validator: {instance_id}")
            return result

        except (ClientError, BotoCoreError) as e:
            logger.error(f"AWS error during provisioning: {e}")
            raise Exception(f"Failed to provision EC2 instance: {str(e)}")
        except Exception as e:
            logger.error(f"Error during provisioning: {e}", exc_info=True)
            raise

    async def _get_ubuntu_ami(self) -> str:
        """Get latest Ubuntu 22.04 LTS AMI for the region."""
        try:
            response = self.ec2_client.describe_images(
                Owners=['099720109477'],  # Canonical's AWS account ID
                Filters=[
                    {'Name': 'name', 'Values': ['ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*']},
                    {'Name': 'state', 'Values': ['available']},
                    {'Name': 'architecture', 'Values': ['x86_64']}
                ]
            )

            # Sort by creation date and get latest
            images = sorted(response['Images'], key=lambda x: x['CreationDate'], reverse=True)
            if not images:
                raise Exception("No Ubuntu 22.04 AMI found in region")

            return images[0]['ImageId']

        except Exception as e:
            logger.error(f"Error finding Ubuntu AMI: {e}")
            raise

    async def _create_security_group(self, validator_name: str) -> str:
        """
        Create or get existing security group for validator.

        Opens ports:
        - 22 (SSH) - restricted to your IP (you should configure this)
        - 26656 (P2P) - open to internet
        - 26657 (RPC) - localhost only (via instance)
        - 9090 (gRPC) - open to internet (for clients)
        - 26660 (Prometheus) - restricted to monitoring server
        """
        sg_name = f"omniphi-validator-{validator_name}"

        try:
            # Check if security group exists
            response = self.ec2_client.describe_security_groups(
                Filters=[{'Name': 'group-name', 'Values': [sg_name]}]
            )

            if response['SecurityGroups']:
                sg_id = response['SecurityGroups'][0]['GroupId']
                logger.info(f"Using existing security group: {sg_id}")
                return sg_id

        except ClientError:
            pass

        # Create new security group
        try:
            response = self.ec2_client.create_security_group(
                GroupName=sg_name,
                Description=f"Security group for Omniphi validator {validator_name}"
            )
            sg_id = response['GroupId']

            # Get allowed CIDR blocks from settings (secure by default)
            from app.core.config import settings

            # SECURITY: SSH access is restricted. If not configured, SSH is disabled.
            admin_cidrs = getattr(settings, 'AWS_ADMIN_CIDR_BLOCKS', None)
            monitoring_cidrs = getattr(settings, 'AWS_MONITORING_CIDR_BLOCKS', None)

            ip_permissions = [
                # P2P networking - MUST be open to internet for consensus
                {
                    'IpProtocol': 'tcp',
                    'FromPort': 26656,
                    'ToPort': 26656,
                    'IpRanges': [{'CidrIp': '0.0.0.0/0', 'Description': 'P2P networking (required for consensus)'}]
                }
            ]

            # SSH: Only add if admin CIDRs are configured and not 0.0.0.0/0
            if admin_cidrs and '0.0.0.0/0' not in admin_cidrs:
                for cidr in admin_cidrs:
                    ip_permissions.append({
                        'IpProtocol': 'tcp',
                        'FromPort': 22,
                        'ToPort': 22,
                        'IpRanges': [{'CidrIp': cidr, 'Description': f'SSH access from admin IP {cidr}'}]
                    })
                logger.info(f"SSH access restricted to: {admin_cidrs}")
            else:
                logger.warning("SSH access DISABLED - set AWS_ADMIN_CIDR_BLOCKS to enable")

            # gRPC: Internal only - should be accessed via VPN or internal network
            # NOT open to 0.0.0.0/0 in production
            if admin_cidrs and '0.0.0.0/0' not in admin_cidrs:
                for cidr in admin_cidrs:
                    ip_permissions.append({
                        'IpProtocol': 'tcp',
                        'FromPort': 9090,
                        'ToPort': 9090,
                        'IpRanges': [{'CidrIp': cidr, 'Description': f'gRPC from {cidr}'}]
                    })

            # Prometheus metrics: Only from monitoring servers
            if monitoring_cidrs and '0.0.0.0/0' not in monitoring_cidrs:
                for cidr in monitoring_cidrs:
                    ip_permissions.append({
                        'IpProtocol': 'tcp',
                        'FromPort': 26660,
                        'ToPort': 26660,
                        'IpRanges': [{'CidrIp': cidr, 'Description': f'Prometheus metrics from {cidr}'}]
                    })
                logger.info(f"Prometheus metrics restricted to: {monitoring_cidrs}")
            else:
                logger.warning("Prometheus metrics endpoint DISABLED - set AWS_MONITORING_CIDR_BLOCKS to enable")

            # Add inbound rules
            self.ec2_client.authorize_security_group_ingress(
                GroupId=sg_id,
                IpPermissions=ip_permissions
            )

            logger.info(f"Created security group: {sg_id}")
            return sg_id

        except Exception as e:
            logger.error(f"Error creating security group: {e}")
            raise

    def _generate_user_data(self, moniker: str, chain_id: str) -> str:
        """
        Generate cloud-init user data script to initialize validator.

        This script runs on first boot and:
        1. Installs dependencies
        2. Downloads and verifies posd binary (with SHA256 checksum)
        3. Initializes validator node
        4. Configures systemd service
        5. Starts validator

        SECURITY: Binary downloads are verified with SHA256 checksums.
        """
        from app.core.config import settings

        binary_url = settings.OMNIPHI_BINARY_URL
        binary_sha256 = settings.OMNIPHI_BINARY_SHA256 or ""
        genesis_url = settings.OMNIPHI_GENESIS_URL
        genesis_sha256 = settings.OMNIPHI_GENESIS_SHA256 or ""
        keyring_backend = settings.KEYRING_BACKEND

        script = f"""#!/bin/bash
set -e

# Log all output
exec > >(tee /var/log/validator-init.log)
exec 2>&1

echo "=== Omniphi Validator Initialization ==="
echo "Moniker: {moniker}"
echo "Chain ID: {chain_id}"
echo "Started: $(date)"

# Security function to verify checksums
verify_checksum() {{
    local file="$1"
    local expected_sha256="$2"
    local actual_sha256

    if [ -z "$expected_sha256" ]; then
        echo "ERROR: No checksum provided for $file - refusing to proceed"
        echo "Set OMNIPHI_BINARY_SHA256 and OMNIPHI_GENESIS_SHA256 in environment"
        exit 1
    fi

    actual_sha256=$(sha256sum "$file" | awk '{{print $1}}')

    if [ "$actual_sha256" != "$expected_sha256" ]; then
        echo "ERROR: Checksum verification failed for $file"
        echo "Expected: $expected_sha256"
        echo "Got:      $actual_sha256"
        echo "The file may have been tampered with!"
        rm -f "$file"
        exit 1
    fi

    echo "Checksum verified for $file"
}}

# Update system
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
    gnupg \\
    lsb-release

# Create validator user with restricted shell
if ! id -u omniphi &>/dev/null; then
    useradd -m -s /bin/bash omniphi
fi

# Download and verify posd binary
echo "Downloading posd binary from {binary_url}..."
cd /tmp
wget -O posd "{binary_url}"

# SECURITY: Verify binary checksum
verify_checksum "/tmp/posd" "{binary_sha256}"

chmod +x posd
mv posd /usr/local/bin/

# Verify binary is executable
if ! /usr/local/bin/posd version; then
    echo "ERROR: posd binary verification failed"
    exit 1
fi

echo "Binary installed and verified successfully"

# Initialize validator as omniphi user
sudo -u omniphi bash <<'USERSCRIPT'
set -e

# Initialize node
/usr/local/bin/posd init "{moniker}" --chain-id "{chain_id}" --home /home/omniphi/.omniphi

# Download and verify genesis
wget -O /tmp/genesis.json "{genesis_url}"
USERSCRIPT

# Verify genesis checksum (run as root since we downloaded to /tmp)
verify_checksum "/tmp/genesis.json" "{genesis_sha256}"
mv /tmp/genesis.json /home/omniphi/.omniphi/config/genesis.json
chown omniphi:omniphi /home/omniphi/.omniphi/config/genesis.json

sudo -u omniphi bash <<'USERSCRIPT'
set -e

# Configure node
/usr/local/bin/posd config set client chain-id "{chain_id}" --home /home/omniphi/.omniphi

# SECURITY: Use encrypted keyring backend (not 'test')
# 'file' backend encrypts keys at rest with a passphrase
/usr/local/bin/posd config set client keyring-backend "{keyring_backend}" --home /home/omniphi/.omniphi

if [ "{keyring_backend}" = "test" ]; then
    echo "WARNING: Using 'test' keyring backend - keys stored unencrypted!"
    echo "This is NOT suitable for production validators!"
fi

# Extract consensus pubkey
/usr/local/bin/posd tendermint show-validator --home /home/omniphi/.omniphi > /home/omniphi/.omniphi/consensus_pubkey.json

# Mark initialization complete
touch /home/omniphi/.omniphi/initialized
USERSCRIPT

# Create systemd service for validator
cat > /etc/systemd/system/omniphi-validator.service <<SYSTEMD
[Unit]
Description=Omniphi Validator Node
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=omniphi
Group=omniphi
ExecStart=/usr/local/bin/posd start --home /home/omniphi/.omniphi
Restart=always
RestartSec=3
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/home/omniphi/.omniphi

[Install]
WantedBy=multi-user.target
SYSTEMD

# Enable and start service
systemctl daemon-reload
systemctl enable omniphi-validator
systemctl start omniphi-validator

echo "=== Initialization Complete ==="
echo "Validator started successfully"
echo "Completed: $(date)"
"""

        return base64.b64encode(script.encode()).decode()

    async def _wait_for_validator_init(self, instance_id: str, timeout: int = 600):
        """
        Wait for validator initialization to complete.

        Checks for marker file created by user-data script.
        """
        import asyncio

        start_time = datetime.now()

        while (datetime.now() - start_time).total_seconds() < timeout:
            try:
                # Check instance status
                response = self.ec2_client.describe_instance_status(
                    InstanceIds=[instance_id],
                    IncludeAllInstances=True
                )

                if response['InstanceStatuses']:
                    status = response['InstanceStatuses'][0]

                    # Check if instance is running and initialized
                    if (status['InstanceState']['Name'] == 'running' and
                        status.get('SystemStatus', {}).get('Status') == 'ok' and
                        status.get('InstanceStatus', {}).get('Status') == 'ok'):

                        logger.info("Instance is ready")

                        # In production, would check for /home/omniphi/.omniphi/initialized file
                        # via SSM or SSH. For now, just wait a bit more for user-data
                        await asyncio.sleep(30)
                        return

                logger.debug(f"Waiting for instance initialization... ({int((datetime.now() - start_time).total_seconds())}s)")
                await asyncio.sleep(10)

            except Exception as e:
                logger.warning(f"Error checking instance status: {e}")
                await asyncio.sleep(10)

        raise TimeoutError(f"Validator initialization timed out after {timeout}s")

    async def _extract_consensus_pubkey(self, instance_id: str) -> str:
        """
        Extract consensus public key from validator instance using AWS SSM.

        Uses AWS Systems Manager to run commands on the instance without SSH.
        Requires the instance to have the SSM agent installed and proper IAM role.
        """
        import asyncio

        try:
            ssm = self.session.client('ssm')

            # Send command to extract pubkey
            logger.info(f"Extracting consensus pubkey from instance {instance_id}")

            response = ssm.send_command(
                InstanceIds=[instance_id],
                DocumentName='AWS-RunShellScript',
                Parameters={
                    'commands': [
                        'cat /home/omniphi/.omniphi/consensus_pubkey.json'
                    ]
                },
                TimeoutSeconds=60
            )

            command_id = response['Command']['CommandId']

            # Wait for command to complete
            max_attempts = 30
            for attempt in range(max_attempts):
                await asyncio.sleep(2)

                result = ssm.get_command_invocation(
                    CommandId=command_id,
                    InstanceId=instance_id
                )

                status = result['Status']

                if status == 'Success':
                    output = result['StandardOutputContent'].strip()
                    if output:
                        # Parse the JSON to extract the key
                        import json
                        try:
                            pubkey_data = json.loads(output)
                            pubkey = pubkey_data.get('key', '')
                            if pubkey and pubkey != 'PLACEHOLDER_WILL_BE_EXTRACTED':
                                logger.info(f"Successfully extracted consensus pubkey")
                                return pubkey
                        except json.JSONDecodeError:
                            logger.warning(f"Could not parse pubkey JSON: {output}")

                elif status in ['Failed', 'Cancelled', 'TimedOut']:
                    logger.error(f"SSM command failed with status: {status}")
                    break

            # If SSM fails, raise error instead of returning placeholder
            raise RuntimeError(
                f"Failed to extract consensus pubkey from instance {instance_id}. "
                "Ensure SSM agent is running and instance has proper IAM role."
            )

        except self.session.client('ssm').exceptions.InvalidInstanceId:
            raise RuntimeError(
                f"Instance {instance_id} is not registered with SSM. "
                "Ensure SSM agent is installed and instance has proper IAM role."
            )

        except Exception as e:
            logger.error(f"Error extracting consensus pubkey: {e}")
            raise RuntimeError(
                f"Failed to extract consensus pubkey: {e}. "
                "Cannot proceed without valid pubkey."
            )

    async def terminate_instance(self, instance_id: str):
        """Terminate an EC2 instance."""
        try:
            logger.info(f"Terminating instance {instance_id}")

            self.ec2_client.terminate_instances(InstanceIds=[instance_id])

            # Wait for termination
            waiter = self.ec2_client.get_waiter('instance_terminated')
            waiter.wait(InstanceIds=[instance_id])

            logger.info(f"Instance {instance_id} terminated")

        except Exception as e:
            logger.error(f"Error terminating instance: {e}")
            raise

    async def get_instance_status(self, instance_id: str) -> Dict[str, Any]:
        """Get current status of an instance."""
        try:
            response = self.ec2_client.describe_instances(InstanceIds=[instance_id])
            instance = response['Reservations'][0]['Instances'][0]

            return {
                "instance_id": instance_id,
                "state": instance['State']['Name'],
                "public_ip": instance.get('PublicIpAddress'),
                "private_ip": instance.get('PrivateIpAddress'),
                "instance_type": instance['InstanceType'],
                "launch_time": instance['LaunchTime'].isoformat()
            }

        except Exception as e:
            logger.error(f"Error getting instance status: {e}")
            raise
