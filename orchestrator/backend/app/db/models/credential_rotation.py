"""
Credential Rotation Model for Automated Key Rotation System

Part of HIGH-1 Security Remediation: Tracks credential rotation operations
across all credential types (API keys, cloud provider credentials, etc.)

Implements:
- Zero-downtime rotation tracking
- Multi-stage rotation lifecycle
- Rollback capability
- Comprehensive audit trail
- Emergency rotation support
"""

import uuid
from datetime import datetime
from typing import Optional, Dict, Any

from sqlalchemy import Column, String, DateTime, Index, ForeignKey, Interval, Boolean, Integer
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship

from app.db.models.base import AuditableModel
from app.db.models.enums import RotationStatus, CredentialType


class CredentialRotation(AuditableModel):
    """
    Tracks credential rotation operations for zero-downtime key management.

    Rotation lifecycle:
    1. PENDING: Rotation scheduled but not started
    2. GENERATING: New credentials being generated
    3. DEPLOYING: New credentials being deployed to systems
    4. TESTING: Verifying new credentials work
    5. ACTIVE: Both old and new credentials valid (overlap period)
    6. FINALIZING: Revoking old credentials
    7. COMPLETED: Rotation successfully completed
    8. FAILED: Rotation failed (old credentials still valid)
    9. ROLLED_BACK: Rotation aborted, reverted to old credentials
    """

    __tablename__ = "credential_rotations"

    # Rotation identification
    rotation_name = Column(
        String(255),
        nullable=False,
        doc="Human-readable name for this rotation operation"
    )

    # Credential type and target
    credential_type = Column(
        String(50),
        nullable=False,
        index=True,
        doc="Type: api_key, aws_iam, digitalocean_token, master_key, etc."
    )

    resource_type = Column(
        String(100),
        nullable=True,
        doc="Resource being rotated: service_name, provider_id, etc."
    )

    resource_id = Column(
        String(255),
        nullable=True,
        index=True,
        doc="Specific resource identifier (e.g., provider UUID, service name)"
    )

    # Rotation status and lifecycle
    status = Column(
        String(50),
        nullable=False,
        default=RotationStatus.PENDING.value,
        index=True,
        doc="Current rotation status"
    )

    # Credential references
    old_credential_id = Column(
        UUID(as_uuid=True),
        nullable=True,
        doc="Reference to old credential being rotated out"
    )

    new_credential_id = Column(
        UUID(as_uuid=True),
        nullable=True,
        doc="Reference to new credential being rotated in"
    )

    # Timing
    scheduled_at = Column(
        DateTime,
        nullable=True,
        index=True,
        doc="When this rotation is scheduled to begin"
    )

    started_at = Column(
        DateTime,
        nullable=True,
        doc="When rotation actually started"
    )

    completed_at = Column(
        DateTime,
        nullable=True,
        doc="When rotation finished (success or failure)"
    )

    overlap_duration = Column(
        Interval,
        nullable=True,
        doc="How long both old and new credentials remain valid"
    )

    # Rotation trigger
    rotation_reason = Column(
        String(500),
        nullable=False,
        doc="Reason: scheduled, manual, emergency, compromised, policy"
    )

    triggered_by = Column(
        UUID(as_uuid=True),
        ForeignKey("users.id", ondelete="SET NULL"),
        nullable=True,
        doc="User who initiated rotation (null for automated)"
    )

    # Error tracking
    error_message = Column(
        String(2000),
        nullable=True,
        doc="Error details if rotation failed"
    )

    error_stage = Column(
        String(100),
        nullable=True,
        doc="Which stage failed: generating, deploying, testing, finalizing"
    )

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
        doc="Maximum retry attempts before marking as failed"
    )

    # Rollback support
    can_rollback = Column(
        Boolean,
        nullable=False,
        default=True,
        doc="Whether this rotation can be rolled back"
    )

    rolled_back_at = Column(
        DateTime,
        nullable=True,
        doc="When rotation was rolled back"
    )

    rollback_reason = Column(
        String(500),
        nullable=True,
        doc="Why rotation was rolled back"
    )

    # Validation and testing
    validation_tests = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="List of validation tests performed: [{test: 'api_auth', status: 'passed'}]"
    )

    # Metadata
    metadata = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional context: affected_services, downstream_dependencies, etc."
    )

    # Relationships
    api_keys = relationship(
        "APIKey",
        foreign_keys="APIKey.rotation_id",
        back_populates="rotation"
    )

    # Table indexes
    __table_args__ = (
        Index('ix_rotations_status_scheduled', 'status', 'scheduled_at'),
        Index('ix_rotations_type_resource', 'credential_type', 'resource_id'),
        Index('ix_rotations_created_status', 'created_at', 'status'),
    )

    def start(self) -> None:
        """Mark rotation as started."""
        self.status = RotationStatus.GENERATING.value
        self.started_at = datetime.utcnow()

    def advance_to_deploying(self) -> None:
        """Move to deploying stage."""
        self.status = RotationStatus.DEPLOYING.value

    def advance_to_testing(self) -> None:
        """Move to testing stage."""
        self.status = RotationStatus.TESTING.value

    def advance_to_active(self) -> None:
        """Move to active overlap period."""
        self.status = RotationStatus.ACTIVE.value

    def advance_to_finalizing(self) -> None:
        """Move to finalizing stage (revoking old credentials)."""
        self.status = RotationStatus.FINALIZING.value

    def mark_completed(self) -> None:
        """Mark rotation as successfully completed."""
        self.status = RotationStatus.COMPLETED.value
        self.completed_at = datetime.utcnow()

    def mark_failed(self, error_message: str, error_stage: str) -> None:
        """
        Mark rotation as failed.

        Args:
            error_message: Description of what went wrong
            error_stage: Which stage failed
        """
        self.status = RotationStatus.FAILED.value
        self.completed_at = datetime.utcnow()
        self.error_message = error_message
        self.error_stage = error_stage
        self.retry_count += 1

    def rollback(self, reason: str) -> None:
        """
        Rollback rotation to previous credentials.

        Args:
            reason: Why rollback is needed
        """
        if not self.can_rollback:
            raise ValueError("This rotation cannot be rolled back")

        self.status = RotationStatus.ROLLED_BACK.value
        self.rolled_back_at = datetime.utcnow()
        self.rollback_reason = reason

    def add_validation_test(self, test_name: str, status: str, details: Optional[Dict] = None) -> None:
        """
        Add a validation test result.

        Args:
            test_name: Name of test (e.g., 'api_authentication')
            status: 'passed', 'failed', 'skipped'
            details: Additional test details
        """
        if self.validation_tests is None:
            self.validation_tests = []

        self.validation_tests.append({
            'test': test_name,
            'status': status,
            'timestamp': datetime.utcnow().isoformat(),
            'details': details or {}
        })

    def can_retry(self) -> bool:
        """Check if rotation can be retried."""
        return self.retry_count < self.max_retries

    def is_in_overlap_period(self) -> bool:
        """Check if currently in credential overlap period."""
        return self.status == RotationStatus.ACTIVE.value

    def __repr__(self) -> str:
        return f"<CredentialRotation(type={self.credential_type}, status={self.status})>"
