"""
Validator Setup Request Model

Tracks pending and completed validator onboarding requests.
This is the entry point for all validator provisioning.

Table: validator_setup_requests
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
    Text,
    Index,
)
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship, Mapped

from app.db.database import Base
from app.db.models.enums import RunMode, SetupStatus

if TYPE_CHECKING:
    from app.db.models.validator_node import ValidatorNode
    from app.db.models.provider import Provider
    from app.db.models.region import Region


class ValidatorSetupRequest(Base):
    """
    Validator setup request tracking table.

    Tracks the full lifecycle of validator onboarding from initial request
    through provisioning, configuration, chain registration, and activation.
    """

    __tablename__ = "validator_setup_requests"

    # Primary key
    id = Column(
        UUID(as_uuid=True),
        primary_key=True,
        default=uuid.uuid4,
        index=True
    )

    # Wallet and identity
    wallet_address = Column(
        String(100),
        nullable=False,
        index=True,
        doc="Bech32 wallet address (omni...)"
    )
    validator_name = Column(
        String(100),
        nullable=False,
        doc="Validator moniker/name"
    )
    website = Column(
        String(255),
        nullable=True,
        doc="Validator website URL"
    )
    description = Column(
        Text,
        nullable=True,
        doc="Validator description"
    )
    security_contact = Column(
        String(255),
        nullable=True,
        doc="Security contact email"
    )
    identity = Column(
        String(100),
        nullable=True,
        doc="Keybase or similar identity verification"
    )

    # Commission settings
    commission_rate = Column(
        Float,
        nullable=False,
        doc="Commission rate (0.0-1.0, e.g., 0.10 for 10%)"
    )
    commission_max_rate = Column(
        Float,
        nullable=False,
        default=0.20,
        doc="Maximum commission rate"
    )
    commission_max_change_rate = Column(
        Float,
        nullable=False,
        default=0.01,
        doc="Maximum daily commission change rate"
    )

    # Stake configuration
    stake_amount = Column(
        Integer,
        nullable=True,
        doc="Self-delegation amount in base denom"
    )
    min_self_delegation = Column(
        Integer,
        nullable=False,
        default=1,
        doc="Minimum self-delegation"
    )

    # Deployment configuration
    run_mode = Column(
        String(20),
        nullable=False,
        default=RunMode.CLOUD.value,
        index=True,
        doc="Deployment mode: cloud or local"
    )

    # Provider and region (for cloud mode)
    provider_id = Column(
        UUID(as_uuid=True),
        ForeignKey("providers.id", ondelete="SET NULL"),
        nullable=True,
        index=True,
        doc="Hosting provider"
    )
    region_id = Column(
        UUID(as_uuid=True),
        ForeignKey("regions.id", ondelete="SET NULL"),
        nullable=True,
        index=True,
        doc="Deployment region"
    )
    machine_type = Column(
        String(50),
        nullable=True,
        doc="Selected machine type"
    )

    # Consensus key (generated during provisioning)
    consensus_pubkey = Column(
        String(255),
        nullable=True,
        index=True,
        doc="Tendermint consensus public key (base64)"
    )
    consensus_pubkey_type = Column(
        String(50),
        nullable=False,
        default="ed25519",
        doc="Public key type"
    )

    # Status tracking
    status = Column(
        String(50),
        nullable=False,
        default=SetupStatus.PENDING.value,
        index=True,
        doc="Current setup status"
    )
    status_message = Column(
        Text,
        nullable=True,
        doc="Human-readable status message"
    )
    error_message = Column(
        Text,
        nullable=True,
        doc="Error message if failed"
    )
    error_code = Column(
        String(50),
        nullable=True,
        doc="Error code for programmatic handling"
    )

    # Progress tracking
    progress_percent = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Setup progress (0-100)"
    )
    current_step = Column(
        String(100),
        nullable=True,
        doc="Current setup step"
    )

    # Chain transaction info
    chain_tx_hash = Column(
        String(100),
        nullable=True,
        index=True,
        doc="MsgCreateValidator transaction hash"
    )
    validator_operator_address = Column(
        String(100),
        nullable=True,
        index=True,
        doc="Validator operator address (omnivaloper...)"
    )

    # Retry tracking
    retry_count = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of retry attempts"
    )
    max_retries = Column(
        Integer,
        nullable=False,
        default=3,
        doc="Maximum retry attempts"
    )
    last_retry_at = Column(
        DateTime,
        nullable=True,
        doc="Last retry timestamp"
    )

    # Extra data
    extra_data = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional metadata"
    )
    source = Column(
        String(50),
        nullable=False,
        default="web",
        doc="Request source (web, api, cli)"
    )
    ip_address = Column(
        String(45),
        nullable=True,
        doc="Client IP address"
    )
    user_agent = Column(
        String(500),
        nullable=True,
        doc="Client user agent"
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
    completed_at = Column(
        DateTime,
        nullable=True,
        doc="Completion timestamp"
    )
    failed_at = Column(
        DateTime,
        nullable=True,
        doc="Failure timestamp"
    )

    # Relationships
    nodes: Mapped[list["ValidatorNode"]] = relationship(
        "ValidatorNode",
        back_populates="setup_request",
        cascade="all, delete-orphan",
        lazy="selectin"
    )
    provider: Mapped[Optional["Provider"]] = relationship(
        "Provider",
        foreign_keys=[provider_id]
    )
    region: Mapped[Optional["Region"]] = relationship(
        "Region",
        foreign_keys=[region_id]
    )

    # Indexes
    __table_args__ = (
        Index("ix_setup_requests_wallet_status", "wallet_address", "status"),
        Index("ix_setup_requests_provider_status", "provider_id", "status"),
        Index("ix_setup_requests_region_status", "region_id", "status"),
        Index("ix_setup_requests_created", "created_at"),
    )

    def __repr__(self) -> str:
        return f"<ValidatorSetupRequest {self.validator_name} ({self.wallet_address})>"

    @property
    def is_cloud_mode(self) -> bool:
        """Check if this is a cloud deployment."""
        return self.run_mode == RunMode.CLOUD.value

    @property
    def is_local_mode(self) -> bool:
        """Check if this is a local deployment."""
        return self.run_mode == RunMode.LOCAL.value

    @property
    def is_pending(self) -> bool:
        """Check if request is pending."""
        return self.status == SetupStatus.PENDING.value

    @property
    def is_in_progress(self) -> bool:
        """Check if request is in progress."""
        return self.status in [
            SetupStatus.PROVISIONING.value,
            SetupStatus.CONFIGURING.value,
            SetupStatus.READY_FOR_CHAIN_TX.value,
        ]

    @property
    def is_completed(self) -> bool:
        """Check if request is completed."""
        return self.status in [
            SetupStatus.ACTIVE.value,
            SetupStatus.COMPLETED.value,
        ]

    @property
    def is_failed(self) -> bool:
        """Check if request has failed."""
        return self.status == SetupStatus.FAILED.value

    @property
    def can_retry(self) -> bool:
        """Check if request can be retried."""
        return self.is_failed and self.retry_count < self.max_retries

    @property
    def active_node(self) -> Optional["ValidatorNode"]:
        """Get the active node for this request."""
        if not self.nodes:
            return None
        for node in self.nodes:
            if node.is_active:
                return node
        return self.nodes[0] if self.nodes else None

    def set_status(
        self,
        status: SetupStatus,
        message: Optional[str] = None,
        error: Optional[str] = None,
        error_code: Optional[str] = None,
    ) -> None:
        """
        Update request status with optional messages.

        Args:
            status: New status
            message: Status message
            error: Error message (for failed status)
            error_code: Error code
        """
        self.status = status.value
        self.status_message = message

        if status == SetupStatus.FAILED:
            self.error_message = error
            self.error_code = error_code
            self.failed_at = datetime.utcnow()
        elif status in [SetupStatus.ACTIVE, SetupStatus.COMPLETED]:
            self.completed_at = datetime.utcnow()
            self.progress_percent = 100

    def increment_retry(self) -> bool:
        """
        Increment retry count if possible.

        Returns:
            True if retry is allowed
        """
        if not self.can_retry:
            return False

        self.retry_count += 1
        self.last_retry_at = datetime.utcnow()
        self.status = SetupStatus.PENDING.value
        self.error_message = None
        self.error_code = None
        return True
