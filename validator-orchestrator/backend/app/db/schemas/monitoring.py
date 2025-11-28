"""
Monitoring Pydantic Schemas

Schemas for node metrics and incidents.
"""

from datetime import datetime
from typing import Any, Dict, List, Optional
from uuid import UUID

from pydantic import Field

from app.db.schemas.base import BaseSchema, PaginatedResponse


# =============================================================================
# NODE METRICS SCHEMAS
# =============================================================================

class NodeMetricsCreate(BaseSchema):
    """Schema for creating node metrics."""

    validator_node_id: UUID
    recorded_at: Optional[datetime] = None
    period_type: str = Field("minute")

    # CPU
    cpu_percent: Optional[float] = Field(None, ge=0, le=100)
    cpu_cores_used: Optional[float] = Field(None, ge=0)
    load_average_1m: Optional[float] = Field(None, ge=0)
    load_average_5m: Optional[float] = Field(None, ge=0)
    load_average_15m: Optional[float] = Field(None, ge=0)

    # Memory
    memory_percent: Optional[float] = Field(None, ge=0, le=100)
    memory_used_gb: Optional[float] = Field(None, ge=0)
    memory_available_gb: Optional[float] = Field(None, ge=0)
    swap_percent: Optional[float] = Field(None, ge=0, le=100)

    # Disk
    disk_percent: Optional[float] = Field(None, ge=0, le=100)
    disk_used_gb: Optional[float] = Field(None, ge=0)
    disk_available_gb: Optional[float] = Field(None, ge=0)
    disk_read_mb_s: Optional[float] = Field(None, ge=0)
    disk_write_mb_s: Optional[float] = Field(None, ge=0)
    disk_iops: Optional[int] = Field(None, ge=0)

    # Network
    network_rx_mb_s: Optional[float] = Field(None, ge=0)
    network_tx_mb_s: Optional[float] = Field(None, ge=0)
    network_connections: Optional[int] = Field(None, ge=0)

    # Chain
    block_height: Optional[int] = Field(None, ge=0)
    blocks_behind: Optional[int] = Field(None, ge=0)
    is_syncing: Optional[bool] = None
    sync_speed_blocks_per_sec: Optional[float] = Field(None, ge=0)

    # P2P
    peer_count: Optional[int] = Field(None, ge=0)
    inbound_peers: Optional[int] = Field(None, ge=0)
    outbound_peers: Optional[int] = Field(None, ge=0)
    peer_latency_avg_ms: Optional[float] = Field(None, ge=0)

    # Validator
    voting_power: Optional[str] = None
    missed_blocks: Optional[int] = Field(None, ge=0)
    missed_blocks_window: Optional[int] = Field(None, ge=0)
    uptime_percent: Optional[float] = Field(None, ge=0, le=100)
    is_jailed: Optional[bool] = None

    # RPC
    rpc_requests_per_sec: Optional[float] = Field(None, ge=0)
    rpc_latency_avg_ms: Optional[float] = Field(None, ge=0)
    rpc_error_rate: Optional[float] = Field(None, ge=0, le=100)

    # Process
    process_cpu_percent: Optional[float] = Field(None, ge=0, le=100)
    process_memory_mb: Optional[float] = Field(None, ge=0)
    goroutines: Optional[int] = Field(None, ge=0)
    open_files: Optional[int] = Field(None, ge=0)

    # Extra
    extra_metrics: Dict[str, Any] = Field(default_factory=dict)


class NodeMetricsResponse(BaseSchema):
    """Schema for node metrics response."""

    id: UUID
    validator_node_id: UUID
    recorded_at: datetime
    period_type: str

    # CPU
    cpu_percent: Optional[float]
    cpu_cores_used: Optional[float]
    load_average_1m: Optional[float]
    load_average_5m: Optional[float]
    load_average_15m: Optional[float]

    # Memory
    memory_percent: Optional[float]
    memory_used_gb: Optional[float]
    memory_available_gb: Optional[float]
    swap_percent: Optional[float]

    # Disk
    disk_percent: Optional[float]
    disk_used_gb: Optional[float]
    disk_available_gb: Optional[float]
    disk_read_mb_s: Optional[float]
    disk_write_mb_s: Optional[float]
    disk_iops: Optional[int]

    # Network
    network_rx_mb_s: Optional[float]
    network_tx_mb_s: Optional[float]
    network_connections: Optional[int]

    # Chain
    block_height: Optional[int]
    blocks_behind: Optional[int]
    is_syncing: Optional[bool]
    sync_speed_blocks_per_sec: Optional[float]

    # P2P
    peer_count: Optional[int]
    inbound_peers: Optional[int]
    outbound_peers: Optional[int]
    peer_latency_avg_ms: Optional[float]

    # Validator
    voting_power: Optional[str]
    missed_blocks: Optional[int]
    missed_blocks_window: Optional[int]
    uptime_percent: Optional[float]
    is_jailed: Optional[bool]
    commission_earned: Optional[float]

    # RPC
    rpc_requests_per_sec: Optional[float]
    rpc_latency_avg_ms: Optional[float]
    rpc_error_rate: Optional[float]

    # Process
    process_cpu_percent: Optional[float]
    process_memory_mb: Optional[float]
    goroutines: Optional[int]
    open_files: Optional[int]

    # Health
    health_score: Optional[float]
    health_status: Optional[str]

    extra_metrics: Dict[str, Any]

    # Computed
    is_healthy: bool
    has_resource_warning: bool
    has_resource_critical: bool


class MetricsSummary(BaseSchema):
    """Summary of metrics for a node."""

    validator_node_id: UUID
    latest_recorded_at: datetime
    avg_cpu_percent: float
    avg_memory_percent: float
    avg_disk_percent: float
    avg_peer_count: float
    max_block_height: int
    avg_health_score: float
    data_points: int


# =============================================================================
# INCIDENT SCHEMAS
# =============================================================================

class IncidentBase(BaseSchema):
    """Base schema for incident."""

    title: str = Field(..., min_length=5, max_length=255, description="Incident title")
    severity: str = Field(..., description="Incident severity")
    description: Optional[str] = Field(None, description="Detailed description")


class IncidentCreate(IncidentBase):
    """Schema for creating an incident."""

    validator_node_id: Optional[UUID] = None
    region_id: Optional[UUID] = None
    region_code: Optional[str] = Field(None, max_length=50)
    alert_type: Optional[str] = Field(None, max_length=50)
    category: Optional[str] = Field(None, max_length=50)
    impact: Optional[str] = None
    affected_validators: int = Field(1, ge=0)
    affected_customers: int = Field(0, ge=0)
    detected_by: Optional[str] = Field(None, max_length=100)
    detected_at: Optional[datetime] = None
    alert_id: Optional[str] = Field(None, max_length=255)
    assigned_to: Optional[str] = Field(None, max_length=100)
    tags: List[str] = Field(default_factory=list)


class IncidentUpdate(BaseSchema):
    """Schema for updating an incident."""

    title: Optional[str] = Field(None, min_length=5, max_length=255)
    severity: Optional[str] = None
    status: Optional[str] = None
    description: Optional[str] = None
    impact: Optional[str] = None
    assigned_to: Optional[str] = Field(None, max_length=100)
    root_cause: Optional[str] = None
    root_cause_category: Optional[str] = Field(None, max_length=50)
    resolution: Optional[str] = None
    resolution_type: Optional[str] = Field(None, max_length=50)
    public_message: Optional[str] = None
    lessons_learned: Optional[str] = None
    action_items: Optional[List[Dict[str, Any]]] = None
    tags: Optional[List[str]] = None


class IncidentResponse(IncidentBase):
    """Schema for incident response."""

    id: UUID
    validator_node_id: Optional[UUID]
    region_id: Optional[UUID]
    region_code: Optional[str]
    incident_number: str
    alert_type: Optional[str]
    category: Optional[str]
    status: str
    impact: Optional[str]
    affected_validators: int
    affected_customers: int
    detected_by: Optional[str]
    detected_at: datetime
    alert_id: Optional[str]
    acknowledged_by: Optional[str]
    acknowledged_at: Optional[datetime]
    assigned_to: Optional[str]
    escalated: bool
    escalated_at: Optional[datetime]
    root_cause: Optional[str]
    root_cause_category: Optional[str]
    contributing_factors: List[str]
    resolution: Optional[str]
    resolution_type: Optional[str]
    resolved_by: Optional[str]
    resolved_at: Optional[datetime]
    time_to_acknowledge_minutes: Optional[float]
    time_to_resolve_minutes: Optional[float]
    downtime_minutes: Optional[float]
    post_mortem_completed: bool
    post_mortem_url: Optional[str]
    lessons_learned: Optional[str]
    action_items: List[Dict[str, Any]]
    public_message: Optional[str]
    status_page_updated: bool
    customers_notified: bool
    related_incidents: List[str]
    timeline: List[Dict[str, Any]]
    attachments: List[Dict[str, Any]]
    tags: List[str]
    created_at: datetime
    updated_at: datetime
    closed_at: Optional[datetime]

    # Computed
    is_open: bool
    is_resolved: bool
    is_critical: bool
    is_acknowledged: bool
    age_hours: float
    meets_sla: bool


class IncidentSummary(BaseSchema):
    """Compact incident summary."""

    id: UUID
    incident_number: str
    title: str
    severity: str
    status: str
    detected_at: datetime
    affected_validators: int
    is_acknowledged: bool
    assigned_to: Optional[str]


class IncidentTimelineEvent(BaseSchema):
    """Incident timeline event."""

    timestamp: datetime
    message: str
    by: Optional[str]
    event_type: Optional[str] = None


class IncidentAcknowledge(BaseSchema):
    """Request to acknowledge an incident."""

    acknowledged_by: str = Field(..., min_length=1, max_length=100)
    notes: Optional[str] = None


class IncidentResolve(BaseSchema):
    """Request to resolve an incident."""

    resolved_by: str = Field(..., min_length=1, max_length=100)
    resolution: str = Field(..., min_length=10)
    resolution_type: str = Field("fixed")
    root_cause: Optional[str] = None
    root_cause_category: Optional[str] = None


class IncidentEscalate(BaseSchema):
    """Request to escalate an incident."""

    escalate_to: str = Field(..., min_length=1, max_length=100)
    escalated_by: Optional[str] = Field(None, max_length=100)
    reason: Optional[str] = None


class IncidentStats(BaseSchema):
    """Incident statistics."""

    total_incidents: int
    open_incidents: int
    critical_incidents: int
    avg_time_to_acknowledge_minutes: Optional[float]
    avg_time_to_resolve_minutes: Optional[float]
    sla_compliance_percent: float
    incidents_by_severity: Dict[str, int]
    incidents_by_category: Dict[str, int]
