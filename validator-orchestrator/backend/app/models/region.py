"""
Multi-Region Infrastructure Models

Database models for region management, server pools, and regional health monitoring.
Supports US-East, US-West, EU-Central, and Asia-Pacific regions.
"""

import enum
from datetime import datetime
from typing import Optional
from uuid import uuid4

from sqlalchemy import (
    Column,
    String,
    Integer,
    Float,
    Boolean,
    DateTime,
    ForeignKey,
    Enum,
    JSON,
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship

from app.database import Base


class RegionStatus(str, enum.Enum):
    """Region operational status"""
    ACTIVE = "active"
    DEGRADED = "degraded"
    MAINTENANCE = "maintenance"
    OFFLINE = "offline"


class RegionCode(str, enum.Enum):
    """Supported region codes"""
    US_EAST = "us-east"
    US_WEST = "us-west"
    EU_CENTRAL = "eu-central"
    ASIA_PACIFIC = "asia-pacific"


class MachineType(str, enum.Enum):
    """Available machine types for validators"""
    SMALL = "small"      # 2 CPU, 4GB RAM, 100GB SSD
    MEDIUM = "medium"    # 4 CPU, 8GB RAM, 200GB SSD
    LARGE = "large"      # 8 CPU, 16GB RAM, 500GB SSD
    XLARGE = "xlarge"    # 16 CPU, 32GB RAM, 1TB SSD


class Region(Base):
    """
    Region model representing a geographic deployment zone.

    Each region contains server pools and has its own capacity limits.
    """
    __tablename__ = "regions"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    code = Column(Enum(RegionCode), unique=True, nullable=False, index=True)
    name = Column(String(100), nullable=False)
    display_name = Column(String(100), nullable=False)

    # Cloud provider zones (e.g., aws:us-east-1, gcp:us-east4)
    cloud_zones = Column(JSON, nullable=False, default=list)

    # Capacity configuration
    max_validators = Column(Integer, nullable=False, default=1000)
    max_cpu_cores = Column(Integer, nullable=False, default=5000)
    max_memory_gb = Column(Integer, nullable=False, default=10000)
    max_disk_gb = Column(Integer, nullable=False, default=50000)

    # Current usage
    active_validators = Column(Integer, nullable=False, default=0)
    used_cpu_cores = Column(Integer, nullable=False, default=0)
    used_memory_gb = Column(Integer, nullable=False, default=0)
    used_disk_gb = Column(Integer, nullable=False, default=0)

    # Status and health
    status = Column(Enum(RegionStatus), nullable=False, default=RegionStatus.ACTIVE)
    is_accepting_new = Column(Boolean, nullable=False, default=True)

    # Pricing (monthly USD per validator tier)
    base_monthly_cost = Column(Float, nullable=False, default=50.0)

    # Metadata
    description = Column(Text, nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    server_pools = relationship("ServerPool", back_populates="region", cascade="all, delete-orphan")
    health_records = relationship("RegionHealth", back_populates="region", cascade="all, delete-orphan")

    def __repr__(self):
        return f"<Region {self.code.value}: {self.display_name}>"

    @property
    def capacity_percent(self) -> float:
        """Calculate capacity utilization percentage"""
        if self.max_validators == 0:
            return 0.0
        return (self.active_validators / self.max_validators) * 100

    @property
    def cpu_utilization(self) -> float:
        """Calculate CPU utilization percentage"""
        if self.max_cpu_cores == 0:
            return 0.0
        return (self.used_cpu_cores / self.max_cpu_cores) * 100

    @property
    def memory_utilization(self) -> float:
        """Calculate memory utilization percentage"""
        if self.max_memory_gb == 0:
            return 0.0
        return (self.used_memory_gb / self.max_memory_gb) * 100


class ServerPool(Base):
    """
    Server pool within a region.

    Represents a group of machines of the same type available for provisioning.
    """
    __tablename__ = "server_pools"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    region_id = Column(UUID(as_uuid=True), ForeignKey("regions.id", ondelete="CASCADE"), nullable=False)

    # Pool configuration
    name = Column(String(100), nullable=False)
    machine_type = Column(Enum(MachineType), nullable=False)
    provider = Column(String(50), nullable=False, default="omniphi-cloud")

    # Machine specifications
    cpu_cores = Column(Integer, nullable=False)
    memory_gb = Column(Integer, nullable=False)
    disk_gb = Column(Integer, nullable=False)

    # Pool capacity
    total_machines = Column(Integer, nullable=False, default=0)
    available_machines = Column(Integer, nullable=False, default=0)
    reserved_machines = Column(Integer, nullable=False, default=0)

    # Pricing
    hourly_cost = Column(Float, nullable=False, default=0.10)
    monthly_cost = Column(Float, nullable=False, default=50.0)

    # Status
    is_active = Column(Boolean, nullable=False, default=True)

    # Metadata
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    # Relationships
    region = relationship("Region", back_populates="server_pools")

    __table_args__ = (
        Index("ix_server_pools_region_machine", "region_id", "machine_type"),
    )

    def __repr__(self):
        return f"<ServerPool {self.name} ({self.machine_type.value}) in {self.region_id}>"

    @property
    def utilization_percent(self) -> float:
        """Calculate pool utilization percentage"""
        if self.total_machines == 0:
            return 0.0
        used = self.total_machines - self.available_machines
        return (used / self.total_machines) * 100


class RegionHealth(Base):
    """
    Region health monitoring records.

    Tracks latency, success rates, and overall health status over time.
    """
    __tablename__ = "region_health"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    region_id = Column(UUID(as_uuid=True), ForeignKey("regions.id", ondelete="CASCADE"), nullable=False)

    # Health metrics
    latency_ms = Column(Float, nullable=False, default=0.0)
    success_rate = Column(Float, nullable=False, default=100.0)  # Percentage
    error_rate = Column(Float, nullable=False, default=0.0)      # Percentage

    # Resource health
    avg_cpu_percent = Column(Float, nullable=False, default=0.0)
    avg_memory_percent = Column(Float, nullable=False, default=0.0)
    avg_disk_percent = Column(Float, nullable=False, default=0.0)

    # Network health
    p2p_connectivity = Column(Float, nullable=False, default=100.0)  # Percentage of peers reachable
    rpc_availability = Column(Float, nullable=False, default=100.0)  # Percentage uptime

    # Node statistics
    total_nodes = Column(Integer, nullable=False, default=0)
    healthy_nodes = Column(Integer, nullable=False, default=0)
    warning_nodes = Column(Integer, nullable=False, default=0)
    error_nodes = Column(Integer, nullable=False, default=0)

    # Chain metrics
    avg_block_height = Column(Integer, nullable=False, default=0)
    max_blocks_behind = Column(Integer, nullable=False, default=0)
    avg_peers = Column(Integer, nullable=False, default=0)

    # Incident tracking
    active_incidents = Column(Integer, nullable=False, default=0)

    # Timestamps
    checked_at = Column(DateTime, nullable=False, default=datetime.utcnow, index=True)

    # Relationships
    region = relationship("Region", back_populates="health_records")

    __table_args__ = (
        Index("ix_region_health_region_time", "region_id", "checked_at"),
    )

    def __repr__(self):
        return f"<RegionHealth {self.region_id} @ {self.checked_at}>"

    @property
    def is_healthy(self) -> bool:
        """Determine if region is healthy based on metrics"""
        return (
            self.success_rate >= 95.0 and
            self.error_rate <= 5.0 and
            self.latency_ms <= 500 and
            self.healthy_nodes >= self.total_nodes * 0.9
        )

    @property
    def health_score(self) -> float:
        """Calculate overall health score (0-100)"""
        # Weight factors for different metrics
        success_weight = 0.3
        latency_weight = 0.2
        nodes_weight = 0.3
        p2p_weight = 0.2

        # Normalize latency (0-1000ms maps to 100-0 score)
        latency_score = max(0, 100 - (self.latency_ms / 10))

        # Node health ratio
        node_health = (self.healthy_nodes / max(1, self.total_nodes)) * 100

        return (
            self.success_rate * success_weight +
            latency_score * latency_weight +
            node_health * nodes_weight +
            self.p2p_connectivity * p2p_weight
        )


class RegionServer(Base):
    """
    Individual server/machine in a region.

    Tracks physical or virtual machines available for hosting validators.
    """
    __tablename__ = "region_servers"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    region_id = Column(UUID(as_uuid=True), ForeignKey("regions.id", ondelete="CASCADE"), nullable=False)
    pool_id = Column(UUID(as_uuid=True), ForeignKey("server_pools.id", ondelete="SET NULL"), nullable=True)

    # Server identification
    hostname = Column(String(255), nullable=False, unique=True)
    ip_address = Column(String(45), nullable=False)  # IPv4 or IPv6
    internal_ip = Column(String(45), nullable=True)

    # Cloud provider info
    provider = Column(String(50), nullable=False)
    provider_instance_id = Column(String(255), nullable=True)
    availability_zone = Column(String(50), nullable=True)

    # Specifications
    machine_type = Column(Enum(MachineType), nullable=False)
    cpu_cores = Column(Integer, nullable=False)
    memory_gb = Column(Integer, nullable=False)
    disk_gb = Column(Integer, nullable=False)

    # Current usage
    used_cpu_cores = Column(Integer, nullable=False, default=0)
    used_memory_gb = Column(Integer, nullable=False, default=0)
    used_disk_gb = Column(Integer, nullable=False, default=0)
    validators_hosted = Column(Integer, nullable=False, default=0)
    max_validators = Column(Integer, nullable=False, default=10)

    # Status
    is_active = Column(Boolean, nullable=False, default=True)
    is_available = Column(Boolean, nullable=False, default=True)
    last_heartbeat = Column(DateTime, nullable=True)

    # Metadata
    tags = Column(JSON, nullable=True, default=dict)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    __table_args__ = (
        Index("ix_region_servers_region_available", "region_id", "is_available"),
        Index("ix_region_servers_pool", "pool_id"),
    )

    def __repr__(self):
        return f"<RegionServer {self.hostname} ({self.machine_type.value})>"

    @property
    def can_accept_validator(self) -> bool:
        """Check if server can accept a new validator"""
        return (
            self.is_active and
            self.is_available and
            self.validators_hosted < self.max_validators
        )
