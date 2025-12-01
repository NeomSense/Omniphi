"""Orchestrator Settings API endpoints."""

from datetime import datetime
import logging

from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel
from typing import List, Dict, Any, Optional
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import OrchestratorSettings, AuditLog, AuditAction

logger = logging.getLogger(__name__)

router = APIRouter()


class ChainRpcEndpoint(BaseModel):
    chain_id: str
    endpoints: List[str]
    priority: int


class SnapshotUrl(BaseModel):
    chain_id: str
    url: str
    type: str  # "pruned" or "archive"
    provider: str


class AlertThresholds(BaseModel):
    max_provisioning_time_minutes: int
    min_success_rate_percent: int
    max_consecutive_failures: int
    health_check_timeout_seconds: int


class SettingsUpdate(BaseModel):
    default_provider: str
    max_parallel_jobs: int
    provisioning_retry_limit: int
    heartbeat_interval_seconds: int
    log_retention_days: int
    chain_rpc_endpoints: List[ChainRpcEndpoint]
    snapshot_urls: List[SnapshotUrl]
    alert_thresholds: AlertThresholds
    auto_failover_enabled: Optional[bool] = True


def get_or_create_settings(db: Session) -> OrchestratorSettings:
    """Get existing settings or create default ones."""
    settings = db.query(OrchestratorSettings).first()

    if not settings:
        settings = OrchestratorSettings.get_default_settings()
        db.add(settings)
        db.commit()
        db.refresh(settings)

    return settings


@router.get("")
async def get_settings(db: Session = Depends(get_db)):
    """
    Get current orchestrator settings.
    """
    settings = get_or_create_settings(db)

    return {
        "id": str(settings.id),
        "default_provider": settings.default_provider,
        "max_parallel_jobs": settings.max_parallel_jobs,
        "provisioning_retry_limit": settings.provisioning_retry_limit,
        "heartbeat_interval_seconds": settings.heartbeat_interval_seconds,
        "log_retention_days": settings.log_retention_days,
        "chain_rpc_endpoints": settings.chain_rpc_endpoints or [],
        "snapshot_urls": settings.snapshot_urls or [],
        "alert_thresholds": settings.alert_thresholds or {
            "max_provisioning_time_minutes": 30,
            "min_success_rate_percent": 90,
            "max_consecutive_failures": 3,
            "health_check_timeout_seconds": 60
        },
        "auto_failover_enabled": settings.auto_failover_enabled,
        "created_at": settings.created_at.isoformat(),
        "updated_at": settings.updated_at.isoformat()
    }


@router.put("")
async def update_settings(
    update: SettingsUpdate,
    db: Session = Depends(get_db)
):
    """
    Update orchestrator settings.
    """
    settings = get_or_create_settings(db)

    # Track changes for audit log
    changes = {}

    if settings.default_provider != update.default_provider:
        changes["default_provider"] = {"old": settings.default_provider, "new": update.default_provider}
        settings.default_provider = update.default_provider

    if settings.max_parallel_jobs != update.max_parallel_jobs:
        changes["max_parallel_jobs"] = {"old": settings.max_parallel_jobs, "new": update.max_parallel_jobs}
        settings.max_parallel_jobs = update.max_parallel_jobs

    if settings.provisioning_retry_limit != update.provisioning_retry_limit:
        changes["provisioning_retry_limit"] = {"old": settings.provisioning_retry_limit, "new": update.provisioning_retry_limit}
        settings.provisioning_retry_limit = update.provisioning_retry_limit

    if settings.heartbeat_interval_seconds != update.heartbeat_interval_seconds:
        changes["heartbeat_interval_seconds"] = {"old": settings.heartbeat_interval_seconds, "new": update.heartbeat_interval_seconds}
        settings.heartbeat_interval_seconds = update.heartbeat_interval_seconds

    if settings.log_retention_days != update.log_retention_days:
        changes["log_retention_days"] = {"old": settings.log_retention_days, "new": update.log_retention_days}
        settings.log_retention_days = update.log_retention_days

    # Update JSON fields
    settings.chain_rpc_endpoints = [endpoint.model_dump() for endpoint in update.chain_rpc_endpoints]
    settings.snapshot_urls = [snapshot.model_dump() for snapshot in update.snapshot_urls]
    settings.alert_thresholds = update.alert_thresholds.model_dump()

    if update.auto_failover_enabled is not None:
        settings.auto_failover_enabled = update.auto_failover_enabled

    settings.updated_at = datetime.utcnow()

    # Create audit log entry
    if changes:
        audit = AuditLog(
            user_id="admin",
            username="admin",
            action=AuditAction.UPDATE_SETTINGS,
            resource_type="settings",
            resource_id=str(settings.id),
            details=changes,
            ip_address="127.0.0.1"
        )
        db.add(audit)

    db.commit()
    db.refresh(settings)

    logger.info(f"Settings updated: {changes}")

    return {
        "id": str(settings.id),
        "default_provider": settings.default_provider,
        "max_parallel_jobs": settings.max_parallel_jobs,
        "provisioning_retry_limit": settings.provisioning_retry_limit,
        "heartbeat_interval_seconds": settings.heartbeat_interval_seconds,
        "log_retention_days": settings.log_retention_days,
        "chain_rpc_endpoints": settings.chain_rpc_endpoints,
        "snapshot_urls": settings.snapshot_urls,
        "alert_thresholds": settings.alert_thresholds,
        "auto_failover_enabled": settings.auto_failover_enabled,
        "created_at": settings.created_at.isoformat(),
        "updated_at": settings.updated_at.isoformat(),
        "message": "Settings updated successfully"
    }
