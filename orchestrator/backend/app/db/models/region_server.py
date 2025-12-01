"""
Region Server Model

Represents individual physical or virtual servers within a region.
These are the actual machines that host validator containers/processes.

Table: region_servers
"""

import uuid
from datetime import datetime
from typing import Optional, TYPE_CHECKING

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    ForeignKey,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base
from app.db.models.enums import MachineType, ServerStatus

if TYPE_CHECKING:
    from app.db.models.region import Region
    from app.db.models.server_pool import ServerPool


class RegionServer(Base):
    """
    Individual server/machine in a region.

    Tracks physical or virtual machines available for hosting validators.
    Each server belongs to a region and optionally to a server pool.
    """

    __tablename__ = "region_servers"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Foreign keys
    region_id = Column(
        UUID(as_uuid=True),
        ForeignKey("regions.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
        doc="Parent region"
    )
    pool_id = Column(
        UUID(as_uuid=True),
        ForeignKey("server_pools.id", ondelete="SET NULL"),
        nullable=True,
        index=True,
        doc="Server pool membership"
    )

    # Server identification
    hostname = Column(
        String(255),
        nullable=False,
        unique=True,
        index=True,
        doc="Unique server hostname"
    )
    ip_address = Column(
        String(45),
        nullable=False,
        doc="Public IP address (IPv4 or IPv6)"
    )
    internal_ip = Column(
        String(45),
        nullable=True,
        doc="Internal/private IP address"
    )

    # Cloud provider info
    provider = Column(
        String(50),
        nullable=False,
        default="omniphi-cloud",
        doc="Cloud provider identifier"
    )
    provider_instance_id = Column(
        String(255),
        nullable=True,
        doc="Provider's instance ID (e.g., AWS EC2 instance ID)"
    )
    availability_zone = Column(
        String(50),
        nullable=True,
        doc="Cloud availability zone"
    )

    # Hardware specifications
    machine_type = Column(
        String(50),
        nullable=False,
        default=MachineType.MEDIUM.value,
        doc="Machine type classification"
    )
    cpu_cores = Column(
        Integer,
        nullable=False,
        doc="Total CPU cores"
    )
    memory_gb = Column(
        Integer,
        nullable=False,
        doc="Total memory in GB"
    )
    disk_gb = Column(
        Integer,
        nullable=False,
        doc="Total disk space in GB"
    )
    disk_type = Column(
        String(50),
        nullable=False,
        default="ssd",
        doc="Disk type (ssd, nvme, hdd)"
    )

    # Current resource usage
    used_cpu_cores = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Currently allocated CPU cores"
    )
    used_memory_gb = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Currently allocated memory in GB"
    )
    used_disk_gb = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Currently allocated disk in GB"
    )

    # Validator hosting
    validators_hosted = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of validators currently hosted"
    )
    max_validators = Column(
        Integer,
        nullable=False,
        default=10,
        doc="Maximum validators this server can host"
    )

    # Status and availability
    status = Column(
        String(50),
        nullable=False,
        default=ServerStatus.ACTIVE.value,
        index=True,
        doc="Server operational status"
    )
    is_active = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True,
        doc="Whether server is active"
    )
    is_available = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True,
        doc="Whether server accepts new validators"
    )

    # Health monitoring
    last_heartbeat = Column(
        DateTime,
        nullable=True,
        doc="Last health check timestamp"
    )
    health_score = Column(
        Float,
        nullable=False,
        default=100.0,
        doc="Health score (0-100)"
    )

    # Metadata
    labels = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Custom labels/tags"
    )
    annotations = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional annotations"
    )

    # Timestamps
    created_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow
    )
    updated_at = Column(
        DateTime,
        nullable=False,
        default=datetime.utcnow,
        onupdate=datetime.utcnow
    )

    # Relationships
    region: Mapped["Region"] = relationship(
        "Region",
        back_populates="servers"
    )
    pool: Mapped[Optional["ServerPool"]] = relationship(
        "ServerPool",
        back_populates="servers"
    )

    # Indexes
    __table_args__ = (
        Index("ix_region_servers_region_available", "region_id", "is_available"),
        Index("ix_region_servers_region_status", "region_id", "status"),
        Index("ix_region_servers_pool_status", "pool_id", "status"),
        Index("ix_region_servers_provider", "provider", "status"),
    )

    def __repr__(self) -> str:
        return f"<RegionServer {self.hostname} ({self.machine_type})>"

    @property
    def available_cpu(self) -> int:
        """Get available CPU cores."""
        return max(0, self.cpu_cores - self.used_cpu_cores)

    @property
    def available_memory(self) -> int:
        """Get available memory in GB."""
        return max(0, self.memory_gb - self.used_memory_gb)

    @property
    def available_disk(self) -> int:
        """Get available disk in GB."""
        return max(0, self.disk_gb - self.used_disk_gb)

    @property
    def cpu_utilization(self) -> float:
        """Calculate CPU utilization percentage."""
        if self.cpu_cores == 0:
            return 0.0
        return round((self.used_cpu_cores / self.cpu_cores) * 100, 2)

    @property
    def memory_utilization(self) -> float:
        """Calculate memory utilization percentage."""
        if self.memory_gb == 0:
            return 0.0
        return round((self.used_memory_gb / self.memory_gb) * 100, 2)

    @property
    def disk_utilization(self) -> float:
        """Calculate disk utilization percentage."""
        if self.disk_gb == 0:
            return 0.0
        return round((self.used_disk_gb / self.disk_gb) * 100, 2)

    @property
    def available_validator_slots(self) -> int:
        """Get number of available validator slots."""
        return max(0, self.max_validators - self.validators_hosted)

    @property
    def can_accept_validator(self) -> bool:
        """Check if server can accept a new validator."""
        return (
            self.is_active and
            self.is_available and
            self.status == ServerStatus.ACTIVE.value and
            self.validators_hosted < self.max_validators
        )

    def can_provision(self, cpu: int, memory: int, disk: int) -> bool:
        """
        Check if server can accommodate resource requirements.

        Args:
            cpu: Required CPU cores
            memory: Required memory in GB
            disk: Required disk in GB

        Returns:
            True if resources are available
        """
        return (
            self.can_accept_validator and
            (self.used_cpu_cores + cpu) <= self.cpu_cores and
            (self.used_memory_gb + memory) <= self.memory_gb and
            (self.used_disk_gb + disk) <= self.disk_gb
        )

    def allocate_validator(self, cpu: int, memory: int, disk: int) -> bool:
        """
        Allocate resources for a validator.

        Args:
            cpu: CPU cores to allocate
            memory: Memory in GB to allocate
            disk: Disk in GB to allocate

        Returns:
            True if allocation successful
        """
        if not self.can_provision(cpu, memory, disk):
            return False

        self.validators_hosted += 1
        self.used_cpu_cores += cpu
        self.used_memory_gb += memory
        self.used_disk_gb += disk

        # Update availability if at capacity
        if self.validators_hosted >= self.max_validators:
            self.is_available = False

        return True

    def release_validator(self, cpu: int, memory: int, disk: int) -> None:
        """
        Release resources from a validator.

        Args:
            cpu: CPU cores to release
            memory: Memory in GB to release
            disk: Disk in GB to release
        """
        self.validators_hosted = max(0, self.validators_hosted - 1)
        self.used_cpu_cores = max(0, self.used_cpu_cores - cpu)
        self.used_memory_gb = max(0, self.used_memory_gb - memory)
        self.used_disk_gb = max(0, self.used_disk_gb - disk)

        # Update availability if was at capacity
        if self.validators_hosted < self.max_validators:
            self.is_available = True

    def update_heartbeat(self) -> None:
        """Update last heartbeat timestamp."""
        self.last_heartbeat = datetime.utcnow()
