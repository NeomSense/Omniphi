"""
Snapshot CRUD Repository

Repository for chain state snapshot operations.
"""

from datetime import datetime, timedelta
from typing import List, Optional
from uuid import UUID

from sqlalchemy import and_, desc, func
from sqlalchemy.orm import Session

from app.db.crud.base import BaseRepository
from app.db.models.snapshot import Snapshot


class SnapshotRepository(BaseRepository[Snapshot]):
    """Repository for Snapshot model operations."""

    def __init__(self, db: Session):
        super().__init__(Snapshot, db)

    def get_by_chain(self, chain_id: str) -> List[Snapshot]:
        """Get all snapshots for a chain."""
        return (
            self.db.query(Snapshot)
            .filter(Snapshot.chain_id == chain_id)
            .order_by(desc(Snapshot.block_height))
            .all()
        )

    def get_latest(self, chain_id: str) -> Optional[Snapshot]:
        """Get latest snapshot for a chain."""
        return (
            self.db.query(Snapshot)
            .filter(
                Snapshot.chain_id == chain_id,
                Snapshot.is_available == True,
            )
            .order_by(desc(Snapshot.block_height))
            .first()
        )

    def get_by_version(
        self, chain_id: str, chain_version: str
    ) -> List[Snapshot]:
        """Get snapshots for a specific chain version."""
        return (
            self.db.query(Snapshot)
            .filter(
                Snapshot.chain_id == chain_id,
                Snapshot.chain_version == chain_version,
                Snapshot.is_available == True,
            )
            .order_by(desc(Snapshot.block_height))
            .all()
        )

    def get_by_block_height(
        self, chain_id: str, block_height: int
    ) -> Optional[Snapshot]:
        """Get snapshot at or near a specific block height."""
        return (
            self.db.query(Snapshot)
            .filter(
                Snapshot.chain_id == chain_id,
                Snapshot.block_height <= block_height,
                Snapshot.is_available == True,
            )
            .order_by(desc(Snapshot.block_height))
            .first()
        )

    def get_in_region(self, chain_id: str, region_code: str) -> List[Snapshot]:
        """Get snapshots available in a specific region."""
        return (
            self.db.query(Snapshot)
            .filter(
                Snapshot.chain_id == chain_id,
                Snapshot.is_available == True,
                Snapshot.available_regions.contains([region_code]),
            )
            .order_by(desc(Snapshot.block_height))
            .all()
        )

    def get_available(self, chain_id: Optional[str] = None) -> List[Snapshot]:
        """Get all available snapshots, optionally filtered by chain."""
        q = self.db.query(Snapshot).filter(Snapshot.is_available == True)

        if chain_id:
            q = q.filter(Snapshot.chain_id == chain_id)

        return q.order_by(Snapshot.chain_id, desc(Snapshot.block_height)).all()

    def get_verified(self, chain_id: str) -> List[Snapshot]:
        """Get verified snapshots for a chain."""
        return (
            self.db.query(Snapshot)
            .filter(
                Snapshot.chain_id == chain_id,
                Snapshot.is_available == True,
                Snapshot.is_verified == True,
            )
            .order_by(desc(Snapshot.block_height))
            .all()
        )

    def get_pruned(self, chain_id: str) -> List[Snapshot]:
        """Get pruned snapshots for a chain."""
        return (
            self.db.query(Snapshot)
            .filter(
                Snapshot.chain_id == chain_id,
                Snapshot.is_available == True,
                Snapshot.is_pruned == True,
            )
            .order_by(desc(Snapshot.block_height))
            .all()
        )

    def get_by_type(self, chain_id: str, snapshot_type: str) -> List[Snapshot]:
        """Get snapshots by type (full, pruned, archive)."""
        return (
            self.db.query(Snapshot)
            .filter(
                Snapshot.chain_id == chain_id,
                Snapshot.snapshot_type == snapshot_type,
                Snapshot.is_available == True,
            )
            .order_by(desc(Snapshot.block_height))
            .all()
        )

    def get_recent(self, hours: int = 24) -> List[Snapshot]:
        """Get recently created snapshots."""
        threshold = datetime.utcnow() - timedelta(hours=hours)
        return (
            self.db.query(Snapshot)
            .filter(Snapshot.created_at >= threshold)
            .order_by(desc(Snapshot.created_at))
            .all()
        )

    def set_available(
        self, id: UUID, available: bool = True
    ) -> Optional[Snapshot]:
        """Set snapshot availability."""
        snapshot = self.get(id)
        if not snapshot:
            return None

        snapshot.is_available = available
        self.db.commit()
        self.db.refresh(snapshot)
        return snapshot

    def set_verified(
        self,
        id: UUID,
        verified: bool = True,
        verified_by: Optional[str] = None,
    ) -> Optional[Snapshot]:
        """Set snapshot verification status."""
        snapshot = self.get(id)
        if not snapshot:
            return None

        snapshot.is_verified = verified
        snapshot.verified_at = datetime.utcnow() if verified else None
        snapshot.verified_by = verified_by
        self.db.commit()
        self.db.refresh(snapshot)
        return snapshot

    def increment_downloads(self, id: UUID) -> Optional[Snapshot]:
        """Increment download count for a snapshot."""
        snapshot = self.get(id)
        if not snapshot:
            return None

        snapshot.download_count = (snapshot.download_count or 0) + 1
        snapshot.last_downloaded_at = datetime.utcnow()
        self.db.commit()
        self.db.refresh(snapshot)
        return snapshot

    def get_download_stats(self, chain_id: str) -> dict:
        """Get download statistics for a chain's snapshots."""
        result = (
            self.db.query(
                func.sum(Snapshot.download_count),
                func.count(Snapshot.id),
                func.max(Snapshot.download_count),
            )
            .filter(Snapshot.chain_id == chain_id)
            .first()
        )

        return {
            "total_downloads": int(result[0] or 0),
            "snapshot_count": int(result[1] or 0),
            "max_downloads": int(result[2] or 0),
        }

    def get_storage_stats(self) -> dict:
        """Get storage statistics across all snapshots."""
        result = (
            self.db.query(
                func.sum(Snapshot.file_size_bytes),
                func.count(Snapshot.id),
                func.avg(Snapshot.file_size_bytes),
            )
            .filter(Snapshot.is_available == True)
            .first()
        )

        return {
            "total_size_bytes": int(result[0] or 0),
            "snapshot_count": int(result[1] or 0),
            "avg_size_bytes": int(result[2] or 0),
        }

    def cleanup_old(
        self,
        chain_id: str,
        keep_count: int = 5,
        keep_days: int = 30,
    ) -> int:
        """Mark old snapshots as unavailable, keeping recent ones."""
        threshold = datetime.utcnow() - timedelta(days=keep_days)

        # Get IDs of snapshots to keep (most recent by block height)
        keep_ids = (
            self.db.query(Snapshot.id)
            .filter(
                Snapshot.chain_id == chain_id,
                Snapshot.is_available == True,
            )
            .order_by(desc(Snapshot.block_height))
            .limit(keep_count)
            .all()
        )
        keep_ids = [s[0] for s in keep_ids]

        # Mark old snapshots as unavailable
        result = (
            self.db.query(Snapshot)
            .filter(
                Snapshot.chain_id == chain_id,
                Snapshot.is_available == True,
                Snapshot.created_at < threshold,
                ~Snapshot.id.in_(keep_ids),
            )
            .update({"is_available": False}, synchronize_session=False)
        )

        self.db.commit()
        return result

    def get_chains_with_snapshots(self) -> List[dict]:
        """Get list of chains that have snapshots."""
        results = (
            self.db.query(
                Snapshot.chain_id,
                func.count(Snapshot.id),
                func.max(Snapshot.block_height),
                func.max(Snapshot.created_at),
            )
            .filter(Snapshot.is_available == True)
            .group_by(Snapshot.chain_id)
            .all()
        )

        return [
            {
                "chain_id": chain_id,
                "snapshot_count": count,
                "latest_block_height": max_height,
                "latest_snapshot_at": latest_at,
            }
            for chain_id, count, max_height, latest_at in results
        ]
