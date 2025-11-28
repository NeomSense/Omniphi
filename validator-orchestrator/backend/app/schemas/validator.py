"""Pydantic schemas for validator operations."""

from datetime import datetime
from typing import Optional
from uuid import UUID

from pydantic import BaseModel, Field, field_validator


# ==================== Setup Request Schemas ====================

class ValidatorSetupRequestCreate(BaseModel):
    """Schema for creating a new validator setup request."""

    wallet_address: str = Field(..., description="Bech32 wallet address")
    validator_name: str = Field(..., min_length=3, max_length=100)
    website: Optional[str] = Field(None, max_length=255)
    description: Optional[str] = Field(None, max_length=1000)
    commission_rate: float = Field(..., ge=0.0, le=1.0, description="Commission rate (0.0-1.0)")
    run_mode: str = Field(..., description="cloud or local")
    provider: str = Field(default="omniphi_cloud", description="Cloud provider")

    @field_validator("commission_rate")
    @classmethod
    def validate_commission(cls, v):
        if not 0.0 <= v <= 1.0:
            raise ValueError("Commission must be between 0 and 1")
        return v

    @field_validator("run_mode")
    @classmethod
    def validate_run_mode(cls, v):
        if v not in ["cloud", "local"]:
            raise ValueError("run_mode must be 'cloud' or 'local'")
        return v


class ValidatorSetupRequestResponse(BaseModel):
    """Response schema for validator setup request."""

    id: UUID
    wallet_address: str
    validator_name: str
    website: Optional[str]
    description: Optional[str]
    commission_rate: float
    run_mode: str
    provider: str
    consensus_pubkey: Optional[str]
    status: str
    error_message: Optional[str]
    created_at: datetime
    updated_at: datetime
    completed_at: Optional[datetime]

    class Config:
        from_attributes = True


class ValidatorSetupRequestUpdate(BaseModel):
    """Schema for updating setup request status."""

    status: Optional[str] = None
    consensus_pubkey: Optional[str] = None
    error_message: Optional[str] = None


# ==================== Node Schemas ====================

class ValidatorNodeResponse(BaseModel):
    """Response schema for validator node."""

    id: UUID
    setup_request_id: UUID
    provider: str
    node_internal_id: str
    rpc_endpoint: Optional[str]
    p2p_endpoint: Optional[str]
    grpc_endpoint: Optional[str]
    status: str
    logs_url: Optional[str]
    last_block_height: Optional[str]
    last_health_check: Optional[datetime]
    cpu_cores: Optional[str]
    memory_gb: Optional[str]
    disk_gb: Optional[str]
    created_at: datetime
    updated_at: datetime

    class Config:
        from_attributes = True


# ==================== Heartbeat Schemas ====================

class LocalValidatorHeartbeatCreate(BaseModel):
    """Schema for local validator heartbeat."""

    wallet_address: str
    consensus_pubkey: str
    block_height: int
    uptime_seconds: int
    local_rpc_port: Optional[int] = None
    local_p2p_port: Optional[int] = None


class LocalValidatorHeartbeatResponse(BaseModel):
    """Response schema for local validator heartbeat."""

    id: UUID
    wallet_address: str
    consensus_pubkey: str
    block_height: int
    uptime_seconds: int
    local_rpc_port: Optional[int]
    local_p2p_port: Optional[int]
    first_seen: datetime
    last_seen: datetime

    class Config:
        from_attributes = True


# ==================== Chain Validator Schemas ====================

class ChainValidatorInfo(BaseModel):
    """Information about validator on-chain status."""

    operator_address: str
    consensus_pubkey: str
    jailed: bool
    status: str  # BOND_STATUS_UNBONDED, BOND_STATUS_BONDED, etc.
    tokens: str
    delegator_shares: str
    description: dict
    commission: dict
    min_self_delegation: str
    unbonding_height: str
    unbonding_time: str


# ==================== Combined Response ====================

class ValidatorCompleteInfo(BaseModel):
    """Complete validator information combining all sources."""

    setup_request: ValidatorSetupRequestResponse
    node: Optional[ValidatorNodeResponse]
    chain_info: Optional[ChainValidatorInfo]
    heartbeat: Optional[LocalValidatorHeartbeatResponse]


# ==================== Control Schemas ====================

class ValidatorStopRequest(BaseModel):
    """Request to stop a validator."""

    setup_request_id: UUID


class ValidatorRedeployRequest(BaseModel):
    """Request to redeploy a validator."""

    setup_request_id: UUID


# ==================== Health Schema ====================

class HealthResponse(BaseModel):
    """API health check response."""

    status: str
    version: str
    database: str
    timestamp: datetime
