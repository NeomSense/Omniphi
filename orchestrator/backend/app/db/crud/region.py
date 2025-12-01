"""
Region CRUD Repositories

Repositories for region, server pool, and region server operations.
"""

from typing import List, Optional
from uuid import UUID

from sqlalchemy import and_, desc
from sqlalchemy.orm import Session

from app.db.crud.base import BaseRepository
from app.db.models.region import Region
from app.db.models.server_pool import ServerPool
from app.db.models.region_server import RegionServer
from app.db.models.enums import RegionStatus, ServerStatus


class RegionRepository(BaseRepository[Region]):
    """Repository for Region model operations."""

    def __init__(self, db: Session):
        super().__init__(Region, db)

    def get_by_code(self, code: str) -> Optional[Region]:
        """Get region by unique code."""
        return self.db.query(Region).filter(Region.code == code).first()

    def get_active(self) -> List[Region]:
        """Get all active regions."""
        return (
            self.db.query(Region)
            .filter(Region.is_active == True, Region.status == RegionStatus.ACTIVE.value)
            .order_by(Region.display_name)
            .all()
        )

    def get_accepting_new(self) -> List[Region]:
        """Get regions accepting new validators."""
        return (
            self.db.query(Region)
            .filter(
                Region.is_active == True,
                Region.is_accepting_new == True,
                Region.status == RegionStatus.ACTIVE.value,
            )
            .order_by(Region.display_name)
            .all()
        )

    def get_by_status(self, status: str) -> List[Region]:
        """Get regions by status."""
        return (
            self.db.query(Region)
            .filter(Region.status == status)
            .order_by(Region.display_name)
            .all()
        )

    def get_with_capacity(self, min_validators: int = 1) -> List[Region]:
        """Get regions with available capacity."""
        return (
            self.db.query(Region)
            .filter(
                Region.is_active == True,
                Region.is_accepting_new == True,
                (Region.max_validators - Region.active_validators) >= min_validators,
            )
            .order_by(Region.display_name)
            .all()
        )

    def update_usage(
        self,
        id: UUID,
        validators_delta: int = 0,
        cpu_delta: int = 0,
        memory_delta: int = 0,
        disk_delta: int = 0,
    ) -> Optional[Region]:
        """Update region resource usage."""
        region = self.get(id)
        if not region:
            return None

        region.active_validators = max(0, region.active_validators + validators_delta)
        region.used_cpu_cores = max(0, region.used_cpu_cores + cpu_delta)
        region.used_memory_gb = max(0, region.used_memory_gb + memory_delta)
        region.used_disk_gb = max(0, region.used_disk_gb + disk_delta)

        self.db.commit()
        self.db.refresh(region)
        return region

    def set_status(self, id: UUID, status: str) -> Optional[Region]:
        """Set region status."""
        region = self.get(id)
        if not region:
            return None

        region.status = status
        if status != RegionStatus.ACTIVE.value:
            region.is_accepting_new = False

        self.db.commit()
        self.db.refresh(region)
        return region


class ServerPoolRepository(BaseRepository[ServerPool]):
    """Repository for ServerPool model operations."""

    def __init__(self, db: Session):
        super().__init__(ServerPool, db)

    def get_by_region(self, region_id: UUID) -> List[ServerPool]:
        """Get all pools in a region."""
        return (
            self.db.query(ServerPool)
            .filter(ServerPool.region_id == region_id)
            .order_by(ServerPool.machine_type, ServerPool.name)
            .all()
        )

    def get_by_code(self, region_id: UUID, code: str) -> Optional[ServerPool]:
        """Get pool by region and code."""
        return (
            self.db.query(ServerPool)
            .filter(ServerPool.region_id == region_id, ServerPool.code == code)
            .first()
        )

    def get_active_by_region(self, region_id: UUID) -> List[ServerPool]:
        """Get active pools in a region."""
        return (
            self.db.query(ServerPool)
            .filter(ServerPool.region_id == region_id, ServerPool.is_active == True)
            .order_by(ServerPool.machine_type)
            .all()
        )

    def get_available(self, region_id: UUID, machine_type: Optional[str] = None) -> List[ServerPool]:
        """Get pools with available capacity."""
        query = self.db.query(ServerPool).filter(
            ServerPool.region_id == region_id,
            ServerPool.is_active == True,
            ServerPool.is_available == True,
            ServerPool.used_validators < ServerPool.total_validators,
        )

        if machine_type:
            query = query.filter(ServerPool.machine_type == machine_type)

        return query.order_by(ServerPool.monthly_cost).all()

    def update_counts(self, id: UUID) -> Optional[ServerPool]:
        """Update pool machine and validator counts from servers."""
        pool = self.get(id)
        if not pool:
            return None

        pool.update_machine_counts()
        self.db.commit()
        self.db.refresh(pool)
        return pool


class RegionServerRepository(BaseRepository[RegionServer]):
    """Repository for RegionServer model operations."""

    def __init__(self, db: Session):
        super().__init__(RegionServer, db)

    def get_by_hostname(self, hostname: str) -> Optional[RegionServer]:
        """Get server by hostname."""
        return self.db.query(RegionServer).filter(RegionServer.hostname == hostname).first()

    def get_by_region(self, region_id: UUID) -> List[RegionServer]:
        """Get all servers in a region."""
        return (
            self.db.query(RegionServer)
            .filter(RegionServer.region_id == region_id)
            .order_by(RegionServer.hostname)
            .all()
        )

    def get_by_pool(self, pool_id: UUID) -> List[RegionServer]:
        """Get all servers in a pool."""
        return (
            self.db.query(RegionServer)
            .filter(RegionServer.pool_id == pool_id)
            .order_by(RegionServer.hostname)
            .all()
        )

    def get_available(
        self,
        region_id: UUID,
        cpu_required: int = 0,
        memory_required: int = 0,
        disk_required: int = 0,
    ) -> List[RegionServer]:
        """Get servers with available resources."""
        query = self.db.query(RegionServer).filter(
            RegionServer.region_id == region_id,
            RegionServer.is_active == True,
            RegionServer.is_available == True,
            RegionServer.status == ServerStatus.ACTIVE.value,
            RegionServer.validators_hosted < RegionServer.max_validators,
        )

        if cpu_required:
            query = query.filter(
                (RegionServer.cpu_cores - RegionServer.used_cpu_cores) >= cpu_required
            )
        if memory_required:
            query = query.filter(
                (RegionServer.memory_gb - RegionServer.used_memory_gb) >= memory_required
            )
        if disk_required:
            query = query.filter(
                (RegionServer.disk_gb - RegionServer.used_disk_gb) >= disk_required
            )

        return query.order_by(RegionServer.validators_hosted).all()

    def get_best_available(
        self,
        region_id: UUID,
        cpu_required: int,
        memory_required: int,
        disk_required: int,
    ) -> Optional[RegionServer]:
        """Get best available server (least loaded with required resources)."""
        servers = self.get_available(region_id, cpu_required, memory_required, disk_required)
        return servers[0] if servers else None

    def allocate_resources(
        self,
        id: UUID,
        cpu: int,
        memory: int,
        disk: int,
    ) -> Optional[RegionServer]:
        """Allocate resources on a server."""
        server = self.get(id)
        if not server:
            return None

        if not server.allocate_validator(cpu, memory, disk):
            return None

        self.db.commit()
        self.db.refresh(server)
        return server

    def release_resources(
        self,
        id: UUID,
        cpu: int,
        memory: int,
        disk: int,
    ) -> Optional[RegionServer]:
        """Release resources on a server."""
        server = self.get(id)
        if not server:
            return None

        server.release_validator(cpu, memory, disk)
        self.db.commit()
        self.db.refresh(server)
        return server

    def update_heartbeat(self, id: UUID) -> Optional[RegionServer]:
        """Update server heartbeat timestamp."""
        server = self.get(id)
        if not server:
            return None

        server.update_heartbeat()
        self.db.commit()
        self.db.refresh(server)
        return server

    def get_stale(self, minutes: int = 5) -> List[RegionServer]:
        """Get servers with stale heartbeats."""
        from datetime import datetime, timedelta

        threshold = datetime.utcnow() - timedelta(minutes=minutes)
        return (
            self.db.query(RegionServer)
            .filter(
                RegionServer.is_active == True,
                RegionServer.last_heartbeat < threshold,
            )
            .all()
        )
