"""Orchestrator Settings Model."""

import uuid
from datetime import datetime
from typing import Optional

from sqlalchemy import Column, String, DateTime, Integer, JSON, Boolean
from sqlalchemy.dialects.postgresql import UUID

from app.db.base_class import Base


class OrchestratorSettings(Base):
    """Orchestrator system settings table."""

    __tablename__ = "orchestrator_settings"

    # Primary key
    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4, index=True)

    # General Settings
    default_provider = Column(String, nullable=False, default="aws")
    max_parallel_jobs = Column(Integer, nullable=False, default=5)
    provisioning_retry_limit = Column(Integer, nullable=False, default=3)
    heartbeat_interval_seconds = Column(Integer, nullable=False, default=30)
    log_retention_days = Column(Integer, nullable=False, default=30)

    # Chain RPC Endpoints (JSON array)
    chain_rpc_endpoints = Column(JSON, nullable=False, default=list)

    # Snapshot URLs (JSON array)
    snapshot_urls = Column(JSON, nullable=False, default=list)

    # Alert Thresholds (JSON object)
    alert_thresholds = Column(JSON, nullable=False, default=dict)

    # Auto-failover settings
    auto_failover_enabled = Column(Boolean, nullable=False, default=True)

    # Timestamps
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow, onupdate=datetime.utcnow)

    def __repr__(self):
        return f"<OrchestratorSettings {self.id}>"

    @classmethod
    def get_default_settings(cls):
        """Return default settings object."""
        return cls(
            default_provider="aws",
            max_parallel_jobs=5,
            provisioning_retry_limit=3,
            heartbeat_interval_seconds=30,
            log_retention_days=30,
            chain_rpc_endpoints=[
                {
                    "chain_id": "omniphi-mainnet-1",
                    "endpoints": [
                        "https://rpc.omniphi.network",
                        "https://rpc-backup.omniphi.network"
                    ],
                    "priority": 1
                }
            ],
            snapshot_urls=[
                {
                    "chain_id": "omniphi-mainnet-1",
                    "url": "https://snapshots.omniphi.network/latest.tar.gz",
                    "type": "pruned",
                    "provider": "omniphi"
                }
            ],
            alert_thresholds={
                "max_provisioning_time_minutes": 30,
                "min_success_rate_percent": 90,
                "max_consecutive_failures": 3,
                "health_check_timeout_seconds": 60
            },
            auto_failover_enabled=True
        )
