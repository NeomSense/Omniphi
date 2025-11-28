"""
Region Model

Represents a geographic deployment zone for validator infrastructure.
Each region contains server pools and has its own capacity limits.

Table: regions
"""

import uuid
from datetime import datetime
from typing import List, Optional, TYPE_CHECKING

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base
from app.db.models.enums import RegionCode, RegionStatus

if TYPE_CHECKING:
    from app.db.models.server_pool import ServerPool
    from app.db.models.region_server import RegionServer


class Region(Base):
    """
    Region model representing a geographic deployment zone.

    Each region has:
    - Unique region code (us-east, eu-central, etc.)
    - Capacity limits for validators, CPU, memory, disk
    - Current usage tracking
    - Associated server pools and health records
    """

    __tablename__ = "regions"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Region identification
    code = Column(
        String(50),
        unique=True,
        nullable=False,
        index=True,
        doc="Unique region code (e.g., us-east, eu-central)"
    )
    name = Column(
        String(100),
        nullable=False,
        doc="Internal region name"
    )
    display_name = Column(
        String(100),
        nullable=False,
        doc="Human-readable display name"
    )
    location = Column(
        String(200),
        nullable=True,
        doc="Physical location description"
    )

    # Cloud provider zone mappings (e.g., {"aws": "us-east-1", "gcp": "us-east4"})
    cloud_zones = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Mapping of cloud provider zones"
    )

    # Capacity configuration - maximum limits
    max_validators = Column(
        Integer,
        nullable=False,
        default=1000,
        doc="Maximum number of validators in this region"
    )
    max_cpu_cores = Column(
        Integer,
        nullable=False,
        default=5000,
        doc="Maximum CPU cores available"
    )
    max_memory_gb = Column(
        Integer,
        nullable=False,
        default=10000,
        doc="Maximum memory in GB"
    )
    max_disk_gb = Column(
        Integer,
        nullable=False,
        default=50000,
        doc="Maximum disk space in GB"
    )

    # Current usage tracking
    active_validators = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Current number of active validators"
    )
    used_cpu_cores = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Currently used CPU cores"
    )
    used_memory_gb = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Currently used memory in GB"
    )
    used_disk_gb = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Currently used disk space in GB"
    )

    # Status and availability
    status = Column(
        String(50),
        nullable=False,
        default=RegionStatus.ACTIVE.value,
        index=True,
        doc="Region operational status"
    )
    is_active = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True,
        doc="Whether region is active"
    )
    is_accepting_new = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Whether accepting new validators"
    )

    # Pricing
    base_monthly_cost = Column(
        Float,
        nullable=False,
        default=50.0,
        doc="Base monthly cost per validator (USD)"
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD",
        doc="Pricing currency"
    )

    # Metadata
    description = Column(
        Text,
        nullable=True,
        doc="Region description"
    )
    features = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Region features and capabilities"
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
    server_pools: Mapped[List["ServerPool"]] = relationship(
        "ServerPool",
        back_populates="region",
        cascade="all, delete-orphan",
        lazy="selectin"
    )
    servers: Mapped[List["RegionServer"]] = relationship(
        "RegionServer",
        back_populates="region",
        cascade="all, delete-orphan",
        lazy="selectin"
    )

    # Indexes
    __table_args__ = (
        Index("ix_regions_status_active", "status", "is_active"),
        Index("ix_regions_accepting", "is_accepting_new", "is_active"),
    )

    def __repr__(self) -> str:
        return f"<Region {self.code}: {self.display_name}>"

    @property
    def capacity_percent(self) -> float:
        """Calculate validator capacity utilization percentage."""
        if self.max_validators == 0:
            return 0.0
        return round((self.active_validators / self.max_validators) * 100, 2)

    @property
    def cpu_utilization(self) -> float:
        """Calculate CPU utilization percentage."""
        if self.max_cpu_cores == 0:
            return 0.0
        return round((self.used_cpu_cores / self.max_cpu_cores) * 100, 2)

    @property
    def memory_utilization(self) -> float:
        """Calculate memory utilization percentage."""
        if self.max_memory_gb == 0:
            return 0.0
        return round((self.used_memory_gb / self.max_memory_gb) * 100, 2)

    @property
    def disk_utilization(self) -> float:
        """Calculate disk utilization percentage."""
        if self.max_disk_gb == 0:
            return 0.0
        return round((self.used_disk_gb / self.max_disk_gb) * 100, 2)

    @property
    def available_validators(self) -> int:
        """Get number of available validator slots."""
        return max(0, self.max_validators - self.active_validators)

    @property
    def is_at_capacity(self) -> bool:
        """Check if region is at capacity."""
        return self.active_validators >= self.max_validators

    def can_provision(self, cpu: int = 0, memory: int = 0, disk: int = 0) -> bool:
        """
        Check if region can accommodate new resource requirements.

        Args:
            cpu: Required CPU cores
            memory: Required memory in GB
            disk: Required disk in GB

        Returns:
            True if resources are available
        """
        return (
            self.is_active and
            self.is_accepting_new and
            self.status == RegionStatus.ACTIVE.value and
            self.active_validators < self.max_validators and
            (self.used_cpu_cores + cpu) <= self.max_cpu_cores and
            (self.used_memory_gb + memory) <= self.max_memory_gb and
            (self.used_disk_gb + disk) <= self.max_disk_gb
        )

    def allocate_resources(self, cpu: int, memory: int, disk: int) -> bool:
        """
        Allocate resources in this region.

        Args:
            cpu: CPU cores to allocate
            memory: Memory in GB to allocate
            disk: Disk in GB to allocate

        Returns:
            True if allocation successful
        """
        if not self.can_provision(cpu, memory, disk):
            return False

        self.active_validators += 1
        self.used_cpu_cores += cpu
        self.used_memory_gb += memory
        self.used_disk_gb += disk
        return True

    def release_resources(self, cpu: int, memory: int, disk: int) -> None:
        """
        Release allocated resources.

        Args:
            cpu: CPU cores to release
            memory: Memory in GB to release
            disk: Disk in GB to release
        """
        self.active_validators = max(0, self.active_validators - 1)
        self.used_cpu_cores = max(0, self.used_cpu_cores - cpu)
        self.used_memory_gb = max(0, self.used_memory_gb - memory)
        self.used_disk_gb = max(0, self.used_disk_gb - disk)
