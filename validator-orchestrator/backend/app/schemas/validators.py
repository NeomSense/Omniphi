"""Pydantic schemas for validator operations - matching frontend expectations."""

from datetime import datetime
from typing import Optional, Dict, Any
from uuid import UUID

from pydantic import BaseModel, Field, field_validator


# ==================== Setup Request Schemas ====================

class ValidatorSetupRequestCreate(BaseModel):
    """Schema for creating validator setup request."""

    walletAddress: str = Field(..., description="Bech32 wallet address (omni...)")
    validatorName: str = Field(..., min_length=3, max_length=100, description="Validator moniker")
    website: Optional[str] = Field(None, max_length=255, description="Validator website URL")
    description: Optional[str] = Field(None, max_length=1000, description="Validator description")
    commissionRate: float = Field(..., ge=0.0, le=1.0, description="Commission rate (0.0-1.0, e.g., 0.10 for 10%)")
    runMode: str = Field(..., description="'cloud' or 'local'")
    provider: str = Field(default="omniphi_cloud", description="Cloud provider")

    @field_validator("commissionRate")
    @classmethod
    def validate_commission(cls, v):
        if not 0.0 <= v <= 1.0:
            raise ValueError("Commission must be between 0.0 and 1.0")
        return v

    @field_validator("runMode")
    @classmethod
    def validate_run_mode(cls, v):
        if v not in ["cloud", "local"]:
            raise ValueError("runMode must be 'cloud' or 'local'")
        return v


class SetupRequestData(BaseModel):
    """Setup request data structure."""

    id: str
    status: str
    walletAddress: str
    validatorName: str
    runMode: str
    consensusPubkey: Optional[str] = None
    createdAt: str
    updatedAt: str


class NodeData(BaseModel):
    """Node data structure."""

    id: str
    status: str
    rpcEndpoint: Optional[str] = None
    p2pEndpoint: Optional[str] = None
    logsUrl: Optional[str] = None


class ValidatorSetupRequestResponse(BaseModel):
    """Response for setup request endpoint."""

    setupRequest: SetupRequestData
    node: Optional[NodeData] = None


# ==================== Validator By Wallet Schemas ====================

class SetupRequestSummary(BaseModel):
    """Summary of setup request."""

    id: str
    status: str
    validatorName: str
    runMode: str
    consensusPubkey: Optional[str] = None


class NodeSummary(BaseModel):
    """Summary of validator node."""

    status: str
    rpcEndpoint: Optional[str] = None
    p2pEndpoint: Optional[str] = None


class HeartbeatSummary(BaseModel):
    """Summary of local validator heartbeat."""

    blockHeight: int
    lastSeen: str


class ValidatorByWalletResponse(BaseModel):
    """Response for by-wallet endpoint."""

    setupRequest: SetupRequestSummary
    node: Optional[NodeSummary] = None
    chainInfo: Optional[Dict[str, Any]] = None
    heartbeat: Optional[HeartbeatSummary] = None


# ==================== Control Schemas ====================

class ValidatorStopRequest(BaseModel):
    """Request to stop a validator."""

    setupRequestId: str = Field(..., description="Setup request UUID")


class ValidatorRedeployRequest(BaseModel):
    """Request to redeploy a validator."""

    setupRequestId: str = Field(..., description="Setup request UUID")


# ==================== Local Validator Heartbeat Schemas ====================

class LocalValidatorHeartbeatCreate(BaseModel):
    """Schema for local validator heartbeat submission."""

    walletAddress: str = Field(..., description="Wallet address")
    consensusPubkey: str = Field(..., description="Consensus public key")
    blockHeight: int = Field(..., ge=0, description="Current block height")
    uptimeSeconds: int = Field(..., ge=0, description="Uptime in seconds")
    localRpcPort: Optional[int] = Field(None, description="Local RPC port")
    localP2pPort: Optional[int] = Field(None, description="Local P2P port")


class LocalValidatorHeartbeatResponse(BaseModel):
    """Response for heartbeat endpoint."""

    id: str
    blockHeight: int
    lastSeen: str
    message: str


# ==================== Chain Integration Schemas ====================

class ChainValidatorInfo(BaseModel):
    """Information about validator on-chain status."""

    isActiveValidator: bool = Field(..., description="Whether validator is active on chain")
    validatorAddress: Optional[str] = Field(None, description="Validator operator address (omnivaloper...)")
    votingPower: Optional[str] = Field(None, description="Voting power")
    jailed: Optional[bool] = Field(None, description="Whether validator is jailed")
    commission: Optional[float] = Field(None, description="Current commission rate")
    tokens: Optional[str] = Field(None, description="Staked tokens")


class CreateValidatorTxRequest(BaseModel):
    """Request to build MsgCreateValidator transaction."""

    walletAddress: str
    consensusPubkey: str
    selfDelegationAmount: int = Field(..., description="Self-delegation amount in base denom")
    validatorName: str
    commissionRate: float
    website: Optional[str] = None
    description: Optional[str] = None


class CreateValidatorTxResponse(BaseModel):
    """Response containing unsigned transaction body."""

    txBody: Dict[str, Any] = Field(..., description="Unsigned transaction body for wallet to sign")
    instructions: str = Field(..., description="Instructions for user")


class BroadcastTxRequest(BaseModel):
    """Request to broadcast signed transaction."""

    signedTx: Dict[str, Any] = Field(..., description="Fully signed transaction")


class BroadcastTxResponse(BaseModel):
    """Response from transaction broadcast."""

    success: bool
    txHash: Optional[str] = None
    code: Optional[int] = None
    rawLog: Optional[str] = None
    error: Optional[str] = None
