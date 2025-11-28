"""
Validator CRUD Repositories

Repositories for validator setup requests, nodes, and heartbeats.
"""

from datetime import datetime, timedelta
from typing import List, Optional
from uuid import UUID

from sqlalchemy import and_, desc, or_
from sqlalchemy.orm import Session

from app.db.crud.base import BaseRepository
from app.db.models.validator_setup_request import ValidatorSetupRequest
from app.db.models.validator_node import ValidatorNode
from app.db.models.local_validator_heartbeat import LocalValidatorHeartbeat
from app.db.models.enums import SetupStatus, NodeStatus, RunMode


class ValidatorSetupRequestRepository(BaseRepository[ValidatorSetupRequest]):
    """Repository for ValidatorSetupRequest model operations."""

    def __init__(self, db: Session):
        super().__init__(ValidatorSetupRequest, db)

    def get_by_wallet(self, wallet_address: str) -> List[ValidatorSetupRequest]:
        """Get all setup requests for a wallet."""
        return (
            self.db.query(ValidatorSetupRequest)
            .filter(ValidatorSetupRequest.wallet_address == wallet_address)
            .order_by(desc(ValidatorSetupRequest.created_at))
            .all()
        )

    def get_active_by_wallet(self, wallet_address: str) -> List[ValidatorSetupRequest]:
        """Get active (non-failed, non-cancelled) requests for a wallet."""
        return (
            self.db.query(ValidatorSetupRequest)
            .filter(
                ValidatorSetupRequest.wallet_address == wallet_address,
                ValidatorSetupRequest.status.notin_([
                    SetupStatus.FAILED.value,
                    SetupStatus.CANCELLED.value,
                ]),
            )
            .order_by(desc(ValidatorSetupRequest.created_at))
            .all()
        )

    def get_by_consensus_pubkey(self, pubkey: str) -> Optional[ValidatorSetupRequest]:
        """Get request by consensus public key."""
        return (
            self.db.query(ValidatorSetupRequest)
            .filter(ValidatorSetupRequest.consensus_pubkey == pubkey)
            .first()
        )

    def get_by_status(self, status: str, limit: int = 100) -> List[ValidatorSetupRequest]:
        """Get requests by status."""
        return (
            self.db.query(ValidatorSetupRequest)
            .filter(ValidatorSetupRequest.status == status)
            .order_by(ValidatorSetupRequest.created_at)
            .limit(limit)
            .all()
        )

    def get_pending(self, limit: int = 100) -> List[ValidatorSetupRequest]:
        """Get pending requests ready for processing."""
        return self.get_by_status(SetupStatus.PENDING.value, limit)

    def get_provisioning(self) -> List[ValidatorSetupRequest]:
        """Get requests currently being provisioned."""
        return self.get_by_status(SetupStatus.PROVISIONING.value)

    def get_by_provider(self, provider_id: UUID) -> List[ValidatorSetupRequest]:
        """Get requests for a specific provider."""
        return (
            self.db.query(ValidatorSetupRequest)
            .filter(ValidatorSetupRequest.provider_id == provider_id)
            .order_by(desc(ValidatorSetupRequest.created_at))
            .all()
        )

    def get_by_region(self, region_id: UUID) -> List[ValidatorSetupRequest]:
        """Get requests for a specific region."""
        return (
            self.db.query(ValidatorSetupRequest)
            .filter(ValidatorSetupRequest.region_id == region_id)
            .order_by(desc(ValidatorSetupRequest.created_at))
            .all()
        )

    def get_retryable(self) -> List[ValidatorSetupRequest]:
        """Get failed requests that can be retried."""
        return (
            self.db.query(ValidatorSetupRequest)
            .filter(
                ValidatorSetupRequest.status == SetupStatus.FAILED.value,
                ValidatorSetupRequest.retry_count < ValidatorSetupRequest.max_retries,
            )
            .order_by(ValidatorSetupRequest.created_at)
            .all()
        )

    def set_status(
        self,
        id: UUID,
        status: SetupStatus,
        message: Optional[str] = None,
        error: Optional[str] = None,
    ) -> Optional[ValidatorSetupRequest]:
        """Update request status."""
        request = self.get(id)
        if not request:
            return None

        request.set_status(status, message, error)
        self.db.commit()
        self.db.refresh(request)
        return request

    def set_consensus_pubkey(self, id: UUID, pubkey: str) -> Optional[ValidatorSetupRequest]:
        """Set consensus public key after provisioning."""
        request = self.get(id)
        if not request:
            return None

        request.consensus_pubkey = pubkey
        self.db.commit()
        self.db.refresh(request)
        return request

    def retry(self, id: UUID) -> Optional[ValidatorSetupRequest]:
        """Retry a failed request."""
        request = self.get(id)
        if not request or not request.increment_retry():
            return None

        self.db.commit()
        self.db.refresh(request)
        return request

    def count_by_status(self) -> dict:
        """Get count of requests by status."""
        from sqlalchemy import func

        results = (
            self.db.query(ValidatorSetupRequest.status, func.count(ValidatorSetupRequest.id))
            .group_by(ValidatorSetupRequest.status)
            .all()
        )
        return {status: count for status, count in results}


class ValidatorNodeRepository(BaseRepository[ValidatorNode]):
    """Repository for ValidatorNode model operations."""

    def __init__(self, db: Session):
        super().__init__(ValidatorNode, db)

    def get_by_setup_request(self, setup_request_id: UUID) -> List[ValidatorNode]:
        """Get all nodes for a setup request."""
        return (
            self.db.query(ValidatorNode)
            .filter(ValidatorNode.setup_request_id == setup_request_id)
            .order_by(desc(ValidatorNode.created_at))
            .all()
        )

    def get_active_by_setup_request(self, setup_request_id: UUID) -> Optional[ValidatorNode]:
        """Get active node for a setup request."""
        return (
            self.db.query(ValidatorNode)
            .filter(
                ValidatorNode.setup_request_id == setup_request_id,
                ValidatorNode.is_active == True,
            )
            .first()
        )

    def get_by_container_id(self, container_id: str) -> Optional[ValidatorNode]:
        """Get node by container ID."""
        return (
            self.db.query(ValidatorNode)
            .filter(ValidatorNode.container_id == container_id)
            .first()
        )

    def get_by_status(self, status: str) -> List[ValidatorNode]:
        """Get nodes by status."""
        return (
            self.db.query(ValidatorNode)
            .filter(ValidatorNode.status == status)
            .all()
        )

    def get_running(self) -> List[ValidatorNode]:
        """Get all running nodes."""
        return (
            self.db.query(ValidatorNode)
            .filter(
                ValidatorNode.is_active == True,
                ValidatorNode.status.in_([
                    NodeStatus.RUNNING.value,
                    NodeStatus.SYNCING.value,
                    NodeStatus.SYNCED.value,
                ]),
            )
            .all()
        )

    def get_by_region(self, region_id: UUID) -> List[ValidatorNode]:
        """Get nodes in a region."""
        return (
            self.db.query(ValidatorNode)
            .filter(ValidatorNode.region_id == region_id)
            .all()
        )

    def get_by_server(self, server_id: UUID) -> List[ValidatorNode]:
        """Get nodes on a server."""
        return (
            self.db.query(ValidatorNode)
            .filter(ValidatorNode.server_id == server_id)
            .all()
        )

    def get_unhealthy(self, threshold: float = 50.0) -> List[ValidatorNode]:
        """Get nodes with low health score."""
        return (
            self.db.query(ValidatorNode)
            .filter(
                ValidatorNode.is_active == True,
                ValidatorNode.health_score < threshold,
            )
            .all()
        )

    def get_jailed(self) -> List[ValidatorNode]:
        """Get jailed validator nodes."""
        return (
            self.db.query(ValidatorNode)
            .filter(
                ValidatorNode.is_active == True,
                ValidatorNode.is_jailed == True,
            )
            .all()
        )

    def get_stale(self, minutes: int = 5) -> List[ValidatorNode]:
        """Get nodes with stale heartbeats."""
        threshold = datetime.utcnow() - timedelta(minutes=minutes)
        return (
            self.db.query(ValidatorNode)
            .filter(
                ValidatorNode.is_active == True,
                ValidatorNode.last_heartbeat < threshold,
            )
            .all()
        )

    def set_status(self, id: UUID, status: NodeStatus) -> Optional[ValidatorNode]:
        """Update node status."""
        node = self.get(id)
        if not node:
            return None

        node.set_status(status)
        self.db.commit()
        self.db.refresh(node)
        return node

    def update_heartbeat(self, id: UUID) -> Optional[ValidatorNode]:
        """Update node heartbeat."""
        node = self.get(id)
        if not node:
            return None

        node.update_heartbeat()
        self.db.commit()
        self.db.refresh(node)
        return node

    def update_chain_status(
        self,
        id: UUID,
        block_height: int,
        peer_count: int,
        catching_up: bool,
        synced: bool,
    ) -> Optional[ValidatorNode]:
        """Update node chain sync status."""
        node = self.get(id)
        if not node:
            return None

        node.update_chain_status(block_height, peer_count, catching_up, synced)
        self.db.commit()
        self.db.refresh(node)
        return node


class LocalValidatorHeartbeatRepository(BaseRepository[LocalValidatorHeartbeat]):
    """Repository for LocalValidatorHeartbeat model operations."""

    def __init__(self, db: Session):
        super().__init__(LocalValidatorHeartbeat, db)

    def get_by_wallet(self, wallet_address: str) -> List[LocalValidatorHeartbeat]:
        """Get all heartbeats for a wallet."""
        return (
            self.db.query(LocalValidatorHeartbeat)
            .filter(LocalValidatorHeartbeat.wallet_address == wallet_address)
            .order_by(desc(LocalValidatorHeartbeat.last_seen))
            .all()
        )

    def get_by_consensus_pubkey(self, pubkey: str) -> Optional[LocalValidatorHeartbeat]:
        """Get heartbeat by consensus public key."""
        return (
            self.db.query(LocalValidatorHeartbeat)
            .filter(LocalValidatorHeartbeat.consensus_pubkey == pubkey)
            .first()
        )

    def get_online(self, minutes: int = 5) -> List[LocalValidatorHeartbeat]:
        """Get validators with recent heartbeats."""
        threshold = datetime.utcnow() - timedelta(minutes=minutes)
        return (
            self.db.query(LocalValidatorHeartbeat)
            .filter(LocalValidatorHeartbeat.last_seen >= threshold)
            .all()
        )

    def get_active_validators(self) -> List[LocalValidatorHeartbeat]:
        """Get active (on-chain) local validators."""
        threshold = datetime.utcnow() - timedelta(minutes=5)
        return (
            self.db.query(LocalValidatorHeartbeat)
            .filter(
                LocalValidatorHeartbeat.is_active_validator == True,
                LocalValidatorHeartbeat.last_seen >= threshold,
            )
            .all()
        )

    def upsert(self, data: dict) -> LocalValidatorHeartbeat:
        """Create or update heartbeat by consensus pubkey."""
        existing = self.get_by_consensus_pubkey(data.get("consensus_pubkey"))

        if existing:
            for key, value in data.items():
                if hasattr(existing, key) and value is not None:
                    setattr(existing, key, value)
            existing.last_seen = datetime.utcnow()
            self.db.commit()
            self.db.refresh(existing)
            return existing
        else:
            data["first_seen"] = datetime.utcnow()
            data["last_seen"] = datetime.utcnow()
            return self.create(data)

    def cleanup_stale(self, days: int = 7) -> int:
        """Remove heartbeats not seen for specified days."""
        threshold = datetime.utcnow() - timedelta(days=days)
        result = (
            self.db.query(LocalValidatorHeartbeat)
            .filter(LocalValidatorHeartbeat.last_seen < threshold)
            .delete(synchronize_session=False)
        )
        self.db.commit()
        return result
