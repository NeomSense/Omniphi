"""
Server Pool Model

Represents a pool of machines of the same type within a region.
Defines capacity limits and pricing for a specific machine tier.

Table: server_pools
"""

import uuid
from datetime import datetime
from typing import List, TYPE_CHECKING

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    ForeignKey,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base
from app.db.models.enums import MachineType

if TYPE_CHECKING:
    from app.db.models.region import Region
    from app.db.models.region_server import RegionServer


class ServerPool(Base):
    """
    Server pool within a region.

    Represents a group of machines of the same type available for provisioning.
    Pools define capacity, pricing, and specifications for validator hosting.
    """

    __tablename__ = "server_pools"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Foreign key
    region_id = Column(
        UUID(as_uuid=True),
        ForeignKey("regions.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
        doc="Parent region"
    )

    # Pool identification
    name = Column(
        String(100),
        nullable=False,
        doc="Pool display name"
    )
    code = Column(
        String(50),
        nullable=False,
        doc="Unique pool code within region"
    )
    description = Column(
        Text,
        nullable=True,
        doc="Pool description"
    )

    # Machine type and provider
    machine_type = Column(
        String(50),
        nullable=False,
        default=MachineType.MEDIUM.value,
        doc="Machine type classification"
    )
    provider = Column(
        String(50),
        nullable=False,
        default="omniphi-cloud",
        doc="Infrastructure provider"
    )

    # Machine specifications
    cpu_cores = Column(
        Integer,
        nullable=False,
        doc="CPU cores per machine"
    )
    memory_gb = Column(
        Integer,
        nullable=False,
        doc="Memory in GB per machine"
    )
    disk_gb = Column(
        Integer,
        nullable=False,
        doc="Disk space in GB per machine"
    )
    bandwidth_gbps = Column(
        Float,
        nullable=False,
        default=1.0,
        doc="Network bandwidth in Gbps"
    )

    # Pool capacity
    total_machines = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Total machines in pool"
    )
    available_machines = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Available machines"
    )
    reserved_machines = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Reserved machines"
    )

    # Validator capacity
    total_validators = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Total validator capacity"
    )
    used_validators = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Currently used validator slots"
    )

    # Pricing
    hourly_cost = Column(
        Float,
        nullable=False,
        default=0.10,
        doc="Hourly cost in USD"
    )
    monthly_cost = Column(
        Float,
        nullable=False,
        default=50.0,
        doc="Monthly cost in USD"
    )
    setup_fee = Column(
        Float,
        nullable=False,
        default=0.0,
        doc="One-time setup fee"
    )
    currency = Column(
        String(3),
        nullable=False,
        default="USD",
        doc="Pricing currency"
    )

    # Status
    is_active = Column(
        Boolean,
        nullable=False,
        default=True,
        index=True,
        doc="Whether pool is active"
    )
    is_available = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Whether pool accepts new validators"
    )

    # Performance metrics (cached)
    avg_latency_ms = Column(
        Float,
        nullable=True,
        doc="Average network latency"
    )
    uptime_percent = Column(
        Float,
        nullable=False,
        default=99.9,
        doc="Uptime percentage"
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
        back_populates="server_pools"
    )
    servers: Mapped[List["RegionServer"]] = relationship(
        "RegionServer",
        back_populates="pool",
        lazy="selectin"
    )

    # Indexes
    __table_args__ = (
        Index("ix_server_pools_region_machine", "region_id", "machine_type"),
        Index("ix_server_pools_region_active", "region_id", "is_active"),
        Index("ix_server_pools_region_code", "region_id", "code", unique=True),
    )

    def __repr__(self) -> str:
        return f"<ServerPool {self.name} ({self.machine_type})>"

    @property
    def utilization_percent(self) -> float:
        """Calculate pool utilization percentage."""
        if self.total_validators == 0:
            return 0.0
        return round((self.used_validators / self.total_validators) * 100, 2)

    @property
    def machine_utilization(self) -> float:
        """Calculate machine utilization percentage."""
        if self.total_machines == 0:
            return 0.0
        used = self.total_machines - self.available_machines
        return round((used / self.total_machines) * 100, 2)

    @property
    def available_validators(self) -> int:
        """Get number of available validator slots."""
        return max(0, self.total_validators - self.used_validators)

    @property
    def is_at_capacity(self) -> bool:
        """Check if pool is at capacity."""
        return self.used_validators >= self.total_validators

    def can_provision(self) -> bool:
        """Check if pool can provision a new validator."""
        return (
            self.is_active and
            self.is_available and
            self.used_validators < self.total_validators and
            self.available_machines > 0
        )

    def allocate_validator(self) -> bool:
        """
        Allocate a validator slot in the pool.

        Returns:
            True if allocation successful
        """
        if not self.can_provision():
            return False

        self.used_validators += 1

        # Update availability if at capacity
        if self.used_validators >= self.total_validators:
            self.is_available = False

        return True

    def release_validator(self) -> None:
        """Release a validator slot in the pool."""
        self.used_validators = max(0, self.used_validators - 1)

        # Update availability if was at capacity
        if self.used_validators < self.total_validators:
            self.is_available = True

    def update_machine_counts(self) -> None:
        """Update machine counts from associated servers."""
        if self.servers:
            self.total_machines = len(self.servers)
            self.available_machines = sum(
                1 for s in self.servers
                if s.is_available and s.can_accept_validator
            )
            self.total_validators = sum(s.max_validators for s in self.servers)
            self.used_validators = sum(s.validators_hosted for s in self.servers)
