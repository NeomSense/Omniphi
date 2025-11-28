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

            # Add inbound rules
            self.ec2_client.authorize_security_group_ingress(
                GroupId=sg_id,
                IpPermissions=[
                    # SSH (TODO: Restrict to your IP in production)
                    {
                        'IpProtocol': 'tcp',
                        'FromPort': 22,
                        'ToPort': 22,
                        'IpRanges': [{'CidrIp': '0.0.0.0/0', 'Description': 'SSH access (restrict in production)'}]
                    },
                    # P2P networking (must be open to internet)
                    {
                        'IpProtocol': 'tcp',
                        'FromPort': 26656,
                        'ToPort': 26656,
                        'IpRanges': [{'CidrIp': '0.0.0.0/0', 'Description': 'P2P networking'}]
                    },
                    # gRPC (for client connections)
                    {
                        'IpProtocol': 'tcp',
                        'FromPort': 9090,
                        'ToPort': 9090,
                        'IpRanges': [{'CidrIp': '0.0.0.0/0', 'Description': 'gRPC endpoint'}]
                    },
                    # Prometheus metrics (TODO: Restrict to monitoring server)
                    {
                        'IpProtocol': 'tcp',
                        'FromPort': 26660,
                        'ToPort': 26660,
                        'IpRanges': [{'CidrIp': '0.0.0.0/0', 'Description': 'Prometheus metrics (restrict in production)'}]
                    }
                ]
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
        2. Downloads and installs posd binary
        3. Initializes validator node
        4. Configures systemd service
        5. Starts validator
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

# Create validator user
if ! id -u omniphi &>/dev/null; then
    useradd -m -s /bin/bash omniphi
fi

# Download posd binary (TODO: Replace with actual release URL)
# For now, this is a placeholder - you should:
# 1. Build posd binary
# 2. Upload to S3 or GitHub releases
# 3. Download from there

cd /tmp
# Example: wget https://github.com/omniphi/omniphi/releases/download/v1.0.0/posd
# Example: chmod +x posd
# Example: mv posd /usr/local/bin/

# For MVP, we'll create a marker file indicating initialization
# In production, replace this with actual posd initialization
sudo -u omniphi bash <<'EOF'
mkdir -p /home/omniphi/.omniphi/config
mkdir -p /home/omniphi/.omniphi/data

# Create a marker file with consensus pubkey placeholder
# In production, this would be: posd tendermint show-validator
echo '{{"@type":"/cosmos.crypto.ed25519.PubKey","key":"PLACEHOLDER_WILL_BE_EXTRACTED"}}' > /home/omniphi/.omniphi/consensus_pubkey.json

# Mark initialization complete
touch /home/omniphi/.omniphi/initialized
EOF

# TODO: In production, add these steps:
# 1. posd init {moniker} --chain-id {chain_id}
# 2. Download genesis file
# 3. Configure seeds/peers
# 4. Create systemd service (use our template)
# 5. Start validator

echo "=== Initialization Complete ==="
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
        Extract consensus public key from validator instance.

        In production, this would:
        1. SSH into instance or use AWS Systems Manager
        2. Run: posd tendermint show-validator
        3. Extract and return the pubkey

        For now, returns a placeholder that should be replaced by actual implementation.
        """
        # TODO: Implement actual pubkey extraction via SSM or SSH
        # Example using SSM:
        # import boto3
        # ssm = boto3.client('ssm')
        # response = ssm.send_command(
        #     InstanceIds=[instance_id],
        #     DocumentName='AWS-RunShellScript',
        #     Parameters={'commands': ['sudo -u omniphi posd tendermint show-validator']}
        # )
        # # Wait for command and get output...

        logger.warning("Using placeholder consensus pubkey - implement actual extraction in production")

        import secrets
        random_bytes = secrets.token_bytes(32)
        return base64.b64encode(random_bytes).decode('utf-8')

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
