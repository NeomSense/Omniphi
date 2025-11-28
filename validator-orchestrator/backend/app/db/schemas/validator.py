"""
Validator Pydantic Schemas

Schemas for validator setup requests, nodes, and heartbeats.
"""

from datetime import datetime
from typing import Any, Dict, List, Optional
from uuid import UUID

from pydantic import Field, field_validator

from app.db.schemas.base import BaseSchema, PaginatedResponse


# =============================================================================
# VALIDATOR SETUP REQUEST SCHEMAS
# =============================================================================

class ValidatorSetupRequestBase(BaseSchema):
    """Base schema for validator setup request."""

    wallet_address: str = Field(
        ...,
        min_length=10,
        max_length=100,
        description="Bech32 wallet address (omni...)"
    )
    validator_name: str = Field(
        ...,
        min_length=3,
        max_length=100,
        description="Validator moniker"
    )
    commission_rate: float = Field(
        ...,
        ge=0.0,
        le=1.0,
        description="Commission rate (0.0-1.0)"
    )

    @field_validator("commission_rate")
    @classmethod
    def validate_commission(cls, v: float) -> float:
        """Validate commission rate is reasonable."""
        if v < 0 or v > 1:
            raise ValueError("Commission must be between 0.0 and 1.0")
        return round(v, 4)


class ValidatorSetupRequestCreate(ValidatorSetupRequestBase):
    """Schema for creating a validator setup request."""

    # Identity
    website: Optional[str] = Field(None, max_length=255, description="Validator website")
    description: Optional[str] = Field(None, max_length=1000, description="Description")
    security_contact: Optional[str] = Field(None, max_length=255, description="Security email")
    identity: Optional[str] = Field(None, max_length=100, description="Keybase identity")

    # Commission settings
    commission_max_rate: float = Field(0.20, ge=0.0, le=1.0, description="Max commission")
    commission_max_change_rate: float = Field(0.01, ge=0.0, le=0.1, description="Max daily change")

    # Stake
    stake_amount: Optional[int] = Field(None, ge=0, description="Self-delegation amount")
    min_self_delegation: int = Field(1, ge=1, description="Min self-delegation")

    # Deployment
    run_mode: str = Field("cloud", description="Deployment mode (cloud/local)")
    provider_id: Optional[UUID] = Field(None, description="Provider ID for cloud mode")
    region_id: Optional[UUID] = Field(None, description="Region ID for cloud mode")
    machine_type: Optional[str] = Field(None, description="Machine type")

    @field_validator("run_mode")
    @classmethod
    def validate_run_mode(cls, v: str) -> str:
        """Validate run mode."""
        if v not in ["cloud", "local"]:
            raise ValueError("run_mode must be 'cloud' or 'local'")
        return v


class ValidatorSetupRequestUpdate(BaseSchema):
    """Schema for updating a validator setup request."""

    status: Optional[str] = None
    status_message: Optional[str] = None
    error_message: Optional[str] = None
    consensus_pubkey: Optional[str] = None
    progress_percent: Optional[int] = Field(None, ge=0, le=100)
    current_step: Optional[str] = None
    chain_tx_hash: Optional[str] = None
    validator_operator_address: Optional[str] = None


class ValidatorSetupRequestResponse(ValidatorSetupRequestBase):
    """Schema for validator setup request response."""

    id: UUID
    website: Optional[str]
    description: Optional[str]
    security_contact: Optional[str]
    identity: Optional[str]
    commission_max_rate: float
    commission_max_change_rate: float
    stake_amount: Optional[int]
    min_self_delegation: int
    run_mode: str
    provider_id: Optional[UUID]
    region_id: Optional[UUID]
    machine_type: Optional[str]
    consensus_pubkey: Optional[str]
    consensus_pubkey_type: str
    status: str
    status_message: Optional[str]
    error_message: Optional[str]
    progress_percent: int
    current_step: Optional[str]
    chain_tx_hash: Optional[str]
    validator_operator_address: Optional[str]
    retry_count: int
    max_retries: int
    source: str
    created_at: datetime
    updated_at: datetime
    completed_at: Optional[datetime]
    failed_at: Optional[datetime]

    # Computed
    is_cloud_mode: bool
    is_pending: bool
    is_in_progress: bool
    is_completed: bool
    is_failed: bool
    can_retry: bool


class ValidatorSetupRequestSummary(BaseSchema):
    """Compact setup request summary."""

    id: UUID
    wallet_address: str
    validator_name: str
    run_mode: str
    status: str
    consensus_pubkey: Optional[str]
    progress_percent: int
    created_at: datetime


# =============================================================================
# VALIDATOR NODE SCHEMAS
# =============================================================================

class ValidatorNodeBase(BaseSchema):
    """Base schema for validator node."""

    rpc_endpoint: Optional[str] = Field(None, description="RPC endpoint URL")
    p2p_endpoint: Optional[str] = Field(None, description="P2P endpoint")
    grpc_endpoint: Optional[str] = Field(None, description="gRPC endpoint")


class ValidatorNodeCreate(ValidatorNodeBase):
    """Schema for creating a validator node."""

    setup_request_id: UUID = Field(..., description="Parent setup request")
    provider_id: Optional[UUID] = None
    region_id: Optional[UUID] = None
    server_id: Optional[UUID] = None
    container_id: Optional[str] = None
    vm_instance_id: Optional[str] = None
    rest_endpoint: Optional[str] = None
    metrics_endpoint: Optional[str] = None
    internal_ip: Optional[str] = None
    external_ip: Optional[str] = None
    p2p_port: int = Field(26656)
    rpc_port: int = Field(26657)
    cpu_cores: Optional[int] = None
    memory_gb: Optional[int] = None
    disk_gb: Optional[int] = None
    node_version: Optional[str] = None
    chain_id: Optional[str] = None


class ValidatorNodeUpdate(BaseSchema):
    """Schema for updating a validator node."""

    status: Optional[str] = None
    is_active: Optional[bool] = None
    last_block_height: Optional[int] = None
    is_synced: Optional[bool] = None
    peer_count: Optional[int] = None
    catching_up: Optional[bool] = None
    is_validator: Optional[bool] = None
    voting_power: Optional[str] = None
    is_jailed: Optional[bool] = None
    health_score: Optional[float] = Field(None, ge=0, le=100)
    node_version: Optional[str] = None
    labels: Optional[Dict[str, str]] = None


class ValidatorNodeResponse(ValidatorNodeBase):
    """Schema for validator node response."""

    id: UUID
    setup_request_id: UUID
    provider_id: Optional[UUID]
    region_id: Optional[UUID]
    server_id: Optional[UUID]
    container_id: Optional[str]
    vm_instance_id: Optional[str]
    kubernetes_pod: Optional[str]
    rest_endpoint: Optional[str]
    metrics_endpoint: Optional[str]
    internal_ip: Optional[str]
    external_ip: Optional[str]
    p2p_port: int
    rpc_port: int
    status: str
    is_active: bool
    last_block_height: Optional[int]
    is_synced: bool
    peer_count: Optional[int]
    catching_up: bool
    is_validator: bool
    voting_power: Optional[str]
    is_jailed: bool
    jailed_until: Optional[datetime]
    missed_blocks: int
    cpu_cores: Optional[int]
    memory_gb: Optional[int]
    disk_gb: Optional[int]
    bandwidth_gbps: Optional[float]
    node_version: Optional[str]
    chain_id: Optional[str]
    last_heartbeat: Optional[datetime]
    last_health_check: Optional[datetime]
    health_score: float
    uptime_percent: float
    logs_url: Optional[str]
    created_at: datetime
    updated_at: datetime
    started_at: Optional[datetime]
    stopped_at: Optional[datetime]
    terminated_at: Optional[datetime]

    # Computed
    is_running: bool
    is_healthy: bool
    needs_attention: bool


# =============================================================================
# LOCAL VALIDATOR HEARTBEAT SCHEMAS
# =============================================================================

class LocalValidatorHeartbeatCreate(BaseSchema):
    """Schema for submitting a local validator heartbeat."""

    wallet_address: str = Field(..., description="Wallet address")
    consensus_pubkey: str = Field(..., description="Consensus public key")
    block_height: int = Field(..., ge=0, description="Current block height")
    uptime_seconds: int = Field(..., ge=0, description="Uptime in seconds")

    # Optional fields
    peer_count: Optional[int] = Field(None, ge=0)
    is_synced: bool = Field(False)
    catching_up: bool = Field(True)
    cpu_percent: Optional[float] = Field(None, ge=0, le=100)
    memory_percent: Optional[float] = Field(None, ge=0, le=100)
    disk_percent: Optional[float] = Field(None, ge=0, le=100)
    local_rpc_port: Optional[int] = Field(26657)
    local_p2p_port: Optional[int] = Field(26656)
    local_grpc_port: Optional[int] = Field(9090)
    node_version: Optional[str] = None
    app_version: Optional[str] = None
    chain_id: Optional[str] = None
    os_type: Optional[str] = None
    os_version: Optional[str] = None
    machine_id: Optional[str] = None


class LocalValidatorHeartbeatResponse(BaseSchema):
    """Schema for local validator heartbeat response."""

    id: UUID
    wallet_address: str
    consensus_pubkey: str
    validator_operator_address: Optional[str]
    block_height: int
    is_synced: bool
    catching_up: bool
    peer_count: Optional[int]
    is_active_validator: bool
    voting_power: Optional[str]
    is_jailed: bool
    uptime_seconds: int
    cpu_percent: Optional[float]
    memory_percent: Optional[float]
    disk_percent: Optional[float]
    local_rpc_port: Optional[int]
    local_p2p_port: Optional[int]
    local_grpc_port: Optional[int]
    node_version: Optional[str]
    app_version: Optional[str]
    chain_id: Optional[str]
    first_seen: datetime
    last_seen: datetime
    created_at: datetime

    # Computed
    is_online: bool
    is_healthy: bool
    uptime_hours: float
