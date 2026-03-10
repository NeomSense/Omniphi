"""
API Key Model for Automated Key Rotation System

Part of HIGH-1 Security Remediation: Automated API key rotation with
zero-downtime credential management, audit trails, and emergency revocation.

Implements:
- Hashed key storage with bcrypt
- Key lifecycle management (active, rotating, expired, revoked)
- Scoped permissions system
- Last-used tracking
- Rotation chains for zero-downtime rotation
"""

import uuid
from datetime import datetime, timedelta
from typing import Optional, List, Dict, Any

from sqlalchemy import Column, String, DateTime, Index, ForeignKey, Integer
from sqlalchemy.dialects.postgresql import UUID, JSONB
from sqlalchemy.orm import relationship

from app.db.models.base import AuditableModel
from app.db.models.enums import APIKeyStatus


class APIKey(AuditableModel):
    """
    API Key model for authentication and authorization.

    Security features:
    - Keys are stored hashed (never in plaintext)
    - Only key_prefix (first 8 chars) stored for display/logging
    - Constant-time comparison for validation
    - Automatic expiration and rotation support
    - Comprehensive audit trail
    """

    __tablename__ = "api_keys"

    # Key identification and storage
    key_hash = Column(
        String(255),
        nullable=False,
        index=True,
        doc="bcrypt hash of the API key (NEVER store plaintext)"
    )

    key_prefix = Column(
        String(8),
        nullable=False,
        index=True,
        doc="First 8 characters of key for display (e.g., 'ak_12345')"
    )

    # Key metadata
    name = Column(
        String(255),
        nullable=False,
        doc="Human-readable name/description for this key"
    )

    status = Column(
        String(50),
        nullable=False,
        default=APIKeyStatus.ACTIVE.value,
        index=True,
        doc="Key lifecycle status: active, rotating, expired, revoked"
    )

    # Lifecycle timestamps
    expires_at = Column(
        DateTime,
        nullable=True,
        index=True,
        doc="Expiration timestamp (null = never expires)"
    )

    last_used_at = Column(
        DateTime,
        nullable=True,
        doc="Last successful authentication with this key"
    )

    revoked_at = Column(
        DateTime,
        nullable=True,
        doc="Timestamp when key was revoked (if applicable)"
    )

    revoked_reason = Column(
        String(500),
        nullable=True,
        doc="Reason for revocation: emergency, compromised, scheduled_rotation"
    )

    # Permissions and scopes
    scopes = Column(
        JSONB,
        nullable=False,
        default=list,
        doc="List of permission scopes: ['read:validators', 'write:providers']"
    )

    # Usage tracking
    usage_count = Column(
        Integer,
        nullable=False,
        default=0,
        doc="Number of times this key has been used successfully"
    )

    last_used_ip = Column(
        String(45),
        nullable=True,
        doc="IP address of last successful authentication"
    )

    # Rotation chain - for zero-downtime key rotation
    rotation_id = Column(
        UUID(as_uuid=True),
        ForeignKey("credential_rotations.id", ondelete="SET NULL"),
        nullable=True,
        index=True,
        doc="Links to rotation record if this key is part of a rotation"
    )

    replaces_key_id = Column(
        UUID(as_uuid=True),
        ForeignKey("api_keys.id", ondelete="SET NULL"),
        nullable=True,
        doc="Points to the old key this key is replacing during rotation"
    )

    # Metadata
    metadata = Column(
        JSONB,
        nullable=False,
        default=dict,
        doc="Additional metadata: environment, version, tags, etc."
    )

    # Relationships
    rotation = relationship(
        "CredentialRotation",
        foreign_keys=[rotation_id],
        back_populates="api_keys"
    )

    # Table indexes for performance
    __table_args__ = (
        Index('ix_api_keys_status_expires', 'status', 'expires_at'),
        Index('ix_api_keys_created_by_status', 'created_by', 'status'),
    )

    def is_valid(self) -> bool:
        """
        Check if API key is currently valid for use.

        Returns:
            True if key can be used for authentication
        """
        # Must be active or rotating (during transition period)
        if self.status not in [APIKeyStatus.ACTIVE.value, APIKeyStatus.ROTATING.value]:
            return False

        # Must not be soft-deleted
        if self.is_deleted:
            return False

        # Check expiration
        if self.expires_at and self.expires_at < datetime.utcnow():
            return False

        return True

    def mark_used(self, ip_address: Optional[str] = None) -> None:
        """
        Update last-used tracking when key is successfully used.

        Args:
            ip_address: IP address making the request
        """
        self.last_used_at = datetime.utcnow()
        self.usage_count += 1
        if ip_address:
            self.last_used_ip = ip_address

    def revoke(self, reason: str) -> None:
        """
        Immediately revoke this API key.

        Args:
            reason: Reason for revocation (for audit trail)
        """
        self.status = APIKeyStatus.REVOKED.value
        self.revoked_at = datetime.utcnow()
        self.revoked_reason = reason

    def mark_expired(self) -> None:
        """Mark key as expired (called by automated cleanup job)."""
        self.status = APIKeyStatus.EXPIRED.value

    def start_rotation(self, rotation_id: uuid.UUID) -> None:
        """
        Mark key as entering rotation phase.

        Args:
            rotation_id: ID of the CredentialRotation record
        """
        self.status = APIKeyStatus.ROTATING.value
        self.rotation_id = rotation_id

    def to_safe_dict(self) -> Dict[str, Any]:
        """
        Convert to dictionary with sensitive fields excluded.

        Returns:
            Dictionary safe for API responses (no key_hash)
        """
        return self.to_dict(exclude={'key_hash', 'deleted_at'})

    def __repr__(self) -> str:
        return f"<APIKey(prefix={self.key_prefix}, status={self.status})>"
