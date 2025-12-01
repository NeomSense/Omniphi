"""
Upgrade CRUD Repositories

Repositories for chain upgrade and rollout operations.
"""

from datetime import datetime, timedelta
from typing import List, Optional
from uuid import UUID

from sqlalchemy import and_, desc, func, or_
from sqlalchemy.orm import Session

from app.db.crud.base import BaseRepository
from app.db.models.upgrade import Upgrade
from app.db.models.upgrade_rollout import UpgradeRollout
from app.db.models.enums import UpgradeStatus, RolloutStatus


class UpgradeRepository(BaseRepository[Upgrade]):
    """Repository for Upgrade model operations."""

    def __init__(self, db: Session):
        super().__init__(Upgrade, db)

    def get_by_chain(self, chain_id: str) -> List[Upgrade]:
        """Get all upgrades for a chain."""
        return (
            self.db.query(Upgrade)
            .filter(Upgrade.chain_id == chain_id)
            .order_by(desc(Upgrade.upgrade_height))
            .all()
        )

    def get_by_name(self, chain_id: str, name: str) -> Optional[Upgrade]:
        """Get upgrade by chain and name."""
        return (
            self.db.query(Upgrade)
            .filter(
                Upgrade.chain_id == chain_id,
                Upgrade.name == name,
            )
            .first()
        )

    def get_by_height(self, chain_id: str, height: int) -> Optional[Upgrade]:
        """Get upgrade at a specific block height."""
        return (
            self.db.query(Upgrade)
            .filter(
                Upgrade.chain_id == chain_id,
                Upgrade.upgrade_height == height,
            )
            .first()
        )

    def get_pending(self, chain_id: Optional[str] = None) -> List[Upgrade]:
        """Get pending upgrades."""
        q = self.db.query(Upgrade).filter(
            Upgrade.status.in_([
                UpgradeStatus.PENDING.value,
                UpgradeStatus.SCHEDULED.value,
            ])
        )

        if chain_id:
            q = q.filter(Upgrade.chain_id == chain_id)

        return q.order_by(Upgrade.scheduled_time).all()

    def get_in_progress(self, chain_id: Optional[str] = None) -> List[Upgrade]:
        """Get upgrades currently in progress."""
        q = self.db.query(Upgrade).filter(
            Upgrade.status == UpgradeStatus.IN_PROGRESS.value
        )

        if chain_id:
            q = q.filter(Upgrade.chain_id == chain_id)

        return q.all()

    def get_upcoming(self, hours: int = 48) -> List[Upgrade]:
        """Get upgrades scheduled within specified hours."""
        threshold = datetime.utcnow() + timedelta(hours=hours)
        return (
            self.db.query(Upgrade)
            .filter(
                Upgrade.status.in_([
                    UpgradeStatus.PENDING.value,
                    UpgradeStatus.SCHEDULED.value,
                ]),
                Upgrade.scheduled_time <= threshold,
            )
            .order_by(Upgrade.scheduled_time)
            .all()
        )

    def get_completed(
        self,
        chain_id: str,
        limit: int = 10,
    ) -> List[Upgrade]:
        """Get completed upgrades for a chain."""
        return (
            self.db.query(Upgrade)
            .filter(
                Upgrade.chain_id == chain_id,
                Upgrade.status == UpgradeStatus.COMPLETED.value,
            )
            .order_by(desc(Upgrade.completed_at))
            .limit(limit)
            .all()
        )

    def get_failed(self, chain_id: Optional[str] = None) -> List[Upgrade]:
        """Get failed upgrades."""
        q = self.db.query(Upgrade).filter(
            Upgrade.status == UpgradeStatus.FAILED.value
        )

        if chain_id:
            q = q.filter(Upgrade.chain_id == chain_id)

        return q.order_by(desc(Upgrade.created_at)).all()

    def set_status(
        self,
        id: UUID,
        status: UpgradeStatus,
        error_message: Optional[str] = None,
    ) -> Optional[Upgrade]:
        """Update upgrade status."""
        upgrade = self.get(id)
        if not upgrade:
            return None

        upgrade.status = status.value

        if status == UpgradeStatus.IN_PROGRESS:
            upgrade.started_at = datetime.utcnow()
        elif status == UpgradeStatus.COMPLETED:
            upgrade.completed_at = datetime.utcnow()
        elif status == UpgradeStatus.FAILED:
            upgrade.failed_at = datetime.utcnow()
            upgrade.error_message = error_message
        elif status == UpgradeStatus.ROLLED_BACK:
            upgrade.rolled_back_at = datetime.utcnow()

        self.db.commit()
        self.db.refresh(upgrade)
        return upgrade

    def update_progress(
        self,
        id: UUID,
        nodes_upgraded: int,
        nodes_failed: int,
    ) -> Optional[Upgrade]:
        """Update upgrade progress."""
        upgrade = self.get(id)
        if not upgrade:
            return None

        upgrade.nodes_upgraded = nodes_upgraded
        upgrade.nodes_failed = nodes_failed

        if upgrade.total_nodes and upgrade.total_nodes > 0:
            upgrade.progress_percent = round(
                (nodes_upgraded / upgrade.total_nodes) * 100, 2
            )

        self.db.commit()
        self.db.refresh(upgrade)
        return upgrade

    def schedule(
        self,
        id: UUID,
        scheduled_time: datetime,
        scheduled_by: Optional[str] = None,
    ) -> Optional[Upgrade]:
        """Schedule an upgrade."""
        upgrade = self.get(id)
        if not upgrade:
            return None

        upgrade.status = UpgradeStatus.SCHEDULED.value
        upgrade.scheduled_time = scheduled_time
        upgrade.scheduled_by = scheduled_by

        self.db.commit()
        self.db.refresh(upgrade)
        return upgrade

    def cancel(
        self,
        id: UUID,
        cancelled_by: Optional[str] = None,
        reason: Optional[str] = None,
    ) -> Optional[Upgrade]:
        """Cancel a scheduled upgrade."""
        upgrade = self.get(id)
        if not upgrade:
            return None

        upgrade.status = UpgradeStatus.CANCELLED.value
        upgrade.cancelled_at = datetime.utcnow()
        upgrade.cancelled_by = cancelled_by
        upgrade.cancellation_reason = reason

        self.db.commit()
        self.db.refresh(upgrade)
        return upgrade

    def get_stats_by_chain(self, chain_id: str) -> dict:
        """Get upgrade statistics for a chain."""
        results = (
            self.db.query(Upgrade.status, func.count(Upgrade.id))
            .filter(Upgrade.chain_id == chain_id)
            .group_by(Upgrade.status)
            .all()
        )

        return {status: count for status, count in results}


class UpgradeRolloutRepository(BaseRepository[UpgradeRollout]):
    """Repository for UpgradeRollout model operations."""

    def __init__(self, db: Session):
        super().__init__(UpgradeRollout, db)

    def get_by_upgrade(self, upgrade_id: UUID) -> List[UpgradeRollout]:
        """Get all rollouts for an upgrade."""
        return (
            self.db.query(UpgradeRollout)
            .filter(UpgradeRollout.upgrade_id == upgrade_id)
            .order_by(UpgradeRollout.batch_number, UpgradeRollout.region_code)
            .all()
        )

    def get_by_region(
        self,
        upgrade_id: UUID,
        region_code: str,
    ) -> Optional[UpgradeRollout]:
        """Get rollout for a specific region."""
        return (
            self.db.query(UpgradeRollout)
            .filter(
                UpgradeRollout.upgrade_id == upgrade_id,
                UpgradeRollout.region_code == region_code,
            )
            .first()
        )

    def get_by_batch(
        self, upgrade_id: UUID, batch_number: int
    ) -> List[UpgradeRollout]:
        """Get rollouts for a specific batch."""
        return (
            self.db.query(UpgradeRollout)
            .filter(
                UpgradeRollout.upgrade_id == upgrade_id,
                UpgradeRollout.batch_number == batch_number,
            )
            .order_by(UpgradeRollout.region_code)
            .all()
        )

    def get_pending(self, upgrade_id: UUID) -> List[UpgradeRollout]:
        """Get pending rollouts for an upgrade."""
        return (
            self.db.query(UpgradeRollout)
            .filter(
                UpgradeRollout.upgrade_id == upgrade_id,
                UpgradeRollout.status == RolloutStatus.PENDING.value,
            )
            .order_by(UpgradeRollout.batch_number)
            .all()
        )

    def get_in_progress(self, upgrade_id: UUID) -> List[UpgradeRollout]:
        """Get in-progress rollouts for an upgrade."""
        return (
            self.db.query(UpgradeRollout)
            .filter(
                UpgradeRollout.upgrade_id == upgrade_id,
                UpgradeRollout.status == RolloutStatus.IN_PROGRESS.value,
            )
            .all()
        )

    def get_next_batch(self, upgrade_id: UUID) -> Optional[int]:
        """Get the next batch number to process."""
        result = (
            self.db.query(func.min(UpgradeRollout.batch_number))
            .filter(
                UpgradeRollout.upgrade_id == upgrade_id,
                UpgradeRollout.status == RolloutStatus.PENDING.value,
            )
            .scalar()
        )
        return result

    def set_status(
        self,
        id: UUID,
        status: RolloutStatus,
        error_message: Optional[str] = None,
    ) -> Optional[UpgradeRollout]:
        """Update rollout status."""
        rollout = self.get(id)
        if not rollout:
            return None

        rollout.status = status.value

        if status == RolloutStatus.IN_PROGRESS:
            rollout.started_at = datetime.utcnow()
        elif status == RolloutStatus.COMPLETED:
            rollout.completed_at = datetime.utcnow()
        elif status == RolloutStatus.FAILED:
            rollout.failed_at = datetime.utcnow()
            rollout.error_message = error_message
        elif status == RolloutStatus.ROLLED_BACK:
            rollout.rolled_back_at = datetime.utcnow()

        self.db.commit()
        self.db.refresh(rollout)
        return rollout

    def update_progress(
        self,
        id: UUID,
        nodes_upgraded: int,
        nodes_failed: int,
    ) -> Optional[UpgradeRollout]:
        """Update rollout progress."""
        rollout = self.get(id)
        if not rollout:
            return None

        rollout.nodes_upgraded = nodes_upgraded
        rollout.nodes_failed = nodes_failed

        if rollout.total_nodes and rollout.total_nodes > 0:
            rollout.progress_percent = round(
                (nodes_upgraded / rollout.total_nodes) * 100, 2
            )

        self.db.commit()
        self.db.refresh(rollout)
        return rollout

    def record_health_check(
        self,
        id: UUID,
        passed: bool,
        details: Optional[dict] = None,
    ) -> Optional[UpgradeRollout]:
        """Record health check result."""
        rollout = self.get(id)
        if not rollout:
            return None

        rollout.health_check_passed = passed
        rollout.health_check_at = datetime.utcnow()
        if details:
            rollout.health_check_details = details

        self.db.commit()
        self.db.refresh(rollout)
        return rollout

    def get_summary(self, upgrade_id: UUID) -> dict:
        """Get rollout summary for an upgrade."""
        results = (
            self.db.query(
                UpgradeRollout.status,
                func.count(UpgradeRollout.id),
                func.sum(UpgradeRollout.nodes_upgraded),
                func.sum(UpgradeRollout.nodes_failed),
            )
            .filter(UpgradeRollout.upgrade_id == upgrade_id)
            .group_by(UpgradeRollout.status)
            .all()
        )

        summary = {
            "by_status": {},
            "total_nodes_upgraded": 0,
            "total_nodes_failed": 0,
            "total_regions": 0,
        }

        for status, count, upgraded, failed in results:
            summary["by_status"][status] = count
            summary["total_nodes_upgraded"] += int(upgraded or 0)
            summary["total_nodes_failed"] += int(failed or 0)
            summary["total_regions"] += count

        return summary

    def is_batch_complete(self, upgrade_id: UUID, batch_number: int) -> bool:
        """Check if all rollouts in a batch are complete."""
        pending_count = (
            self.db.query(func.count(UpgradeRollout.id))
            .filter(
                UpgradeRollout.upgrade_id == upgrade_id,
                UpgradeRollout.batch_number == batch_number,
                UpgradeRollout.status.notin_([
                    RolloutStatus.COMPLETED.value,
                    RolloutStatus.SKIPPED.value,
                ]),
            )
            .scalar()
        )
        return pending_count == 0

    def get_failed_rollouts(self, upgrade_id: UUID) -> List[UpgradeRollout]:
        """Get failed rollouts for an upgrade."""
        return (
            self.db.query(UpgradeRollout)
            .filter(
                UpgradeRollout.upgrade_id == upgrade_id,
                UpgradeRollout.status == RolloutStatus.FAILED.value,
            )
            .all()
        )
